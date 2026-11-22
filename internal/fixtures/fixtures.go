// Package fixtures holds real API replies captured on 2026-11-21 (plus the
// official Assignment reply from 2026-11-19, a different Major Order) and
// serves them the way the two APIs do. Tests, the self-test and the preview
// read them. Do not edit them by hand: tests pin numbers taken from them.
package fixtures

import (
	"embed"
	"net/http"
	"net/http/httptest"
)

// FS holds the captured replies.
//
//go:embed *.json
var FS embed.FS

// Read returns one captured reply. It panics when the file is missing.
func Read(name string) []byte {
	b, err := FS.ReadFile(name)
	if err != nil {
		panic("fixtures: " + err.Error())
	}
	return b
}

var routes = map[string]string{
	"/api/v1/planets":              "community-planets.json",
	"/api/v1/campaigns":            "community-campaigns.json",
	"/api/v1/assignments":          "community-assignments.json",
	"/api/WarSeason/current/WarID": "official-warid.json",
	"/api/WarSeason/801/WarInfo":   "official-warinfo.json",
	"/api/WarSeason/801/Status":    "official-status.json",
	"/api/v2/Assignment/War/801":   "official-assignment.json",
}

// Handler answers GET requests for both APIs' paths with the captured
// replies, and 404 for anything else.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := routes[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(Read(name))
	})
}

// Serve starts a test server with Handler. Both sources take its URL as
// their base.
func Serve() *httptest.Server { return httptest.NewServer(Handler()) }
