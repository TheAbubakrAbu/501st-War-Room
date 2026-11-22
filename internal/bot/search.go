package bot

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

const maxChoices = 25 // Discord's limit for autocomplete

// The curly apostrophes are what phones type in "Widow's Harbor".
var accents = strings.NewReplacer("\u2019", "'", "\u2018", "'", "ö", "o", "ä", "a", "ü", "u", "é", "e", "è", "e", "á", "a", "í", "i", "ó", "o", "ú", "u", "ñ", "n")

// fold makes names comparable: lower case, no accents, no outer spaces.
func fold(s string) string { return accents.Replace(strings.ToLower(strings.TrimSpace(s))) }

// matches returns the planets whose name contains typed: names starting
// with it first, then the rest, each group busiest first, then by name.
func matches(s *war.Snapshot, typed string) []*war.Planet {
	q := fold(typed)
	var starts, inside []*war.Planet
	for i := range s.Planets {
		p := &s.Planets[i]
		n := fold(p.Name)
		switch {
		case strings.HasPrefix(n, q):
			starts = append(starts, p)
		case strings.Contains(n, q):
			inside = append(inside, p)
		}
	}
	order := func(a, b *war.Planet) int {
		return cmp.Or(cmp.Compare(b.Players, a.Players), cmp.Compare(a.Name, b.Name))
	}
	slices.SortFunc(starts, order)
	slices.SortFunc(inside, order)
	return append(starts, inside...)
}

// Suggest offers up to 25 planets for /planet's autocomplete. Each choice's
// value is the planet's index.
func Suggest(s *war.Snapshot, typed string) []*discordgo.ApplicationCommandOptionChoice {
	out := []*discordgo.ApplicationCommandOptionChoice{}
	if s == nil {
		return out
	}
	for _, p := range matches(s, typed) {
		if len(out) == maxChoices {
			break
		}
		out = append(out, &discordgo.ApplicationCommandOptionChoice{Name: war.Cut(p.Name, 100), Value: strconv.Itoa(p.Index)})
	}
	return out
}

// Resolve turns what /planet received into a planet: a chosen index, an
// exact name, or the first name starting with the text. It returns nil
// when nothing fits.
func Resolve(s *war.Snapshot, value string) *war.Planet {
	if s == nil || strings.TrimSpace(value) == "" {
		return nil
	}
	if i, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return s.Planet(i)
	}
	q := fold(value)
	for i := range s.Planets {
		if fold(s.Planets[i].Name) == q {
			return &s.Planets[i]
		}
	}
	if m := matches(s, value); len(m) > 0 && strings.HasPrefix(fold(m[0].Name), q) {
		return m[0]
	}
	return nil
}
