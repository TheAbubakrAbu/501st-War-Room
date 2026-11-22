// Package war holds the Galactic War as plain values: planets, defenses and
// Major Orders, plus the pure functions the replies and the map use. Nothing
// here touches the network.
package war

import (
	"cmp"
	"slices"
	"time"
)

// Faction is a side in the war, numbered the way both APIs number owners.
type Faction int

const (
	SuperEarth Faction = 1
	Terminids  Faction = 2
	Automatons Faction = 3
	Illuminate Faction = 4
)

var factionInfo = map[Faction]struct {
	name, adjective, square string
	color                   int
}{
	SuperEarth: {"Super Earth", "Super Earth", "🟦", 0x4EA3FF},
	Terminids:  {"Terminids", "Terminid", "🟨", 0xF2C230},
	Automatons: {"Automatons", "Automaton", "🟥", 0xFF5A4E},
	Illuminate: {"Illuminate", "Illuminate", "🟪", 0xB07BFF},
}

// Valid reports whether f is one of the four factions.
func (f Faction) Valid() bool { _, ok := factionInfo[f]; return ok }

// Name is the faction as a noun: "Terminids".
func (f Faction) Name() string {
	if i, ok := factionInfo[f]; ok {
		return i.name
	}
	return "Unknown"
}

// Adjective is the faction before a noun: "Terminid".
func (f Faction) Adjective() string {
	if i, ok := factionInfo[f]; ok {
		return i.adjective
	}
	return "Unknown"
}

// Color is the faction's map and embed color.
func (f Faction) Color() int { return factionInfo[f].color }

// Square is the faction's colored square emoji.
func (f Faction) Square() string {
	if i, ok := factionInfo[f]; ok {
		return i.square
	}
	return "⬜"
}

// Planet is one planet as the latest round saw it.
type Planet struct {
	Index      int
	Name       string
	Sector     string
	X, Y       float64
	Owner      Faction
	Health     int64
	MaxHealth  int64
	Players    int
	Waypoints  []int
	InCampaign bool
	Defense    *Defense // nil unless under attack
	Biome      string   // empty when the source has none
	Hazards    []string
}

// Defense is an enemy attack on a Super Earth planet, with a deadline.
type Defense struct {
	Enemy     Faction
	Health    int64
	MaxHealth int64
	Ends      time.Time
}

// Order is a Major Order (or another assignment) from Super Earth Command.
type Order struct {
	ID       int64
	Title    string
	Briefing string
	Tasks    []Task
	Reward   string // "40 Medals"
	Ends     time.Time
}

// Task is one goal of an Order. Values holds the task's values keyed by
// their value type (see the VT constants).
type Task struct {
	Type     int
	Values   map[int]int64
	Progress int64
}

// Snapshot is one round's whole war. It is never changed after it is built.
type Snapshot struct {
	Fetched time.Time
	Source  string // "community" or "official"
	Planets []Planet
	Orders  []Order

	byIndex map[int]int
}

// NewSnapshot wraps one round's data and builds the index lookup.
func NewSnapshot(fetched time.Time, source string, planets []Planet, orders []Order) *Snapshot {
	s := &Snapshot{Fetched: fetched, Source: source, Planets: planets, Orders: orders, byIndex: make(map[int]int, len(planets))}
	for i, p := range planets {
		s.byIndex[p.Index] = i
	}
	return s
}

func progress(health, max int64) float64 {
	if max <= 0 {
		return 0
	}
	return 100 * (1 - float64(health)/float64(max))
}

// Liberation is how far Super Earth has taken an enemy planet back, 0 to
// 100. A planet Super Earth holds has none.
func (p *Planet) Liberation() float64 {
	if p.Owner == SuperEarth {
		return 0
	}
	return progress(p.Health, p.MaxHealth)
}

// DefenseProgress is how far a defense has gone, 0 to 100 (0 without one).
func (p *Planet) DefenseProgress() float64 {
	if p.Defense == nil {
		return 0
	}
	return progress(p.Defense.Health, p.Defense.MaxHealth)
}

// Enemy is who Helldivers fight there: the attacker in a defense,
// otherwise the owner.
func (p *Planet) Enemy() Faction {
	if p.Defense != nil {
		return p.Defense.Enemy
	}
	return p.Owner
}

// Progress is the defense progress during a defense, else the liberation.
func (p *Planet) Progress() float64 {
	if p.Defense != nil {
		return p.DefenseProgress()
	}
	return p.Liberation()
}

// Planet returns the planet with this index, or nil.
func (s *Snapshot) Planet(index int) *Planet {
	if i, ok := s.byIndex[index]; ok {
		return &s.Planets[i]
	}
	return nil
}

// Fronts returns up to n planets in a campaign, busiest first: most
// Helldivers, then most progress, then by name.
func (s *Snapshot) Fronts(n int) []*Planet {
	var out []*Planet
	for i := range s.Planets {
		if s.Planets[i].InCampaign {
			out = append(out, &s.Planets[i])
		}
	}
	slices.SortStableFunc(out, func(a, b *Planet) int {
		if c := cmp.Compare(b.Players, a.Players); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Progress(), a.Progress()); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Held returns how many planets Super Earth holds, and how many there are.
func (s *Snapshot) Held() (held, total int) {
	for _, p := range s.Planets {
		if p.Owner == SuperEarth {
			held++
		}
	}
	return held, len(s.Planets)
}

// Players is the number of Helldivers on all planets.
func (s *Snapshot) Players() int {
	n := 0
	for _, p := range s.Planets {
		n += p.Players
	}
	return n
}

// Battles is the number of planets in a campaign.
func (s *Snapshot) Battles() int {
	n := 0
	for _, p := range s.Planets {
		if p.InCampaign {
			n++
		}
	}
	return n
}

// Links returns the planets joined to index by a supply line in either
// direction, sorted by name.
func (s *Snapshot) Links(index int) []*Planet {
	seen := map[int]bool{}
	for _, pr := range s.Pairs() {
		switch index {
		case pr[0]:
			seen[pr[1]] = true
		case pr[1]:
			seen[pr[0]] = true
		}
	}
	var out []*Planet
	for i := range seen {
		out = append(out, s.Planet(i))
	}
	slices.SortFunc(out, func(a, b *Planet) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Index, b.Index))
	})
	return out
}

// Pairs returns every supply line once, smaller index first, sorted.
// Lines to unknown planets are skipped.
func (s *Snapshot) Pairs() [][2]int {
	seen := map[[2]int]bool{}
	var out [][2]int
	for _, p := range s.Planets {
		for _, w := range p.Waypoints {
			if w == p.Index || s.Planet(w) == nil {
				continue
			}
			pr := [2]int{min(p.Index, w), max(p.Index, w)}
			if !seen[pr] {
				seen[pr] = true
				out = append(out, pr)
			}
		}
	}
	slices.SortFunc(out, func(a, b [2]int) int {
		return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
	})
	return out
}
