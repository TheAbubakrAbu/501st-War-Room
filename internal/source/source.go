// Package source fetches the Galactic War from the community API and from
// Arrowhead's official API, and turns either into a war.Snapshot.
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// Source fetches one whole round of the war. It returns either a complete
// snapshot or an error, never part of one.
type Source interface {
	Name() string
	Fetch(ctx context.Context) (*war.Snapshot, error)
}

const defaultTimeout = 15 * time.Second

// maxBody caps one reply; the largest real one is under 1 MB.
const maxBody = 16 << 20

// getJSON fetches u and decodes its JSON body into v. Any status but 200
// is an error naming the path and the status.
func getJSON(ctx context.Context, c *http.Client, timeout time.Duration, u string, headers map[string]string, v any) error {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if c == nil {
		c = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path := u
	if p, err := url.Parse(u); err == nil {
		path = p.Path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(v); err != nil {
		return fmt.Errorf("%s: bad JSON: %w", path, err)
	}
	return nil
}

func ownerFromName(s string) war.Faction {
	switch s {
	case "Humans":
		return war.SuperEarth
	case "Terminids":
		return war.Terminids
	case "Automaton", "Automatons":
		return war.Automatons
	case "Illuminate":
		return war.Illuminate
	}
	return 0
}

// taskValues pairs a task's values with their value types.
func taskValues(values []int64, types []int) map[int]int64 {
	m := make(map[int]int64, len(types))
	for i, vt := range types {
		if i < len(values) {
			m[vt] = values[i]
		}
	}
	return m
}
