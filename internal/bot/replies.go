package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

// staleAfter is how old data can be before footers warn about it.
const staleAfter = 15 * time.Minute

// Notice is a private one-line reply.
func Notice(text string) Reply { return Reply{Content: text, Private: true} }

func stamp(t time.Time, style string) string {
	return "<t:" + strconv.FormatInt(t.Unix(), 10) + ":" + style + ">"
}

// Footer is the line under every war reply: the war at a glance, or a
// warning when the data is old.
func Footer(st *poller.State, now time.Time) string {
	s := st.Snap
	if s == nil {
		return textUpdates
	}
	if age := now.Sub(s.Fetched); age > staleAfter {
		ago := fmt.Sprintf("%d minutes", int(age.Minutes()))
		if age >= 2*time.Hour {
			ago = fmt.Sprintf("%d hours", int(age.Hours()))
		}
		return "⚠️ War data from " + ago + " ago. The war feeds aren't answering."
	}
	held, total := s.Held()
	f := war.Comma(int64(s.Players())) + " Helldivers deployed · Super Earth holds " +
		strconv.Itoa(held) + " of " + strconv.Itoa(total) + " planets · " + textUpdates
	if s.Source == "official" {
		f += " · " + textBackup
	}
	return f
}

func footed(e *discordgo.MessageEmbed, st *poller.State, now time.Time) *discordgo.MessageEmbed {
	e.Footer = &discordgo.MessageEmbedFooter{Text: Footer(st, now)}
	if st.Snap != nil {
		e.Timestamp = st.Snap.Fetched.UTC().Format(time.RFC3339)
	}
	return e
}

// embed starts an embed in the house style the other two bots share: a
// colored bar and the logo as its thumbnail. Every embed starts here.
func embed(title string, color int) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{Title: title, Color: color, Thumbnail: &discordgo.MessageEmbedThumbnail{URL: Logo}}
}

func mapImage() *discordgo.MessageEmbedImage {
	return &discordgo.MessageEmbedImage{URL: "attachment://" + mapFile}
}

// Briefing is the war-room message (spec 4.1).
func Briefing(st *poller.State, now time.Time) Reply {
	e := embed(titled(IconBriefing, "Galactic War Briefing"), ColorSuperEarth)
	s := st.Snap
	if s == nil {
		e.Description = textGathering
		e.Footer = &discordgo.MessageEmbedFooter{Text: textUpdates}
		return Reply{Embeds: []*discordgo.MessageEmbed{e}}
	}
	orders := s.Active(now)
	if len(orders) > maxBriefOrders {
		orders = orders[:maxBriefOrders]
	}
	if len(orders) == 0 {
		e.Description = textNoOrder
	} else {
		// Share the description fairly so every order shows.
		per := (maxDescription - 2*(len(orders)-1)) / len(orders)
		blocks := make([]string, len(orders))
		for i, o := range orders {
			blocks[i] = orderBlock(s, o, per)
		}
		e.Description = strings.Join(blocks, "\n\n")
	}
	e.Fields = []*discordgo.MessageEmbedField{{Name: titled(IconFronts, "Top Fronts"), Value: fronts(s)}}
	if len(st.Map) > 0 {
		e.Image = mapImage()
	}
	return Reply{Embeds: []*discordgo.MessageEmbed{footed(e, st, now)}, Map: st.Map}
}

// orderBlock is one order in the briefing, at most max runes: the header,
// the briefing (cut first) and the task lines.
func orderBlock(s *war.Snapshot, o war.Order, max int) string {
	head := "**" + titled(IconOrder, "Major Order") + "** · ends " + stamp(o.Ends, "R")
	var lines []string
	for i, t := range o.Tasks {
		lines = append(lines, s.Line(t, i+1))
	}
	tasks := strings.Join(lines, "\n")
	room := max - runes(head) - runes(tasks) - 2
	parts := []string{head}
	if o.Briefing != "" && room > minBriefing {
		parts = append(parts, war.Cut(o.Briefing, room))
	}
	if tasks != "" {
		parts = append(parts, tasks)
	}
	return war.Cut(strings.Join(parts, "\n"), max)
}

func runes(s string) int { return len([]rune(s)) }

