package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func names(choices []*discordgo.ApplicationCommandOptionChoice) []string {
	out := []string{}
	for _, c := range choices {
		out = append(out, c.Name)
	}
	return out
}

func TestSuggest(t *testing.T) {
	s := community(t)
	zea := Suggest(s, "zea")
	eq(t, zea[0].Name, "Zea Rugosia")
	eq(t, zea[0].Value, "7")
	all := names(Suggest(s, ""))
	eq(t, len(all), 25)
	eq(t, all[:5], []string{"Matar Bay", "Brilliance", "Heze Bay", "Luxuriant", "Alderidge Cove"})
	eq(t, names(Suggest(s, "bay")), []string{"Matar Bay", "Heze Bay", "Aurora Bay", "Phact Bay", "Vega Bay", "Afoyay Bay", "Chort Bay",
		"East Iridium Trading Bay", "Electra Bay", "Martyr's Bay", "Rirga Bay", "Skat Bay"})
	eq(t, names(Suggest(s, "xyzzy")), []string{})
	eq(t, names(Suggest(s, "  zea  "))[0], "Zea Rugosia")
}

func TestSuggestFoldsCaseAndAccents(t *testing.T) {
	s := community(t)
	eq(t, names(Suggest(s, "popli")), []string{"Pöpli IX"})
	eq(t, names(Suggest(s, "PÖPLI")), []string{"Pöpli IX"})
	eq(t, names(Suggest(s, "ZEA"))[0], "Zea Rugosia")
	eq(t, names(Suggest(s, "widow's"))[0], "Widow's Harbor")
}

func TestSuggestStartsFirst(t *testing.T) {
	s := community(t)
	got := names(Suggest(s, "m"))
	eq(t, got[0], "Matar Bay") // starts with m and busiest
	for i, n := range got {
		if n[0] != 'M' && i < 5 {
			t.Fatalf("names starting with the text come first: %v", got)
		}
	}
}

func TestResolve(t *testing.T) {
	s := community(t)
	eq(t, Resolve(s, "7").Name, "Zea Rugosia")
	eq(t, Resolve(s, "zea rugosia").Name, "Zea Rugosia")
	eq(t, Resolve(s, "Zea").Name, "Zea Rugosia")
	eq(t, Resolve(s, "pöpli ix").Index, 107)
	if Resolve(s, "nothing") != nil || Resolve(s, "999") != nil || Resolve(s, "") != nil {
		t.Fatal("want nil")
	}
}

// iPhones type a curly apostrophe; names use a straight one.
func TestSuggestFoldsCurlyApostrophe(t *testing.T) {
	s := community(t)
	eq(t, names(Suggest(s, "widow\u2019s"))[0], "Widow's Harbor")
	eq(t, Resolve(s, "martyr\u2019s bay").Name, "Martyr's Bay")
}
