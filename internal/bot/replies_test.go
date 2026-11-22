package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

func TestBriefing(t *testing.T) {
	st := state(t)
	r := Briefing(st, testNow)
	e := r.Embeds[0]
	eq(t, e.Title, "📡 Galactic War Briefing")
	eq(t, e.Color, 0x4EA3FF)
	eq(t, e.Image.URL, "attachment://map.png")
	eq(t, string(r.Map), "png")
	prefix(t, e.Description, "**🎯 Major Order** · ends <t:1795728744:R>\n")
	contains(t, e.Description, "Eliminate the mandated amount of Troopers")
	contains(t, e.Description, "○ Kill 135,000,000 Troopers\n`▱▱▱▱▱▱▱▱▱▱` 5.2% (7,007,086)")
	eq(t, e.Fields[0].Name, "⚔️ Top Fronts")
	eq(t, strings.Split(e.Fields[0].Value, "\n"), []string{
		"🟥 **Matar Bay** · 0.4% · 10,523 Helldivers",
		"🟨 **Brilliance** · 0.0% · 7,222 Helldivers",
		"🟪 **Heze Bay** · 10.6% · 4,613 Helldivers",
		"🟨 **Luxuriant** · 0.0% · 3,567 Helldivers",
		"🟪 **Alderidge Cove** · Defense 3.6% · 2,347 Helldivers"})
	eq(t, e.Footer.Text, "42,318 Helldivers deployed · Super Earth holds 212 of 273 planets · Updates every 5 minutes")
	eq(t, e.Timestamp, testNow.Format(time.RFC3339))
}

func TestFooterStates(t *testing.T) {
	off, err := sample.Official(testNow)
	must(t, err)
	f := Footer(&poller.State{Snap: off}, testNow)
	if !strings.HasSuffix(f, " · Backup feed") {
		t.Fatalf("official footer: %q", f)
	}
	st := state(t)
	eq(t, Footer(st, testNow.Add(15*time.Minute)), "42,318 Helldivers deployed · Super Earth holds 212 of 273 planets · Updates every 5 minutes")
	eq(t, Footer(st, testNow.Add(25*time.Minute)), "⚠️ War data from 25 minutes ago. The war feeds aren't answering.")
	eq(t, Footer(st, testNow.Add(3*time.Hour+10*time.Minute)), "⚠️ War data from 3 hours ago. The war feeds aren't answering.")
}

func TestBriefingPlaceholder(t *testing.T) {
	r := Briefing(&poller.State{}, testNow)
	e := r.Embeds[0]
	eq(t, e.Title, "📡 Galactic War Briefing")
	eq(t, e.Description, "Gathering war intel. This message fills in within a few minutes.")
	if e.Image != nil || r.Map != nil {
		t.Fatal("placeholder has a picture")
	}
}

func TestBriefingOrders(t *testing.T) {
	st := state(t)
	none := &poller.State{Snap: withOrders(st.Snap, nil), Map: st.Map}
	eq(t, Briefing(none, testNow).Embeds[0].Description, "No Major Order right now. High Command will issue new orders soon.")
	var four []war.Order
	for i := int64(1); i <= 4; i++ {
		o := st.Snap.Orders[0]
		o.ID = i
		four = append(four, o)
	}
	many := &poller.State{Snap: withOrders(st.Snap, four), Map: st.Map}
	eq(t, strings.Count(Briefing(many, testNow).Embeds[0].Description, "**🎯 Major Order**"), 3)
}

func TestBriefingHidesEndedOrder(t *testing.T) {
	st := state(t)
	o := st.Snap.Orders[0]
	o.Ends = testNow.Add(-time.Minute)
	ended := &poller.State{Snap: withOrders(st.Snap, []war.Order{o}), Map: st.Map}
	eq(t, Briefing(ended, testNow).Embeds[0].Description, "No Major Order right now. High Command will issue new orders soon.")
	eq(t, MajorOrder(ended, testNow).Embeds[0].Description, "No Major Order right now. High Command will issue new orders soon.")
}

func TestBriefingFitsLimits(t *testing.T) {
	st := state(t)
	o := st.Snap.Orders[0]
	o.Briefing = strings.Repeat("Democracy needs you. ", 400)
	big := &poller.State{Snap: withOrders(st.Snap, []war.Order{o, o, o}), Map: st.Map}
	for _, r := range []Reply{Briefing(big, testNow), MajorOrder(big, testNow), Announcement(big.Snap, o, "")} {
		for _, e := range r.Embeds {
			if n := len([]rune(e.Description)); n > 4096 {
				t.Fatalf("%q description is %d runes", e.Title, n)
			}
			for _, f := range e.Fields {
				if n := len([]rune(f.Value)); n > 1024 || n == 0 {
					t.Fatalf("field %q is %d runes", f.Name, n)
				}
			}
		}
	}
}