func fronts(s *war.Snapshot) string {
	var lines []string
	for _, p := range s.Fronts(5) {
		pct := war.Pct(p.Liberation())
		if p.Defense != nil {
			pct = "Defense " + war.Pct(p.DefenseProgress())
		}
		lines = append(lines, p.Enemy().Square()+" **"+p.Name+"** · "+pct+" · "+war.Comma(int64(p.Players))+" Helldivers")
	}
	if len(lines) == 0 {
		return textNoFronts
	}
	return war.Cut(strings.Join(lines, "\n"), maxFieldValue)
}

// MapReply answers /map.
func MapReply(st *poller.State, now time.Time) Reply {
	if st.Snap == nil {
		return Notice(NoticeGathering)
	}
	if len(st.Map) == 0 {
		return Notice(NoticeNoMap)
	}
	e := embed(titled(IconMap, "Galactic War Map"), ColorSuperEarth)
	e.Image = mapImage()
	return Reply{Embeds: []*discordgo.MessageEmbed{footed(e, st, now)}, Map: st.Map}
}

func ends(o war.Order) string { return stamp(o.Ends, "f") + ", " + stamp(o.Ends, "R") }

// MajorOrder answers /major-order: one embed per running order.
func MajorOrder(st *poller.State, now time.Time) Reply {
	s := st.Snap
	if s == nil {
		return Notice(NoticeGathering)
	}
	orders := s.Active(now)
	if len(orders) > maxBriefOrders {
		orders = orders[:maxBriefOrders]
	}
	if len(orders) == 0 {
		e := embed(titled(IconOrder, "Major Order"), ColorOrder)
		e.Description = textNoOrder
		return Reply{Embeds: []*discordgo.MessageEmbed{footed(e, st, now)}}
	}
	var embeds []*discordgo.MessageEmbed
	for _, o := range orders {
		e := embed(titled(IconOrder, "Major Order"), ColorOrder)
		for i, t := range o.Tasks {
			if i == maxAnnounceTask {
				break
			}
			e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Task " + strconv.Itoa(i+1), Value: war.Cut(s.Line(t, i+1), maxFieldValue)})
		}
		if o.Reward != "" {
			e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Reward", Value: o.Reward, Inline: true})
		}
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Ends", Value: ends(o), Inline: true})
		embeds = append(embeds, e)
	}
	footed(embeds[len(embeds)-1], st, now)
	// The task lines come first; the briefings share what is left of
	// Discord's 6000 characters.
	brief := min(maxDescription, (maxEmbedsTotal-embedChars(embeds))/len(orders))
	if brief >= minBriefing {
		for i, o := range orders {
			embeds[i].Description = war.Cut(o.Briefing, brief)
		}
	}
	return Reply{Embeds: embeds}
}

// minBriefing is the shortest cut of a briefing worth showing.
const minBriefing = 20

// embedChars counts what Discord counts toward a message's 6000 characters.
func embedChars(embeds []*discordgo.MessageEmbed) int {
	n := 0
	for _, e := range embeds {
		n += runes(e.Title) + runes(e.Description)
		for _, f := range e.Fields {
			n += runes(f.Name) + runes(f.Value)
		}
		if e.Footer != nil {
			n += runes(e.Footer.Text)
		}
		if e.Author != nil {
			n += runes(e.Author.Name)
		}
	}
	return n
}

// Announcement posts a new Major Order (spec 4.2), pinging only role.
func Announcement(s *war.Snapshot, o war.Order, role string) Reply {
	e := embed(titled(IconNewOrder, "New Major Order"), ColorOrder)
	e.Description = war.Cut(o.Briefing, maxDescription)
	for i, t := range o.Tasks {
		if i == maxAnnounceTask {
			break
		}
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Task " + strconv.Itoa(i+1), Value: war.Cut(s.Describe(t, i+1).Text, maxFieldValue), Inline: true})
	}
	if o.Reward != "" {
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Reward", Value: o.Reward, Inline: true})
	}
	e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Ends", Value: ends(o), Inline: true})
	e.Footer = &discordgo.MessageEmbedFooter{Text: textLiveOrder}
	content := textNewOrders
	if role != "" {
		content = "<@&" + role + "> " + content
	}
	return Reply{Content: content, Embeds: []*discordgo.MessageEmbed{e}, PingRole: role}
}

