package war_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func orderSnap(orders []war.Order) *war.Snapshot {
	return war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 7, Name: "Zea Rugosia", Owner: war.Illuminate, Health: 226100, MaxHealth: 1700000},
		{Index: 5, Name: "Hellmire", Owner: war.SuperEarth, Health: 1000000, MaxHealth: 1000000},
		{Index: 273, Name: "Alderidge Cove", Owner: war.SuperEarth, Health: 1000000, MaxHealth: 1000000,
			Defense: &war.Defense{Enemy: war.Illuminate, Health: 530109, MaxHealth: 550000}},
	}, orders)
}

func TestDescribe(t *testing.T) {
	s := orderSnap(nil)
	type V = map[int]int64
	rows := []struct {
		typ      int
		values   V
		n        int
		progress int64
		want     string
		prefix   bool // want is only the start of the line
	}{
		{13, V{3: 1, 11: 1, 12: 7}, 1, 0, "○ Hold **Zea Rugosia** (86.7% liberated)", false},
		{13, V{3: 1, 11: 1, 12: 7}, 1, 1, "✅ Hold **Zea Rugosia**", false},
		{3, V{1: 4, 3: 215000000, 4: 4211847317}, 1, 208156482, "○ Kill 215,000,000 Voteless\n`▰▰▰▰▰▰▰▰▰▱` 96.8% (208,156,482)", false},
		{3, V{1: 3, 3: 135000000, 4: 878778730}, 1, 7007086, "○ Kill 135,000,000 Troopers\n`▱▱▱▱▱▱▱▱▱▱` 5.2% (7,007,086)", false},
		{3, V{1: 3, 3: 2000000, 4: 1153658728}, 1, 0, "○ Kill 2,000,000 Factory Striders\n", true},
		{3, V{1: 2, 3: 25000000, 4: 1379865898}, 1, 0, "○ Kill 25,000,000 Bile Spewers\n", true},
		{3, V{1: 2, 3: 250000000, 4: 0}, 1, 0, "○ Kill 250,000,000 Terminids\n", true},
		{3, V{1: 2, 3: 25000000, 4: 999}, 1, 0, "○ Kill 25,000,000 Terminid targets (see the briefing)\n", true},
		{3, V{1: 2, 3: 1000, 4: 0, 11: 1, 12: 7}, 1, 0, "○ Kill 1,000 Terminids on **Zea Rugosia**\n", true},
		{3, V{3: 1000}, 1, 0, "○ Kill 1,000 enemies\n", true},
		{11, V{3: 1, 11: 1, 12: 5}, 1, 1, "✅ Liberate **Hellmire**", false},
		{11, V{3: 5}, 1, 2, "○ Liberate 5 planets\n`▰▰▰▰▱▱▱▱▱▱` 40.0% (2)", false},
		{13, V{3: 5}, 1, 0, "○ Hold 5 planets\n", true},
		{12, V{1: 3, 3: 5}, 1, 1, "○ Defend 5 planets from the Automatons\n`▰▰▱▱▱▱▱▱▱▱` 20.0% (1)", false},
		{12, V{3: 1, 11: 1, 12: 5}, 1, 0, "○ Defend **Hellmire** (held)", false},
		{11, V{3: 1, 11: 1, 12: 5}, 1, 0, "○ Liberate **Hellmire** (held)", false},
		{12, V{3: 1, 11: 1, 12: 273}, 1, 0, "○ Defend **Alderidge Cove** (Defense 3.6%)", false},
		{13, V{3: 1, 11: 1, 12: 273}, 1, 0, "○ Hold **Alderidge Cove** (Defense 3.6%)", false},
		{7, V{1: 2, 3: 500000, 9: 7, 11: 1, 12: 7}, 1, 0, "○ Complete 500,000 missions against the Terminids on **Zea Rugosia** on Difficulty 7 or higher\n", true},
		{9, V{3: 10}, 1, 0, "○ Complete 10 operations\n", true},
		{2, V{3: 1000000}, 1, 0, "○ Complete 1,000,000 extractions\n", true},
		{15, V{1: 4, 3: 5}, 1, 0, "○ Take 5 planets from the Illuminate\n", true},
		{99, V{3: 10}, 2, 5, "○ Objective 2 (see the briefing above)\n`▰▰▰▰▰▱▱▱▱▱` 50.0% (5)", false},
		{99, V{}, 3, 0, "○ Objective 3 (see the briefing above)", false},
		{13, V{3: 1, 11: 1, 12: 999}, 1, 0, "○ Hold **Planet 999**", false},
	}
	for _, r := range rows {
		task := war.Task{Type: r.typ, Values: r.values, Progress: r.progress}
		got := s.Line(task, r.n)
		name := fmt.Sprintf("type %d %v progress %d", r.typ, r.values, r.progress)
		if r.prefix {
			if !strings.HasPrefix(got, r.want) {
				t.Errorf("%s: got %q, want prefix %q", name, got, r.want)
			}
			continue
		}
		if got != r.want {
			t.Errorf("%s: got %q, want %q", name, got, r.want)
		}
	}
}

