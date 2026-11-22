package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// LogoSize is the logo's width and height in pixels.
const LogoSize = 1024

// Logo draws the bot's logo in the map's style: the same sky and rings,
// planets in each faction's color with a battle ringed on each front, and
// "501ST WAR ROOM" locked in Major Order brackets. Everything sits inside
// the badge's rim, because Discord crops avatars round.
func Logo() ([]byte, error) {
	fs, err := newFaces()
	if err != nil {
		return nil, err
	}
	defer fs.close()
	const mid = LogoSize / 2.0
	c := newCanvas(LogoSize)
	fillSpace(c)
	skyGlow(c, mid, mid, LogoSize*0.62)
	stars(c, 420)

	blue := factionColor(war.SuperEarth)
	for _, r := range []float64{150, 270, 390} {
		c.ring(mid, mid, r, 2, color.NRGBA{140, 170, 230, 40})
	}

	// Planets on the outer rings, outside the lettering, joined by supply
	// lines: the Terminids hold the upper left, the Automatons the upper
	// right, the Illuminate the bottom, and Super Earth the rest.
	type planet struct {
		r, deg float64
		f      war.Faction
	}
	owner := func(deg float64) war.Faction {
		switch {
		case deg > -165 && deg < -105:
			return war.Terminids
		case deg > -75 && deg < -15:
			return war.Automatons
		case deg > 60 && deg < 120:
			return war.Illuminate
		}
		return war.SuperEarth
	}
	var outer []planet
	for deg := -180.0; deg < 180; deg += 15 {
		outer = append(outer, planet{390, deg, owner(deg)})
	}
	at := func(p planet) (float64, float64) {
		a := p.deg * math.Pi / 180
		return mid + p.r*math.Cos(a), mid + p.r*math.Sin(a)
	}
	inner := []planet{{270, -90, war.SuperEarth}, {270, -125, war.Terminids}, {270, -55, war.Automatons}, {270, 90, war.Illuminate}}
	link := func(a, b planet) {
		x1, y1 := at(a)
		x2, y2 := at(b)
		col := color.NRGBA{150, 175, 220, 60}
		if a.f == b.f && a.f != war.SuperEarth {
			col = withAlpha(factionColor(a.f), 0.45)
		}
		c.line(x1, y1, x2, y2, 2.5, 0, col)
	}
	for i, p := range outer {
		link(p, outer[(i+1)%len(outer)])
	}
	link(inner[0], outer[6])
	link(inner[1], outer[3])
	link(inner[2], outer[9])
	link(inner[3], outer[18])
	for _, p := range append(outer, inner...) {
		x, y := at(p)
		if p.f == war.SuperEarth {
			c.circle(x, y, 7, withAlpha(blue, 0.85))
		} else {
			c.circle(x, y, 9, factionColor(p.f))
		}
	}

	// One battle on each front, ringed as on the map, with the blue arc of
	// liberation.
	for _, b := range []struct {
		p        planet
		progress float64
	}{{outer[3], 0.62}, {outer[9], 0.35}, {outer[18], 0.8}} {
		x, y := at(b.p)
		col := factionColor(b.p.f)
		for i, a := range []float64{0.16, 0.09, 0.05} {
			c.ring(x, y, 30, 7+8*float64(i+1), withAlpha(col, a))
		}
		c.ring(x, y, 30, 7, withAlpha(col, 0.95))
		c.arc(x, y, 30, 9, -math.Pi/2, 2*math.Pi*b.progress, blue)
	}

	// The badge's rim.
	c.ring(mid, mid, 482, 4, withAlpha(blue, 0.35))
	c.ring(mid, mid, 462, 14, blue)

	// The lettering, in Major Order brackets.
	big, small := fs.Bold(212), fs.MonoBold(60)
	const top, gap = 392.0, 50.0
	bigBase := top + capHeight(big)
	smallBase := bigBase + gap + capHeight(small)
	w := math.Max(spacedWidth(big, "501ST", 4), spacedWidth(small, "WAR ROOM", 20))
	bx, by := mid-w/2-52, top-52
	bw, bh := w+104, smallBase-top+104
	for _, d := range [][2]float64{{0, 0}, {bw, 0}, {bw, bh}, {0, bh}} {
		x, y := bx+d[0], by+d[1]
		sx, sy := 1.0, 1.0
		if d[0] > 0 {
			sx = -1
		}
		if d[1] > 0 {
			sy = -1
		}
		c.line(x, y, x+sx*64, y, 12, 6, white)
		c.line(x, y, x, y+sy*64, 12, 6, white)
	}
	c.glowText(big, "501ST", mid, bigBase, 4, 18, white, withAlpha(blue, 0.9))
	c.glowText(small, "WAR ROOM", mid, smallBase, 20, 10, blue, withAlpha(blue, 0.55))

	return encode(c.img)
}

// capHeight is how far a capital letter reaches above the baseline.
func capHeight(face font.Face) float64 {
	b, _ := font.BoundString(face, "H")
	return float64(-b.Min.Y) / 64
}

func spacedWidth(face font.Face, s string, tracking float64) float64 {
	w, n := 0.0, 0
	for _, r := range s {
		w += width(face, string(r))
		n++
	}
	return w + tracking*float64(max(n-1, 0))
}

// drawSpaced draws s from x with its baseline at y, tracking pixels between
// the letters.
func drawSpaced(dst draw.Image, src image.Image, face font.Face, s string, x, y, tracking float64) {
	d := font.Drawer{Dst: dst, Src: src, Face: face}
	for _, r := range s {
		d.Dot = fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)}
		d.DrawString(string(r))
		x += width(face, string(r)) + tracking
	}
}

// glowText draws s centered on x with its baseline at y: a soft glow in
// glow's color first, blurred by about blur pixels, then the letters in col.
func (c *canvas) glowText(face font.Face, s string, x, y, tracking float64, blur int, col, glow color.NRGBA) {
	x -= spacedWidth(face, s, tracking) / 2
	mask := image.NewAlpha(c.img.Bounds())
	drawSpaced(mask, image.Opaque, face, s, x, y, tracking)
	blurAlpha(mask, blur)
	draw.DrawMask(c.img, c.img.Bounds(), image.NewUniform(glow), image.Point{}, mask, image.Point{}, draw.Over)
	drawSpaced(c.img, image.NewUniform(col), face, s, x, y, tracking)
}

// blurAlpha softens m in place with three box blurs of radius r each way,
// which comes close to a Gaussian blur.
func blurAlpha(m *image.Alpha, r int) {
	w, h := m.Bounds().Dx(), m.Bounds().Dy()
	tmp := make([]uint8, len(m.Pix))
	for range 3 {
		for y := range h {
			blurLine(m.Pix, tmp, y*m.Stride, 1, w, r)
		}
		for x := range w {
			blurLine(tmp, m.Pix, x, m.Stride, h, r)
		}
	}
}

// blurLine averages n values, src[off], src[off+step], ..., over a window
// of r on each side (outside counts as 0), into the same places in dst.
func blurLine(src, dst []uint8, off, step, n, r int) {
	sum := 0
	for i := 0; i < r && i < n; i++ {
		sum += int(src[off+i*step])
	}
	for i := range n {
		if j := i + r; j < n {
			sum += int(src[off+j*step])
		}
		if j := i - r - 1; j >= 0 {
			sum -= int(src[off+j*step])
		}
		dst[off+i*step] = uint8(sum / (2*r + 1))
	}
}
