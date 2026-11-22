// Package logx writes log lines in the format the other bots use and the
// daily bot-alerts report reads: "INFO: msg k=v", "WARN: ...", "ERROR: ...".
package logx

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

// New returns a logger that writes one line per record to w and drops
// records below level.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(&handler{out: &output{w: w}, level: level})
}

type output struct {
	mu sync.Mutex
	w  io.Writer
}

type handler struct {
	out    *output
	level  slog.Level
	attrs  string // already formatted " k=v" pairs from With
	prefix string // group prefix for later keys, like "poll."
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(levelName(r.Level))
	b.WriteString(": ")
	b.WriteString(r.Message)
	b.WriteString(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, h.prefix, a)
		return true
	})
	b.WriteByte('\n')
	h.out.mu.Lock()
	defer h.out.mu.Unlock()
	_, err := io.WriteString(h.out.w, b.String())
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var b strings.Builder
	for _, a := range attrs {
		writeAttr(&b, h.prefix, a)
	}
	c := *h
	c.attrs += b.String()
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.prefix += name + "."
	return &c
}

func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func writeAttr(b *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		p := prefix
		if a.Key != "" {
			p += a.Key + "."
		}
		for _, g := range a.Value.Group() {
			writeAttr(b, p, g)
		}
		return
	}
	b.WriteByte(' ')
	b.WriteString(prefix)
	b.WriteString(a.Key)
	b.WriteByte('=')
	v := a.Value.String()
	if v == "" || strings.ContainsAny(v, " \"=\n\t") {
		v = strconv.Quote(v)
	}
	b.WriteString(v)
}
