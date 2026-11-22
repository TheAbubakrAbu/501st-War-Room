// Command planets regenerates internal/source/planets.json, the planet names
// built into the bot. Rerun it when new planets appear in the game:
//
//	go run ./tools/planets
//
// Names and sectors come from the community API (the bot's main source); the
// capitalization of each name comes from the helldivers-2/json names file,
// matched by name rather than by index, because that file has some indexes
// swapped (spec 5.3).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/atomicfile"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
)

const (
	planetsURL = "https://api.helldivers2.dev/api/v1/planets"
	namesURL   = "https://raw.githubusercontent.com/helldivers-2/json/master/planets/planets.json"
)

// The names file has these in capitals.
var overrides = map[string]string{"NEW INSIGHT": "New Insight", "WAYWARD": "Wayward"}

func main() {
	planetsFile := flag.String("planets", "", "community /api/v1/planets reply (default: fetch it live)")
	namesFile := flag.String("names", "", "helldivers-2/json planets/planets.json (default: fetch it live)")
	out := flag.String("out", "internal/source/planets.json", "file to write")
	flag.Parse()

	if err := run(*planetsFile, *namesFile, *out); err != nil {
		fmt.Fprintln(os.Stderr, "planets:", err)
		os.Exit(1)
	}
}

func run(planetsFile, namesFile, out string) error {
	pb, err := load(planetsFile, planetsURL)
	if err != nil {
		return err
	}
	nb, err := load(namesFile, namesURL)
	if err != nil {
		return err
	}
	var planets []struct {
		Index  int    `json:"index"`
		Name   string `json:"name"`
		Sector string `json:"sector"`
	}
	if err := json.Unmarshal(pb, &planets); err != nil {
		return fmt.Errorf("planets: %w", err)
	}
	var names map[string]struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(nb, &names); err != nil {
		return fmt.Errorf("names: %w", err)
	}
	casing := map[string]string{}
	for _, n := range names {
		casing[strings.ToUpper(n.Name)] = n.Name
	}
	maps := map[int]source.PlanetName{}
	for _, p := range planets {
		up := strings.ToUpper(strings.TrimSpace(p.Name))
		name, ok := overrides[up]
		if !ok {
			name, ok = casing[up]
		}
		if !ok {
			name = source.TitleCase(p.Name)
		}
		maps[p.Index] = source.PlanetName{Name: name, Sector: p.Sector}
	}
	if len(maps) == 0 {
		return fmt.Errorf("no planets in the reply")
	}
	if err := atomicfile.Write(out, source.EncodeNames(maps)); err != nil {
		return err
	}
	fmt.Printf("wrote %d planets to %s\n", len(maps), out)
	return nil
}

func load(file, url string) ([]byte, error) {
	if file != "" {
		return os.ReadFile(file)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Super-Client", "501st-war-room")
	req.Header.Set("X-Super-Contact", "501st-war-room")
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