// PlanetInfo answers /planet.
func PlanetInfo(st *poller.State, now time.Time, p *war.Planet) Reply {
	s := st.Snap
	if s == nil || p == nil {
		return Notice(NoticeGathering)
	}
	e := embed(titled(IconPlanet, p.Name), p.Owner.Color())
	var desc []string
	if p.Sector != "" {
		desc = append(desc, p.Sector+" Sector")
	}
	if activeTargets(s, now)[p.Index] {
		desc = append(desc, titled(IconOrder, "Major Order target"))
	}
	e.Description = strings.Join(desc, " · ")
	add := func(name, value string, inline bool) {
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: name, Value: war.Cut(value, maxFieldValue), Inline: inline})
	}
	add("Held By", p.Owner.Square()+" "+p.Owner.Name(), true)
	if p.InCampaign && p.Owner != war.SuperEarth {
		add("Liberation", war.Bar(p.Liberation()/100)+" "+war.Pct(p.Liberation()), true)
	}
	if d := p.Defense; d != nil {
		add("Defense", d.Enemy.Name()+"\n"+war.Bar(p.DefenseProgress()/100)+" "+war.Pct(p.DefenseProgress())+"\nends "+stamp(d.Ends, "R"), true)
	}
	add("Helldivers", war.Comma(int64(p.Players)), true)
	if p.Biome != "" {
		add("Biome", p.Biome, true)
	}
	if len(p.Hazards) > 0 {
		add("Hazards", strings.Join(p.Hazards, ", "), true)
	}
	var links []string
	for _, l := range s.Links(p.Index) {
		links = append(links, l.Name)
	}
	if len(links) == 0 {
		links = []string{"None"}
	}
	add("Supply Lines", strings.Join(links, ", "), false)
	return Reply{Embeds: []*discordgo.MessageEmbed{footed(e, st, now)}}
}

// activeTargets are the planets that running orders point at.
func activeTargets(s *war.Snapshot, now time.Time) map[int]bool {
	out := map[int]bool{}
	for _, o := range s.Active(now) {
		for _, t := range o.Tasks {
			if i, ok := t.PlanetIndex(); ok {
				out[i] = true
			}
		}
	}
	return out
}

// Help answers /help in the channel, like the other two bots' /help. appID
// is the bot's own application, for the Add button.
func Help(appID string) Reply {
	e := embed(titled(IconHelp, "501st War Room"), ColorSuperEarth)
	e.Description = "Follow the Helldivers 2 **Galactic War** without leaving the server: the war map, the **Major Order** " +
		"and the busiest fronts, in one message that updates itself every 5 minutes."
	e.Fields = []*discordgo.MessageEmbedField{
		{Name: titled(IconBriefing, "War Room"), Value: "`/warroom set` puts the war room in a channel: the Major Order with live progress, " +
			"the five busiest fronts and a fresh map, edited in place every 5 minutes so the channel never fills up.\n" +
			"Add a role to ping it when a new Major Order arrives, and `/warroom clear` turns the war room off. Both need **Manage Server**."},
		{Name: titled(IconCommands, "More Commands"), Value: IconMap + " `/map` Post the latest war map\n" +
			IconOrder + " `/major-order` Show the Major Order with live progress\n" +
			IconPlanet + " `/planet` Look up a planet: who holds it, the battle there, Helldivers and supply lines\n" +
			IconHelp + " `/help` Show how to use the 501st War Room"},
		// The same ending as Sabacc Droid's and Aurebesh Droid's /help (see theme.go).
		{Name: titled(IconWarData, "War Data"), Value: "From the [Helldivers 2 Community API](https://github.com/helldivers-2/api), " +
			"with Arrowhead's official feed as a backup. Planet name spellings from [helldivers-2/json](https://github.com/helldivers-2/json)."},
		{Name: "🌌 More Star Wars", Value: moreStarWars},
		{Name: titled(IconMadeBy, "Made By"), Value: "Created by **" + Creator + "** with [discordgo](https://github.com/bwmarrin/discordgo) and " +
			"[x/image](https://pkg.go.dev/golang.org/x/image)."},
	}
	e.Footer = &discordgo.MessageEmbedFooter{Text: Disclaimer}
	link := func(icon, label, url string) discordgo.MessageComponent {
		return discordgo.Button{Label: label, Style: discordgo.LinkButton, URL: url, Emoji: &discordgo.ComponentEmoji{Name: icon}}
	}
	return Reply{
		Embeds: []*discordgo.MessageEmbed{e},
		Components: []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			link(IconAddBot, "Add 501st War Room", InviteLink(appID)),
			link(IconApp, "Datapad App", DatapadApp),
			link(IconGitHub, "GitHub", GitHubLink),
		}}},
	}
}
