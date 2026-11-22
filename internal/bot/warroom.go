package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
	"github.com/bwmarrin/discordgo"
)

// Messenger is the part of Discord the war room uses.
type Messenger interface {
	Send(channelID string, m *discordgo.MessageSend) (*discordgo.Message, error)
	Edit(m *discordgo.MessageEdit) (*discordgo.Message, error)
	Delete(channelID, messageID string) error
}

// ErrNoAccess means Discord refused to let the bot post in the channel.
var ErrNoAccess = errors.New("no access to the channel")

// Discord error codes the war room acts on.
const (
	codeUnknownChannel = 10003
	codeUnknownMessage = 10008
	codeMissingAccess  = 50001
	codeMissingPerms   = 50013
)

func apiCode(err error) int {
	var re *discordgo.RESTError
	if errors.As(err, &re) && re.Message != nil {
		return re.Message.Code
	}
	return 0
}

func noAccess(code int) bool {
	return code == codeMissingAccess || code == codeMissingPerms || code == codeUnknownChannel
}

// retryPaused is how often a paused war room is tried again.
const retryPaused = time.Hour

// WarRoom keeps each server's war-room message up to date and announces
// new Major Orders (spec 4.1, 4.2 and 11). One operation runs at a time.
type WarRoom struct {
	Msg   Messenger
	Store *store.Store
	State func() *poller.State
	Now   func() time.Time
	Log   *slog.Logger

	mu    sync.Mutex
	tried map[string]time.Time // guild -> when it was paused or last tried again
}

// seed marks every running order as announced, so setting up the war room
// never announces an order that is already running.
func (w *WarRoom) seed(sv *store.Server, st *poller.State) {
	if st.Snap == nil {
		return
	}
	for _, o := range st.Snap.Active(w.Now()) {
		sv.MarkAnnounced(o.ID)
	}
	sv.Seeded = true
}

// Set puts the war room in channel, pinging role for new orders. It posts
// the message (or edits the one already there), then saves the settings.
func (w *WarRoom) Set(guild, channel, role string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.State()
	r := Briefing(st, w.Now())
	old, had := w.Store.Get(guild)

	msgID, gone := "", false
	// A paused war room gets a fresh message, which proves the bot can post
	// in the channel again (an edit can work while new posts are refused).
	if had && old.Channel == channel && old.Message != "" && old.Paused == "" {
		_, err := w.Msg.Edit(r.Edit(channel, old.Message))
		switch code := apiCode(err); {
		case err == nil:
			msgID = old.Message
		case code == codeUnknownMessage:
			gone = true
		case noAccess(code):
			return ErrNoAccess
		default:
			return err
		}
	}
	if msgID == "" {
		m, err := w.Msg.Send(channel, r.Send())
		if err != nil {
			if noAccess(apiCode(err)) {
				return ErrNoAccess
			}
			return err
		}
		msgID = m.ID
	}

	sv := store.Server{Guild: guild, Channel: channel, Message: msgID, Role: role}
	if had {
		sv.Announced, sv.Seeded = old.Announced, old.Seeded
	}
	if !sv.Seeded {
		w.seed(&sv, st)
	}
	if err := w.Store.Put(sv); err != nil {
		return err
	}
	if had && old.Message != "" && old.Message != msgID && !gone {
		if err := w.Msg.Delete(old.Channel, old.Message); err != nil {
			w.Log.Warn("could not delete the old war-room message", "guild", guild, "channel", old.Channel, "err", err)
		}
	}
	delete(w.tried, guild)
	w.Log.Info("war room set", "guild", guild, "channel", channel)
	return nil
}

// Clear turns the war room off: it deletes the message (best effort) and
// the settings. It reports whether there was a war room.
func (w *WarRoom) Clear(guild string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sv, ok := w.Store.Get(guild)
	if !ok {
		return false, nil
	}
	if sv.Message != "" {
		if err := w.Msg.Delete(sv.Channel, sv.Message); err != nil {
			w.Log.Warn("could not delete the war-room message", "guild", guild, "err", err)
		}
	}
	if err := w.Store.Delete(guild); err != nil {
		return true, err
	}
	delete(w.tried, guild)
	w.Log.Info("war room cleared", "guild", guild)
	return true, nil
}

