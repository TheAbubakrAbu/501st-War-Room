package source_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/fixtures"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func official(srv *httptest.Server, now func() time.Time) *source.Official {
	return &source.Official{Base: srv.URL + "/api", Client: srv.Client(), Now: now,
		Names: source.LoadNames("", quiet()), Log: quiet()}
}

func fixedNow() time.Time { return testNow }

func TestOfficialFixtures(t *testing.T) {
	srv := fixtures.Serve()
	defer srv.Close()
	o := official(srv, fixedNow)
	eq(t, o.Name(), "official")
	s, err := o.Fetch(context.Background())
	must(t, err)
	eq(t, s.Source, "official")
	eq(t, s.Fetched, testNow)
	held, total := s.Held()
	eq(t, held, 212)
	eq(t, total, 273)
	eq(t, s.Players(), 39641)
	eq(t, s.Battles(), 43)
	var fronts []int
	for _, p := range s.Fronts(5) {
		fronts = append(fronts, p.Index)
	}
	eq(t, fmt.Sprint(fronts), "[200 269 182 268 273]")
	zr := s.Planet(7)
	eq(t, zr.Name, "Zea Rugosia")
	eq(t, zr.Sector, "Ferris")
	eq(t, zr.Biome, "")
	eq(t, s.Planet(200).Players, 9923)
	eq(t, s.Planet(200).Owner, war.Automatons)
	d := s.Planet(273).Defense
	if d == nil {
		t.Fatal("273 should be under defense")
	}
	eq(t, *d, war.Defense{Enemy: war.Illuminate, Health: 530058, MaxHealth: 550000, Ends: testNow.Add(71710 * time.Second)})
	eq(t, len(s.Orders), 1)
	ord := s.Orders[0]
	eq(t, ord.Title, "MAJOR ORDER")
	eq(t, ord.Reward, "45 Medals")
	eq(t, ord.Ends, testNow.Add(415208*time.Second))
	eq(t, ord.Tasks[0].Progress, int64(7036475))
	eq(t, ord.Tasks[3].Progress, int64(22118170))
	eq(t, strings.HasSuffix(ord.Briefing, "production."), true)
}

func TestOfficialOrderIDMatchesCommunity(t *testing.T) {
	srv := fixtures.Serve()
	defer srv.Close()
	c, err := community(t, srv).Fetch(context.Background())
	must(t, err)
	o, err := official(srv, fixedNow).Fetch(context.Background())
	must(t, err)
	eq(t, o.Orders[0].ID, c.Orders[0].ID)
	eq(t, o.Orders[0].ID, int64(1278711574))
	eq(t, len(o.Orders[0].Tasks), len(c.Orders[0].Tasks))
}

func TestSourcesAgreeOnTheMap(t *testing.T) {
	srv := fixtures.Serve()
	defer srv.Close()
	c, err := community(t, srv).Fetch(context.Background())
	must(t, err)
	o, err := official(srv, fixedNow).Fetch(context.Background())
	must(t, err)
	eq(t, len(o.Planets), len(c.Planets))
	for _, cp := range c.Planets {
		op := o.Planet(cp.Index)
		if op == nil {
			t.Fatalf("official lacks planet %d", cp.Index)
		}
		if op.X != cp.X || op.Y != cp.Y || fmt.Sprint(op.Waypoints) != fmt.Sprint(cp.Waypoints) {
			t.Fatalf("planet %d differs: %v %v %v vs %v %v %v", cp.Index, op.X, op.Y, op.Waypoints, cp.X, cp.Y, cp.Waypoints)
		}
		eq(t, op.Name, cp.Name)
	}
	eq(t, len(o.Pairs()), 336)
	eq(t, fmt.Sprint(o.Pairs()), fmt.Sprint(c.Pairs()))
}

func TestOfficialOrder1119(t *testing.T) {
	h := fixtures.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/Assignment/War/801" {
			w.Write(fixtures.Read("official-assignment-2026-11-19.json"))
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s, err := official(srv, fixedNow).Fetch(context.Background())
	must(t, err)
	o := s.Orders[0]
	eq(t, o.ID, int64(2890701424))
	eq(t, o.Reward, "40 Medals")
	eq(t, o.Tasks[0].Type, 13)
	i, ok := o.Tasks[0].PlanetIndex()
	eq(t, i, 7)
	eq(t, ok, true)
	eq(t, o.Tasks[1].Progress, int64(208156482))
	eq(t, o.Ends, testNow.Add(110650*time.Second))
	contains(t, s.Line(o.Tasks[1], 2), "96.8% (208,156,482)")
	eq(t, s.Line(o.Tasks[0], 1), "○ Hold **Zea Rugosia** (held)") // Super Earth held it on 2026-11-19
}

func TestOfficialWarID(t *testing.T) {
	h := fixtures.Handler()
	var mu sync.Mutex
	idAsks := 0
	idStatus := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p := r.URL.Path
		switch {
		case p == "/api/WarSeason/current/WarID":
			idAsks++
			if idStatus != http.StatusOK {
				http.Error(w, "down", idStatus)
				return
			}
			w.Write([]byte(`{"id":802}`))
			return
		case strings.Contains(p, "/802/") || strings.HasSuffix(p, "/802"):
			r.URL.Path = strings.Replace(p, "802", "801", 1)
		case strings.Contains(p, "801"):
			if idStatus == http.StatusOK {
				http.NotFound(w, r) // only 802 is live while the ID works
				return
			}
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()

	now := testNow
	o := official(srv, func() time.Time { return now })
	_, err := o.Fetch(context.Background())
	must(t, err)
	eq(t, idAsks, 1)
	now = testNow.Add(30 * time.Minute)
	_, err = o.Fetch(context.Background())
	must(t, err)
	eq(t, idAsks, 1)
	now = testNow.Add(61 * time.Minute)
	_, err = o.Fetch(context.Background())
	must(t, err)
	eq(t, idAsks, 2)

	// The war ID request failing never fails the round.
	mu.Lock()
	idStatus = http.StatusInternalServerError
	mu.Unlock()
	fresh := official(srv, fixedNow)
	_, err = fresh.Fetch(context.Background())
	must(t, err)
	mu.Lock()
	eq(t, idAsks, 3)
	mu.Unlock()
	_, err = fresh.Fetch(context.Background()) // not retried every round
	must(t, err)
	mu.Lock()
	eq(t, idAsks, 3)
	mu.Unlock()
}

func TestOfficialFailsWhole(t *testing.T) {
	h := fixtures.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/WarSeason/801/Status" {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s, err := official(srv, fixedNow).Fetch(context.Background())
	if s != nil || err == nil {
		t.Fatal("want an error")
	}
	contains(t, err.Error(), "Status")
	contains(t, err.Error(), "429")
}
