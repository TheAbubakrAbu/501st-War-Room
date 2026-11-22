package war_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func TestLiberationAndDefense(t *testing.T) {
	p := war.Planet{Owner: war.Illuminate, Health: 1594032, MaxHealth: 1600000}
	if got := p.Liberation(); math.Abs(got-0.373) > 1e-9 {
		t.Fatal(got)
	}
	d := war.Planet{Owner: war.SuperEarth, Defense: &war.Defense{Enemy: war.Illuminate, Health: 530109, MaxHealth: 550000}}
	if got := d.DefenseProgress(); math.Abs(got-3.61654545) > 1e-6 {
		t.Fatal(got)
	}
	if d.Enemy() != war.Illuminate || p.Enemy() != war.Illuminate {
		t.Fatal("enemy")
	}
	if d.Progress() != d.DefenseProgress() || p.Progress() != p.Liberation() {
		t.Fatal("progress")
	}
}

func TestLiberationOfHeldPlanetIsZero(t *testing.T) {
	p := war.Planet{Owner: war.SuperEarth, Health: 500, MaxHealth: 1000}
	eq(t, p.Liberation(), 0.0)
	eq(t, p.DefenseProgress(), 0.0)
	z := war.Planet{Owner: war.Terminids}
	eq(t, z.Liberation(), 0.0)
}

func TestFronts(t *testing.T) {
	// a: 10 players; b and c tie on 5 players, c has more progress; d is not in a campaign
	s := war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 1, Name: "A", InCampaign: true, Players: 10, Owner: war.Terminids, MaxHealth: 100, Health: 100},
		{Index: 2, Name: "B", InCampaign: true, Players: 5, Owner: war.Terminids, MaxHealth: 100, Health: 90},
		{Index: 3, Name: "C", InCampaign: true, Players: 5, Owner: war.Terminids, MaxHealth: 100, Health: 50},
		{Index: 4, Name: "D", Players: 99, Owner: war.Terminids, MaxHealth: 100, Health: 100},
	}, nil)
	got := s.Fronts(5)
	eq(t, fmt.Sprint(indexes(got)), "[1 3 2]")
	eq(t, fmt.Sprint(indexes(s.Fronts(2))), "[1 3]")
}

func TestFrontsNameBreaksTies(t *testing.T) {
	s := war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 1, Name: "Zed", InCampaign: true, Players: 5, Owner: war.Terminids, MaxHealth: 100, Health: 50},
		{Index: 2, Name: "Abe", InCampaign: true, Players: 5, Owner: war.Terminids, MaxHealth: 100, Health: 50},
	}, nil)
	eq(t, fmt.Sprint(indexes(s.Fronts(5))), "[2 1]")
}

func TestHeldPlayersBattles(t *testing.T) {
	s := war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 0, Name: "Super Earth", Owner: war.SuperEarth, Players: 100},
		{Index: 1, Name: "A", Owner: war.SuperEarth, Players: 20, InCampaign: true},
		{Index: 2, Name: "B", Owner: war.Automatons, Players: 3, InCampaign: true},
	}, nil)
	held, total := s.Held()
	eq(t, held, 2)
	eq(t, total, 3)
	eq(t, s.Players(), 123)
	eq(t, s.Battles(), 2)
}

func TestLinksBothDirections(t *testing.T) {
	s := war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 1, Name: "A", Waypoints: []int{3, 2}},
		{Index: 2, Name: "C", Waypoints: []int{1}},
		{Index: 3, Name: "B", Waypoints: []int{1}},
	}, nil)
	var names []string
	for _, p := range s.Links(1) {
		names = append(names, p.Name)
	}
	eq(t, fmt.Sprint(names), "[B C]")
	eq(t, fmt.Sprint(s.Pairs()), "[[1 2] [1 3]]")

	// 2 lists 1 only on one side: still a link of 1.
	s = war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 1, Name: "A"},
		{Index: 2, Name: "B", Waypoints: []int{1}},
		{Index: 3, Name: "C", Waypoints: []int{99}}, // unknown index is skipped
	}, nil)
	eq(t, len(s.Links(1)), 1)
	eq(t, fmt.Sprint(s.Pairs()), "[[1 2]]")
}

func TestPlanetMissing(t *testing.T) {
	s := war.NewSnapshot(testNow, "community", []war.Planet{{Index: 0, Name: "Super Earth"}}, nil)
	if s.Planet(263) != nil {
		t.Fatal("263 should be missing")
	}
	if p := s.Planet(0); p == nil || p.Name != "Super Earth" {
		t.Fatal("0 should be Super Earth")
	}
	eq(t, s.Fetched, testNow)
	eq(t, s.Source, "community")
}

func TestFactions(t *testing.T) {
	fs := []war.Faction{war.SuperEarth, war.Terminids, war.Automatons, war.Illuminate}
	names := []string{"Super Earth", "Terminids", "Automatons", "Illuminate"}
	adjs := []string{"Super Earth", "Terminid", "Automaton", "Illuminate"}
	colors := []int{0x4EA3FF, 0xF2C230, 0xFF5A4E, 0xB07BFF}
	squares := []string{"🟦", "🟨", "🟥", "🟪"}
	for i, f := range fs {
		eq(t, f.Name(), names[i])
		eq(t, f.Adjective(), adjs[i])
		eq(t, f.Color(), colors[i])
		eq(t, f.Square(), squares[i])
		eq(t, f.Valid(), true)
	}
	eq(t, war.Faction(0).Valid(), false)
	eq(t, war.Faction(5).Valid(), false)
}

func indexes(ps []*war.Planet) []int {
	var out []int
	for _, p := range ps {
		out = append(out, p.Index)
	}
	return out
}