func TestDescribeFields(t *testing.T) {
	s := orderSnap(nil)
	v := s.Describe(war.Task{Type: 13, Values: map[int]int64{3: 1, 11: 1, 12: 7}}, 1)
	eq(t, v.Text, "Hold **Zea Rugosia**")
	eq(t, v.Note, "86.7% liberated")
	eq(t, v.Bar, "")
	eq(t, v.Done, false)
	c := s.Describe(war.Task{Type: 3, Values: map[int]int64{1: 4, 3: 10, 4: 0}, Progress: 12}, 1)
	eq(t, c.Done, true)
	eq(t, c.Bar, "`▰▰▰▰▰▰▰▰▰▰` 100% (12)")
}

func TestTaskValues(t *testing.T) {
	task := war.Task{Type: 7, Values: map[int]int64{war.VTFaction: 2, war.VTGoal: 500, war.VTUnit: 9, war.VTDifficulty: 7, war.VTLocationType: 1, war.VTLocationIndex: 7}}
	eq(t, task.Goal(), int64(500))
	eq(t, task.Faction(), war.Terminids)
	eq(t, task.Unit(), int64(9))
	eq(t, task.Difficulty(), int64(7))
	i, ok := task.PlanetIndex()
	eq(t, i, 7)
	eq(t, ok, true)
	_, ok = war.Task{Values: map[int]int64{war.VTLocationType: 2, war.VTLocationIndex: 7}}.PlanetIndex()
	eq(t, ok, false)
	// A planet task with no goal counts as goal 1.
	eq(t, war.Task{Type: 11, Values: map[int]int64{11: 1, 12: 5}, Progress: 1}.Done(), true)
	eq(t, war.Task{Type: 11, Values: map[int]int64{11: 1, 12: 5}}.Done(), false)
	eq(t, war.Task{Type: 3, Values: map[int]int64{3: 10}, Progress: 9}.Done(), false)
}

func TestReward(t *testing.T) {
	eq(t, war.Reward(1, 45), "45 Medals")
	eq(t, war.Reward(1, 1), "1 Medal")
	eq(t, war.Reward(3, 40), "40 (reward type 3)")
}

func TestCleanBriefing(t *testing.T) {
	eq(t, war.CleanBriefing("  Hold <i=1>Zea Rugosia</i>.  Kill  Voteless. "), "Hold **Zea Rugosia**. Kill Voteless.")
	eq(t, war.CleanBriefing("Line one.  \n  Line <i=3>two</i>."), "Line one.\nLine **two**.")
}

func TestTargetsAndActive(t *testing.T) {
	s := orderSnap([]war.Order{
		{ID: 1, Ends: testNow.Add(time.Hour), Tasks: []war.Task{
			{Type: 13, Values: map[int]int64{3: 1, 11: 1, 12: 7}},
			{Type: 3, Values: map[int]int64{1: 4, 3: 1000}},
		}},
		{ID: 2, Ends: testNow.Add(-time.Minute), Tasks: []war.Task{
			{Type: 11, Values: map[int]int64{3: 1, 11: 1, 12: 5}},
		}},
	})
	eq(t, fmt.Sprint(s.Targets()), "map[7:true]") // order 2 ended before the snapshot
	active := s.Active(testNow)
	eq(t, len(active), 1)
	eq(t, active[0].ID, int64(1))
	eq(t, len(s.Active(testNow.Add(time.Hour))), 0) // ending exactly now is over
}