// Update runs after every poll round: it edits every running war room and,
// after a good round, announces orders each server has not seen. A paused
// war room is tried again once an hour, and runs again when that works.
func (w *WarRoom) Update(st *poller.State) {
	if st.Snap == nil {
		return // no data since the restart: keep the last good war rooms as they are
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.Now()
	r := Briefing(st, now)
	for _, sv := range w.Store.All() {
		retry := sv.Paused != ""
		if retry && !w.due(sv.Guild, now) {
			continue
		}
		if !w.refresh(&sv, r, retry) {
			continue
		}
		if st.Failing == 0 {
			w.announce(&sv, st, now, retry)
		}
		if retry && sv.Paused == "" {
			w.Log.Info("war room resumed", "guild", sv.Guild)
		}
	}
}

// due reports whether a paused war room is due another try, and if so notes
// the try. After a restart, each paused war room is tried on the first
// round with data.
func (w *WarRoom) due(guild string, now time.Time) bool {
	if last, ok := w.tried[guild]; ok && now.Sub(last) < retryPaused {
		return false
	}
	w.noteTry(guild, now)
	return true
}

func (w *WarRoom) noteTry(guild string, now time.Time) {
	if w.tried == nil {
		w.tried = map[string]time.Time{}
	}
	w.tried[guild] = now
}

// refresh edits one server's message, reposting it if it was deleted. It
// reports whether the server is running. A paused server being tried again
// (retry) runs again when the edit goes through.
func (w *WarRoom) refresh(sv *store.Server, r Reply, retry bool) bool {
	_, err := w.Msg.Edit(r.Edit(sv.Channel, sv.Message))
	code := apiCode(err)
	if err != nil && code == codeUnknownMessage {
		var m *discordgo.Message
		if m, err = w.Msg.Send(sv.Channel, r.Send()); err == nil {
			sv.Message, sv.Paused = m.ID, ""
			w.save(*sv)
			w.Log.Info("war-room message was deleted, posted a new one", "guild", sv.Guild)
			return true
		}
		code = apiCode(err)
	}
	switch {
	case err == nil:
		if retry {
			sv.Paused = ""
			w.save(*sv)
		}
		return true
	case noAccess(code):
		w.pause(sv, fmt.Sprintf("no access (code %d)", code), retry, "lost access to the war-room channel", "channel", sv.Channel, "code", code)
		return false
	case retry:
		w.Log.Info("paused war room still failing, trying again in an hour", "guild", sv.Guild, "err", err)
		return false
	default:
		w.Log.Warn("war-room edit failed, trying next round", "guild", sv.Guild, "err", err)
		return true
	}
}

// pause stops a server's updates until the next hourly try. The first pause
// logs a warning; a try that is refused again only logs an info line.
func (w *WarRoom) pause(sv *store.Server, reason string, retry bool, msg string, args ...any) {
	if sv.Paused != reason {
		sv.Paused = reason
		w.save(*sv)
	}
	w.noteTry(sv.Guild, w.Now())
	args = append([]any{"guild", sv.Guild}, args...)
	if retry {
		w.Log.Info(msg+", still paused", args...)
		return
	}
	w.Log.Warn(msg+", paused, trying again hourly", args...)
}

// announce posts each running order the server has not seen, saving after
// each one so a crash never posts one twice.
func (w *WarRoom) announce(sv *store.Server, st *poller.State, now time.Time, retry bool) {
	if !sv.Seeded {
		w.seed(sv, st)
		w.save(*sv)
		return
	}
	for _, o := range st.Snap.Active(now) {
		if sv.WasAnnounced(o.ID) {
			continue
		}
		if _, err := w.Msg.Send(sv.Channel, Announcement(st.Snap, o, sv.Role).Send()); err != nil {
			if code := apiCode(err); noAccess(code) {
				w.blocked(sv, st, now, code, retry)
				return
			}
			w.Log.Warn("Major Order announcement failed, trying next round", "guild", sv.Guild, "order", o.ID, "err", err)
			return
		}
		sv.MarkAnnounced(o.ID)
		w.save(*sv)
		w.Log.Info("announced a new Major Order", "guild", sv.Guild, "order", o.ID)
	}
}

// blocked handles a channel where the bot can edit its message but not
// post: it says so in the war room, so the admins see it, and pauses.
func (w *WarRoom) blocked(sv *store.Server, st *poller.State, now time.Time, code int, retry bool) {
	r := Briefing(st, now)
	r.Embeds[0].Footer = &discordgo.MessageEmbedFooter{Text: textAlertsBlocked}
	if _, err := w.Msg.Edit(r.Edit(sv.Channel, sv.Message)); err != nil {
		w.Log.Warn("could not show the alerts warning", "guild", sv.Guild, "err", err)
	}
	w.pause(sv, fmt.Sprintf("can't post alerts (code %d)", code), retry, "can't post Major Order alerts in the war-room channel", "channel", sv.Channel, "code", code)
}

func (w *WarRoom) save(sv store.Server) {
	if err := w.Store.Put(sv); err != nil {
		w.Log.Error("could not save server settings", "guild", sv.Guild, "err", err)
	}
}

// Forget drops a server the bot was removed from. During a Discord outage
// (unavailable) the settings stay.
func (w *WarRoom) Forget(guild string, unavailable bool) {
	if unavailable {
		w.Log.Info("server unavailable, keeping its war room", "guild", guild)
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.Store.Get(guild); !ok {
		return
	}
	if err := w.Store.Delete(guild); err != nil {
		w.Log.Error("could not delete server settings", "guild", guild, "err", err)
		return
	}
	delete(w.tried, guild)
	w.Log.Info("removed from a server, deleted its war room", "guild", guild)
}

// Lock waits for any running war-room operation and then holds the lock
// for good, so nothing posts during shutdown. It returns false if ctx ends
// first.
func (w *WarRoom) Lock(ctx context.Context) bool {
	got := make(chan struct{})
	go func() {
		w.mu.Lock()
		close(got)
	}()
	select {
	case <-got:
		return true
	case <-ctx.Done():
		return false
	}
}
