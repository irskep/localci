package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"

	"localci/internal/docs"
)

type docsFlags struct {
	Plain bool
	Roff  bool
}

func (a App) runDocs(flags docsFlags) error {
	if flags.Roff {
		_, err := fmt.Fprint(a.Stdout, docs.ManPage())
		return err
	}
	if flags.Plain || !a.stdoutIsTerminal() {
		_, err := fmt.Fprint(a.Stdout, docs.PlainText())
		return err
	}

	path, cleanup, err := writeTempManPage(docs.ManPage())
	if err != nil {
		return err
	}
	defer cleanup()

	if err := a.renderManPage(path); err != nil {
		_, printErr := fmt.Fprint(a.Stdout, docs.PlainText())
		if printErr != nil {
			return printErr
		}
	}
	return nil
}

func (a App) stdoutIsTerminal() bool {
	if a.StdoutIsTerminal != nil {
		return a.StdoutIsTerminal()
	}
	file, ok := a.Stdout.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
}

func writeTempManPage(content string) (string, func(), error) {
	file, err := os.CreateTemp("", "localci-docs-*.1")
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(path)
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { os.Remove(path) }, nil
}

func (a App) renderManPage(path string) error {
	if a.RenderManPage != nil {
		return a.RenderManPage(path)
	}
	cmd := exec.Command("man", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = a.Stdout
	cmd.Stderr = a.Stderr
	return cmd.Run()
}
