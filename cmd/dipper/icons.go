package main

import "github.com/egoist/mygo/ui"

// Reicon outline icons (MIT, https://reicon.dev/docs), normalized to 24x24
// with 1.5 DIP round strokes and rendered at 18x18 inside a 24x24 box.
const reiconSVGPrefix = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">`

var iconPaths = map[string]string{
	// Reicon Grid22
	"browse": `<path d="M9 22H15C20 22 22 20 22 15V9C22 4 20 2 15 2H9C4 2 2 4 2 9V15C2 20 4 22 9 22Z"/><path d="M12 2V22"/><path d="M2 12H22"/>`,
	// Reicon Bookmark22
	"following": `<path d="M14 2C16 2 17 3.01 17 5.03V12.08C17 14.07 15.59 14.84 13.86 13.8L12.54 13C12.24 12.82 11.76 12.82 11.46 13L10.14 13.8C8.41 14.84 7 14.07 7 12.08V5.03C7 3.01 8 2 10 2H14Z"/><path d="M6.82 4.99C3.41 5.56 2 7.66 2 11.9V14.93C2 19.98 4 22 9 22H15C20 22 22 19.98 22 14.93V11.9C22 7.59 20.54 5.48 17 4.96"/>`,
	// Reicon Import22
	"downloads": `<path d="M9.32 11.68L11.88 14.24L14.44 11.68"/><path d="M11.88 4V14.17"/><path d="M20 12.18C20 16.6 17 20.18 12 20.18C7 20.18 4 16.6 4 12.18"/>`,
	// Reicon Setting42
	"settings": `<path d="M22 6.5H16"/><path d="M6 6.5H2"/><path d="M10 10C11.933 10 13.5 8.433 13.5 6.5C13.5 4.567 11.933 3 10 3C8.067 3 6.5 4.567 6.5 6.5C6.5 8.433 8.067 10 10 10Z"/><path d="M22 17.5H18"/><path d="M8 17.5H2"/><path d="M14 21C15.933 21 17.5 19.433 17.5 17.5C17.5 15.567 15.933 14 14 14C12.067 14 10.5 15.567 10.5 17.5C10.5 19.433 12.067 21 14 21Z"/>`,
	// Reicon Folder22
	"folder": `<path d="M22 11V17C22 21 21 22 17 22H7C3 22 2 21 2 17V7C2 3 3 2 7 2H8.5C10 2 10.33 2.44 10.9 3.2L12.4 5.2C12.78 5.7 13 6 14 6H17C21 6 22 7 22 11Z"/><path d="M8 2H17C19 2 20 3 20 5V6.38"/>`,
	// Reicon ArrowLeft12
	"back": `<path d="M15 19.92L8.48 13.4C7.71 12.63 7.71 11.37 8.48 10.6L15 4.08"/>`,
	// Reicon ArrowRight12
	"next": `<path d="M8.91 19.92L15.43 13.4C16.2 12.63 16.2 11.37 15.43 10.6L8.91 4.08"/>`,
	// Reicon MessageQuestion2
	"help": `<path d="M17 18.43H13L8.55 21.39C7.89 21.83 7 21.36 7 20.56V18.43C4 18.43 2 16.43 2 13.43V7.43C2 4.43 4 2.43 7 2.43H17C20 2.43 22 4.43 22 7.43V13.43C22 16.43 20 18.43 17 18.43Z"/><path d="M12 11.36V11.15C12 10.47 12.42 10.11 12.84 9.82C13.25 9.54 13.66 9.18 13.66 8.52C13.66 7.6 12.92 6.86 12 6.86C11.08 6.86 10.34 7.6 10.34 8.52"/><path d="M11.995 13.75H12.005"/>`,
	// Reicon More2
	"more": `<path d="M5 10C3.9 10 3 10.9 3 12C3 13.1 3.9 14 5 14C6.1 14 7 13.1 7 12C7 10.9 6.1 10 5 10Z"/><path d="M19 10C17.9 10 17 10.9 17 12C17 13.1 17.9 14 19 14C20.1 14 21 13.1 21 12C21 10.9 20.1 10 19 10Z"/><path d="M12 10C10.9 10 10 10.9 10 12C10 13.1 10.9 14 12 14C13.1 14 14 13.1 14 12C14 10.9 13.1 10 12 10Z"/>`,
	// Reicon TickCircle2 checkmark
	"check": `<path d="M5.5 12.5L9.8 16.8L18.5 7.5"/>`,
	// Reicon SearchNormal12
	"search": `<path d="M11.5 21C16.7467 21 21 16.7467 21 11.5C21 6.25329 16.7467 2 11.5 2C6.25329 2 2 6.25329 2 11.5C2 16.7467 6.25329 21 11.5 21Z"/><path d="M22 22L20 20"/>`,
	// Reicon Xmark (scaled to 24x24 with 1.5 stroke)
	"close": `<path d="M18.67 5.33L5.33 18.67"/><path d="M5.33 5.33L18.67 18.67"/>`,
	// Reicon Minus3
	"minimize": `<path d="M6 12H18"/>`,
	// Reicon rounded square contour (window maximize)
	"maximize": `<path d="M8.5 20H15.5C18.5 20 20 18.5 20 15.5V8.5C20 5.5 18.5 4 15.5 4H8.5C5.5 4 4 5.5 4 8.5V15.5C4 18.5 5.5 20 8.5 20Z"/>`,
	// Reicon Copy3 (window restore)
	"restore": `<path d="M16 12.9V17.1C16 20.6 14.6 22 11.1 22H6.9C3.4 22 2 20.6 2 17.1V12.9C2 9.4 3.4 8 6.9 8H11.1C14.6 8 16 9.4 16 12.9Z"/><path d="M22 6.9V11.1C22 14.6 20.6 16 17.1 16H16V12.9C16 9.4 14.6 8 11.1 8H8V6.9C8 3.4 9.4 2 12.9 2H17.1C20.6 2 22 3.4 22 6.9Z"/>`,
	// Reicon Pause3
	"pause": `<path d="M10.65 19.11V4.89C10.65 3.54 10.08 3 8.64 3H5.01C3.57 3 3 3.54 3 4.89V19.11C3 20.46 3.57 21 5.01 21H8.64C10.08 21 10.65 20.46 10.65 19.11Z"/><path d="M21 19.11V4.89C21 3.54 20.43 3 18.99 3H15.36C13.93 3 13.35 3.54 13.35 4.89V19.11C13.35 20.46 13.92 21 15.36 21H18.99C20.43 21 21 20.46 21 19.11Z"/>`,
	// Reicon Play3
	"play": `<path d="M4 12V8.44C4 4.02 7.13 2.21 10.96 4.42L14.05 6.2L17.14 7.98C20.97 10.19 20.97 13.81 17.14 16.02L14.05 17.8L10.96 19.58C7.13 21.79 4 19.98 4 15.56V12Z"/>`,
	// Reicon Clock12
	"clock": `<path d="M22 12C22 17.52 17.52 22 12 22C6.48 22 2 17.52 2 12C2 6.48 6.48 2 12 2C17.52 2 22 6.48 22 12Z"/><path d="M15.71 15.18L12.61 13.33C12.07 13.01 11.63 12.24 11.63 11.61V7.51"/>`,
	// Reicon Refresh3
	"refresh": `<path d="M14.55 21.67C18.84 20.54 22 16.64 22 12C22 6.48 17.56 2 12 2C5.33 2 2 7.56 2 7.56M2 7.56V3M2 7.56H4.01H6.44"/><path d="M2 12C2 17.52 6.48 22 12 22"/>`,
	// Reicon Import22
	"download": `<path d="M9.32 11.68L11.88 14.24L14.44 11.68"/><path d="M11.88 4V14.17"/><path d="M20 12.18C20 16.6 17 20.18 12 20.18C7 20.18 4 16.6 4 12.18"/>`,
	// Reicon Export12
	"upload": `<path d="M9.32 6.5L11.88 3.94L14.44 6.5"/><path d="M11.88 14.18V4.01"/><path d="M4 12C4 16.42 7 20 12 20C17 20 20 16.42 20 12"/>`,
}

var icons = map[string]*ui.SVG{}

func loadIcons() {
	for name, path := range iconPaths {
		icons[name] = ui.MustParseSVG([]byte(reiconSVGPrefix + path + `</svg>`))
	}
}

const (
	iconGlyphSize float32 = 18
	iconBoxSize   float32 = 24
)

// icon draws an 18x18 Reicon outline glyph centered in a transparent 24x24
// layout box so icons align consistently across rows and controls.
func icon(c *ui.Context, name string) {
	if svg := icons[name]; svg != nil {
		ui.Box(c).Size(iconBoxSize, iconBoxSize).Shrink(0).Center().Children(func() {
			ui.Icon(c, svg).Size(iconGlyphSize, iconGlyphSize)
		})
	}
}
