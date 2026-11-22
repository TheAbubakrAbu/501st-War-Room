package source

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// Community fetches the war from the community API (spec 5.1).
type Community struct {
	Base    string // "https://api.helldivers2.dev"
	Contact string // X-Super-Contact; "" means "501st-war-room"
	Version string // for the User-Agent
	Client  *http.Client
	Timeout time.Duration // per request; 0 means 15 s
	Names   *Names
	Now     func() time.Time
}

const clientName = "501st-war-room"

// Name is "community".
func (c *Community) Name() string { return "community" }

type communityPlanet struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Sector   string `json:"sector"`
	Position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
	Waypoints    []int  `json:"waypoints"`
	Health       int64  `json:"health"`
	MaxHealth    int64  `json:"maxHealth"`
	CurrentOwner string `json:"currentOwner"`
	Event        *struct {
		Faction   string    `json:"faction"`
		Health    int64     `json:"health"`
		MaxHealth int64     `json:"maxHealth"`
		EndTime   time.Time `json:"endTime"`
	} `json:"event"`
	Statistics struct {
		PlayerCount int `json:"playerCount"`
	} `json:"statistics"`
	Biome *struct {
		Name string `json:"name"`
	} `json:"biome"`
	Hazards []struct {
		Name string `json:"name"`
	} `json:"hazards"`
}

type communityCampaign struct {
	Planet struct {
		Index int `json:"index"`
	} `json:"planet"`
	Type int `json:"type"`
}

type communityReward struct {
	Type   int   `json:"type"`
	Amount int64 `json:"amount"`
}

type communityAssignment struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Briefing string  `json:"briefing"`
	Progress []int64 `json:"progress"`
	Tasks    []struct {
		Type       int     `json:"type"`
		Values     []int64 `json:"values"`
		ValueTypes []int   `json:"valueTypes"`
	} `json:"tasks"`
	Reward     *communityReward  `json:"reward"`
	Rewards    []communityReward `json:"rewards"`
	Expiration time.Time         `json:"expiration"`
}

func (c *Community) headers() map[string]string {
	contact := c.Contact
	if contact == "" {
		contact = clientName
	}
	return map[string]string{
		"X-Super-Client":  clientName,
		"X-Super-Contact": contact,
		"Accept-Language": "en-US",
		"User-Agent":      clientName + "/" + c.Version,
	}
}

// Fetch makes the round's three requests one after another and builds the
// snapshot. Any failure fails the whole round.
func (c *Community) Fetch(ctx context.Context) (*war.Snapshot, error) {
	base := strings.TrimRight(c.Base, "/")
	h := c.headers()
	var planets []communityPlanet
	if err := getJSON(ctx, c.Client, c.Timeout, base+"/api/v1/planets", h, &planets); err != nil {
		return nil, err
	}
	var campaigns []communityCampaign
	if err := getJSON(ctx, c.Client, c.Timeout, base+"/api/v1/campaigns", h, &campaigns); err != nil {
		return nil, err
	}
	var assignments []communityAssignment
	if err := getJSON(ctx, c.Client, c.Timeout, base+"/api/v1/assignments", h, &assignments); err != nil {
		return nil, err
	}

	inCampaign := map[int]bool{}
	for _, cp := range campaigns {
		inCampaign[cp.Planet.Index] = true
	}
	names := make(map[int]PlanetName, len(planets))
	out := make([]war.Planet, 0, len(planets))
	for _, p := range planets {
		wp := war.Planet{
			Index:      p.Index,
			Name:       Display(p.Name),
			Sector:     p.Sector,
			X:          p.Position.X,
			Y:          p.Position.Y,
			Owner:      ownerFromName(p.CurrentOwner),
			Health:     p.Health,
			MaxHealth:  p.MaxHealth,
			Players:    p.Statistics.PlayerCount,
			Waypoints:  p.Waypoints,
			InCampaign: inCampaign[p.Index],
		}
		if p.Biome != nil {
			wp.Biome = p.Biome.Name
		}
		for _, hz := range p.Hazards {
			if hz.Name != "" && hz.Name != "None" {
				wp.Hazards = append(wp.Hazards, hz.Name)
			}
		}
		if e := p.Event; e != nil {
			wp.Defense = &war.Defense{Enemy: ownerFromName(e.Faction), Health: e.Health, MaxHealth: e.MaxHealth, Ends: e.EndTime.Truncate(time.Second).UTC()}
		}
		names[p.Index] = PlanetName{Name: wp.Name, Sector: wp.Sector}
		out = append(out, wp)
	}

	orders := make([]war.Order, 0, len(assignments))
	for _, a := range assignments {
		o := war.Order{
			ID:       a.ID,
			Title:    strings.TrimSpace(a.Title),
			Briefing: war.CleanBriefing(a.Briefing),
			Ends:     a.Expiration.Truncate(time.Second).UTC(),
		}
		r := a.Reward
		if r == nil && len(a.Rewards) > 0 {
			r = &a.Rewards[0]
		}
		if r != nil {
			o.Reward = war.Reward(r.Type, r.Amount)
		}
		for i, t := range a.Tasks {
			task := war.Task{Type: t.Type, Values: taskValues(t.Values, t.ValueTypes)}
			if i < len(a.Progress) {
				task.Progress = a.Progress[i]
			}
			o.Tasks = append(o.Tasks, task)
		}
		orders = append(orders, o)
	}

	snap := war.NewSnapshot(c.Now(), c.Name(), out, orders)
	// Only names from a reply that looks like the real war reach the cache.
	if err := Check(snap); err != nil {
		return nil, err
	}
	if c.Names != nil {
		c.Names.Update(names)
	}
	return snap, nil
}
