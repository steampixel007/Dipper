// Icon generator. Everything Dipper shows is drawn from one bird: a chubby
// dipper crouched over rushing water, white bib flush against its front edge.
// The same geometry lives in the SVG masters beside this file; change the bird
// once, in both places, and regenerate. No dependencies, CGO-free.
//
//	go run build/icon/gen.go app 256 build/icon/dist/app-256.png
//	go run build/icon/gen.go tray 20 build/icon/dist/tray-20.png
//	go run build/icon/gen.go tray -dark 20 build/icon/dist/tray-dark-20.png
//	go run build/icon/gen.go ico build/icon/dist/dipper.ico
//	go run build/icon/gen.go all build/icon/dist
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type pt struct{ x, y float64 }

// affine maps master-grid coordinates: screen = (a*x + c*y + e, b*x + d*y + f).
type affine struct{ a, b, c, d, e, f float64 }

func xfID() affine                      { return affine{1, 0, 0, 1, 0, 0} }
func xfScale(s float64) affine          { return affine{s, 0, 0, s, 0, 0} }
func xfTranslate(tx, ty float64) affine { return affine{1, 0, 0, 1, tx, ty} }

// mul returns n∘m: apply m first, then n.
func (m affine) mul(n affine) affine {
	return affine{
		n.a*m.a + n.c*m.b,
		n.b*m.a + n.d*m.b,
		n.a*m.c + n.c*m.d,
		n.b*m.c + n.d*m.d,
		n.a*m.e + n.c*m.f + n.e,
		n.b*m.e + n.d*m.f + n.f,
	}
}

func (m affine) apply(p pt) pt {
	return pt{m.a*p.x + m.c*p.y + m.e, m.b*p.x + m.d*p.y + m.f}
}

func (m affine) inv() affine {
	det := m.a*m.d - m.b*m.c
	ia, ib := m.d/det, -m.b/det
	ic, id := -m.c/det, m.a/det
	return affine{ia, ib, ic, id, -(ia*m.e + ic*m.f), -(ib*m.e + id*m.f)}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// Shape coverage tests. Anti-aliasing comes from supersampling, so each test
// is a hard inside/outside.

func ellipse(cx, cy, rx, ry float64) func(pt) bool {
	return func(p pt) bool {
		dx, dy := (p.x-cx)/rx, (p.y-cy)/ry
		return dx*dx+dy*dy <= 1
	}
}

func poly(pts []pt) func(pt) bool {
	return func(p pt) bool {
		in := false
		for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
			if (pts[i].y > p.y) != (pts[j].y > p.y) &&
				p.x < (pts[j].x-pts[i].x)*(p.y-pts[i].y)/(pts[j].y-pts[i].y)+pts[i].x {
				in = !in
			}
		}
		return in
	}
}

func capsule(a, b pt, r float64) func(pt) bool {
	return func(p pt) bool {
		ax, ay := p.x-a.x, p.y-a.y
		bx, by := b.x-a.x, b.y-a.y
		t := clamp((ax*bx+ay*by)/(bx*bx+by*by), 0, 1)
		dx, dy := ax-bx*t, ay-by*t
		return dx*dx+dy*dy <= r*r
	}
}

func roundRect(x, y, w, h, r float64) func(pt) bool {
	return func(p pt) bool {
		if p.x < x || p.x > x+w || p.y < y || p.y > y+h {
			return false
		}
		cx := clamp(p.x, x+r, x+w-r)
		cy := clamp(p.y, y+r, y+h-r)
		dx, dy := p.x-cx, p.y-cy
		return dx*dx+dy*dy <= r*r
	}
}

// Curve flattening, endpoints excluded so segments chain without duplicates.

func quad(p0, p1, p2 pt, n int) []pt {
	var out []pt
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		out = append(out, pt{
			u*u*p0.x + 2*u*t*p1.x + t*t*p2.x,
			u*u*p0.y + 2*u*t*p1.y + t*t*p2.y,
		})
	}
	return out
}

