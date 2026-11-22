// Package sample builds war snapshots from the captured API replies in
// internal/fixtures. Tests, the self-test and the --fixtures preview use it.
package sample

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/fixtures"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func quiet() *slog.Logger { return logx.New(io.Discard, slog.LevelInfo) }

// Community is the community API's 2026-11-21 war, fetched at now.
func Community(now time.Time) (*war.Snapshot, error) {
	srv := fixtures.Serve()
	defer srv.Close()
	c := &source.Community{Base: srv.URL, Version: "sample", Client: srv.Client(),
		Names: source.LoadNames("", quiet()), Now: func() time.Time { return now }}
	return c.Fetch(context.Background())
}

// Official is the official API's 2026-11-21 war, fetched at now.
func Official(now time.Time) (*war.Snapshot, error) {
	return OfficialWith(now, "official-assignment.json")
}

// OfficialWith is the official API's war with the Assignment reply taken
// from the fixture file named assignment.
func OfficialWith(now time.Time, assignment string) (*war.Snapshot, error) {
	h := fixtures.Handler()
	body := fixtures.Read(assignment)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/Assignment/War/801" {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	o := &source.Official{Base: srv.URL + "/api", Client: srv.Client(),
		Names: source.LoadNames("", quiet()), Now: func() time.Time { return now }, Log: quiet()}
	return o.Fetch(context.Background())
}
