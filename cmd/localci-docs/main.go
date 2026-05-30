package main

import (
	"flag"
	"fmt"
	"os"

	"localci/internal/cli"
	"localci/internal/docsgen"
)

func main() {
	cliOutDir := flag.String("cli-out", "docs/src/cli", "directory for generated CLI markdown")
	bundleOutDir := flag.String("bundle-out", "internal/docs/generated", "directory for bundled terminal docs")
	sourceDir := flag.String("source", "docs/src", "directory containing narrative documentation")
	filterPath := flag.String("filter", "docs/pandoc/zensical-admonitions.lua", "Pandoc Lua filter for Zensical extensions")
	changelogIn := flag.String("changelog-in", "CHANGELOG.md", "source changelog")
	changelogOut := flag.String("changelog-out", "docs/src/changelog.md", "generated changelog markdown page")
	flag.Parse()
	if err := cli.GenerateMarkdownDocs(*cliOutDir); err != nil {
		fmt.Fprintf(os.Stderr, "localci-docs: %v\n", err)
		os.Exit(1)
	}
	if err := docsgen.GenerateChangelog(*changelogIn, *changelogOut); err != nil {
		fmt.Fprintf(os.Stderr, "localci-docs: %v\n", err)
		os.Exit(1)
	}
	if err := docsgen.Generate(docsgen.Options{
		SourceDir:  *sourceDir,
		OutDir:     *bundleOutDir,
		FilterPath: *filterPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "localci-docs: %v\n", err)
		os.Exit(1)
	}
}