func cubic(p0, p1, p2, p3 pt, n int) []pt {
	var out []pt
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		out = append(out, pt{
			u*u*u*p0.x + 3*u*u*t*p1.x + 3*u*t*t*p2.x + t*t*t*p3.x,
			u*u*u*p0.y + 3*u*u*t*p1.y + 3*u*t*t*p2.y + t*t*t*p3.y,
		})
	}
	return out
}

// arcPts samples the SVG-style arc a→b (small arc, positive sweep).
func arcPts(a, b pt, r float64, n int) []pt {
	mx, my := (a.x+b.x)/2, (a.y+b.y)/2
	dx, dy := b.x-a.x, b.y-a.y
	half := math.Hypot(dx, dy) / 2
	h := math.Sqrt(math.Max(0, r*r-half*half))
	ux, uy := -dy/(2*half), dx/(2*half)
	best := pt{mx + ux*h, my + uy*h}
	bestSweep := math.MaxFloat64
	for _, s := range []float64{1, -1} {
		c := pt{mx + ux*h*s, my + uy*h*s}
		sweep := math.Mod(math.Atan2(b.y-c.y, b.x-c.x)-math.Atan2(a.y-c.y, a.x-c.x), 2*math.Pi)
		if sweep < 0 {
			sweep += 2 * math.Pi
		}
		if sweep < bestSweep {
			bestSweep, best = sweep, c
		}
	}
	var out []pt
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		ang := math.Atan2(a.y-best.y, a.x-best.x) + bestSweep*t
		out = append(out, pt{best.x + r*math.Cos(ang), best.y + r*math.Sin(ang)})
	}
	return out
}

// The bird, on the 256 master grid. Coordinates mirror the SVG masters.

func bibPts() []pt {
	var p []pt
	p = append(p, pt{84, 118})
	p = append(p, quad(pt{84, 118}, pt{110, 130}, pt{138, 114}, 12)...)
	p = append(p, cubic(pt{138, 114}, pt{150, 120}, pt{156, 136}, pt{154, 152}, 12)...)
	p = append(p, cubic(pt{154, 152}, pt{152, 168}, pt{144, 180}, pt{134, 186}, 12)...)
	p = append(p, cubic(pt{134, 186}, pt{123, 191}, pt{108, 190}, pt{97, 187}, 12)...)
	p = append(p, cubic(pt{97, 187}, pt{88, 184}, pt{84, 181}, pt{82, 174}, 12)...)
	p = append(p, cubic(pt{82, 174}, pt{79, 166}, pt{80, 150}, pt{82, 138}, 12)...)
	p = append(p, cubic(pt{82, 138}, pt{84, 128}, pt{84, 122}, pt{84, 118}, 12)...)
	return p
}

func tailPts() []pt {
	p := []pt{{167, 149}, {211, 111}}
	p = append(p, arcPts(pt{211, 111}, pt{223, 123}, 9, 8)...)
	p = append(p, pt{186, 166})
	p = append(p, cubic(pt{186, 166}, pt{180, 163}, pt{172, 157}, pt{167, 149}, 10)...)
	return p
}

func billPts(small bool) []pt {
	if small {
		p := []pt{{76, 100}, {40, 112}, {80, 124}}
		return append(p, cubic(pt{80, 124}, pt{78.5, 116}, pt{77, 108}, pt{76, 100}, 6)...)
	}
	p := []pt{{74, 104}, {44, 114}, {78, 122}}
	return append(p, cubic(pt{78, 122}, pt{76.5, 116}, pt{75, 110}, pt{74, 104}, 6)...)
}

var waterStrokes = [][2]pt{
	{{56, 231}, {96, 227}},
	{{112, 230}, {142, 227}},
	{{154, 229}, {172, 227}},
	{{78, 247}, {104, 244}},
	{{118, 246}, {144, 244}},
}

var (
	colInk   = [4]float64{0, 0, 0, 1}
	colPaper = [4]float64{1, 1, 1, 1}
	colBlue  = hexColor(0x3b, 0x63, 0xfb)
	colWash  = hexColor(0xf5, 0xf9, 0xff)
)

