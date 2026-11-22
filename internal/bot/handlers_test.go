package bot

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/bwmarrin/discordgo"
)

type opt = discordgo.ApplicationCommandInteractionDataOption

func command(name string, opts ...*opt) *discordgo.Interaction {
	return &discordgo.Interaction{Type: discordgo.InteractionApplicationCommand, GuildID: "1",
		Data: discordgo.ApplicationCommandInteractionData{Name: name, Options: opts}}
}

func str(name, value string) *opt {
	return &opt{Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value}
}

func kitBot(t *testing.T) (*Bot, *roomKit) {
	t.Helper()
	k := room(t)
	return &Bot{State: k.w.State, Room: k.w, Now: k.w.Now, Log: k.w.Log}, k
}

func TestCommands(t *testing.T) {
	cmds := Commands()
	var got []string
	for _, c := range cmds {
		got = append(got, c.Name)
		eq(t, *c.Contexts, []discordgo.InteractionContextType{discordgo.InteractionContextGuild})
		eq(t, *c.IntegrationTypes, []discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall})
		if c.Description == "" {
			t.Fatalf("%s has no description", c.Name)
		}
	}
	eq(t, got, []string{"map", "major-order", "planet", "warroom", "help"})

	planet := cmds[2]
	eq(t, len(planet.Options), 1)
	o := planet.Options[0]
	eq(t, o.Name, "name")
	eq(t, o.Type, discordgo.ApplicationCommandOptionString)
	eq(t, o.Required, true)
	eq(t, o.Autocomplete, true)

	wr := cmds[3]
	eq(t, *wr.DefaultMemberPermissions, int64(discordgo.PermissionManageGuild))
	eq(t, len(wr.Options), 2)
	set, clear := wr.Options[0], wr.Options[1]
	eq(t, set.Name, "set")
	eq(t, set.Type, discordgo.ApplicationCommandOptionSubCommand)
	eq(t, clear.Name, "clear")
	eq(t, clear.Type, discordgo.ApplicationCommandOptionSubCommand)
	ch, role := set.Options[0], set.Options[1]
	eq(t, ch.Name, "channel")
	eq(t, ch.Type, discordgo.ApplicationCommandOptionChannel)
	eq(t, ch.Required, true)
	eq(t, ch.ChannelTypes, []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews})
	eq(t, role.Name, "role")
	eq(t, role.Type, discordgo.ApplicationCommandOptionRole)
	eq(t, role.Required, false)
	for _, c := range cmds {
		if c.Name != "warroom" && c.DefaultMemberPermissions != nil {
			t.Fatalf("%s is limited", c.Name)
		}
	}
}

func TestAnswerBeforeData(t *testing.T) {
	b, k := kitBot(t)
	k.st = &poller.State{}
	for _, i := range []*discordgo.Interaction{command("map"), command("major-order"), command("planet", str("name", "7"))} {
		r := b.answer(i)
		eq(t, r.Content, NoticeGathering)
		eq(t, r.Private, true)
	}
}

func TestAnswerMap(t *testing.T) {
	b, _ := kitBot(t)
	r := b.answer(command("map"))
	eq(t, r.Embeds[0].Title, "🗺️ Galactic War Map")
	eq(t, string(r.Map), "png")
	eq(t, r.Private, false)
	eq(t, b.answer(command("major-order")).Embeds[0].Title, "🎯 Major Order")
}

func TestAnswerPlanet(t *testing.T) {
	b, _ := kitBot(t)
	eq(t, b.answer(command("planet", str("name", "7"))).Embeds[0].Title, "🪐 Zea Rugosia")
	eq(t, b.answer(command("planet", str("name", "zea rugosia"))).Embeds[0].Title, "🪐 Zea Rugosia")
	r := b.answer(command("planet", str("name", "xyz")))
	eq(t, r.Private, true)
	eq(t, r.Content, `❌ There's no planet called "xyz". Pick one from the list as you type.`)
}

func TestSuggestHandler(t *testing.T) {
	b, _ := kitBot(t)
	i := command("planet", &opt{Name: "name", Type: discordgo.ApplicationCommandOptionString, Value: "zea", Focused: true})
	i.Type = discordgo.InteractionApplicationCommandAutocomplete
	eq(t, b.suggest(i)[0].Name, "Zea Rugosia")
}

