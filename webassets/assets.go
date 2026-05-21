package webassets

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var embedded embed.FS

func EmbeddedFS() (fs.FS, error) {
	return fs.Sub(embedded, "assets")
}
