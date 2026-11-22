// Package render draws the Galactic War map as a 1200 x 1200 PNG, matching
// the map on the design page (spec 8).
package render

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// Size is the picture's width and height in pixels.
const Size = 1200

const (
	pad        = 92         // the farthest planet sits this far from the edge
	half       = Size / 2   // Super Earth's pixel position
	radius     = half - pad // the galaxy's radius in pixels
	maxPNG     = 1 << 20    // above this, switch to a palette PNG
	labelGap   = 26         // label offset from a planet
	targetGap  = 40         // label offset from a Major Order target
	labelH     = 48         // a label's two lines
	edgeMargin = 12         // labels stay this far inside the picture
)

var (
	space      = rgb(0x060A12)
	spaceFG    = rgb(0xDCE4F0)
	spaceMuted = rgb(0x8C9AB0)
	white      = rgb(0xFFFFFF)
)

func rgb(c int) color.NRGBA { return color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255} }

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(a * 255))
	return c
}

func factionColor(f war.Faction) color.NRGBA {
	if !f.Valid() {
		return spaceMuted
	}
	return rgb(f.Color())
}

// Project turns a war position into pixels: Super Earth (0, 0) at the
// center, the farthest planet (maxAbs) pad pixels from the edge, y up.
func Project(x, y, maxAbs float64) (px, py float64) {
	if maxAbs <= 0 {
		maxAbs = 1
	}
	scale := float64(radius) / maxAbs
	return half + x*scale, half - y*scale
}

// Map draws the snapshot.
func Map(s *war.Snapshot) ([]byte, error) {
	fs, err := newFaces()
	if err != nil {
		return nil, err
	}
	defer fs.close()
	c := newCanvas(Size)
	m := 0.0
	for _, p := range s.Planets {
		m = math.Max(m, math.Max(math.Abs(p.X), math.Abs(p.Y)))
	}
	pos := func(p *war.Planet) (float64, float64) { return Project(p.X, p.Y, m) }

	drawSky(c)
	drawLines(c, s, pos)
	for i := range s.Planets {
		p := &s.Planets[i]
		x, y := pos(p)
		if p.Owner == war.SuperEarth {
			c.circle(x, y, 3.6, withAlpha(factionColor(p.Owner), 0.78))
		} else {
			c.circle(x, y, 4.8, factionColor(p.Owner))
		}
	}
	battles := s.Fronts(len(s.Planets))
	for _, p := range battles {
		x, y := pos(p)
		col := factionColor(p.Enemy())
		for i, a := range []float64{0.16, 0.09, 0.05} {
			c.ring(x, y, 13, 3.5+4*float64(i+1), withAlpha(col, a))
		}
		c.ring(x, y, 13, 3.5, withAlpha(col, 0.95))
		if pr := p.Progress(); pr > 0 {
			c.arc(x, y, 13, 4.5, -math.Pi/2, 2*math.Pi*math.Min(pr, 100)/100, factionColor(war.SuperEarth))
		}
	}
	seX, seY := float64(half), float64(half)
	if se := s.Planet(0); se != nil {
		seX, seY = pos(se)
	}
	c.circle(seX, seY, 9, factionColor(war.SuperEarth))
	c.circle(seX, seY, 3.5, white)

	taken := boxes{
		{24, 24, 560, 84},                // title block
		{24, Size - 220, 220, 196},       // legend
		{Size - 360, Size - 74, 336, 50}, // timestamp
	}
	seLabel := "SUPER EARTH"
	sw := width(fs.Bold(20), seLabel)
	c.text(fs.Bold(20), seLabel, seX, seY+32, center, spaceFG, true)
	taken = append(taken, box{seX - sw/2 - 4, seY - 12, sw + 8, 52})

	// Major Order targets, in a fixed order so every picture is the same.
	targets := slices.Sorted(func(yield func(int) bool) {
		for i := range s.Targets() {
			if !yield(i) {
				return
			}
		}
	})
	var targetPlanets []*war.Planet
	tagFace := fs.MonoBold(17)
	tw := width(tagFace, "MAJOR ORDER")
	for _, i := range targets {
		p := s.Planet(i)
		if p == nil {
			continue
		}
		targetPlanets = append(targetPlanets, p)
		mx, my := pos(p)
		const h = 30
		c.brackets(mx, my, h, 12, 3, white)
		c.rect(mx-h, my-h-30, tw+16, 24, white)
		c.text(tagFace, "MAJOR ORDER", mx-h+8, my-h-12, left, space, false)
		taken = append(taken, box{mx - h - 2, my - h - 2, 2*h + 4, 2*h + 4}, box{mx - h - 2, my - h - 32, tw + 20, 28})
	}

	// Labels: Major Order targets first, then the busiest fronts.
	isTarget := map[int]bool{}
	toLabel := slices.Clone(targetPlanets)
	for _, p := range targetPlanets {
		isTarget[p.Index] = true
	}
	for _, p := range s.Fronts(5) {
		if !isTarget[p.Index] {
			toLabel = append(toLabel, p)
		}
	}
	nameFace, statFace := fs.Bold(25), fs.Mono(17)
	for _, p := range toLabel {
		x, y := pos(p)
		off := float64(labelGap)
		if isTarget[p.Index] {
			off = targetGap
		}
		l1, l2 := label(p)
		w := math.Max(width(nameFace, l1), width(statFace, l2)) + 6
		cands := []box{
			{x + off, y - 26, w, labelH}, {x - off - w, y - 26, w, labelH},
			{x - w/2, y - off - 4 - labelH, w, labelH}, {x - w/2, y + off + 4, w, labelH},
			{x + off, y + 6, w, labelH}, {x - off - w, y + 6, w, labelH},
			{x + off, y - 58, w, labelH}, {x - off - w, y - 58, w, labelH},
		}
		for _, b := range cands {
			if b.x < edgeMargin || b.y < edgeMargin || b.x+b.w > Size-edgeMargin || b.y+b.h > Size-edgeMargin || !taken.free(b) {
				continue
			}
			taken = append(taken, b)
			c.text(nameFace, l1, b.x+3, b.y+22, left, white, true)
			c.text(statFace, l2, b.x+3, b.y+44, left, factionColor(p.Enemy()), true)
			break
		}
	}

	// Title block.
	c.text(fs.Bold(46), "GALACTIC WAR", 36, 68, left, white, true)
	sub := war.Comma(int64(s.Players())) + " HELLDIVERS DEPLOYED · " + war.Comma(int64(len(battles))) + " ACTIVE BATTLES"
	c.text(fs.Mono(18), sub, 38, 98, left, spaceMuted, true)

	drawLegend(c, fs)

	stamp := "UPDATED " + strings.ToUpper(s.Fetched.UTC().Format("Jan 2, 15:04")) + " UTC"
	c.text(fs.Bold(22), "501ST WAR ROOM", Size-36, Size-62, right, white, true)
	c.text(fs.Mono(16), stamp, Size-36, Size-36, right, spaceMuted, true)

	return encode(c.img)
}

