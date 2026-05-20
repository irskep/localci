package main

import (
	"os"

	"localci/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
