package source_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/fixtures"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"log/slog"
)

type fake struct {
	name  string
	snap  *war.Snapshot
	err   error
	calls int
}

func (f *fake) Name() string { return f.name }
func (f *fake) Fetch(context.Context) (*war.Snapshot, error) {
	f.calls++
	return f.snap, f.err
}

func good(t *testing.T) *war.Snapshot {
	t.Helper()
	s, err := sample.Community(testNow)
	must(t, err)
	return s
}

func TestFallbackOrder(t *testing.T) {
	g := good(t)
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo)

	a := &fake{name: "community", snap: g}
	b := &fake{name: "official", snap: g}
	s, err := (&source.Fallback{Sources: []source.Source{a, b}, Log: log}).Fetch(context.Background())
	must(t, err)
	eq(t, s, g)
	eq(t, b.calls, 0)

	other := war.NewSnapshot(testNow, "official", g.Planets, nil)
	a = &fake{name: "community", err: errors.New("status 503")}
	b = &fake{name: "official", snap: other}
	s, err = (&source.Fallback{Sources: []source.Source{a, b}, Log: log}).Fetch(context.Background())
	must(t, err)
	eq(t, s, other)
	contains(t, buf.String(), "WARN: community failed")

	buf.Reset()
	small := war.NewSnapshot(testNow, "community", g.Planets[:150], nil)
	a = &fake{name: "community", snap: small}
	b = &fake{name: "official", snap: other}
	s, err = (&source.Fallback{Sources: []source.Source{a, b}, Log: log}).Fetch(context.Background())
	must(t, err)
	eq(t, s, other)
	contains(t, buf.String(), "WARN: community failed")
	contains(t, buf.String(), "150 planets")

	a = &fake{name: "community", err: errors.New("status 503")}
	b = &fake{name: "official", err: errors.New("timeout")}
	s, err = (&source.Fallback{Sources: []source.Source{a, b}, Log: log}).Fetch(context.Background())
	if s != nil || err == nil {
		t.Fatal("want an error")
	}
	contains(t, err.Error(), "community")
	contains(t, err.Error(), "official")
	eq(t, (&source.Fallback{}).Name(), "fallback")
}

func TestCheck(t *testing.T) {
	g := good(t)
	must(t, source.Check(g))
	edit := func(f func(ps []war.Planet) []war.Planet) error {
		ps := f(slices.Clone(g.Planets))
		return source.Check(war.NewSnapshot(testNow, "community", ps, nil))
	}
	if edit(func(ps []war.Planet) []war.Planet { return ps[:199] }) == nil {
		t.Fatal("199 planets passed")
	}
	if edit(func(ps []war.Planet) []war.Planet { return ps[:200] }) != nil {
		t.Fatal("200 planets failed")
	}
	if edit(func(ps []war.Planet) []war.Planet { ps[5].Owner = 0; return ps }) == nil {
		t.Fatal("owner 0 passed")
	}
	if edit(func(ps []war.Planet) []war.Planet { ps[5].Owner = 5; return ps }) == nil {
		t.Fatal("owner 5 passed")
	}
	if edit(func(ps []war.Planet) []war.Planet { ps[5].X = 1.6; return ps }) == nil {
		t.Fatal("x 1.6 passed")
	}
	if edit(func(ps []war.Planet) []war.Planet { ps[5].Y = -1.6; return ps }) == nil {
		t.Fatal("y -1.6 passed")
	}
	if source.Check(nil) == nil {
		t.Fatal("nil passed")
	}
}

func TestFallbackRealFailures(t *testing.T) {
	off := fixtures.Serve()
	defer off.Close()
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer limited.Close()
	c := community(t, limited)
	fb := &source.Fallback{Sources: []source.Source{c, official(off, fixedNow)}, Log: quiet()}
	s, err := fb.Fetch(context.Background())
	must(t, err)
	eq(t, s.Source, "official")

	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-time.After(time.Second):
		}
	}))
	defer slow.Close()
	defer close(block)
	c = community(t, slow)
	c.Timeout = 100 * time.Millisecond
	fb = &source.Fallback{Sources: []source.Source{c, official(off, fixedNow)}, Log: quiet()}
	start := time.Now()
	s, err = fb.Fetch(context.Background())
	must(t, err)
	eq(t, s.Source, "official")
	if d := time.Since(start); d >= time.Second {
		t.Fatalf("took %v", d)
	}
}

// A fetch cut short by shutdown is not a feed failure: no warnings, and the
// backup is not tried.
func TestFallbackQuietOnShutdown(t *testing.T) {
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &fake{name: "community", err: context.Canceled}
	b := &fake{name: "official", err: context.Canceled}
	_, err := (&source.Fallback{Sources: []source.Source{a, b}, Log: logx.New(&buf, slog.LevelInfo)}).Fetch(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	eq(t, buf.String(), "")
	eq(t, b.calls, 0)
}
