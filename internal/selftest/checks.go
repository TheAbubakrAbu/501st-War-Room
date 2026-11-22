package selftest

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/bot"
	"github.com/bwmarrin/discordgo"
)

// Discord's limits (spec 13).
const (
	maxContent     = 2000
	maxTitle       = 256
	maxDescription = 4096
	maxFields      = 25
	maxFieldName   = 256
	maxFieldValue  = 1024
	maxFooter      = 2048
	maxEmbedTotal  = 6000
	maxEmbeds      = 10
	maxChoices     = 25
	maxChoiceName  = 100
	maxCmdDesc     = 100
	maxOptions     = 25
)

// Characters the conventions forbid. They are escapes here so this file
// never holds them itself.
var forbidden = map[string]string{
	"\u2014": "an em dash",
	"\u2018": "a curly quote",
	"\u2019": "a curly apostrophe",
	"\u201c": "a curly quote",
	"\u201d": "a curly quote",
}

var commandName = regexp.MustCompile(`^[-_a-z0-9]{1,32}$`)

func runes(s string) int { return utf8.RuneCountInString(s) }

// startsWithIcon reports whether s starts with an icon and a space.
func startsWithIcon(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	if r <= 0x2000 || unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	rest := s[size:]
	// Skip variation selectors and joiners that are part of the icon.
	rest = strings.TrimLeft(rest, "️‍")
	return strings.HasPrefix(rest, " ")
}

func textProblems(name, where, s string) []string {
	var out []string
	for ch, what := range forbidden {
		if strings.Contains(s, ch) {
			out = append(out, fmt.Sprintf("%s: %s has %s", name, where, what))
		}
	}
	return out
}

// CheckReply checks one reply against Discord's limits and the reply
// conventions. It returns one line per problem.
func CheckReply(name string, r bot.Reply) []string {
	var p []string
	add := func(format string, a ...any) { p = append(p, name+": "+fmt.Sprintf(format, a...)) }
	if n := runes(r.Content); n > maxContent {
		add("content is %d characters (limit %d)", n, maxContent)
	}
	p = append(p, textProblems(name, "the content", r.Content)...)
	if r.Private && r.Content != "" && !strings.HasPrefix(r.Content, "❌ ") && !strings.HasPrefix(r.Content, "✅ ") {
		add("a private notice must start with ❌ or ✅: %q", r.Content)
	}
	if n := len(r.Embeds); n > maxEmbeds {
		add("%d embeds (limit %d)", n, maxEmbeds)
	}
	total := 0
	for i, e := range r.Embeds {
		where := fmt.Sprintf("embed %d", i+1)
		total += runes(e.Title) + runes(e.Description)
		if n := runes(e.Title); n > maxTitle {
			add("%s title is %d characters (limit %d)", where, n, maxTitle)
		}
		if e.Title != "" && !startsWithIcon(e.Title) {
			add("%s title does not start with an icon: %q", where, e.Title)
		}
		if e.Thumbnail == nil || e.Thumbnail.URL != bot.Logo {
			add("%s does not have the logo as its thumbnail", where)
		}
		if n := runes(e.Description); n > maxDescription {
			add("%s description is %d characters (limit %d)", where, n, maxDescription)
		}
		p = append(p, textProblems(name, where+" title", e.Title)...)
		p = append(p, textProblems(name, where+" description", e.Description)...)
		if n := len(e.Fields); n > maxFields {
			add("%s has %d fields (limit %d)", where, n, maxFields)
		}
		for _, f := range e.Fields {
			total += runes(f.Name) + runes(f.Value)
			if n := runes(f.Name); n == 0 || n > maxFieldName {
				add("%s field %q name is %d characters (1 to %d)", where, f.Name, n, maxFieldName)
			}
			if n := runes(f.Value); n == 0 || n > maxFieldValue {
				add("%s field %q value is %d characters (1 to %d)", where, f.Name, n, maxFieldValue)
			}
			p = append(p, textProblems(name, where+" field "+f.Name, f.Name+f.Value)...)
		}
		if e.Footer != nil {
			total += runes(e.Footer.Text)
			if n := runes(e.Footer.Text); n > maxFooter {
				add("%s footer is %d characters (limit %d)", where, n, maxFooter)
			}
			p = append(p, textProblems(name, where+" footer", e.Footer.Text)...)
		}
		if e.Image != nil && e.Image.URL == "attachment://map.png" && r.Map == nil {
			add("%s shows the map but no map is attached", where)
		}
	}
	if total > maxEmbedTotal {
		add("embeds hold %d characters (limit %d)", total, maxEmbedTotal)
	}
	for _, c := range r.Components {
		row, ok := c.(discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, bc := range row.Components {
			if b, ok := bc.(discordgo.Button); ok && (b.Emoji == nil || b.Emoji.Name == "" || b.Label == "") {
				add("button %q needs an icon and a label", b.Label)
			}
		}
	}
	return p
}

// CheckCommands checks the slash commands against Discord's rules.
func CheckCommands(cmds []*discordgo.ApplicationCommand) []string {
	var p []string
	var options func(where string, opts []*discordgo.ApplicationCommandOption)
	options = func(where string, opts []*discordgo.ApplicationCommandOption) {
		if len(opts) > maxOptions {
			p = append(p, fmt.Sprintf("%s has %d options (limit %d)", where, len(opts), maxOptions))
		}
		for _, o := range opts {
			w := where + " " + o.Name
			if !commandName.MatchString(o.Name) {
				p = append(p, fmt.Sprintf("%s: bad option name %q", where, o.Name))
			}
			if n := runes(o.Description); n == 0 || n > maxCmdDesc {
				p = append(p, fmt.Sprintf("%s: description is %d characters (1 to %d)", w, n, maxCmdDesc))
			}
			options(w, o.Options)
		}
	}
	for _, c := range cmds {
		if !commandName.MatchString(c.Name) {
			p = append(p, fmt.Sprintf("bad command name %q", c.Name))
		}
		if n := runes(c.Description); n == 0 || n > maxCmdDesc {
			p = append(p, fmt.Sprintf("/%s: description is %d characters (1 to %d)", c.Name, n, maxCmdDesc))
		}
		p = append(p, textProblems("/"+c.Name, "the description", c.Description)...)
		options("/"+c.Name, c.Options)
	}
	return p
}

func checkChoices(typed string, cs []*discordgo.ApplicationCommandOptionChoice) []string {
	var p []string
	if len(cs) > maxChoices {
		p = append(p, fmt.Sprintf("autocomplete %q gives %d choices (limit %d)", typed, len(cs), maxChoices))
	}
	for _, c := range cs {
		if n := runes(c.Name); n == 0 || n > maxChoiceName {
			p = append(p, fmt.Sprintf("autocomplete %q choice %q is %d characters (1 to %d)", typed, c.Name, n, maxChoiceName))
		}
	}
	return p
}