func TestEditReplacesMap(t *testing.T) {
	m := Briefing(state(t), testNow).Edit("10", "20")
	eq(t, m.Channel, "10")
	eq(t, m.ID, "20")
	if m.Attachments == nil || len(*m.Attachments) != 0 {
		t.Fatalf("attachments: %v", m.Attachments)
	}
	// An empty attachments list plus a new file swaps the map only under API
	// v9. Under v10 the new file must be listed too (see Reply.Edit).
	if discordgo.APIVersion != "9" {
		t.Fatalf("discordgo now speaks API v%s: list the new map in Reply.Edit's attachments, then update this check", discordgo.APIVersion)
	}
	if len(m.Files) != 1 || m.Files[0].Name != "map.png" || m.Files[0].ContentType != "image/png" {
		t.Fatalf("files: %v", m.Files)
	}
	if m.Content == nil || *m.Content != "" {
		t.Fatal("content must be set to empty")
	}
	if m.AllowedMentions == nil || m.AllowedMentions.Parse == nil || len(m.AllowedMentions.Parse) != 0 || len(m.AllowedMentions.Roles) != 0 {
		t.Fatalf("allowed mentions: %+v", m.AllowedMentions)
	}
	// Each call reads the map afresh.
	m2 := Briefing(state(t), testNow).Edit("10", "20")
	b1 := make([]byte, 3)
	m.Files[0].Reader.Read(b1)
	b2 := make([]byte, 3)
	m2.Files[0].Reader.Read(b2)
	eq(t, string(b1)+string(b2), "pngpng")
}

func TestPingOnlyTheRole(t *testing.T) {
	st := state(t)
	m := Announcement(st.Snap, st.Snap.Orders[0], "42").Send()
	eq(t, m.Content, "<@&42> New orders from Super Earth.")
	eq(t, m.AllowedMentions.Roles, []string{"42"})
	eq(t, len(m.AllowedMentions.Parse), 0)
	m = Announcement(st.Snap, st.Snap.Orders[0], "").Send()
	eq(t, m.Content, "New orders from Super Earth.")
	eq(t, len(m.AllowedMentions.Roles), 0)
	if m.AllowedMentions.Parse == nil {
		t.Fatal("parse must be an empty list, not null")
	}
}

func TestAnnouncement(t *testing.T) {
	st := state(t)
	e := Announcement(st.Snap, st.Snap.Orders[0], "").Embeds[0]
	eq(t, e.Title, "📣 New Major Order")
	eq(t, e.Color, 0xF2C230)
	eq(t, e.Description, st.Snap.Orders[0].Briefing)
	eq(t, e.Fields[0].Name, "Task 1")
	eq(t, e.Fields[0].Value, "Kill 135,000,000 Troopers")
	eq(t, e.Fields[0].Inline, true)
	eq(t, e.Fields[3].Name, "Task 4")
	eq(t, e.Fields[3].Value, "Kill 250,000,000 Terminids")
	eq(t, field(t, e.Fields, "Reward"), "45 Medals")
	eq(t, field(t, e.Fields, "Ends"), "<t:1795728744:f>, <t:1795728744:R>")
	eq(t, e.Footer.Text, "Use /major-order for live progress")
}

func TestMajorOrder(t *testing.T) {
	st := state(t)
	r := MajorOrder(st, testNow)
	e := r.Embeds[0]
	eq(t, e.Title, "🎯 Major Order")
	eq(t, e.Color, 0xF2C230)
	eq(t, r.Private, false)
	o := st.Snap.Orders[0]
	eq(t, field(t, e.Fields, "Task 1"), st.Snap.Line(o.Tasks[0], 1))
	eq(t, field(t, e.Fields, "Reward"), "45 Medals")
	eq(t, field(t, e.Fields, "Ends"), "<t:1795728744:f>, <t:1795728744:R>")
	none := MajorOrder(&poller.State{Snap: withOrders(st.Snap, nil)}, testNow)
	eq(t, len(none.Embeds), 1)
	eq(t, none.Embeds[0].Description, "No Major Order right now. High Command will issue new orders soon.")
}

