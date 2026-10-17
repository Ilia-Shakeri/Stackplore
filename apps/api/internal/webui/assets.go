package webui

import (
	"embed"
	"io/fs"
)

//go:embed static
var embedded embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(embedded, "static")
	if err != nil {
		panic("embedded web assets missing")
	}
	return assets
}