func hexColor(r, g, b uint8) [4]float64 {
	return [4]float64{float64(r) / 255, float64(g) / 255, float64(b) / 255, 1}
}

type op struct {
	f     func(pt) bool
	col   [4]float64
	erase bool
	xf    affine
}

type birdVariant struct {
	pupil     bool
	billSmall bool
}

func birdOps(x affine, v birdVariant, dark bool) []op {
	body := colInk
	detail := colPaper
	if dark {
		body, detail = colPaper, [4]float64{0, 0, 0, 0}
	}
	ops := []op{
		{ellipse(106, 98, 40, 40), body, false, x},
		{ellipse(130, 160, 52, 50), body, false, x},
		{poly(billPts(v.billSmall)), body, false, x},
		{poly(tailPts()), body, false, x},
	}
	if dark {
		ops = append(ops,
			op{poly(bibPts()), [4]float64{}, true, x},
			op{ellipse(104, 94, 7.5, 7.5), [4]float64{}, true, x})
	} else {
		ops = append(ops,
			op{poly(bibPts()), detail, false, x},
			op{ellipse(104, 94, 7.5, 7.5), detail, false, x})
		if v.pupil {
			r := 3.0
			if v.billSmall {
				r = 0 // the small cut drops the pupil
			}
			if r > 0 {
				ops = append(ops, op{ellipse(106, 95, r, r), colInk, false, x})
			}
		}
	}
	return ops
}

func waterOps(x affine, ink bool) []op {
	c := colBlue
	if ink {
		c = colInk
	}
	var ops []op
	for _, s := range waterStrokes {
		ops = append(ops, op{capsule(s[0], s[1], 4.5), c, false, x})
	}
	return ops
}

func appOps() []op {
	ops := []op{{roundRect(0, 0, 256, 256, 58), colWash, false, xfID()}}
	x := xfTranslate(34, 10).mul(xfScale(0.735))
	ops = append(ops, birdOps(x, birdVariant{pupil: true}, false)...)
	return append(ops, waterOps(x, false)...)
}

func symbolOps() []op {
	x := xfTranslate(-5, -26)
	ops := birdOps(x, birdVariant{pupil: true}, false)
	return append(ops, waterOps(x, false)...)
}

func trayOps(dark bool) []op {
	x := xfTranslate(128, 128).mul(xfScale(1.02)).mul(xfTranslate(-131.5, -134))
	return birdOps(x, birdVariant{billSmall: true}, dark)
}

// render rasterizes the ops into a size×size image with 4× supersampling
// (capped so the buffer stays under 2048²).
func render(size int, ops []op) *image.NRGBA {
	ss := 4
	if size*ss > 2048 {
		ss = 2048 / size
		if ss < 1 {
			ss = 1
		}
	}
	w := size * ss
	buf := make([][4]float64, w*w)
	inv := make([]affine, len(ops))
	for i, o := range ops {
		inv[i] = o.xf.inv()
	}
	for py := 0; py < w; py++ {
		y := (float64(py) + 0.5) * 256 / float64(w)
		for px := 0; px < w; px++ {
			x := (float64(px) + 0.5) * 256 / float64(w)
			i := py*w + px
			for k, o := range ops {
				if !o.f(inv[k].apply(pt{x, y})) {
					continue
				}
				if o.erase {
					buf[i][3] = 0
					continue
				}
				aD := buf[i][3]
				aS := o.col[3]
				aO := aS + aD*(1-aS)
				if aO > 0 {
					for c := 0; c < 3; c++ {
						buf[i][c] = (o.col[c]*aS + buf[i][c]*aD*(1-aS)) / aO
					}
				}
				buf[i][3] = aO
			}
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	n := float64(ss * ss)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var pr, pg, pb, pa float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					c := buf[(y*ss+sy)*w+x*ss+sx]
					pr += c[0] * c[3]
					pg += c[1] * c[3]
					pb += c[2] * c[3]
					pa += c[3]
				}
			}
			if pa > 0 {
				pr, pg, pb = pr/pa, pg/pa, pb/pa
			}
			out.SetNRGBA(x, y, color.NRGBA{
				uint8(pr*255 + 0.5), uint8(pg*255 + 0.5), uint8(pb*255 + 0.5),
				uint8(pa / n * 255.5),
			})
		}
	}
	return out
}

