package main

import (
	"embed"
	"io/fs"

	"github.com/egoist/mygo/i18n"
)

//go:embed messages/*.json
var uiMessages embed.FS

func init() {
	messages, err := fs.Sub(uiMessages, "messages")
	if err != nil {
		panic(err)
	}
	catalog, err := i18n.NewCatalog(messages, "zh-Hans")
	if err != nil {
		panic(err)
	}
	i18n.Register(catalog)
}
