package bot

import "github.com/bwmarrin/discordgo"

// Commands are the slash commands, registered with one bulk overwrite at
// every start. All of them are for servers only.
func Commands() []*discordgo.ApplicationCommand {
	manage := int64(discordgo.PermissionManageGuild)
	cmds := []*discordgo.ApplicationCommand{
		{Name: "map", Description: "Posts the latest Galactic War map."},
		{Name: "major-order", Description: "Shows the current Major Order and its progress."},
		{Name: "planet", Description: "Looks up one planet.", Options: []*discordgo.ApplicationCommandOption{{
			Name: "name", Description: "The planet to look up.", Type: discordgo.ApplicationCommandOptionString, Required: true, Autocomplete: true,
		}}},
		{Name: "warroom", Description: "Sets up the auto-updating war room.", DefaultMemberPermissions: &manage, Options: []*discordgo.ApplicationCommandOption{
			{Name: "set", Description: "Picks the war-room channel and an optional role to ping.", Type: discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{Name: "channel", Description: "Where the war room goes.", Type: discordgo.ApplicationCommandOptionChannel, Required: true,
						ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews}},
					{Name: "role", Description: "Pinged for new Major Orders.", Type: discordgo.ApplicationCommandOptionRole},
				}},
			{Name: "clear", Description: "Turns the war room off.", Type: discordgo.ApplicationCommandOptionSubCommand},
		}},
		{Name: "help", Description: "How the 501st War Room works."},
	}
	for _, c := range cmds {
		c.Contexts = &[]discordgo.InteractionContextType{discordgo.InteractionContextGuild}
		c.IntegrationTypes = &[]discordgo.ApplicationIntegrationType{discordgo.ApplicationIntegrationGuildInstall}
	}
	return cmds
}
