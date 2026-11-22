// Package selftest pushes the captured API replies through the whole bot,
// without Discord or the network, and checks every reply against Discord's
// limits and the reply conventions (spec 13). CI runs it before deploying.
package selftest

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/bot"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// The fixtures were captured at this time.
var now = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

const (
	maxMapBytes = 1 << 20
	maxMapTime  = time.Second
)

// Run runs every check, prints one line per problem and a summary, and
// returns the number of problems.
func Run(w io.Writer) int {
	var problems []string
	add := func(p ...string) { problems = append(problems, p...) }
	defer func() {
		for _, p := range problems {
			fmt.Fprintln(w, p)
		}
		if len(problems) == 0 {
			fmt.Fprintln(w, "All checks passed")
		} else {
			fmt.Fprintf(w, "%d problems\n", len(problems))
		}
	}()

	comm, err := sample.Community(now)
	if err != nil {
		add("community fixtures: " + err.Error())
		return len(problems)
	}
	off, err := sample.Official(now)
	if err != nil {
		add("official fixtures: " + err.Error())
		return len(problems)
	}
	old, err := sample.OfficialWith(now, "official-assignment-2026-11-19.json")
	if err != nil {
		add("2026-11-19 fixtures: " + err.Error())
		return len(problems)
	}
	for _, s := range []*war.Snapshot{comm, off} {
		if err := source.Check(s); err != nil {
			add(s.Source + " snapshot fails the sanity check: " + err.Error())
		}
	}
	add(agree(comm, off)...)
	for _, s := range []*war.Snapshot{comm, old} {
		for _, o := range s.Orders {
			for i, t := range o.Tasks {
				if strings.HasPrefix(s.Describe(t, i+1).Text, "Objective") {
					add(fmt.Sprintf("order %d task %d (type %d) has no text", o.ID, i+1, t.Type))
				}
			}
		}
	}

	start := time.Now()
	img, err := render.Map(comm)
	took := time.Since(start)
	switch {
	case err != nil:
		add("map: " + err.Error())
	case len(img) >= maxMapBytes:
		add(fmt.Sprintf("map is %d bytes (limit %d)", len(img), maxMapBytes))
	case took >= maxMapTime && !raceBuild:
		add(fmt.Sprintf("map took %v to draw (limit %v)", took.Round(time.Millisecond), maxMapTime))
	}

	add(replies(comm, off, old, img)...)
	add(CheckCommands(bot.Commands())...)
	add(autocomplete(comm)...)
	add(storeRoundTrip()...)
	return len(problems)
}

// agree checks that both sources draw the same map.
func agree(a, b *war.Snapshot) []string {
	if len(a.Planets) != len(b.Planets) {
		return []string{fmt.Sprintf("sources disagree: %d planets and %d planets", len(a.Planets), len(b.Planets))}
	}
	for _, p := range a.Planets {
		q := b.Planet(p.Index)
		if q == nil || q.X != p.X || q.Y != p.Y || !slices.Equal(q.Waypoints, p.Waypoints) {
			return []string{fmt.Sprintf("sources disagree on planet %d", p.Index)}
		}
	}
	if !slices.Equal(a.Pairs(), b.Pairs()) {
		return []string{"sources disagree on supply lines"}
	}
	return nil
}

