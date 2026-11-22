// Command logo draws logo.png, the bot's logo: every embed's thumbnail, the
// picture in the README, and the app icon to upload in the Discord
// Developer Portal. Rerun it after changing render.Logo:
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
	out := flag.String("out", "logo.png", "file to write")
	flag.Parse()
	b, err := render.Logo()
	if err == nil {
		err = atomicfile.Write(*out, b)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "logo:", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (%d bytes)\n", *out, len(b))
}
