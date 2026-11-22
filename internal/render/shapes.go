package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

type pt struct{ x, y float64 }

// canvas draws anti-aliased shapes onto an RGBA picture. Each shape is
// rasterized only over its own bounding box, which keeps hundreds of small
// shapes cheap.
type canvas struct {
	img *image.RGBA
	z   vector.Rasterizer
}

func newCanvas(size int) *canvas {
	return &canvas{img: image.NewRGBA(image.Rect(0, 0, size, size))}
}

// fill fills the contours with c. Contours of opposite winding cancel, so a
// ring is an outer circle plus an inner one drawn the other way round.
func (c *canvas) fill(contours [][]pt, col color.NRGBA) {
	if col.A == 0 {
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, ct := range contours {
		for _, p := range ct {
			minX, minY = math.Min(minX, p.x), math.Min(minY, p.y)
			maxX, maxY = math.Max(maxX, p.x), math.Max(maxY, p.y)
		}
	}
	r := image.Rect(int(math.Floor(minX))-1, int(math.Floor(minY))-1, int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1).Intersect(c.img.Bounds())
	if r.Empty() {
		return
	}
	c.z.Reset(r.Dx(), r.Dy())
	c.z.DrawOp = draw.Over
	ox, oy := float64(r.Min.X), float64(r.Min.Y)
	for _, ct := range contours {
		if len(ct) < 3 {
			continue
		}
		c.z.MoveTo(float32(ct[0].x-ox), float32(ct[0].y-oy))
		for _, p := range ct[1:] {
			c.z.LineTo(float32(p.x-ox), float32(p.y-oy))
		}
		c.z.ClosePath()
	}
	c.z.Draw(c.img, r, image.NewUniform(col), image.Point{})
}

func segments(radius float64) int {
	n := int(math.Ceil(2 * math.Pi * radius / 1.5))
	return max(16, min(720, n))
}

// arcPoints walks a circle from angle a0 over sweep (radians, clockwise on
// screen because y points down).
func arcPoints(cx, cy, radius, a0, sweep float64) []pt {
	n := max(2, int(math.Ceil(float64(segments(radius))*math.Abs(sweep)/(2*math.Pi)))+1)
	out := make([]pt, n)
	for i := range n {
		a := a0 + sweep*float64(i)/float64(n-1)
		out[i] = pt{cx + radius*math.Cos(a), cy + radius*math.Sin(a)}
	}
	return out
}

func circlePath(cx, cy, radius float64) []pt {
	n := segments(radius)
	out := make([]pt, n)
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = pt{cx + radius*math.Cos(a), cy + radius*math.Sin(a)}
	}
	return out
}

func reversed(p []pt) []pt {
	out := make([]pt, len(p))
	for i, q := range p {
		out[len(p)-1-i] = q
	}
	return out
}

func (c *canvas) circle(cx, cy, radius float64, col color.NRGBA) {
	c.fill([][]pt{circlePath(cx, cy, radius)}, col)
}

// ring strokes a circle of the given radius with a band width wide.
func (c *canvas) ring(cx, cy, radius, width float64, col color.NRGBA) {
	inner := math.Max(0, radius-width/2)
	contours := [][]pt{circlePath(cx, cy, radius+width/2)}
	if inner > 0 {
		contours = append(contours, reversed(circlePath(cx, cy, inner)))
	}
	c.fill(contours, col)
}

// arc strokes part of a circle with round ends, from angle a0 over sweep.
func (c *canvas) arc(cx, cy, radius, width, a0, sweep float64, col color.NRGBA) {
	if sweep <= 0 {
		return
	}
	sweep = math.Min(sweep, 2*math.Pi)
	outer := arcPoints(cx, cy, radius+width/2, a0, sweep)
	inner := reversed(arcPoints(cx, cy, radius-width/2, a0, sweep))
	c.fill([][]pt{append(outer, inner...)}, col)
	if sweep < 2*math.Pi {
		for _, a := range []float64{a0, a0 + sweep} {
			c.circle(cx+radius*math.Cos(a), cy+radius*math.Sin(a), width/2, col)
		}
	}
}

// line strokes a straight line width wide, with square ends extended by ext.
func (c *canvas) line(x1, y1, x2, y2, width, ext float64, col color.NRGBA) {
	dx, dy := x2-x1, y2-y1
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	ux, uy := dx/l, dy/l
	x1, y1 = x1-ux*ext, y1-uy*ext
	x2, y2 = x2+ux*ext, y2+uy*ext
	nx, ny := -uy*width/2, ux*width/2
	c.fill([][]pt{{{x1 + nx, y1 + ny}, {x2 + nx, y2 + ny}, {x2 - nx, y2 - ny}, {x1 - nx, y1 - ny}}}, col)
}

func (c *canvas) rect(x, y, w, h float64, col color.NRGBA) {
	c.fill([][]pt{{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}}, col)
}

// strokeRect outlines a rectangle with a band width wide centered on its edge.
func (c *canvas) strokeRect(x, y, w, h, width float64, col color.NRGBA) {
	o, i := width/2, width/2
	c.fill([][]pt{
		{{x - o, y - o}, {x + w + o, y - o}, {x + w + o, y + h + o}, {x - o, y + h + o}},
		reversed([]pt{{x + i, y + i}, {x + w - i, y + i}, {x + w - i, y + h - i}, {x + i, y + h - i}}),
	}, col)
}

// brackets draws the four corner marks around a Major Order target.
func (c *canvas) brackets(cx, cy, half, arm, width float64, col color.NRGBA) {
	for _, d := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		x, y := cx+d[0]*half, cy+d[1]*half
		c.line(x-d[0]*arm, y, x, y, width, width/2, col)
		c.line(x, y, x, y-d[1]*arm, width, width/2, col)
	}
}
