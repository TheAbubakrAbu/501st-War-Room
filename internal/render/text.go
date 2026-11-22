package render

import (
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The Go fonts ship with x/image under a BSD license.
var (
	parseOnce                        sync.Once
	fontBold, fontMono, fontMonoBold *opentype.Font
	parseErr                         error
)

func parseFonts() error {
	parseOnce.Do(func() {
		for _, f := range []struct {
			dst **opentype.Font
			ttf []byte
		}{{&fontBold, gobold.TTF}, {&fontMono, gomono.TTF}, {&fontMonoBold, gomonobold.TTF}} {
			if *f.dst, parseErr = opentype.Parse(f.ttf); parseErr != nil {
				return
			}
		}
	})
	return parseErr
}

// faces holds the sizes one picture uses. A face is not safe for concurrent
// use, so every Map call makes its own.
type faces struct {
	bold, mono, monoBold map[float64]font.Face
}

func newFaces() (*faces, error) {
	if err := parseFonts(); err != nil {
		return nil, err
	}
	return &faces{bold: map[float64]font.Face{}, mono: map[float64]font.Face{}, monoBold: map[float64]font.Face{}}, nil
}

func (fs *faces) get(cache map[float64]font.Face, f *opentype.Font, size float64) font.Face {
	if face, ok := cache[size]; ok {
		return face
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic("render: font face: " + err.Error()) // the fonts are built in; this cannot fail at run time
	}
	cache[size] = face
	return face
}

func (fs *faces) Bold(size float64) font.Face     { return fs.get(fs.bold, fontBold, size) }
func (fs *faces) Mono(size float64) font.Face     { return fs.get(fs.mono, fontMono, size) }
func (fs *faces) MonoBold(size float64) font.Face { return fs.get(fs.monoBold, fontMonoBold, size) }

func (fs *faces) close() {
	for _, m := range []map[float64]font.Face{fs.bold, fs.mono, fs.monoBold} {
		for _, f := range m {
			f.Close()
		}
	}
}

func width(face font.Face, s string) float64 {
	return float64(font.MeasureString(face, s)) / 64
}

type align int

const (
	left align = iota
	center
	right
)

var outline = color.NRGBA{4, 7, 13, 230}

// text draws s with its baseline at y. With an outline, a dark copy is
// drawn in eight directions first so the text reads over anything.
func (c *canvas) text(face font.Face, s string, x, y float64, a align, col color.NRGBA, outlined bool) {
	switch a {
	case center:
		x -= width(face, s) / 2
	case right:
		x -= width(face, s)
	}
	d := font.Drawer{Dst: c.img, Face: face}
	if outlined {
		d.Src = image.NewUniform(outline)
		for _, o := range [][2]float64{{-2, -2}, {0, -2}, {2, -2}, {-2, 0}, {2, 0}, {-2, 2}, {0, 2}, {2, 2}} {
			d.Dot = fixed.Point26_6{X: fixed.Int26_6((x + o[0]) * 64), Y: fixed.Int26_6((y + o[1]) * 64)}
			d.DrawString(s)
		}
	}
	d.Src = image.NewUniform(col)
	d.Dot = fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
	d.DrawString(s)
}

// box is a rectangle that labels must not overlap.
type box struct{ x, y, w, h float64 }

func (a box) overlaps(b box) bool {
	return a.x < b.x+b.w && a.x+a.w > b.x && a.y < b.y+b.h && a.y+a.h > b.y
}

type boxes []box

func (bs boxes) free(b box) bool {
	for _, t := range bs {
		if t.overlaps(b) {
			return false
		}
	}
	return true
}
