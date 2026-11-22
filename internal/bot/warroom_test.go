package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

type call struct {
	op      string // "send", "edit", "delete"
	channel string
	message string
	content string
	files   int
	roles   []string
	footer  string
}

// fakeMsg records calls and answers with IDs m1, m2, ... A scripted error
// for an op is returned once per entry.
type fakeMsg struct {
	mu    sync.Mutex
	calls []call
	next  int
	fail  map[string][]error
}

func restErr(code int) error {
	return &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: code, Message: "test"}}
}

func (f *fakeMsg) failNext(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == nil {
		f.fail = map[string][]error{}
	}
	f.fail[op] = append(f.fail[op], err)
}

func (f *fakeMsg) take(op string) error {
	if errs := f.fail[op]; len(errs) > 0 {
		f.fail[op] = errs[1:]
		return errs[0]
	}
	return nil
}

func (f *fakeMsg) Send(ch string, m *discordgo.MessageSend) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := call{op: "send", channel: ch, content: m.Content, files: len(m.Files)}
	if m.AllowedMentions != nil {
		c.roles = m.AllowedMentions.Roles
	}
	f.calls = append(f.calls, c)
	if err := f.take("send"); err != nil {
		return nil, err
	}
	f.next++
	return &discordgo.Message{ID: fmt.Sprintf("m%d", f.next), ChannelID: ch}, nil
}

func (f *fakeMsg) Edit(m *discordgo.MessageEdit) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := call{op: "edit", channel: m.Channel, message: m.ID, files: len(m.Files)}
	if m.Embeds != nil && len(*m.Embeds) > 0 && (*m.Embeds)[0].Footer != nil {
		c.footer = (*m.Embeds)[0].Footer.Text
	}
	f.calls = append(f.calls, c)
	if err := f.take("edit"); err != nil {
		return nil, err
	}
	return &discordgo.Message{ID: m.ID, ChannelID: m.Channel}, nil
}

func (f *fakeMsg) Delete(ch, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{op: "delete", channel: ch, message: id})
	return f.take("delete")
}

func (f *fakeMsg) reset() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.calls
	f.calls = nil
	return c
}

func ops(cs []call) string {
	var out []string
	for _, c := range cs {
		out = append(out, c.op+" "+c.channel+" "+c.message)
	}
	return fmt.Sprint(out)
}

type roomKit struct {
	w   *WarRoom
	msg *fakeMsg
	dir string
	log *bytes.Buffer
	st  *poller.State
}

func room(t *testing.T) *roomKit {
	t.Helper()
	k := &roomKit{msg: &fakeMsg{}, dir: t.TempDir(), log: &bytes.Buffer{}, st: state(t)}
	log := logx.New(k.log, slog.LevelInfo)
	s, err := store.Open(k.dir, log)
	must(t, err)
	k.w = &WarRoom{Msg: k.msg, Store: s, State: func() *poller.State { return k.st }, Now: func() time.Time { return testNow }, Log: log}
	return k
}

func (k *roomKit) server(t *testing.T, g string) store.Server {
	t.Helper()
	sv, ok := k.w.Store.Get(g)
	if !ok {
		t.Fatalf("server %s not stored", g)
	}
	return sv
}

func TestSetPostsAndSeeds(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", "42"))
	cs := k.msg.reset()
	eq(t, ops(cs), "[send 10 ]")
	eq(t, cs[0].files, 1)
	eq(t, k.server(t, "1"), store.Server{Guild: "1", Channel: "10", Message: "m1", Role: "42", Announced: []int64{1278711574}, Seeded: true})
}

func TestSetNoAccess(t *testing.T) {
	for _, code := range []int{50013, 50001, 10003} {
		k := room(t)
		k.msg.failNext("send", restErr(code))
		if err := k.w.Set("1", "10", ""); !errors.Is(err, ErrNoAccess) {
			t.Fatalf("code %d: %v", code, err)
		}
		if _, ok := k.w.Store.Get("1"); ok {
			t.Fatalf("code %d: stored", code)
		}
	}
	k := room(t)
	k.msg.failNext("send", errors.New("network"))
	if err := k.w.Set("1", "10", ""); err == nil || errors.Is(err, ErrNoAccess) {
		t.Fatalf("plain error: %v", err)
	}
}

func TestSetSameChannelEdits(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", "42"))
	k.msg.reset()
	must(t, k.w.Set("1", "10", ""))
	eq(t, ops(k.msg.reset()), "[edit 10 m1]")
	got := k.server(t, "1")
	eq(t, got.Role, "")
	eq(t, got.Message, "m1")
}

func TestSetSameChannelMessageGone(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", ""))
	k.msg.reset()
	k.msg.failNext("edit", restErr(10008))
	must(t, k.w.Set("1", "10", ""))
	eq(t, ops(k.msg.reset()), "[edit 10 m1 send 10 ]")
	eq(t, k.server(t, "1").Message, "m2")
}

