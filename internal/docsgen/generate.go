package docsgen

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type Options struct {
	SourceDir  string
	OutDir     string
	FilterPath string
}

func Generate(options Options) error {
	if strings.TrimSpace(options.SourceDir) == "" {
		return fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(options.OutDir) == "" {
		return fmt.Errorf("output directory is required")
	}
	if strings.TrimSpace(options.FilterPath) == "" {
		return fmt.Errorf("pandoc filter path is required")
	}

	source, err := assembleMarkdown(options.SourceDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(options.OutDir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "localci-docs-*.md")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(source); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := runPandoc(tmpPath, options.FilterPath, "man", filepath.Join(options.OutDir, "localci.1")); err != nil {
		return err
	}
	if err := runPandoc(tmpPath, options.FilterPath, "plain", filepath.Join(options.OutDir, "localci.txt")); err != nil {
		return err
	}
	return nil
}

func runPandoc(inputPath string, filterPath string, format string, outPath string) error {
	args := []string{
		"--from", "markdown+fenced_code_attributes",
		"--to", format,
		"--standalone",
		"--lua-filter", filterPath,
		"--output", outPath,
		inputPath,
	}
	if format == "plain" {
		args = append([]string{"--columns", "100"}, args...)
	}

	cmd := exec.Command("pandoc", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return fmt.Errorf("pandoc %s: %w", format, err)
		}
		return fmt.Errorf("pandoc %s: %w: %s", format, err, detail)
	}
	return nil
}

func assembleMarkdown(sourceDir string) ([]byte, error) {
	files := []string{
		"index.md",
		"getting-started.md",
		"tasks.md",
	}
	var out bytes.Buffer
	out.WriteString("% LOCALCI(1) LocalCI Manual\n")
	out.WriteString("% LocalCI\n")
	out.WriteString("% May 2026\n\n")
	out.WriteString("# NAME\n\n")
	out.WriteString("localci - local post-commit validation runner\n\n")
	out.WriteString("# COMMAND HELP\n\n")
	out.WriteString("For command options, run `localci --help` or `localci <command> --help`.\n\n")
	for _, name := range files {
		path := filepath.Join(sourceDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out.Write(preprocess(content))
		out.WriteString("\n\n")
	}
	return out.Bytes(), nil
}

var codeFenceWithTitle = regexp.MustCompile("^```([^`\\s]*)\\s+title=\"([^\"]+)\"\\s*$")

func preprocess(markdown []byte) []byte {
	var out bytes.Buffer
	lines := strings.Split(string(markdown), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<div ") || trimmed == "</div>" {
			continue
		}
		line = strings.ReplaceAll(line, ":lucide-check:{ .lg .middle }", "✓")
		line = strings.ReplaceAll(line, ":lucide-x:{ .lg .middle }", "✕")
		if match := codeFenceWithTitle.FindStringSubmatch(line); match != nil {
			fmt.Fprintf(&out, "File: `%s`\n\n", match[2])
			if match[1] == "" {
				out.WriteString("```\n")
			} else {
				fmt.Fprintf(&out, "```%s\n", match[1])
			}
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}