// label is a planet's two label lines: the name, then the progress (or
// HELD when Super Earth holds it in peace) and the Helldivers there.
func label(p *war.Planet) (name, stat string) {
	progress := war.Pct(p.Progress())
	if p.Owner == war.SuperEarth && p.Defense == nil {
		progress = "HELD"
	}
	return strings.ToUpper(p.Name), progress + " · " + war.Comma(int64(p.Players))
}

func drawSky(c *canvas) {
	fillSpace(c)
	skyGlow(c, half, half, Size*0.62)
	stars(c, 600)
	for _, f := range []float64{0.25, 0.5, 0.75, 1} {
		c.ring(half, half, radius*f, 1.5, color.NRGBA{140, 170, 230, 20})
	}
}

func fillSpace(c *canvas) {
	draw.Draw(c.img, c.img.Bounds(), image.NewUniform(space), image.Point{}, draw.Src)
}

// skyGlow lays a soft blue glow over the picture, brightest at (cx, cy) and
// gone at maxR.
func skyGlow(c *canvas, cx, cy, maxR float64) {
	type stop struct {
		t       float64
		r, g, b float64
		a       float64
	}
	stops := []stop{{0, 46, 84, 160, 0.30}, {0.55, 30, 48, 96, 0.12}, {1, 0, 0, 0, 0}}
	pix := c.img.Pix
	b := c.img.Bounds()
	for y := range b.Dy() {
		for x := range b.Dx() {
			t := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) / maxR
			if t >= 1 {
				continue
			}
			k := 0
			if t > stops[1].t {
				k = 1
			}
			a, b := stops[k], stops[k+1]
			f := (t - a.t) / (b.t - a.t)
			al := a.a + (b.a-a.a)*f
			// Interpolate premultiplied, as a canvas gradient does.
			pr := (a.r*a.a + (b.r*b.a-a.r*a.a)*f)
			pg := (a.g*a.a + (b.g*b.a-a.g*a.a)*f)
			pb := (a.b*a.a + (b.b*b.a-a.b*a.a)*f)
			i := y*c.img.Stride + 4*x
			pix[i+0] = uint8(math.Round(pr + float64(pix[i+0])*(1-al)))
			pix[i+1] = uint8(math.Round(pg + float64(pix[i+1])*(1-al)))
			pix[i+2] = uint8(math.Round(pb + float64(pix[i+2])*(1-al)))
		}
	}
}

