package render_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
)

func TestBanner(t *testing.T) {
	b, err := render.Banner()
	must(t, err)
	img, err := png.Decode(bytes.NewReader(b))
	must(t, err)
	if r := img.Bounds(); r.Dx() != render.BannerWidth || r.Dy() != render.BannerHeight {
		t.Fatalf("banner is %d x %d, want %d x %d", r.Dx(), r.Dy(), render.BannerWidth, render.BannerHeight)
	}
	// Discord's profile banners are 17:6.
	if render.BannerWidth*6 != render.BannerHeight*17 {
		t.Fatalf("banner is not 17:6")
	}
	again, err := render.Banner()
	must(t, err)
	if !bytes.Equal(b, again) {
		t.Fatal("the banner came out different the second time")
	}
	// The corners are dark space.
	for _, p := range [][2]float64{{6, 6}, {render.BannerWidth - 7, 6}, {6, render.BannerHeight - 7}, {render.BannerWidth - 7, render.BannerHeight - 7}} {
		if r, g, bl := rgb(img, p[0], p[1]); r+g+bl > 90 {
			t.Fatalf("corner %v is (%d, %d, %d), want dark space", p, r, g, bl)
		}
	}
	// The lettering sits right of center, clear of the bottom left, where
	// Discord lays the avatar over the banner.
	white := func(x0, x1, y0, y1 int) int {
		n := 0
		for y := y0; y < y1; y += 4 {
			for x := x0; x < x1; x += 4 {
				if r, g, bl := rgb(img, float64(x), float64(y)); r > 240 && g > 240 && bl > 240 {
					n++
				}
			}
		}
		return n
	}
	if n := white(render.BannerWidth*2/5, render.BannerWidth*3/4, render.BannerHeight/4, render.BannerHeight*3/4); n < 400 {
		t.Fatalf("only %d white samples in the lettering area, want the lettering", n)
	}
	if n := white(0, render.BannerWidth/4, render.BannerHeight*3/5, render.BannerHeight); n > 20 {
		t.Fatalf("%d white samples in the bottom left, where the avatar goes", n)
	}
}
