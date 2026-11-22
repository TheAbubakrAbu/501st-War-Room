// Package store keeps one small JSON file of settings per server (spec 10).
// Nothing about users is stored: only server, channel, message and role
// IDs, and the IDs of orders already announced.
package store

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/atomicfile"
)

// Server is one server's war-room settings.
type Server struct {
	Guild     string  `json:"guild"`
	Channel   string  `json:"channel"`
	Message   string  `json:"message"`
	Role      string  `json:"role"`
	Announced []int64 `json:"announced"`
	// Seeded is true once the orders running when the war room was set up
	// are in Announced, so they are never announced as new.
	Seeded bool   `json:"seeded"`
	Paused string `json:"paused"` // why updates stopped; "" while running
}

const keepAnnounced = 20

// MarkAnnounced records an order as announced, keeping the newest 20.
func (s *Server) MarkAnnounced(id int64) {
	if s.WasAnnounced(id) {
		return
	}
	s.Announced = append(s.Announced, id)
	if n := len(s.Announced); n > keepAnnounced {
		s.Announced = slices.Clone(s.Announced[n-keepAnnounced:])
	}
}

// WasAnnounced reports whether the order was announced.
func (s Server) WasAnnounced(id int64) bool { return slices.Contains(s.Announced, id) }

// Store holds every server's settings in memory and writes each change
// through to its file.
type Store struct {
	mu      sync.Mutex
	dir     string // dir/servers
	servers map[string]Server
	log     *slog.Logger
}

// Open loads every settings file in dir/servers. A file that does not parse
// is renamed to <id>.json.bad and skipped.
func Open(dir string, log *slog.Logger) (*Store, error) {
	s := &Store{dir: filepath.Join(dir, "servers"), servers: map[string]Server{}, log: log}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(s.dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var sv Server
		if err := json.Unmarshal(b, &sv); err != nil || !validGuild(sv.Guild) || sv.Guild+".json" != name {
			if err == nil {
				err = errors.New("guild ID does not match the file name")
			}
			log.Error("server settings file is broken, set it aside", "file", name, "err", err)
			if rerr := os.Rename(path, path+".bad"); rerr != nil {
				log.Error("could not set the broken file aside", "file", name, "err", rerr)
			}
			continue
		}
		s.servers[sv.Guild] = sv
	}
	return s, nil
}

func validGuild(g string) bool {
	if g == "" || len(g) > 25 {
		return false
	}
	for _, c := range g {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func clone(s Server) Server {
	s.Announced = slices.Clone(s.Announced)
	return s
}

// Get returns a copy of a server's settings.
func (s *Store) Get(guild string) (Server, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.servers[guild]
	return clone(sv), ok
}

// Put saves a server's settings. Memory always takes the change, even when
// the file write fails: the running bot must never repeat an announcement or
// a repost because the disk refused a write. The error is still returned, so
// the caller logs it.
func (s *Store) Put(sv Server) error {
	if !validGuild(sv.Guild) {
		return fmt.Errorf("bad guild ID %q", sv.Guild)
	}
	b, err := json.MarshalIndent(sv, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers[sv.Guild] = clone(sv)
	return atomicfile.Write(filepath.Join(s.dir, sv.Guild+".json"), append(b, '\n'))
}

// Delete removes a server's settings. Deleting an unknown server is fine.
func (s *Store) Delete(guild string) error {
	if !validGuild(guild) {
		return fmt.Errorf("bad guild ID %q", guild)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(filepath.Join(s.dir, guild+".json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	delete(s.servers, guild)
	return nil
}

// All returns copies of every server's settings, sorted by guild ID.
func (s *Store) All() []Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Server, 0, len(s.servers))
	for _, sv := range s.servers {
		out = append(out, clone(sv))
	}
	slices.SortFunc(out, func(a, b Server) int {
		return cmp.Or(cmp.Compare(len(a.Guild), len(b.Guild)), cmp.Compare(a.Guild, b.Guild))
	})
	return out
}
