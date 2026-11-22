package war

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Value types say what each of a task's values means (spec 7).
const (
	VTFaction       = 1
	VTGoal          = 3
	VTUnit          = 4
	VTDifficulty    = 9
	VTLocationType  = 11
	VTLocationIndex = 12
)

// Task types the bot has text for.
const (
	taskExtract     = 2
	taskEradicate   = 3
	taskMissions    = 7
	taskOperations  = 9
	taskLiberation  = 11
	taskDefense     = 12
	taskControl     = 13
	taskExpand      = 15
	locationPlanet  = 1
	rewardMedals    = 1
	noteLiberated   = " liberated"
	noteHeld        = "held"
	noteDefense     = "Defense "
	unknownBriefing = " (see the briefing above)"
)

// Units names the enemy types seen in orders, by unit ID.
var Units = map[int64]string{
	4211847317: "Voteless",
	878778730:  "Troopers",
	1153658728: "Factory Striders",
	1379865898: "Bile Spewers",
}

// Goal is the count the task asks for (0 when it has none).
func (t Task) Goal() int64 { return t.Values[VTGoal] }

// Faction is the faction the task names (0 when it names none).
func (t Task) Faction() Faction { return Faction(t.Values[VTFaction]) }

// Unit is the enemy type ID (0 means any).
func (t Task) Unit() int64 { return t.Values[VTUnit] }

// Difficulty is the lowest difficulty that counts (0 means any).
func (t Task) Difficulty() int64 { return t.Values[VTDifficulty] }

// PlanetIndex returns the planet the task points at, if it points at one.
func (t Task) PlanetIndex() (int, bool) {
	if t.Values[VTLocationType] != locationPlanet {
		return 0, false
	}
	return int(t.Values[VTLocationIndex]), true
}

// Done reports whether the task is finished. A planet task with no goal
// counts as a goal of 1.
func (t Task) Done() bool {
	goal := t.Goal()
	if _, ok := t.PlanetIndex(); ok && goal == 0 {
		goal = 1
	}
	return goal > 0 && t.Progress >= goal
}

// TaskView is one task as words: the text, a progress bar for count tasks,
// a note for planet tasks, and whether it is finished.
type TaskView struct {
	Text, Bar, Note string
	Done            bool
}

func (s *Snapshot) planetName(i int) string {
	if p := s.Planet(i); p != nil {
		return p.Name
	}
	return "Planet " + strconv.Itoa(i)
}

// Describe turns task number n (1-based) of an order into words.
func (s *Snapshot) Describe(t Task, n int) TaskView {
	v := TaskView{Done: t.Done()}
	idx, onPlanet := t.PlanetIndex()
	where := ""
	if onPlanet {
		where = " on **" + s.planetName(idx) + "**"
	}
	goal := Comma(t.Goal())
	f := t.Faction()
	planetTask := false

	switch t.Type {
	case taskEradicate:
		v.Text = "Kill " + goal + " " + unitName(t.Unit(), f) + where
	case taskLiberation, taskControl, taskDefense:
		verb := map[int]string{taskLiberation: "Liberate", taskControl: "Hold", taskDefense: "Defend"}[t.Type]
		if onPlanet {
			planetTask = true
			v.Text = verb + " **" + s.planetName(idx) + "**"
		} else {
			v.Text = verb + " " + goal + " " + plural(t.Goal(), "planet")
			if t.Type == taskDefense && f.Valid() {
				v.Text += " from the " + f.Name()
			}
		}
	case taskMissions, taskOperations, taskExtract:
		noun := map[int]string{taskMissions: "missions", taskOperations: "operations", taskExtract: "extractions"}[t.Type]
		v.Text = "Complete " + goal + " " + noun
		if f.Valid() {
			v.Text += " against the " + f.Name()
		}
		v.Text += where
		if d := t.Difficulty(); d > 0 {
			v.Text += fmt.Sprintf(" on Difficulty %d or higher", d)
		}
	case taskExpand:
		v.Text = "Take " + goal + " " + plural(t.Goal(), "planet")
		if f.Valid() {
			v.Text += " from the " + f.Name()
		}
	default:
		v.Text = "Objective " + strconv.Itoa(n) + unknownBriefing
	}

	if planetTask {
		if p := s.Planet(idx); p != nil {
			switch {
			case p.Defense != nil:
				v.Note = noteDefense + Pct(p.DefenseProgress())
			case p.Owner != SuperEarth:
				v.Note = Pct(p.Liberation()) + noteLiberated
			default:
				v.Note = noteHeld
			}
		}
	} else if g := t.Goal(); g > 0 {
		frac := float64(t.Progress) / float64(g)
		v.Bar = Bar(frac) + " " + Pct(100*frac) + " (" + Comma(t.Progress) + ")"
	}
	return v
}

// Line is a task as one entry of an order: a mark, the text, the note and
// the bar. A finished task shows only the mark and the text.
func (s *Snapshot) Line(t Task, n int) string {
	v := s.Describe(t, n)
	if v.Done {
		return "✅ " + v.Text
	}
	line := "○ " + v.Text
	if v.Note != "" {
		line += " (" + v.Note + ")"
	}
	if v.Bar != "" {
		line += "\n" + v.Bar
	}
	return line
}

func unitName(unit int64, f Faction) string {
	if name, ok := Units[unit]; ok {
		return name
	}
	switch {
	case unit == 0 && f.Valid():
		return f.Name()
	case unit == 0:
		return "enemies"
	case f.Valid():
		return f.Adjective() + " targets (see the briefing)"
	default:
		return "targets (see the briefing)"
	}
}

func plural(n int64, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Targets returns the planets that tasks of the orders still running when
// the snapshot was fetched point at.
func (s *Snapshot) Targets() map[int]bool {
	out := map[int]bool{}
	for _, o := range s.Active(s.Fetched) {
		for _, t := range o.Tasks {
			if i, ok := t.PlanetIndex(); ok {
				out[i] = true
			}
		}
	}
	return out
}

// Active returns the orders that have not ended at now.
func (s *Snapshot) Active(now time.Time) []Order {
	var out []Order
	for _, o := range s.Orders {
		if o.Ends.After(now) {
			out = append(out, o)
		}
	}
	return out
}

// Reward describes an order's reward: "45 Medals".
func Reward(kind int, amount int64) string {
	if kind == rewardMedals {
		return Comma(amount) + " " + plural(amount, "Medal")
	}
	return fmt.Sprintf("%s (reward type %d)", Comma(amount), kind)
}

var (
	highlight = regexp.MustCompile(`<i=\d+>(.*?)</i>`)
	spaces    = regexp.MustCompile(`[ \t]+`)
	edges     = regexp.MustCompile(` *\n *`)
)

// CleanBriefing turns the game's highlight tags into bold text, collapses
// runs of spaces and trims.
func CleanBriefing(s string) string {
	s = highlight.ReplaceAllString(s, "**$1**")
	s = spaces.ReplaceAllString(s, " ")
	s = edges.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
