// Command logo draws the bot's pictures for the Discord Developer Portal:
// logo.png, the app icon (also every embed's thumbnail and the picture in
// the README), and banner.png, the profile banner. Rerun it after changing
// render.Logo or render.Banner:
//
//	go run ./tools/logo
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/atomicfile"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
)

func main() {
	logo := flag.String("out", "logo.png", "where to write the logo")
	banner := flag.String("banner", "banner.png", "where to write the banner")
	flag.Parse()
	for _, p := range []struct {
		path string
		draw func() ([]byte, error)
	}{{*logo, render.Logo}, {*banner, render.Banner}} {
		b, err := p.draw()
		if err == nil {
			err = atomicfile.Write(p.path, b)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "logo:", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s (%d bytes)\n", p.path, len(b))
	}
}
