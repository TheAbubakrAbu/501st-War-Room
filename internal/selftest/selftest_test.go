package selftest_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/bot"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/selftest"
	"github.com/bwmarrin/discordgo"
)

func TestRunPasses(t *testing.T) {
	var buf bytes.Buffer
	if n := selftest.Run(&buf); n != 0 {
		t.Fatal(buf.String())
	}
	if !strings.HasSuffix(buf.String(), "All checks passed\n") {
		t.Fatalf("output: %q", buf.String())
	}
}

var logo = &discordgo.MessageEmbedThumbnail{URL: bot.Logo}

// embed is a reply with one embed, given the logo as every embed has, so
// each bad case below has only its own problem.
func embed(e *discordgo.MessageEmbed) bot.Reply {
	e.Thumbnail = logo
	return bot.Reply{Embeds: []*discordgo.MessageEmbed{e}}
}

func TestChecksCatchProblems(t *testing.T) {
	ok := &discordgo.MessageEmbed{Title: "📡 Galactic War Briefing", Description: "Fine."}
	if p := selftest.CheckReply("ok", embed(ok)); len(p) != 0 {
		t.Fatalf("good reply flagged: %v", p)
	}
	if p := selftest.CheckReply("no thumbnail", bot.Reply{Embeds: []*discordgo.MessageEmbed{{Title: "📡 X", Description: "Fine."}}}); len(p) != 1 || !strings.Contains(p[0], "thumbnail") {
		t.Errorf("embed without the logo: got %v, want one thumbnail problem", p)
	}
	many := func(n int) []*discordgo.MessageEmbedField {
		var out []*discordgo.MessageEmbedField
		for range n {
			out = append(out, &discordgo.MessageEmbedField{Name: "F", Value: "v"})
		}
		return out
	}
	eleven := bot.Reply{}
	for range 11 {
		eleven.Embeds = append(eleven.Embeds, &discordgo.MessageEmbed{Title: "📡 X", Thumbnail: logo})
	}
	big := bot.Reply{}
	for range 2 {
		big.Embeds = append(big.Embeds, &discordgo.MessageEmbed{Title: "📡 X", Description: strings.Repeat("a", 3500), Thumbnail: logo})
	}
	bad := map[string]bot.Reply{
		"long title":     embed(&discordgo.MessageEmbed{Title: "📡 " + strings.Repeat("a", 255)}),
		"no icon":        embed(&discordgo.MessageEmbed{Title: "Galactic War Briefing"}),
		"em dash":        embed(&discordgo.MessageEmbed{Title: "📡 X", Description: "war \u2014 peace"}),
		"curly single":   embed(&discordgo.MessageEmbed{Title: "📡 X", Description: "Widow\u2019s Harbor"}),
		"curly double":   {Content: "\u201cquoted\u201d"},
		"private notice": {Content: "Gathering war intel.", Private: true},
		"26 fields":      embed(&discordgo.MessageEmbed{Title: "📡 X", Fields: many(26)}),
		"long value":     embed(&discordgo.MessageEmbed{Title: "📡 X", Fields: []*discordgo.MessageEmbedField{{Name: "F", Value: strings.Repeat("a", 1025)}}}),
		"empty value":    embed(&discordgo.MessageEmbed{Title: "📡 X", Fields: []*discordgo.MessageEmbedField{{Name: "F", Value: ""}}}),
		"long footer":    embed(&discordgo.MessageEmbed{Title: "📡 X", Footer: &discordgo.MessageEmbedFooter{Text: strings.Repeat("a", 2049)}}),
		"11 embeds":      eleven,
		"over 6000":      big,
		"long content":   {Content: strings.Repeat("a", 2001)},
		"long desc":      embed(&discordgo.MessageEmbed{Title: "📡 X", Description: strings.Repeat("a", 4097)}),
	}
	for name, r := range bad {
		if p := selftest.CheckReply(name, r); len(p) == 0 {
			t.Errorf("%s: no problem found", name)
		}
	}
	for _, r := range []bot.Reply{bot.Notice("✅ Done."), bot.Notice("❌ Nope.")} {
		if p := selftest.CheckReply("notice", r); len(p) != 0 {
			t.Errorf("good notice flagged: %v", p)
		}
	}

	if p := selftest.CheckCommands(bot.Commands()); len(p) != 0 {
		t.Fatalf("real commands flagged: %v", p)
	}
	if p := selftest.CheckCommands([]*discordgo.ApplicationCommand{{Name: "Map", Description: "x"}}); len(p) == 0 {
		t.Error("capital letter in a name passed")
	}
	if p := selftest.CheckCommands([]*discordgo.ApplicationCommand{{Name: "map", Description: strings.Repeat("a", 101)}}); len(p) == 0 {
		t.Error("101-rune description passed")
	}
	if p := selftest.CheckCommands([]*discordgo.ApplicationCommand{{Name: "map", Description: "x", Options: []*discordgo.ApplicationCommandOption{{Name: "Bad Name", Description: "x"}}}}); len(p) == 0 {
		t.Error("bad option name passed")
	}
}