func replies(comm, off, old *war.Snapshot, img []byte) []string {
	st := &poller.State{Snap: comm, Map: img}
	o := comm.Orders[0]
	var three []war.Order
	for i := range 3 {
		c := o
		c.ID = int64(i + 1)
		three = append(three, c)
	}
	with := func(orders []war.Order) *poller.State {
		return &poller.State{Snap: war.NewSnapshot(comm.Fetched, comm.Source, comm.Planets, orders), Map: img}
	}
	oldSt := &poller.State{Snap: old, Map: img}
	cases := map[string]bot.Reply{
		"briefing":                 bot.Briefing(st, now),
		"briefing, backup feed":    bot.Briefing(&poller.State{Snap: off, Map: img}, now),
		"briefing, stale":          bot.Briefing(st, now.Add(25*time.Minute)),
		"briefing, placeholder":    bot.Briefing(&poller.State{}, now),
		"briefing, no order":       bot.Briefing(with(nil), now),
		"briefing, three orders":   bot.Briefing(with(three), now),
		"briefing, 2026-11-19":     bot.Briefing(oldSt, now),
		"/map":                     bot.MapReply(st, now),
		"/major-order":             bot.MajorOrder(st, now),
		"/major-order, 2026-11-19": bot.MajorOrder(oldSt, now),
		"/major-order, three":      bot.MajorOrder(with(three), now),
		"/major-order, none":       bot.MajorOrder(with(nil), now),
		"announcement":             bot.Announcement(comm, o, ""),
		"announcement, role":       bot.Announcement(comm, o, "123456789012345678"),
		"announcement, 2026-11-19": bot.Announcement(old, old.Orders[0], "123456789012345678"),
		"/help":                    bot.Help("123456789012345678"),
	}
	channel, role := "<#123456789012345678>", "<@&123456789012345678>"
	for name, text := range map[string]string{
		"NoticeGathering": bot.NoticeGathering,
		"NoticeBroken":    bot.NoticeBroken,
		"NoticeNoMap":     bot.NoticeNoMap,
		"NoticeNoPlanet":  fmt.Sprintf(bot.NoticeNoPlanet, strings.Repeat("x", 100)),
		"NoticeNoAccess":  fmt.Sprintf(bot.NoticeNoAccess, channel),
		"NoticeSet":       fmt.Sprintf(bot.NoticeSet, channel) + fmt.Sprintf(bot.NoticeSetRole, role) + fmt.Sprintf(bot.NoticeRoleHeadsUp, role),
		"NoticeCleared":   bot.NoticeCleared,
		"NoticeNoWarRoom": bot.NoticeNoWarRoom,
	} {
		cases[name] = bot.Notice(text)
	}
	var p []string
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range cases {
			if !yield(k) {
				return
			}
		}
	}) {
		p = append(p, CheckReply(name, cases[name])...)
	}
	for _, s := range []*war.Snapshot{comm, old} {
		pst := &poller.State{Snap: s, Map: img}
		for i := range s.Planets {
			pl := &s.Planets[i]
			p = append(p, CheckReply("/planet "+pl.Name, bot.PlanetInfo(pst, now, pl))...)
		}
	}
	return p
}

func autocomplete(s *war.Snapshot) []string {
	typed := map[string]bool{"": true, "a": true, "e": true}
	for _, p := range s.Planets {
		if r := []rune(p.Name); len(r) > 0 {
			typed[strings.ToLower(string(r[0]))] = true
		}
	}
	var p []string
	for t := range typed {
		p = append(p, checkChoices(t, bot.Suggest(s, t))...)
	}
	slices.Sort(p)
	return p
}

func storeRoundTrip() []string {
	dir, err := os.MkdirTemp("", "warroom-selftest-*")
	if err != nil {
		return []string{"store: " + err.Error()}
	}
	defer os.RemoveAll(dir)
	log := logx.New(io.Discard, slog.LevelInfo)
	s, err := store.Open(dir, log)
	if err != nil {
		return []string{"store: " + err.Error()}
	}
	want := store.Server{Guild: "1", Channel: "2", Message: "3", Role: "4", Announced: []int64{1278711574}, Seeded: true}
	if err := s.Put(want); err != nil {
		return []string{"store: " + err.Error()}
	}
	again, err := store.Open(dir, log)
	if err != nil {
		return []string{"store: " + err.Error()}
	}
	got, ok := again.Get("1")
	if !ok || fmt.Sprint(got) != fmt.Sprint(want) {
		return []string{fmt.Sprintf("store round trip gave %+v", got)}
	}
	return nil
}
