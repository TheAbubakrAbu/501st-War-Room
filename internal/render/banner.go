package render

import (
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// BannerWidth and BannerHeight are the banner's size in pixels: twice the
// 680 x 240 that Discord shows profile banners at.
const (
	BannerWidth  = 1360
	BannerHeight = 480
)

// Banner draws the bot's profile banner in the logo's style: the map's rings
// sweep out to both sides with a front in each faction's color, the
// Terminids upper left, the Automatons upper right and the Illuminate lower
// right, and "501ST WAR ROOM" sits a little right of the middle, clear of
// the bottom left, where Discord lays the avatar over the banner.
func Banner() ([]byte, error) {
	fs, err := newFaces()
	if err != nil {
		return nil, err
	}
	defer fs.close()
	const cx, cy = BannerWidth/2.0 + 50, BannerHeight / 2.0
	c := newCanvasRect(BannerWidth, BannerHeight)
	fillSpace(c)
	skyGlow(c, cx, cy, BannerWidth*0.55)
	stars(c, 320)
	for _, r := range []float64{150, 270, 390, 510, 630, 750} {
		c.ring(cx, cy, r, 2, color.NRGBA{140, 170, 230, 40})
	}

	const scale = 0.85
	tw, th := titleSize(fs, scale)
	top := cy - th/2
	// Planets keep out of the lettering's brackets, with room to spare.
	room := 52*scale + 34
	clear := box{cx - tw/2 - room, top - room, tw + 2*room, th + 2*room}

	owner := func(deg float64) war.Faction {
		a, up := math.Abs(deg), deg < 0
		switch {
		case a > 90 && up && 180-a >= 5:
			return war.Terminids
		case a < 90 && up && a >= 5:
			return war.Automatons
		case a < 90 && !up && a >= 5:
			return war.Illuminate
		}
		return war.SuperEarth
	}
	type planet struct {
		x, y, deg float64
		f         war.Faction
	}
	// Planets about 100 pixels apart on each ring, where they fit, each
	// ring turned a little and each planet nudged, the same way every time,
	// so they scatter like the map's instead of lining up.
	rings := []struct {
		r, turn float64
		n       int
	}{{390, 0, 24}, {510, 4, 32}, {630, 2, 40}}
	jitter := rand.New(rand.NewPCG(501, 17))
	on := make([][]*planet, len(rings))
	var all []*planet
	for k, rg := range rings {
		on[k] = make([]*planet, rg.n)
		for i := range rg.n {
			deg := -180 + rg.turn + 360*float64(i)/float64(rg.n) + (jitter.Float64()-0.5)*4
			if deg > 180 {
				deg -= 360
			}
			r := rg.r + (jitter.Float64()-0.5)*36
			a := deg * math.Pi / 180
			x, y := cx+r*math.Cos(a), cy+r*math.Sin(a)
			const margin = 30
			if x < margin || x > BannerWidth-margin || y < margin || y > BannerHeight-margin || clear.overlaps(box{x - 12, y - 12, 24, 24}) {
				continue
			}
			p := &planet{x, y, deg, owner(deg)}
			on[k][i] = p
			all = append(all, p)
		}
	}

	// Supply lines: along each ring, and out to the nearest planet on the
	// next ring.
	link := func(a, b *planet) {
		col := color.NRGBA{150, 175, 220, 60}
		if a.f == b.f && a.f != war.SuperEarth {
			col = withAlpha(factionColor(a.f), 0.45)
		}
		c.line(a.x, a.y, b.x, b.y, 2.5, 0, col)
	}
	for k, ps := range on {
		for i, p := range ps {
			if next := ps[(i+1)%len(ps)]; p != nil && next != nil {
				link(p, next)
			}
			if p == nil || k+1 == len(on) {
				continue
			}
			var near *planet
			best := 150.0
			for _, q := range on[k+1] {
				if q != nil {
					if d := math.Hypot(q.x-p.x, q.y-p.y); d < best {
						near, best = q, d
					}
				}
			}
			if near != nil {
				link(p, near)
			}
		}
	}
	blue := factionColor(war.SuperEarth)
	for _, p := range all {
		if p.f == war.SuperEarth {
			c.circle(p.x, p.y, 7, withAlpha(blue, 0.85))
		} else {
			c.circle(p.x, p.y, 9, factionColor(p.f))
		}
	}

	// One battle on each front, ringed as on the map, with the blue arc of
	// liberation.
	battle := func(k int, f war.Faction, deg, progress float64) {
		var p *planet
		for _, q := range on[k] {
			if q != nil && q.f == f && (p == nil || math.Abs(q.deg-deg) < math.Abs(p.deg-deg)) {
				p = q
			}
		}
		if p == nil {
			return
		}
		col := factionColor(p.f)
		for i, a := range []float64{0.16, 0.09, 0.05} {
			c.ring(p.x, p.y, 26, 6+7*float64(i+1), withAlpha(col, a))
		}
		c.ring(p.x, p.y, 26, 6, withAlpha(col, 0.95))
		c.arc(p.x, p.y, 26, 8, -math.Pi/2, 2*math.Pi*progress, blue)
	}
	battle(1, war.Terminids, -168, 0.62)
	battle(0, war.Automatons, -15, 0.35)
	battle(1, war.Illuminate, 20, 0.8)

	c.title(fs, cx, top, scale)
	return encode(c.img)
}
