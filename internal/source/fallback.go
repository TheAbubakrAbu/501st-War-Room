package source

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// Fallback tries each source in order and keeps the first whole snapshot
// that passes Check (spec 5.4).
type Fallback struct {
	Sources []Source
	Log     *slog.Logger
}

// Name is "fallback".
func (f *Fallback) Name() string { return "fallback" }

// Fetch returns the first good snapshot. Each failed source logs a warning;
// when all fail, the error joins every failure. A fetch cut short by
// shutdown returns at once, without warnings.
func (f *Fallback) Fetch(ctx context.Context) (*war.Snapshot, error) {
	var errs []error
	for _, s := range f.Sources {
		snap, err := s.Fetch(ctx)
		if err == nil {
			err = Check(snap)
		}
		if err == nil {
			return snap, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if f.Log != nil {
			f.Log.Warn(s.Name()+" failed", "err", err)
		}
		errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
	}
	if len(errs) == 0 {
		return nil, errors.New("no war sources")
	}
	return nil, errors.Join(errs...)
}

// Limits for Check.
const (
	minPlanets  = 200
	maxPosition = 1.5
)

// Check rejects a snapshot that cannot be the real war: too few planets, an
// owner that is not a faction, or a position off the map.
func Check(s *war.Snapshot) error {
	if s == nil {
		return errors.New("no snapshot")
	}
	if n := len(s.Planets); n < minPlanets {
		return fmt.Errorf("only %d planets (want at least %d)", n, minPlanets)
	}
	for _, p := range s.Planets {
		if !p.Owner.Valid() {
			return fmt.Errorf("planet %d has owner %d", p.Index, p.Owner)
		}
		if math.Abs(p.X) > maxPosition || math.Abs(p.Y) > maxPosition || math.IsNaN(p.X) || math.IsNaN(p.Y) {
			return fmt.Errorf("planet %d is off the map at (%g, %g)", p.Index, p.X, p.Y)
		}
	}
	return nil
}