func TestSetMovesChannel(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", ""))
	k.msg.reset()
	must(t, k.w.Set("1", "11", ""))
	eq(t, ops(k.msg.reset()), "[send 11  delete 10 m1]")
	eq(t, k.server(t, "1").Channel, "11")
	eq(t, k.server(t, "1").Message, "m2")

	k.msg.failNext("delete", restErr(10008))
	must(t, k.w.Set("1", "12", ""))
	contains(t, k.log.String(), "WARN: ")
	eq(t, k.server(t, "1").Channel, "12")
}

func TestClear(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", ""))
	k.msg.reset()
	had, err := k.w.Clear("1")
	must(t, err)
	eq(t, had, true)
	eq(t, ops(k.msg.reset()), "[delete 10 m1]")
	eq(t, len(k.w.Store.All()), 0)
	had, err = k.w.Clear("1")
	must(t, err)
	eq(t, had, false)
}

func TestUpdateEditsEveryServer(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true, Announced: []int64{1278711574}}))
	must(t, k.w.Store.Put(store.Server{Guild: "2", Channel: "20", Message: "m8", Seeded: true, Paused: "no access (code 50001)"}))
	k.w.noteTry("2", testNow) // paused just now: skipped until the hourly try
	k.w.Update(k.st)
	cs := k.msg.reset()
	eq(t, ops(cs), "[edit 10 m7]")
	eq(t, cs[0].files, 1)
}

func TestUpdateReplacesDeletedMessage(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true, Announced: []int64{1278711574}}))
	k.msg.failNext("edit", restErr(10008))
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7 send 10 ]")
	eq(t, k.server(t, "1").Message, "m1")
}

func TestUpdatePausesOnLostAccess(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	k.msg.failNext("edit", restErr(50001))
	k.w.Update(k.st)
	if k.server(t, "1").Paused == "" {
		t.Fatal("not paused")
	}
	contains(t, k.log.String(), "WARN: ")
	k.msg.reset()
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[]")
}

func TestUpdateOtherErrorRetries(t *testing.T) {
	k := room(t)
	before := store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true, Announced: []int64{1278711574}}
	must(t, k.w.Store.Put(before))
	k.msg.failNext("edit", errors.New("connection reset"))
	k.w.Update(k.st)
	contains(t, k.log.String(), "WARN: ")
	eq(t, k.server(t, "1"), before)
	k.msg.reset()
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
}

func TestAnnounceOnce(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Role: "42", Seeded: true}))
	k.w.Update(k.st)
	cs := k.msg.reset()
	eq(t, ops(cs), "[edit 10 m7 send 10 ]")
	eq(t, cs[1].content, "<@&42> New orders from Super Earth.")
	eq(t, cs[1].roles, []string{"42"})
	eq(t, k.server(t, "1").Announced, []int64{1278711574})

	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")

	// After a restart the announced list on disk still holds.
	s2, err := store.Open(k.dir, logx.New(k.log, slog.LevelInfo))
	must(t, err)
	w2 := &WarRoom{Msg: k.msg, Store: s2, State: k.w.State, Now: k.w.Now, Log: k.w.Log}
	w2.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
}

func TestAnnounceFailureRetries(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	k.msg.failNext("send", errors.New("network"))
	k.w.Update(k.st)
	contains(t, k.log.String(), "WARN: ")
	eq(t, len(k.server(t, "1").Announced), 0)
	k.msg.reset()
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7 send 10 ]")
	eq(t, k.server(t, "1").Announced, []int64{1278711574})
}

func TestNoAnnounceOnFailedRound(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	failing := &poller.State{Snap: k.st.Snap, Map: k.st.Map, Failing: 1}
	k.w.Update(failing)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
}

func TestUpdateSkipsEndedOrders(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	o := k.st.Snap.Orders[0]
	o.Ends = testNow.Add(-time.Minute)
	ended := &poller.State{Snap: withOrders(k.st.Snap, []war.Order{o}), Map: k.st.Map}
	k.w.Update(ended)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
}

func TestSetBeforeDataSeedsOnFirstRound(t *testing.T) {
	k := room(t)
	data := k.st
	k.st = &poller.State{}
	must(t, k.w.Set("1", "10", ""))
	cs := k.msg.reset()
	eq(t, ops(cs), "[send 10 ]")
	eq(t, cs[0].files, 0)
	eq(t, k.server(t, "1").Seeded, false)

	k.st = data
	k.w.Update(data)
	eq(t, ops(k.msg.reset()), "[edit 10 m1]")
	sv := k.server(t, "1")
	eq(t, sv.Seeded, true)
	eq(t, sv.Announced, []int64{1278711574})
}

