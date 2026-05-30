package docsgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateChangelog(inPath string, outPath string) error {
	source, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}

	var out bytes.Buffer
	for _, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "<!--") {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, bytes.TrimRight(out.Bytes(), "\n"), 0o644); err != nil {
		return fmt.Errorf("write generated changelog: %w", err)
	}
	return nil
}
