package localci

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type taskLogWriters struct {
	combinedFile *os.File
	writer       io.Writer
}

func newTaskLogWriters(outputDir string, stdout io.Writer, publishAppend func(offset int64, text string)) (*taskLogWriters, error) {
	combinedFile, err := os.Create(filepath.Join(outputDir, combinedLogName))
	if err != nil {
		return nil, fmt.Errorf("create combined log: %w", err)
	}

	combinedWriter := io.Writer(combinedFile)
	if publishAppend != nil {
		combinedWriter = &appendPublishingWriter{
			writer:        combinedFile,
			publishAppend: publishAppend,
		}
	}

	return &taskLogWriters{
		combinedFile: combinedFile,
		writer:       io.MultiWriter(combinedWriter, stdout),
	}, nil
}

func (w *taskLogWriters) Close() {
	_ = w.combinedFile.Close()
}

type appendPublishingWriter struct {
	writer        io.Writer
	offset        int64
	publishAppend func(offset int64, text string)
}

func (w *appendPublishingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.publishAppend(w.offset, string(p[:n]))
		w.offset += int64(n)
	}
	return n, err
}