func savePNG(path string, size int, ops []op) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b bytes.Buffer
	if err := png.Encode(&b, render(size, ops)); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// saveICO writes a Vista-style ICO: one PNG-embedded image per size.
func saveICO(path string, sizes []int, ops func() []op) error {
	var pngs [][]byte
	for _, s := range sizes {
		var b bytes.Buffer
		if err := png.Encode(&b, render(s, ops())); err != nil {
			return err
		}
		pngs = append(pngs, b.Bytes())
	}
	var buf bytes.Buffer
	for _, v := range []uint16{0, 2, uint16(len(pngs))} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	offset := uint32(6 + 16*len(pngs))
	for i, s := range sizes {
		w, h := uint8(s), uint8(s)
		if s >= 256 {
			w, h = 0, 0
		}
		for _, v := range []any{w, h, uint8(0), uint8(0), uint16(1), uint16(32),
			uint32(len(pngs[i])), offset} {
			if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
				return err
			}
		}
		offset += uint32(len(pngs[i]))
	}
	for _, p := range pngs {
		buf.Write(p)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen:", err)
	os.Exit(1)
}

func atoi(s string) int {
	var n int
	if _, err := fmt.Sscan(s, &n); err != nil || n <= 0 {
		fatal(fmt.Errorf("bad size %q", s))
	}
	return n
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gen.go app <size> <out.png> | gen.go symbol <size> <out.png> |"+
		" gen.go tray [-dark] <size> <out.png> | gen.go ico <out.ico> | gen.go all <dir>")
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "app":
		if len(os.Args) != 4 {
			usage()
		}
		if err := savePNG(os.Args[3], atoi(os.Args[2]), appOps()); err != nil {
			fatal(err)
		}
	case "symbol":
		if len(os.Args) != 4 {
			usage()
		}
		if err := savePNG(os.Args[3], atoi(os.Args[2]), symbolOps()); err != nil {
			fatal(err)
		}
	case "tray":
		args := os.Args[2:]
		dark := false
		if len(args) > 0 && args[0] == "-dark" {
			dark = true
			args = args[1:]
		}
		if len(args) != 2 {
			usage()
		}
		if err := savePNG(args[1], atoi(args[0]), trayOps(dark)); err != nil {
			fatal(err)
		}
	case "ico":
		if len(os.Args) != 3 {
			usage()
		}
		if err := saveICO(os.Args[2], []int{16, 24, 32, 48, 64, 128, 256}, appOps); err != nil {
			fatal(err)
		}
	case "all":
		if len(os.Args) != 3 {
			usage()
		}
		dir := os.Args[2]
		jobs := []struct {
			path string
			run  func(string) error
		}{
			{"symbol-512.png", func(p string) error { return savePNG(p, 512, symbolOps()) }},
			{"app-512.png", func(p string) error { return savePNG(p, 512, appOps()) }},
			{"app-256.png", func(p string) error { return savePNG(p, 256, appOps()) }},
			{"dipper.ico", func(p string) error {
				return saveICO(p, []int{16, 24, 32, 48, 64, 128, 256}, appOps)
			}},
		}
		for _, size := range []int{16, 20, 24, 32} {
			s := size
			jobs = append(jobs,
				struct {
					path string
					run  func(string) error
				}{fmt.Sprintf("tray-%d.png", s), func(p string) error { return savePNG(p, s, trayOps(false)) }},
				struct {
					path string
					run  func(string) error
				}{fmt.Sprintf("tray-dark-%d.png", s), func(p string) error { return savePNG(p, s, trayOps(true)) }},
			)
		}
		for _, j := range jobs {
			if err := j.run(filepath.Join(dir, j.path)); err != nil {
				fatal(err)
			}
			fmt.Println("wrote", filepath.Join(dir, j.path))
		}
	default:
		usage()
	}
}
