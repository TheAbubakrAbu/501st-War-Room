package render_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
)

func TestLogo(t *testing.T) {
	b, err := render.Logo()
	must(t, err)
	img, err := png.Decode(bytes.NewReader(b))
	must(t, err)
	if r := img.Bounds(); r.Dx() != render.LogoSize || r.Dy() != render.LogoSize {
		t.Fatalf("logo is %d x %d, want %d square", r.Dx(), r.Dy(), render.LogoSize)
	}
	again, err := render.Logo()
	must(t, err)
	if !bytes.Equal(b, again) {
		t.Fatal("the logo came out different the second time")
	}
	// The corners are dark space, because Discord crops avatars round, and
	// the middle carries the white lettering.
	if r, g, bl := rgb(img, 6, 6); r+g+bl > 90 {
		t.Fatalf("corner is (%d, %d, %d), want dark space", r, g, bl)
	}
	white := 0
	for y := render.LogoSize * 2 / 5; y < render.LogoSize*3/5; y += 4 {
		for x := render.LogoSize / 4; x < render.LogoSize*3/4; x += 4 {
			if r, g, bl := rgb(img, float64(x), float64(y)); r > 240 && g > 240 && bl > 240 {
				white++
			}
		}
	}
	if white < 400 {
		t.Fatalf("only %d white samples in the middle, want the lettering", white)
	}
}
