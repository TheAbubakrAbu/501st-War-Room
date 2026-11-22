package source

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

// Official fetches the war from Arrowhead's official API (spec 5.2), the
// backup source. It has no names, so names come from Names.
type Official struct {
	Base    string // "https://api.live.prod.thehelldiversgame.com/api"
	Client  *http.Client
	Timeout time.Duration // per request; 0 means 15 s
	Names   *Names
	Now     func() time.Time
	Log     *slog.Logger

	mu      sync.Mutex
	warID   int
	checked time.Time
}

const (
	defaultWarID    = 801
	warIDCheckEvery = time.Hour
)

// Name is "official".
func (o *Official) Name() string { return "official" }

type officialWarInfo struct {
	PlanetInfos []struct {
		Index    int `json:"index"`
		Position struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"position"`
		Waypoints []int `json:"waypoints"`
		MaxHealth int64 `json:"maxHealth"`
	} `json:"planetInfos"`
}

type officialStatus struct {
	Time         int64 `json:"time"`
	PlanetStatus []struct {
		Index   int   `json:"index"`
		Owner   int   `json:"owner"`
		Health  int64 `json:"health"`
		Players int   `json:"players"`
	} `json:"planetStatus"`
	Campaigns []struct {
		PlanetIndex int `json:"planetIndex"`
	} `json:"campaigns"`
	PlanetEvents []struct {
		PlanetIndex int   `json:"planetIndex"`
		Race        int   `json:"race"`
		Health      int64 `json:"health"`
		MaxHealth   int64 `json:"maxHealth"`
		ExpireTime  int64 `json:"expireTime"`
	} `json:"planetEvents"`
}

type officialAssignment struct {
	ID32      int64   `json:"id32"`
	Progress  []int64 `json:"progress"`
	ExpiresIn int64   `json:"expiresIn"`
	Setting   struct {
		OverrideTitle string `json:"overrideTitle"`
		OverrideBrief string `json:"overrideBrief"`
		Tasks         []struct {
			Type       int     `json:"type"`
			Values     []int64 `json:"values"`
			ValueTypes []int   `json:"valueTypes"`
		} `json:"tasks"`
		Reward *struct {
			Type   int   `json:"type"`
			Amount int64 `json:"amount"`
		} `json:"reward"`
	} `json:"setting"`
}

var officialHeaders = map[string]string{"Accept-Language": "en-US"}

// war returns the war ID to use, asking the API at most once an hour. A
// failed ask keeps the old ID and never fails the round.
func (o *Official) war(ctx context.Context, base string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.warID == 0 {
		o.warID = defaultWarID
	}
	now := o.Now()
	if !o.checked.IsZero() && now.Sub(o.checked) < warIDCheckEvery {
		return o.warID
	}
	o.checked = now
	var got struct {
		ID int `json:"id"`
	}
	err := getJSON(ctx, o.Client, o.Timeout, base+"/WarSeason/current/WarID", officialHeaders, &got)
	switch {
	case err != nil:
		o.Log.Warn("could not check the war ID, keeping the old one", "war", o.warID, "err", err)
	case got.ID <= 0:
		o.Log.Warn("war ID reply had no ID, keeping the old one", "war", o.warID)
	default:
		if got.ID != o.warID {
			o.Log.Info("war ID changed", "from", o.warID, "to", got.ID)
		}
		o.warID = got.ID
	}
	return o.warID
}

// Fetch makes the round's requests one after another and builds the
// snapshot. Any failure fails the whole round.
func (o *Official) Fetch(ctx context.Context) (*war.Snapshot, error) {
	base := strings.TrimRight(o.Base, "/")
	id := o.war(ctx, base)
	var info officialWarInfo
	if err := getJSON(ctx, o.Client, o.Timeout, fmt.Sprintf("%s/WarSeason/%d/WarInfo", base, id), officialHeaders, &info); err != nil {
		return nil, err
	}
	var status officialStatus
	if err := getJSON(ctx, o.Client, o.Timeout, fmt.Sprintf("%s/WarSeason/%d/Status", base, id), officialHeaders, &status); err != nil {
		return nil, err
	}
	var assignments []officialAssignment
	if err := getJSON(ctx, o.Client, o.Timeout, fmt.Sprintf("%s/v2/Assignment/War/%d", base, id), officialHeaders, &assignments); err != nil {
		return nil, err
	}
	fetched := o.Now()

	type live struct {
		owner   int
		health  int64
		players int
	}
	byIndex := make(map[int]live, len(status.PlanetStatus))
	for _, ps := range status.PlanetStatus {
		byIndex[ps.Index] = live{ps.Owner, ps.Health, ps.Players}
	}
	inCampaign := map[int]bool{}
	for _, c := range status.Campaigns {
		inCampaign[c.PlanetIndex] = true
	}
	defenses := map[int]*war.Defense{}
	for _, e := range status.PlanetEvents {
		defenses[e.PlanetIndex] = &war.Defense{
			Enemy:     war.Faction(e.Race),
			Health:    e.Health,
			MaxHealth: e.MaxHealth,
			Ends:      fetched.Add(time.Duration(e.ExpireTime-status.Time) * time.Second),
		}
	}

	planets := make([]war.Planet, 0, len(info.PlanetInfos))
	for _, pi := range info.PlanetInfos {
		st, ok := byIndex[pi.Index]
		if !ok {
			return nil, fmt.Errorf("Status has no planet %d", pi.Index)
		}
		n := o.Names.Get(pi.Index)
		planets = append(planets, war.Planet{
			Index:      pi.Index,
			Name:       n.Name,
			Sector:     n.Sector,
			X:          pi.Position.X,
			Y:          pi.Position.Y,
			Owner:      war.Faction(st.owner),
			Health:     st.health,
			MaxHealth:  pi.MaxHealth,
			Players:    st.players,
			Waypoints:  pi.Waypoints,
			InCampaign: inCampaign[pi.Index],
			Defense:    defenses[pi.Index],
		})
	}

	orders := make([]war.Order, 0, len(assignments))
	for _, a := range assignments {
		ord := war.Order{
			ID:       a.ID32,
			Title:    strings.TrimSpace(a.Setting.OverrideTitle),
			Briefing: war.CleanBriefing(a.Setting.OverrideBrief),
			Ends:     fetched.Add(time.Duration(a.ExpiresIn) * time.Second),
		}
		if r := a.Setting.Reward; r != nil {
			ord.Reward = war.Reward(r.Type, r.Amount)
		}
		for i, t := range a.Setting.Tasks {
			task := war.Task{Type: t.Type, Values: taskValues(t.Values, t.ValueTypes)}
			if i < len(a.Progress) {
				task.Progress = a.Progress[i]
			}
			ord.Tasks = append(ord.Tasks, task)
		}
		orders = append(orders, ord)
	}
	return war.NewSnapshot(fetched, o.Name(), planets, orders), nil
}