func warroomSet(channel, role string, mentionable bool) *discordgo.Interaction {
	sub := &opt{Name: "set", Type: discordgo.ApplicationCommandOptionSubCommand,
		Options: []*opt{{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: channel}}}
	i := command("warroom", sub)
	if role != "" {
		sub.Options = append(sub.Options, &opt{Name: "role", Type: discordgo.ApplicationCommandOptionRole, Value: role})
		d := i.Data.(discordgo.ApplicationCommandInteractionData)
		d.Resolved = &discordgo.ApplicationCommandInteractionDataResolved{Roles: map[string]*discordgo.Role{role: {ID: role, Name: "Helldivers", Mentionable: mentionable}}}
		i.Data = d
	}
	return i
}

func TestAnswerWarroomSet(t *testing.T) {
	b, k := kitBot(t)
	r := b.answer(warroomSet("10", "42", false))
	eq(t, r.Private, true)
	eq(t, r.Content, fmt.Sprintf(NoticeSet, "<#10>")+fmt.Sprintf(NoticeSetRole, "<@&42>")+fmt.Sprintf(NoticeRoleHeadsUp, "<@&42>"))
	eq(t, k.server(t, "1").Role, "42")

	r = b.answer(warroomSet("10", "42", true))
	eq(t, r.Content, fmt.Sprintf(NoticeSet, "<#10>")+fmt.Sprintf(NoticeSetRole, "<@&42>"))

	r = b.answer(warroomSet("10", "", false))
	eq(t, r.Content, fmt.Sprintf(NoticeSet, "<#10>"))

	b2, k2 := kitBot(t)
	k2.msg.failNext("send", restErr(50013))
	r = b2.answer(warroomSet("10", "", false))
	eq(t, r.Content, fmt.Sprintf(NoticeNoAccess, "<#10>"))
	eq(t, r.Private, true)
}

func TestAnswerWarroomEveryoneIsNoPing(t *testing.T) {
	b, k := kitBot(t)
	r := b.answer(warroomSet("10", "1", true)) // the @everyone role has the guild's ID
	eq(t, r.Content, fmt.Sprintf(NoticeSet, "<#10>"))
	eq(t, k.server(t, "1").Role, "")
}

func TestAnswerWarroomClear(t *testing.T) {
	b, _ := kitBot(t)
	clear := command("warroom", &opt{Name: "clear", Type: discordgo.ApplicationCommandOptionSubCommand})
	r := b.answer(clear)
	eq(t, r.Content, NoticeNoWarRoom)
	eq(t, r.Private, true)
	b.answer(warroomSet("10", "", false))
	eq(t, b.answer(clear).Content, NoticeCleared)
}

func TestAnswerRecoversPanic(t *testing.T) {
	var buf bytes.Buffer
	b := &Bot{State: func() *poller.State { panic("boom") }, Now: func() time.Time { return testNow }, Log: logx.New(&buf, slog.LevelInfo)}
	r := b.answer(command("map"))
	eq(t, r.Content, NoticeBroken)
	eq(t, r.Private, true)
	contains(t, buf.String(), "ERROR: ")
	contains(t, buf.String(), "panic")
}

// /help answers in the channel, like the other two bots, and its Add button
// invites the bot that answered.
func TestAnswerHelp(t *testing.T) {
	b, _ := kitBot(t)
	i := command("help")
	i.AppID = "1234"
	r := b.answer(i)
	eq(t, r.Private, false)
	eq(t, r.Embeds[0].Title, "📖 501st War Room")
	eq(t, r.Components[0].(discordgo.ActionsRow).Components[0].(discordgo.Button).URL, InviteLink("1234"))
}

func TestDeferral(t *testing.T) {
	b, k := kitBot(t)
	check := func(i *discordgo.Interaction, later, private bool) {
		t.Helper()
		l, p := b.deferral(i)
		eq(t, [2]bool{l, p}, [2]bool{later, private})
	}
	check(command("warroom"), true, true)
	// The map upload can take longer than the 3 seconds Discord allows.
	check(command("map"), true, false)
	for _, n := range []string{"planet", "help", "major-order"} {
		check(command(n), false, false)
	}
	ac := command("warroom")
	ac.Type = discordgo.InteractionApplicationCommandAutocomplete
	check(ac, false, false)
	// Before there is data, /map answers at once with a private notice.
	k.st = &poller.State{}
	check(command("map"), false, false)
}
