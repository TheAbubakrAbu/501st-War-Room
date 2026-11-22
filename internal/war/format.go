package war

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Comma writes n with thousands separators: 215,000,000.
func Comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, s = "-", s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + b.String()
}

// Pct writes a percentage with one decimal. It never shows 100 before the
// value is really 100.
func Pct(p float64) string {
	if p >= 100 {
		return "100%"
	}
	s := fmt.Sprintf("%.1f", p)
	if s == "100.0" {
		s = "99.9"
	}
	return s + "%"
}

// Bar draws a ten-cell progress bar for a fraction from 0 to 1.
func Bar(fraction float64) string {
	n := int(math.Floor(fraction * 10))
	n = max(0, min(10, n))
	return "`" + strings.Repeat("▰", n) + strings.Repeat("▱", 10-n) + "`"
}

// Cut shortens s to at most max runes, ending in "…" at a word boundary
// when it has to cut.
func Cut(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	head := r[:max-1]
	if i := lastSpace(head); i > 0 {
		head = head[:i]
	}
	return strings.TrimRight(string(head), " ") + "…"
}

func lastSpace(r []rune) int {
	for i := len(r) - 1; i >= 0; i-- {
		if r[i] == ' ' || r[i] == '\n' {
			return i
		}
	}
	return -1
}