func TestPlanetInfo(t *testing.T) {
	st := state(t)
	e := PlanetInfo(st, testNow, st.Snap.Planet(273)).Embeds[0]
	eq(t, e.Title, "🪐 Alderidge Cove")
	eq(t, e.Description, "Omega Sector")
	eq(t, e.Color, 0x4EA3FF)
	eq(t, field(t, e.Fields, "Held By"), "🟦 Super Earth")
	eq(t, field(t, e.Fields, "Defense"), "Illuminate\n`▱▱▱▱▱▱▱▱▱▱` 3.6%\nends <t:1795385236:R>")
	eq(t, field(t, e.Fields, "Helldivers"), "2,347")
	eq(t, field(t, e.Fields, "Biome"), "Deadlands")
	eq(t, field(t, e.Fields, "Supply Lines"), "Bellatrix, Botein, Elysian Meadows")
	for _, f := range e.Fields {
		if f.Name == "Hazards" || f.Name == "Liberation" {
			t.Fatalf("unexpected field %q", f.Name)
		}
	}
	var names []string
	for _, f := range e.Fields {
		names = append(names, f.Name)
	}
	eq(t, fmt.Sprint(names), "[Held By Defense Helldivers Biome Supply Lines]")
	eq(t, e.Footer.Text, Footer(st, testNow))

	mb := PlanetInfo(st, testNow, st.Snap.Planet(200)).Embeds[0]
	eq(t, field(t, mb.Fields, "Liberation"), "`▱▱▱▱▱▱▱▱▱▱` 0.4%")
	eq(t, mb.Color, 0xFF5A4E)

	ctl := war.Order{ID: 7, Ends: testNow.Add(time.Hour), Tasks: []war.Task{{Type: 13, Values: map[int]int64{3: 1, 11: 1, 12: 7}}}}
	tgt := &poller.State{Snap: withOrders(st.Snap, []war.Order{ctl}), Map: st.Map}
	eq(t, PlanetInfo(tgt, testNow, tgt.Snap.Planet(7)).Embeds[0].Description, "Ferris Sector · 🎯 Major Order target")
	zr := PlanetInfo(tgt, testNow, tgt.Snap.Planet(7)).Embeds[0]
	eq(t, field(t, zr.Fields, "Hazards"), "Tremors")
	// An order that has ended no longer marks its target.
	ctl.Ends = testNow.Add(-time.Minute)
	old := &poller.State{Snap: withOrders(st.Snap, []war.Order{ctl}), Map: st.Map}
	eq(t, PlanetInfo(old, testNow, old.Snap.Planet(7)).Embeds[0].Description, "Ferris Sector")
}

// /help ends the way Sabacc Droid's and Aurebesh Droid's do: the credits,
// More Star Wars and Made By fields, link buttons to add the bot, the
// Datapad app and GitHub, and the disclaimer as its footer.
func TestHelp(t *testing.T) {
	r := Help("42")
	eq(t, r.Private, false)
	e := r.Embeds[0]
	eq(t, e.Title, "📖 501st War Room")
	var names []string
	for _, f := range e.Fields {
		names = append(names, f.Name)
	}
	eq(t, names, []string{"📡 War Room", "🧭 More Commands", "🛰️ War Data", "🌌 More Star Wars", "🛠️ Made By"})
	for _, cmd := range []string{"`/warroom set`", "`/warroom clear`", "`/map`", "`/major-order`", "`/planet`", "`/help`"} {
		contains(t, field(t, e.Fields, "📡 War Room")+field(t, e.Fields, "🧭 More Commands"), cmd)
	}
	contains(t, field(t, e.Fields, "🛰️ War Data"), "https://github.com/helldivers-2/api")
	more := field(t, e.Fields, "🌌 More Star Wars")
	for _, link := range []string{SabaccDroidLink, AurebeshDroidLink, DatapadApp, TranslatorApp} {
		contains(t, more, link)
	}
	prefix(t, field(t, e.Fields, "🛠️ Made By"), "Created by **"+Creator+"** with ")
	eq(t, e.Footer.Text, Disclaimer)
	eq(t, len(r.Components), 1)
	type link struct{ emoji, label, url string }
	var got []link
	for _, c := range r.Components[0].(discordgo.ActionsRow).Components {
		b := c.(discordgo.Button)
		eq(t, b.Style, discordgo.LinkButton)
		got = append(got, link{b.Emoji.Name, b.Label, b.URL})
	}
	eq(t, got, []link{{"➕", "Add 501st War Room", InviteLink("42")}, {"📱", "Datapad App", DatapadApp}, {"📂", "GitHub", GitHubLink}})
}

