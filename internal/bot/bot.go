package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
	"github.com/bwmarrin/discordgo"
)

// Bot is the connection to Discord.
type Bot struct {
	State func() *poller.State
	Room  *WarRoom
	Now   func() time.Time
	Log   *slog.Logger

	s     *discordgo.Session
	fatal chan error
	dying sync.Once
}

// Fatal delivers the error, at most one, that the bot cannot recover from.
// The caller shuts down and exits with an error, so systemd restarts it
// and the down alerts see it, as with a bad token at startup.
func (b *Bot) Fatal() <-chan error { return b.fatal }

// sessionMessenger sends war-room messages through the Discord session.
type sessionMessenger struct{ s *discordgo.Session }

func (m sessionMessenger) Send(ch string, msg *discordgo.MessageSend) (*discordgo.Message, error) {
	return m.s.ChannelMessageSendComplex(ch, msg)
}

func (m sessionMessenger) Edit(msg *discordgo.MessageEdit) (*discordgo.Message, error) {
	return m.s.ChannelMessageEditComplex(msg)
}

func (m sessionMessenger) Delete(ch, id string) error { return m.s.ChannelMessageDelete(ch, id) }

// New makes the bot. Nothing connects until Start.
func New(token string, p *poller.Poller, st *store.Store, log *slog.Logger) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	s.Identify.Intents = discordgo.IntentsGuilds
	b := &Bot{State: p.Current, Now: time.Now, Log: log, s: s, fatal: make(chan error, 1)}
	discordgo.Logger = func(level, _ int, format string, a ...any) { b.discordLog(level, fmt.Sprintf(format, a...)) }
	b.Room = &WarRoom{Msg: sessionMessenger{s}, Store: st, State: p.Current, Now: time.Now, Log: log}
	s.AddHandler(b.onReady)
	s.AddHandler(b.onInteraction)
	s.AddHandler(b.onGuildDelete)
	return b, nil
}

// tokenRejected is how discordgo's log lines show Discord refusing the
// token: gateway close code 4004. discordgo offers no other hook for it,
// and keeps reconnecting forever on its own.
const tokenRejected = "close 4004"

// discordLog takes discordgo's own log lines. They are mostly gateway
// reconnects it recovers from by itself, so they are warnings here, not
// errors. The one exception is a rejected token, which never recovers.
func (b *Bot) discordLog(level int, msg string) {
	if strings.Contains(msg, tokenRejected) {
		b.dying.Do(func() {
			b.Log.Error("Discord rejected the bot token (gateway close 4004): put a new token in .env and restart")
			select {
			case b.fatal <- errors.New("the bot token was rejected"):
			default:
			}
		})
		return
	}
	switch level {
	case discordgo.LogError, discordgo.LogWarning:
		b.Log.Warn("discord: " + msg)
	case discordgo.LogInformational:
		b.Log.Info("discord: " + msg)
	}
}

// Start connects and registers the commands.
func (b *Bot) Start() error {
	if err := b.s.Open(); err != nil {
		return err
	}
	if _, err := b.s.ApplicationCommandBulkOverwrite(b.s.State.User.ID, "", Commands()); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}
	b.Log.Info("connected to Discord", "user", b.s.State.User.Username)
	return nil
}

// OnRound runs after every poll round.
func (b *Bot) OnRound(st *poller.State) { b.Room.Update(st) }

// Close waits for a running war-room operation (until ctx ends), then
// disconnects.
func (b *Bot) Close(ctx context.Context) error {
	if !b.Room.Lock(ctx) {
		b.Log.Warn("a war-room update was still running at shutdown")
	}
	return b.s.Close()
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	if err := s.UpdateWatchStatus(0, Presence); err != nil {
		b.Log.Warn("could not set the presence", "err", err)
	}
	b.Log.Info("ready", "servers", len(r.Guilds))
}

func (b *Bot) onGuildDelete(_ *discordgo.Session, g *discordgo.GuildDelete) {
	b.Room.Forget(g.ID, g.Unavailable)
}

func (b *Bot) onInteraction(s *discordgo.Session, ic *discordgo.InteractionCreate) {
	i := ic.Interaction
	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		err := s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: b.suggest(i)},
		})
		if err != nil {
			b.Log.Warn("autocomplete reply failed", "err", err)
		}
	case discordgo.InteractionApplicationCommand:
		if later, private := b.deferral(i); later {
			data := &discordgo.InteractionResponseData{}
			if private {
				data.Flags = discordgo.MessageFlagsEphemeral
			}
			err := s.InteractionRespond(i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: data,
			})
			if err != nil {
				b.Log.Warn("deferred reply failed", "command", i.ApplicationCommandData().Name, "err", err)
				return
			}
			if _, err := s.InteractionResponseEdit(i, b.answer(i).WebhookEdit()); err != nil {
				b.Log.Warn("reply failed", "command", i.ApplicationCommandData().Name, "err", err)
			}
			return
		}
		err := s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: b.answer(i).Response(),
		})
		if err != nil {
			b.Log.Warn("reply failed", "command", i.ApplicationCommandData().Name, "err", err)
		}
	}
}
