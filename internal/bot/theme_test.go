package bot

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The thumbnail is served from the repository, so the file must be there:
// a square PNG at least 512 pixels wide, which Discord also takes as the
// app's icon.
func TestLogoIsInRepo(t *testing.T) {
	path, ok := strings.CutPrefix(Logo, repoFiles)
	if !ok {
		t.Fatalf("Logo %q is not a file in the repository", Logo)
	}
	f, err := os.Open(filepath.Join("..", "..", filepath.FromSlash(path)))
	must(t, err)
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	must(t, err)
	if cfg.Width != cfg.Height || cfg.Width < 512 {
		t.Fatalf("logo is %d x %d, want a square of at least 512", cfg.Width, cfg.Height)
	}
}

func TestInviteLink(t *testing.T) {
	// View Channel, Send Messages, Embed Links, Attach Files and Read
	// Message History, the same permissions the README asks for.
	eq(t, InviteLink("42"), "https://discord.com/oauth2/authorize?client_id=42&permissions=117760&scope=bot+applications.commands")
}
