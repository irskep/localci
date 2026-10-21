package localci

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type taskLogWriters struct {
	combinedFile *os.File
	writer       io.Writer
	publisher    *appendPublishingWriter
}

func newTaskLogWriters(outputDir string, stdout io.Writer, publishAppend func(offset int64, text string)) (*taskLogWriters, error) {
	combinedFile, err := os.Create(filepath.Join(outputDir, combinedLogName))
	if err != nil {
		return nil, fmt.Errorf("create combined log: %w", err)
	}

	combinedWriter := io.Writer(combinedFile)
	var publisher *appendPublishingWriter
	if publishAppend != nil {
		publisher = &appendPublishingWriter{
			writer:        combinedFile,
			publishAppend: publishAppend,
		}
		combinedWriter = publisher
	}

	return &taskLogWriters{
		combinedFile: combinedFile,
		publisher:    publisher,
		writer:       io.MultiWriter(combinedWriter, stdout),
	}, nil
}

func (w *taskLogWriters) Close() {
	if w.publisher != nil {
		w.publisher.flush(true)
	}
	_ = w.combinedFile.Close()
}

type appendPublishingWriter struct {
	writer        io.Writer
	offset        int64
	pending       []byte
	publishAppend func(offset int64, text string)
}

func (w *appendPublishingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.pending = append(w.pending, p[:n]...)
		w.flush(false)
	}
	return n, err
}

// Preserve UTF-8 characters split across subprocess writes. Offsets still refer
// to original file bytes, including invalid bytes replaced by JSON encoding.
func (w *appendPublishingWriter) flush(final bool) {
	n := len(w.pending)
	if !final {
		n = completeUTF8Prefix(w.pending)
	}
	if n == 0 {
		return
	}
	w.publishAppend(w.offset, string(w.pending[:n]))
	w.offset += int64(n)
	w.pending = append(w.pending[:0], w.pending[n:]...)
}

func completeUTF8Prefix(data []byte) int {
	start := len(data) - 1
	for start >= 0 && len(data)-start < utf8.UTFMax && !utf8.RuneStart(data[start]) {
		start--
	}
	if start >= 0 && !utf8.FullRune(data[start:]) {
		return start
	}
	return len(data)
}
