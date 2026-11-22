package bot

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/bwmarrin/discordgo"
)

// A revoked token closes the gateway with code 4004 for good. That is one
// ERROR line, and the bot stops, so systemd and the down alerts see it
// the way they see a bad token at startup.
func TestRevokedTokenIsFatal(t *testing.T) {
	var buf bytes.Buffer
	b := &Bot{Log: logx.New(&buf, slog.LevelInfo), fatal: make(chan error, 1)}
	b.discordLog(discordgo.LogWarning, "error reading from gateway wss://gateway.discord.gg websocket, websocket: close 1006 (abnormal closure): unexpected EOF")
	contains(t, buf.String(), "WARN: discord: error reading from gateway")
	select {
	case err := <-b.Fatal():
		t.Fatalf("a dropped connection is not fatal: %v", err)
	default:
	}
	b.discordLog(discordgo.LogWarning, "error reading from gateway wss://gateway.discord.gg websocket, websocket: close 4004: Authentication failed.")
	b.discordLog(discordgo.LogError, "error reconnecting to gateway, websocket: close 4004: Authentication failed.")
	eq(t, strings.Count(buf.String(), "ERROR: "), 1)
	select {
	case err := <-b.Fatal():
		contains(t, err.Error(), "token")
	default:
		t.Fatal("a revoked token must stop the bot")
	}
}
