package source_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/fixtures"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func quiet() *slog.Logger { return logx.New(io.Discard, slog.LevelInfo) }

func community(t *testing.T, srv *httptest.Server) *source.Community {
	t.Helper()
	return &source.Community{Base: srv.URL, Version: "test", Client: srv.Client(),
		Names: source.LoadNames("", quiet()), Now: func() time.Time { return testNow }}
}

// serveWith serves the fixtures, letting override answer first.
func serveWith(override func(w http.ResponseWriter, r *http.Request) bool) *httptest.Server {
	h := fixtures.Handler()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if override != nil && override(w, r) {
			return
		}
		h.ServeHTTP(w, r)
	}))
}

func TestCommunityFixtures(t *testing.T) {
	srv := fixtures.Serve()
	defer srv.Close()
	s, err := community(t, srv).Fetch(context.Background())
	must(t, err)
	eq(t, s.Source, "community")
	eq(t, s.Fetched, testNow)
	held, total := s.Held()
	eq(t, held, 212)
	eq(t, total, 273)
	eq(t, s.Players(), 42318)
	eq(t, s.Battles(), 43)
	var fronts []int
	for _, p := range s.Fronts(5) {
		fronts = append(fronts, p.Index)
	}
	eq(t, fmt.Sprint(fronts), "[200 269 182 268 273]")

	mb := s.Planet(200)
	eq(t, mb.Name, "Matar Bay")
	eq(t, mb.Owner, war.Automatons)
	eq(t, mb.Health, int64(1594032))
	eq(t, mb.MaxHealth, int64(1600000))
	eq(t, mb.Players, 10523)
	eq(t, mb.InCampaign, true)

	ac := s.Planet(273)
	eq(t, ac.Name, "Alderidge Cove")
	eq(t, ac.Owner, war.SuperEarth)
	if ac.Defense == nil {
		t.Fatal("273 should be under defense")
	}
	eq(t, *ac.Defense, war.Defense{Enemy: war.Illuminate, Health: 530109, MaxHealth: 550000, Ends: time.Date(2026, 11, 22, 22, 7, 16, 0, time.UTC)})
	eq(t, ac.Defense.Ends.Unix(), int64(1795385236))
	eq(t, len(ac.Hazards), 0) // the API's "None" hazard is dropped
	eq(t, fmt.Sprint(ac.Waypoints), "[98 145]")

	zr := s.Planet(7)
	eq(t, zr.Name, "Zea Rugosia")
	eq(t, zr.Sector, "Ferris")
	eq(t, zr.Biome, "Desert Cliffs")
	eq(t, fmt.Sprint(zr.Hazards), "[Tremors]")
	eq(t, zr.X, 0.36159223)
	eq(t, zr.Y, -0.52270126)

	eq(t, len(s.Orders), 1)
	o := s.Orders[0]
	eq(t, o.ID, int64(1278711574))
	eq(t, o.Title, "MAJOR ORDER")
	eq(t, o.Reward, "45 Medals")
	eq(t, o.Ends.Unix(), int64(1795728744))
	eq(t, strings.HasPrefix(o.Briefing, "Eliminate the mandated amount of Troopers"), true)
	eq(t, strings.HasSuffix(o.Briefing, "production."), true)
	eq(t, len(o.Tasks), 4)
	eq(t, o.Tasks[0].Type, 3)
	eq(t, o.Tasks[0].Progress, int64(7007086))
	eq(t, o.Tasks[0].Goal(), int64(135000000))
	eq(t, o.Tasks[0].Unit(), int64(878778730))
	eq(t, o.Tasks[3].Progress, int64(22019053))
	eq(t, o.Tasks[3].Faction(), war.Terminids)
}

func TestCommunityHeaders(t *testing.T) {
	var mu sync.Mutex
	var seen []http.Header
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		return false
	})
	defer srv.Close()
	c := community(t, srv)
	_, err := c.Fetch(context.Background())
	must(t, err)
	eq(t, len(seen), 3)
	for _, h := range seen {
		eq(t, h.Get("X-Super-Client"), "501st-war-room")
		eq(t, h.Get("X-Super-Contact"), "501st-war-room")
		eq(t, h.Get("User-Agent"), "501st-war-room/test")
		eq(t, h.Get("Accept-Language"), "en-US")
	}
	seen = nil
	c.Contact = "someone"
	_, err = c.Fetch(context.Background())
	must(t, err)
	eq(t, seen[0].Get("X-Super-Contact"), "someone")
}

func TestCommunityFailsWhole(t *testing.T) {
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/campaigns" {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return true
		}
		return false
	})
	defer srv.Close()
	s, err := community(t, srv).Fetch(context.Background())
	if s != nil || err == nil {
		t.Fatalf("want nil snapshot and an error, got %v, %v", s, err)
	}
	contains(t, err.Error(), "campaigns")
	contains(t, err.Error(), "503")
}

func TestCommunityBadJSON(t *testing.T) {
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/assignments" {
			io.WriteString(w, "[{")
			return true
		}
		return false
	})
	defer srv.Close()
	s, err := community(t, srv).Fetch(context.Background())
	if s != nil || err == nil {
		t.Fatalf("want an error, got %v", s)
	}
	contains(t, err.Error(), "assignments")
}

func TestCommunityTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/planets" {
			<-block
			return true
		}
		return false
	})
	defer srv.Close()
	defer close(block)
	c := community(t, srv)
	c.Timeout = 50 * time.Millisecond
	start := time.Now()
	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("want a timeout")
	}
	if time.Since(start) > 900*time.Millisecond {
		t.Fatalf("took %v", time.Since(start))
	}
}

func TestCommunityUpdatesNames(t *testing.T) {
	planets := fixtures.Read("community-planets.json")
	renamed := bytes.Replace(planets, []byte(`"name":"ZEA RUGOSIA"`), []byte(`"name":"ZEA RUGOSIA PRIME"`), 1)
	if bytes.Equal(renamed, planets) {
		renamed = bytes.Replace(planets, []byte(`"name": "ZEA RUGOSIA"`), []byte(`"name": "ZEA RUGOSIA PRIME"`), 1)
	}
	if bytes.Equal(renamed, planets) {
		t.Fatal("fixture has no ZEA RUGOSIA name to replace")
	}
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/planets" {
			w.Write(renamed)
			return true
		}
		return false
	})
	defer srv.Close()
	c := community(t, srv)
	s, err := c.Fetch(context.Background())
	must(t, err)
	eq(t, c.Names.Get(7).Name, "Zea Rugosia Prime")
	eq(t, s.Planet(7).Name, "Zea Rugosia Prime")
}

// A reply that fails the sanity check never reaches the names cache.
func TestCommunityBadReplyKeepsNames(t *testing.T) {
	srv := serveWith(func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/v1/planets" {
			w.Write([]byte(`[{"index":7,"name":"GARBAGE","currentOwner":"Humans"}]`))
			return true
		}
		return false
	})
	defer srv.Close()
	c := community(t, srv)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("want the sanity check to fail")
	}
	eq(t, c.Names.Get(7).Name, "Zea Rugosia")
}
