package poller_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// script returns its results in order, repeating the last one.
type script struct {
	mu      sync.Mutex
	results []error // nil means success
	calls   []time.Time
}

func (s *script) Name() string { return "script" }

func (s *script) Fetch(context.Context) (*war.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := min(len(s.calls), len(s.results)-1)
	s.calls = append(s.calls, time.Now())
	if err := s.results[i]; err != nil {
		return nil, err
	}
	return war.NewSnapshot(time.Now(), "community", []war.Planet{{Index: 0, Name: "Super Earth", Owner: war.SuperEarth}}, nil), nil
}

func (s *script) times() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []time.Duration
	for _, c := range s.calls {
		out = append(out, c.Sub(s.calls[0]))
	}
	return out
}

var errDown = errors.New("down")

func png(*war.Snapshot) ([]byte, error) { return []byte("png"), nil }

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func newPoller(src *script, buf *lockedBuf) *poller.Poller {
	return &poller.Poller{Source: src, Render: png, Every: 5 * time.Minute, Retry: 30 * time.Second,
		Log: logx.New(buf, slog.LevelInfo)}
}

func TestRoundSwapsAtomically(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil, errDown, errDown, nil}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		var got *poller.State
		p.OnRound = func(st *poller.State) { got = st }
		if st := p.Current(); st == nil || st.Snap != nil {
			t.Fatalf("before any round: %+v", st)
		}
		ctx := context.Background()
		first := p.Round(ctx)
		if first.Snap == nil || string(first.Map) != "png" || first.Failing != 0 {
			t.Fatalf("success: %+v", first)
		}
		if got != first || p.Current() != first {
			t.Fatal("OnRound and Current must see the stored state")
		}
		second := p.Round(ctx)
		if second.Snap != first.Snap || &second.Map[0] != &first.Map[0] || second.Failing != 1 {
			t.Fatalf("failure should keep the old data: %+v", second)
		}
		if p.Round(ctx).Failing != 2 {
			t.Fatal("want 2 failures")
		}
		if st := p.Round(ctx); st.Failing != 0 || st.Snap == first.Snap {
			t.Fatalf("recovery: %+v", st)
		}
	})
}

func TestRunRetriesFastUntilFirstData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{errDown, errDown, nil, nil, errDown, nil}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { p.Run(ctx); close(done) }()
		time.Sleep(17 * time.Minute)
		cancel()
		<-done
		want := []time.Duration{0, 30 * time.Second, time.Minute, 6 * time.Minute, 11 * time.Minute, 16 * time.Minute}
		got := src.times()
		if len(got) != len(want) {
			t.Fatalf("rounds at %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("rounds at %v, want %v", got, want)
			}
		}
	})
}

// The outage ERROR waits for 15 minutes without data, even while startup
// retries every 30 seconds, then repeats at most hourly.
func TestOutageLogging(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{errDown}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { p.Run(ctx); close(done) }()
		errors := func() int { return strings.Count(buf.String(), "ERROR: ") }

		time.Sleep(15*time.Minute - 15*time.Second) // rounds every 30 s up to 14:30
		synctest.Wait()
		if errors() != 0 {
			t.Fatalf("too early: %q", buf.String())
		}
		time.Sleep(30 * time.Second) // the round at 15:00
		synctest.Wait()
		if n := strings.Count(buf.String(), "ERROR: both war feeds are down for=15m0s rounds=31\n"); n != 1 || errors() != 1 {
			t.Fatalf("after 15 minutes: %q", buf.String())
		}
		time.Sleep(time.Hour - 30*time.Second) // up to 15:00 + 1 h - 15 s
		synctest.Wait()
		if errors() != 1 {
			t.Fatalf("repeated within the hour: %q", buf.String())
		}
		time.Sleep(30 * time.Second) // the round at 15:00 + 1 h
		synctest.Wait()
		if errors() != 2 {
			t.Fatalf("want a second line after an hour: %q", buf.String())
		}
		cancel()
		<-done
	})
}

// Once there is data, 3 failed rounds are the 15 minutes.
func TestOutageAlarmAfterGoodData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil, errDown}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { p.Run(ctx); close(done) }()
		time.Sleep(14 * time.Minute) // failed rounds at 5 and 10 minutes
		synctest.Wait()
		if strings.Contains(buf.String(), "ERROR: ") {
			t.Fatalf("too early: %q", buf.String())
		}
		time.Sleep(2 * time.Minute) // the third failed round, at 15 minutes
		synctest.Wait()
		if n := strings.Count(buf.String(), "ERROR: both war feeds are down for=15m0s rounds=3\n"); n != 1 {
			t.Fatalf("after the third failed round: %q", buf.String())
		}
		cancel()
		<-done
	})
}

// A map that fails to draw is not a feed outage: the new data goes out with
// the last good picture, and the error is logged once an hour, not every
// round.
func TestRenderFailureKeepsData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		ctx := context.Background()
		first := p.Round(ctx)
		p.Render = func(*war.Snapshot) ([]byte, error) { return nil, errors.New("no fonts") }
		time.Sleep(5 * time.Minute)
		st := p.Round(ctx)
		if st.Snap == nil || st.Snap == first.Snap || string(st.Map) != "png" || st.Failing != 0 {
			t.Fatalf("got %+v", st)
		}
		time.Sleep(5 * time.Minute)
		p.Round(ctx)
		if n := strings.Count(buf.String(), "ERROR: map render failed"); n != 1 {
			t.Fatalf("want one ERROR line: %q", buf.String())
		}
		time.Sleep(time.Hour)
		p.Round(ctx)
		if n := strings.Count(buf.String(), "ERROR: map render failed"); n != 2 {
			t.Fatalf("want a second line after an hour: %q", buf.String())
		}
		if strings.Contains(buf.String(), "both war feeds are down") {
			t.Fatalf("a render failure is not an outage: %q", buf.String())
		}
	})
}

// With no picture yet, a failed draw still lets the data through.
func TestRenderFailureBeforeAnyMap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		p.Render = func(*war.Snapshot) ([]byte, error) { return nil, errors.New("no fonts") }
		st := p.Round(context.Background())
		if st.Snap == nil || st.Map != nil || st.Failing != 0 {
			t.Fatalf("got %+v", st)
		}
	})
}

func TestRunStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		ctx, cancel := context.WithCancel(context.Background())
		go p.Run(ctx)
		time.Sleep(time.Minute)
		cancel()
		synctest.Wait() // the bubble only ends cleanly when Run has returned
	})
}

// A round cut short by shutdown is not a failed round: nothing is stored
// and OnRound does not run.
func TestCanceledRoundIsNotAFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &script{results: []error{nil, context.Canceled}}
		var buf lockedBuf
		p := newPoller(src, &buf)
		calls := 0
		p.OnRound = func(*poller.State) { calls++ }
		first := p.Round(context.Background())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if st := p.Round(ctx); st != first || p.Current() != first {
			t.Fatalf("canceled round stored %+v", st)
		}
		if calls != 1 {
			t.Fatalf("OnRound ran %d times", calls)
		}
	})
}
