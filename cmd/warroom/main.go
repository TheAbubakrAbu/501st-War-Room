// Command warroom is the 501st War Room Discord bot.
//
//	warroom                     run the bot (needs DISCORD_TOKEN)
//	warroom --self-test         check the whole bot against saved data
//	warroom --preview out.png   draw the map from live data
//	  --fixtures                ...or from the saved data
//	warroom --data dir          where settings live (default "data")
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/bot"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/selftest"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// version is set at build time with -ldflags "-X main.version=<commit>".
var version = "dev"

const (
	communityBase = "https://api.helldivers2.dev"
	officialBase  = "https://api.live.prod.thehelldiversgame.com/api"
	every         = 5 * time.Minute
	retry         = 30 * time.Second
	shutdownWait  = 20 * time.Second
)

// fixtureTime is when the saved data was captured.
var fixtureTime = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("warroom", flag.ContinueOnError)
	fs.SetOutput(stderr)
	selfTest := fs.Bool("self-test", false, "check the whole bot against saved data and exit")
	preview := fs.String("preview", "", "draw the map to this PNG file, print the briefing as JSON, and exit")
	fixtures := fs.Bool("fixtures", false, "with --preview: use the saved data instead of the live APIs")
	data := fs.String("data", "data", "folder for server settings and the planet names cache")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch {
	case *selfTest:
		if selftest.Run(stdout) > 0 {
			return 1
		}
		return 0
	case *preview != "":
		return runPreview(*preview, *fixtures, getenv, stdout, logx.New(stderr, slog.LevelInfo))
	}
	return live(*data, getenv, logx.New(stdout, slog.LevelInfo))
}

func sources(getenv func(string) string, names *source.Names, log *slog.Logger) source.Source {
	client := &http.Client{}
	return &source.Fallback{Log: log, Sources: []source.Source{
		&source.Community{Base: communityBase, Contact: getenv("SUPER_CONTACT"), Version: version, Client: client, Names: names, Now: time.Now},
		&source.Official{Base: officialBase, Client: client, Names: names, Now: time.Now, Log: log},
	}}
}

// fixed is a source that always returns the same snapshot.
type fixed struct{ snap *war.Snapshot }

func (f fixed) Name() string                                 { return "fixtures" }
func (f fixed) Fetch(context.Context) (*war.Snapshot, error) { return f.snap, nil }

func runPreview(out string, fixtures bool, getenv func(string) string, stdout io.Writer, log *slog.Logger) int {
	var src source.Source
	now := time.Now()
	if fixtures {
		snap, err := sample.Community(fixtureTime)
		if err != nil {
			log.Error("fixtures failed", "err", err)
			return 1
		}
		src, now = fixed{snap}, fixtureTime
	} else {
		src = sources(getenv, source.LoadNames("", log), log)
	}
	p := &poller.Poller{Source: src, Render: render.Map, Log: log}
	st := p.Round(context.Background())
	if st.Snap == nil {
		log.Error("no war data")
		return 1
	}
	if err := os.WriteFile(out, st.Map, 0o644); err != nil {
		log.Error("could not write the map", "err", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(bot.Briefing(st, now).Embeds[0]); err != nil {
		log.Error("could not print the briefing", "err", err)
		return 1
	}
	log.Info("map written", "file", out, "bytes", len(st.Map), "source", st.Snap.Source)
	return 0
}

func live(data string, getenv func(string) string, log *slog.Logger) int {
	log.Info("501st War Room starting", "version", version)
	token := getenv("DISCORD_TOKEN")
	if token == "" {
		log.Error("DISCORD_TOKEN is not set")
		return 1
	}
	st, err := store.Open(data, log)
	if err != nil {
		log.Error("could not open the settings folder", "dir", data, "err", err)
		return 1
	}
	names := source.LoadNames(filepath.Join(data, "planets.json"), log)
	p := &poller.Poller{Source: sources(getenv, names, log), Render: render.Map, Every: every, Retry: retry, Log: log}
	b, err := bot.New(token, p, st, log)
	if err != nil {
		log.Error("could not make the Discord session", "err", err)
		return 1
	}
	p.OnRound = b.OnRound
	if err := b.Start(); err != nil {
		log.Error("could not connect to Discord", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()
	exit := 0
	select {
	case <-ctx.Done():
	case <-b.Fatal(): // the bot logged why
		exit = 1
		stop()
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	select {
	case <-done:
	case <-sctx.Done():
	}
	if err := b.Close(sctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Warn("closing the Discord session failed", "err", err)
	}
	log.Info("stopped")
	return exit
}
