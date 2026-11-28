package main

import (
	"embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/fonts/Geist-Regular.ttf assets/fonts/Geist-Medium.ttf assets/fonts/Geist-SemiBold.ttf assets/fonts/Geist-Bold.ttf
var geistFonts embed.FS

const uiFontFamily = "Geist, Microsoft YaHei UI, PingFang SC, Noto Sans CJK SC"

var geistFontFiles = []string{
	"assets/fonts/Geist-Regular.ttf",
	"assets/fonts/Geist-Medium.ttf",
	"assets/fonts/Geist-SemiBold.ttf",
	"assets/fonts/Geist-Bold.ttf",
}

var loadFontsOnce sync.Once

// loadFonts registers the embedded Geist static weights (400, 500, 600, 700)
// with MyGo once so every text role uses Geist with native CJK fallback.
func loadFonts() {
	loadFontsOnce.Do(func() {
		for _, path := range geistFontFiles {
			data, err := geistFonts.ReadFile(path)
			if err != nil {
				panic("read embedded font " + path + ": " + err.Error())
			}
			if err := ui.RegisterFont(data, "Geist"); err != nil {
				panic("register embedded font " + path + ": " + err.Error())
			}
		}
	})
}
