package war_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func TestFormat(t *testing.T) {
	eq(t, war.Comma(215000000), "215,000,000")
	eq(t, war.Comma(0), "0")
	eq(t, war.Comma(-1234), "-1,234")
	eq(t, war.Comma(999), "999")
	eq(t, war.Pct(0.373), "0.4%")
	eq(t, war.Pct(10.5573), "10.6%")
	eq(t, war.Pct(0), "0.0%")
	eq(t, war.Pct(99.96), "99.9%") // never claims 100 before it is
	eq(t, war.Pct(100), "100%")
	eq(t, war.Bar(0.968), "`▰▰▰▰▰▰▰▰▰▱`")
	eq(t, war.Bar(0.0519), "`▱▱▱▱▱▱▱▱▱▱`")
	eq(t, war.Bar(1), "`▰▰▰▰▰▰▰▰▰▰`")
	eq(t, war.Bar(-1), "`▱▱▱▱▱▱▱▱▱▱`")
	eq(t, war.Bar(1.5), "`▰▰▰▰▰▰▰▰▰▰`")
	eq(t, war.Cut("one two three", 9), "one two…")
	eq(t, war.Cut("short", 9), "short")
	eq(t, war.Cut("abcdefghijkl", 5), "abcd…") // no space: cut mid-word
	if n := utf8.RuneCountInString(war.Cut(strings.Repeat("word ", 300), 1024)); n > 1024 {
		t.Fatal(n)
	}
}
