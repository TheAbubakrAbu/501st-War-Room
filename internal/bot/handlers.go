package bot

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

// option finds a named option's value as a string.
func option(opts []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, o := range opts {
		if o.Name == name {
			if s, ok := o.Value.(string); ok {
				return s
			}
			return fmt.Sprint(o.Value)
		}
	}
	return ""
}

// deferral says whether a command answers after a deferred reply, and
// whether that reply is private. /warroom talks to Discord before it can
// answer. /map uploads the map picture, which can take longer than the 3
// seconds Discord gives a first reply; with no picture yet it answers at
// once with a private notice. The picture never goes away once there is
// one, so the answer always matches the deferred reply.
func (b *Bot) deferral(i *discordgo.Interaction) (later, private bool) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return false, false
	}
	switch i.ApplicationCommandData().Name {
	case "warroom":
		return true, true
	case "map":
		return len(b.State().Map) > 0, false
	}
	return false, false
}

// suggest answers /planet's autocomplete.
func (b *Bot) suggest(i *discordgo.Interaction) (out []*discordgo.ApplicationCommandOptionChoice) {
	defer func() {
		if p := recover(); p != nil {
			b.Log.Error("autocomplete panicked", "panic", p, "stack", string(debug.Stack()))
			out = []*discordgo.ApplicationCommandOptionChoice{}
		}
	}()
	typed := ""
	for _, o := range i.ApplicationCommandData().Options {
		if o.Focused {
			typed, _ = o.Value.(string)
		}
	}
	return Suggest(b.State().Snap, typed)
}

// answer builds the reply to a slash command. A panic becomes a polite
// notice and an ERROR line with the stack.
func (b *Bot) answer(i *discordgo.Interaction) (r Reply) {
	name := ""
	defer func() {
		if p := recover(); p != nil {
			b.Log.Error("command panicked", "command", name, "guild", i.GuildID, "panic", p, "stack", string(debug.Stack()))
			r = Notice(NoticeBroken)
		}
	}()
	d := i.ApplicationCommandData()
	name = d.Name
	switch name {
	case "map":
		return MapReply(b.State(), b.Now())
	case "major-order":
		return MajorOrder(b.State(), b.Now())
	case "planet":
		st := b.State()
		if st.Snap == nil {
			return Notice(NoticeGathering)
		}
		typed := option(d.Options, "name")
		p := Resolve(st.Snap, typed)
		if p == nil {
			return Notice(fmt.Sprintf(NoticeNoPlanet, war.Cut(typed, 100)))
		}
		return PlanetInfo(st, b.Now(), p)
	case "warroom":
		if len(d.Options) == 0 || i.GuildID == "" {
			return Notice(NoticeBroken)
		}
		sub := d.Options[0]
		switch sub.Name {
		case "set":
			return b.warroomSet(i, d, sub.Options)
		case "clear":
			had, err := b.Room.Clear(i.GuildID)
			switch {
			case err != nil:
				b.Log.Error("warroom clear failed", "guild", i.GuildID, "err", err)
				return Notice(NoticeBroken)
			case !had:
				return Notice(NoticeNoWarRoom)
			}
			return Notice(NoticeCleared)
		}
	case "help":
		return Help(i.AppID)
	}
	return Notice(NoticeBroken)
}

func (b *Bot) warroomSet(i *discordgo.Interaction, d discordgo.ApplicationCommandInteractionData, opts []*discordgo.ApplicationCommandInteractionDataOption) Reply {
	channel, role := option(opts, "channel"), option(opts, "role")
	if role == i.GuildID {
		role = "" // @everyone: never ping the whole server
	}
	where := "<#" + channel + ">"
	if err := b.Room.Set(i.GuildID, channel, role); err != nil {
		if errors.Is(err, ErrNoAccess) {
			return Notice(fmt.Sprintf(NoticeNoAccess, where))
		}
		b.Log.Error("warroom set failed", "guild", i.GuildID, "err", err)
		return Notice(NoticeBroken)
	}
	text := fmt.Sprintf(NoticeSet, where)
	if role != "" {
		mention := "<@&" + role + ">"
		text += fmt.Sprintf(NoticeSetRole, mention)
		if d.Resolved != nil {
			if r, ok := d.Resolved.Roles[role]; ok && !r.Mentionable {
				text += fmt.Sprintf(NoticeRoleHeadsUp, mention)
			}
		}
	}
	return Notice(text)
}
