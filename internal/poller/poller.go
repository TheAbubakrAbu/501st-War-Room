// Package poller fetches the war on a timer, draws the map once per round,
// and swaps the new snapshot and picture in together. Commands only read
// what it stores, so they never wait on an API.
package poller

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// State is what the latest round left behind. It is never changed after it
// is stored.
type State struct {
	Snap    *war.Snapshot // nil until the first good round
	Map     []byte        // the latest map PNG; older than Snap after a failed draw, nil before any draw worked
	Failing int           // failed rounds in a row
}

// Poller runs the rounds.
type Poller struct {
	Source  source.Source
	Render  func(*war.Snapshot) ([]byte, error)
	Every   time.Duration // between rounds once there is data
	Retry   time.Duration // between rounds until the first data
	OnRound func(*State)  // called after each round, in the poller's goroutine
	Log     *slog.Logger

	state atomic.Pointer[State]

	// Only the goroutine running rounds touches these.
	lastGood    time.Time // the last good round, or the first round
	lastAlarm   time.Time // the last outage ERROR; zero while the feeds work
	lastDrawErr time.Time // the last map render ERROR; zero while drawing works
}

// An outage is logged as an error once there has been no good data for 15
// minutes (3 rounds once there is data, spec 5.4), then at most once an hour
// while it lasts. A map that fails to draw is logged at most hourly too.
const (
	alarmAfter = 15 * time.Minute
	alarmEvery = time.Hour
)

var empty = &State{}

// Current returns the latest state, never nil.
func (p *Poller) Current() *State {
	if st := p.state.Load(); st != nil {
		return st
	}
	return empty
}

// Round fetches and draws once, stores the result and returns it.
func (p *Poller) Round(ctx context.Context) *State {
	now := time.Now()
	if p.lastGood.IsZero() {
		p.lastGood = now
	}
	prev := p.Current()
	snap, err := p.Source.Fetch(ctx)
	if err != nil && ctx.Err() != nil {
		return prev // shutting down: not a failed round
	}
	var next *State
	if err == nil {
		next = &State{Snap: snap, Map: p.draw(snap, prev.Map, now)}
		switch {
		case prev.Snap == nil:
			p.Log.Info("war data ready", "source", snap.Source)
		case prev.Failing > 0:
			p.Log.Info("war data back", "source", snap.Source, "after", prev.Failing)
		case prev.Snap.Source != snap.Source:
			p.Log.Info("war source changed", "source", snap.Source)
		}
		p.lastGood, p.lastAlarm = now, time.Time{}
	} else {
		next = &State{Snap: prev.Snap, Map: prev.Map, Failing: prev.Failing + 1}
		down := now.Sub(p.lastGood)
		if down >= alarmAfter && (p.lastAlarm.IsZero() || now.Sub(p.lastAlarm) >= alarmEvery) {
			p.Log.Error("both war feeds are down", "for", down.Round(time.Second).String(), "rounds", next.Failing)
			p.lastAlarm = now
		}
	}
	p.state.Store(next)
	if p.OnRound != nil {
		p.OnRound(next)
	}
	return next
}

// draw renders the map. When that fails, the data still goes out with the
// last picture: a drawing bug is not a feed outage.
func (p *Poller) draw(snap *war.Snapshot, last []byte, now time.Time) []byte {
	img, err := p.Render(snap)
	if err == nil {
		p.lastDrawErr = time.Time{}
		return img
	}
	if p.lastDrawErr.IsZero() || now.Sub(p.lastDrawErr) >= alarmEvery {
		p.Log.Error("map render failed, keeping the last map", "err", err)
		p.lastDrawErr = now
	}
	return last
}

// Run does a round now, then one every Retry until there is data and every
// Every after that. It returns when ctx is done.
func (p *Poller) Run(ctx context.Context) {
	for {
		st := p.Round(ctx)
		wait := p.Every
		if st.Snap == nil {
			wait = p.Retry
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}
