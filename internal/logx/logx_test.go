package logx_test

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
)

func TestLineFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo)
	log.Info("poll ok", "source", "community")
	log.Warn("community failed", "err", "status 503")
	log.Error("both war feeds are down", "rounds", 3)
	log.Debug("hidden")
	want := "INFO: poll ok source=community\nWARN: community failed err=\"status 503\"\nERROR: both war feeds are down rounds=3\n"
	if buf.String() != want {
		t.Fatalf("got %q", buf.String())
	}
}

// The daily report in ~/bot-alerts.py finds errors with this pattern.
func TestErrorLinesMatchBotAlerts(t *testing.T) {
	re := regexp.MustCompile(`^(?:\[[^\]]*\] )?\[?(?:ERROR|CRITICAL)\s*\]?[: ]\s*(.*)$`)
	var buf bytes.Buffer
	logx.New(&buf, slog.LevelInfo).With("guild", "1").Error("edit failed")
	m := re.FindStringSubmatch(strings.TrimSpace(buf.String()))
	if m == nil || m[1] != "edit failed guild=1" {
		t.Fatalf("no match: %q", buf.String())
	}
}

func TestQuotingAndGroups(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo).WithGroup("poll")
	log.Info("x", "a", `say "hi"`, "b", "k=v", "c", "")
	want := "INFO: x poll.a=\"say \\\"hi\\\"\" poll.b=\"k=v\" poll.c=\"\"\n"
	if buf.String() != want {
		t.Fatalf("got %q", buf.String())
	}
}