// Every embed carries the logo as its thumbnail, like the other two bots.
func TestEveryEmbedHasLogo(t *testing.T) {
	st := state(t)
	s := st.Snap
	o := war.Order{ID: 7, Briefing: "Hold the line.", Ends: testNow.Add(time.Hour), Reward: "50 Medals",
		Tasks: []war.Task{{Type: 13, Values: map[int]int64{3: 1, 11: 1, 12: 7}}}}
	two := &poller.State{Snap: withOrders(s, []war.Order{o, o}), Map: st.Map}
	replies := map[string]Reply{
		"briefing":             Briefing(st, testNow),
		"briefing placeholder": Briefing(&poller.State{}, testNow),
		"map":                  MapReply(st, testNow),
		"major order":          MajorOrder(two, testNow),
		"no major order":       MajorOrder(&poller.State{Snap: withOrders(s, nil)}, testNow),
		"announcement":         Announcement(s, o, ""),
		"planet":               PlanetInfo(st, testNow, s.Planet(7)),
		"help":                 Help("42"),
	}
	for name, r := range replies {
		if len(r.Embeds) == 0 {
			t.Fatalf("%s: no embeds", name)
		}
		for i, e := range r.Embeds {
			if e.Thumbnail == nil || e.Thumbnail.URL != Logo {
				t.Errorf("%s: embed %d has thumbnail %#v, want the logo", name, i+1, e.Thumbnail)
			}
		}
	}
}

func TestNoticeIsPrivate(t *testing.T) {
	r := Notice(NoticeGathering)
	eq(t, r.Private, true)
	eq(t, r.Content, "❌ Gathering war intel. Try again in a minute.")
	eq(t, r.Response().Flags, discordgo.MessageFlagsEphemeral)
	eq(t, Briefing(state(t), testNow).Response().Flags, discordgo.MessageFlags(0))
}

func TestDataRepliesBeforeFirstRound(t *testing.T) {
	for _, r := range []Reply{MapReply(&poller.State{}, testNow), MajorOrder(&poller.State{}, testNow), PlanetInfo(&poller.State{}, testNow, nil)} {
		eq(t, r.Content, NoticeGathering)
		eq(t, r.Private, true)
	}
}

func TestMapReply(t *testing.T) {
	st := state(t)
	r := MapReply(st, testNow)
	e := r.Embeds[0]
	eq(t, e.Title, "🗺️ Galactic War Map")
	eq(t, e.Image.URL, "attachment://map.png")
	eq(t, e.Footer.Text, Footer(st, testNow))
	eq(t, len(r.Response().Files), 1)
	eq(t, len(r.WebhookEdit().Files), 1)
}

// Discord refuses a message whose embeds pass 6000 characters in all: three
// orders of nine long tasks still fit, with every task line whole.
func TestMajorOrderFitsDiscord(t *testing.T) {
	st := state(t)
	long := war.Task{Type: 7, Progress: 500000, Values: map[int]int64{war.VTFaction: 4, war.VTGoal: 1000000,
		war.VTDifficulty: 10, war.VTLocationType: 1, war.VTLocationIndex: 7}}
	o := war.Order{ID: 1, Ends: testNow.Add(time.Hour), Briefing: strings.Repeat("Democracy needs you. ", 200), Reward: "45 Medals"}
	for range 9 {
		o.Tasks = append(o.Tasks, long)
	}
	r := MajorOrder(&poller.State{Snap: withOrders(st.Snap, []war.Order{o, o, o})}, testNow)
	total := 0
	for _, e := range r.Embeds {
		total += runes(e.Title) + runes(e.Description)
		for _, f := range e.Fields {
			total += runes(f.Name) + runes(f.Value)
		}
		if e.Footer != nil {
			total += runes(e.Footer.Text)
		}
	}
	if total > 6000 {
		t.Fatalf("embeds total %d characters", total)
	}
	eq(t, len(r.Embeds), 3)
	eq(t, field(t, r.Embeds[2].Fields, "Task 9"), st.Snap.Line(long, 9))
	prefix(t, r.Embeds[0].Description, "Democracy needs you.")
}

// With data but no picture yet (the first draw failed), replies leave the
// image out instead of pointing at a missing file.
func TestRepliesWithoutMap(t *testing.T) {
	st := &poller.State{Snap: community(t)}
	b := Briefing(st, testNow)
	if b.Embeds[0].Image != nil {
		t.Fatal("the briefing points at a map it does not attach")
	}
	eq(t, len(b.Send().Files), 0)
	eq(t, MapReply(st, testNow).Content, NoticeNoMap)
	eq(t, MapReply(st, testNow).Private, true)
}