// stars scatters n faint stars over the picture, the same ones every time.
func stars(c *canvas, n int) {
	size := float64(c.img.Bounds().Dx())
	r := rand.New(rand.NewPCG(501, 501))
	for range n {
		a := 0.12 + r.Float64()*0.5
		x, y := r.Float64()*size, r.Float64()*size
		rad := 0.4 + r.Float64()*1.1
		c.circle(x, y, rad, color.NRGBA{220, 230, 255, uint8(math.Round(a * 255))})
	}
}

func drawLines(c *canvas, s *war.Snapshot, pos func(*war.Planet) (float64, float64)) {
	for _, pr := range s.Pairs() {
		a, b := s.Planet(pr[0]), s.Planet(pr[1])
		hot := a.InCampaign || b.InCampaign
		enemy := a.Owner != war.SuperEarth && a.Owner == b.Owner
		var col color.NRGBA
		switch {
		case enemy:
			col = withAlpha(factionColor(a.Owner), 0.32)
		case hot:
			col = color.NRGBA{170, 195, 240, 87}
		default:
			col = color.NRGBA{150, 175, 220, 41}
		}
		w := 1.4
		if hot {
			w = 2
		}
		x1, y1 := pos(a)
		x2, y2 := pos(b)
		c.line(x1, y1, x2, y2, w, 0, col)
	}
}

func drawLegend(c *canvas, fs *faces) {
	const lx, lw, lh = 24.0, 220.0, 196.0
	ly := float64(Size) - 24 - lh
	c.rect(lx, ly, lw, lh, color.NRGBA{6, 10, 18, 199})
	c.strokeRect(lx+0.5, ly+0.5, lw-1, lh-1, 1, color.NRGBA{255, 255, 255, 26})
	face := fs.Mono(15)
	y, step, ix, tx := ly+24, 25.0, lx+22, lx+42
	label := func(s string) { c.text(face, s, tx, y+5, left, spaceFG, false) }
	for _, e := range []struct {
		f war.Faction
		s string
	}{{war.SuperEarth, "SUPER EARTH"}, {war.Illuminate, "ILLUMINATE"}, {war.Automatons, "AUTOMATONS"}, {war.Terminids, "TERMINIDS"}} {
		c.circle(ix, y, 6, factionColor(e.f))
		label(e.s)
		y += step
	}
	c.ring(ix, y, 8, 3, factionColor(war.Illuminate))
	c.arc(ix, y, 8, 4, -math.Pi/2, 1.1*math.Pi, factionColor(war.SuperEarth))
	label("BATTLE")
	y += step
	c.strokeRect(ix-7, y-7, 14, 14, 2, white)
	label("MAJOR ORDER")
	y += step
	c.text(fs.Mono(13), "BLUE ARC = % LIBERATED", ix-8, y+4, left, spaceMuted, false)
}

func encode(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	if buf.Len() < maxPNG {
		return buf.Bytes(), nil
	}
	buf.Reset()
	if err := enc.Encode(&buf, quantize(img)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// quantize maps the picture onto 256 colors: ramps from the background to
// each color the map uses.
func quantize(img *image.RGBA) *image.Paletted {
	targets := []color.NRGBA{white, spaceFG, spaceMuted, rgb(0x2E54A0),
		factionColor(war.SuperEarth), factionColor(war.Terminids), factionColor(war.Automatons), factionColor(war.Illuminate)}
	pal := color.Palette{space}
	const steps = 31
	for _, t := range targets {
		for k := 1; k <= steps; k++ {
			f := float64(k) / steps
			mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*f)) }
			pal = append(pal, color.NRGBA{mix(space.R, t.R), mix(space.G, t.G), mix(space.B, t.B), 255})
		}
	}
	out := image.NewPaletted(img.Bounds(), pal)
	cache := map[uint32]uint8{}
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			i := y*img.Stride + 4*x
			key := uint32(img.Pix[i])<<16 | uint32(img.Pix[i+1])<<8 | uint32(img.Pix[i+2])
			idx, ok := cache[key]
			if !ok {
				idx = uint8(pal.Index(color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}))
				cache[key] = idx
			}
			out.Pix[y*out.Stride+x] = idx
		}
	}
	return out
}