func TestForgetKeepsSettingsDuringOutage(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", ""))
	k.w.Forget("1", true)
	k.server(t, "1")
	k.w.Forget("1", false)
	if _, ok := k.w.Store.Get("1"); ok {
		t.Fatal("still stored after removal")
	}
}

func TestLockWaitsAndTimesOut(t *testing.T) {
	k := room(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !k.w.Lock(ctx) {
		t.Fatal("free lock not taken")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	if k.w.Lock(ctx2) {
		t.Fatal("held lock taken twice")
	}
}

func TestAPICode(t *testing.T) {
	eq(t, apiCode(restErr(10008)), 10008)
	eq(t, apiCode(fmt.Errorf("wrapped: %w", restErr(50013))), 50013)
	eq(t, apiCode(errors.New("plain")), 0)
	eq(t, apiCode(&discordgo.RESTError{}), 0)
}

func TestDiskFailureNeverRepeatsAnnouncements(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Role: "42", Seeded: true}))
	servers := filepath.Join(k.dir, "servers")
	must(t, os.Chmod(servers, 0o555))
	defer os.Chmod(servers, 0o755)
	k.msg.failNext("edit", restErr(10008))
	for range 3 {
		k.w.Update(k.st)
	}
	sends := 0
	for _, c := range k.msg.reset() {
		if c.op == "send" {
			sends++
		}
	}
	eq(t, sends, 2) // one repost, one announcement
	eq(t, k.server(t, "1").Message, "m1")
	contains(t, k.log.String(), "ERROR: could not save server settings")
}

// A paused war room gets a fresh message on /warroom set, which proves the
// bot can post there again.
func TestSetAfterPauseReposts(t *testing.T) {
	k := room(t)
	must(t, k.w.Set("1", "10", ""))
	sv := k.server(t, "1")
	sv.Paused = "can't post (code 50013)"
	must(t, k.w.Store.Put(sv))
	k.msg.reset()
	must(t, k.w.Set("1", "10", ""))
	eq(t, ops(k.msg.reset()), "[send 10  delete 10 m1]")
	got := k.server(t, "1")
	eq(t, got.Paused, "")
	eq(t, got.Message, "m2")
}

// When announcements are refused but edits still work, the war room says so
// and pauses until the hourly try, instead of failing quietly every round.
func TestAnnounceNoAccessPausesWithWarning(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Role: "42", Seeded: true}))
	k.msg.failNext("send", restErr(50013))
	k.w.Update(k.st)
	cs := k.msg.reset()
	eq(t, ops(cs), "[edit 10 m7 send 10  edit 10 m7]")
	contains(t, cs[2].footer, "can't post Major Order alerts")
	eq(t, cs[2].files, 1)
	if k.server(t, "1").Paused == "" {
		t.Fatal("not paused")
	}
	contains(t, k.log.String(), "WARN: ")
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[]")
}

// After a restart during an outage there is no data yet: the last good war
// room stays as it is instead of turning back into the placeholder.
func TestUpdateWithoutDataLeavesWarRoom(t *testing.T) {
	k := room(t)
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	k.w.Update(&poller.State{Failing: 3})
	eq(t, ops(k.msg.reset()), "[]")
}

// A paused war room is tried again every hour, so access given back without
// /warroom set still brings it back. A retry that is refused again is not a
// new warning.
func TestPausedServerRetriesHourly(t *testing.T) {
	k := room(t)
	now := testNow
	k.w.Now = func() time.Time { return now }
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true, Announced: []int64{1278711574}}))
	k.msg.failNext("edit", restErr(50001))
	k.w.Update(k.st)
	k.msg.reset()

	now = now.Add(59 * time.Minute)
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[]")

	now = now.Add(time.Minute)
	k.msg.failNext("edit", restErr(50001))
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
	if k.server(t, "1").Paused == "" {
		t.Fatal("a refused retry must stay paused")
	}
	eq(t, strings.Count(k.log.String(), "WARN: "), 1)

	now = now.Add(time.Hour)
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
	eq(t, k.server(t, "1").Paused, "")
	contains(t, k.log.String(), "INFO: war room resumed")
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7]")
}

// A war room paused because alerts were refused tries the alert again after
// an hour, and runs again once it goes through.
func TestBlockedAlertsRetryHourly(t *testing.T) {
	k := room(t)
	now := testNow
	k.w.Now = func() time.Time { return now }
	must(t, k.w.Store.Put(store.Server{Guild: "1", Channel: "10", Message: "m7", Seeded: true}))
	k.msg.failNext("send", restErr(50013))
	k.w.Update(k.st)
	k.msg.reset()

	now = now.Add(time.Hour)
	k.w.Update(k.st)
	eq(t, ops(k.msg.reset()), "[edit 10 m7 send 10 ]")
	sv := k.server(t, "1")
	eq(t, sv.Paused, "")
	eq(t, sv.Announced, []int64{1278711574})
}
