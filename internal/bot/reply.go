package bot

import (
	"bytes"

	"github.com/bwmarrin/discordgo"
)

// Reply is one message the bot sends, in a form that turns into each of
// Discord's message shapes.
type Reply struct {
	Content    string
	Embeds     []*discordgo.MessageEmbed
	Map        []byte // attached as map.png when set
	PingRole   string // the only role the message may ping
	Private    bool   // only the user who asked sees it
	Components []discordgo.MessageComponent
}

// files attaches the map. Each call makes a fresh reader, so a reply can
// be sent more than once.
func (r Reply) files() []*discordgo.File {
	if len(r.Map) == 0 {
		return nil
	}
	return []*discordgo.File{{Name: mapFile, ContentType: "image/png", Reader: bytes.NewReader(r.Map)}}
}

// mentions allows no pings at all, except the one ping role.
func (r Reply) mentions() *discordgo.MessageAllowedMentions {
	m := &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}}
	if r.PingRole != "" {
		m.Roles = []string{r.PingRole}
	}
	return m
}

func (r Reply) embeds() []*discordgo.MessageEmbed {
	if r.Embeds == nil {
		return []*discordgo.MessageEmbed{}
	}
	return r.Embeds
}

func (r Reply) components() []discordgo.MessageComponent {
	if r.Components == nil {
		return []discordgo.MessageComponent{}
	}
	return r.Components
}

// Send is the reply as a new channel message.
func (r Reply) Send() *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: r.Content, Embeds: r.Embeds, Components: r.Components, Files: r.files(), AllowedMentions: r.mentions()}
}

// Edit is the reply as an edit of an existing message. It drops the old
// attachments, so the new map replaces the old one instead of joining it.
// That works because discordgo speaks API v9, where an empty attachments
// list keeps none of the old files and the new ones are added. Under v10
// the new file must be listed too ({id "0", filename map.png}), and
// TestEditReplacesMap fails the day discordgo moves.
func (r Reply) Edit(channel, message string) *discordgo.MessageEdit {
	content, embeds, comps := r.Content, r.embeds(), r.components()
	return &discordgo.MessageEdit{
		Channel: channel, ID: message,
		Content: &content, Embeds: &embeds, Components: &comps,
		Attachments:     &[]*discordgo.MessageAttachment{},
		Files:           r.files(),
		AllowedMentions: r.mentions(),
	}
}

// Response is the reply as an answer to a slash command.
func (r Reply) Response() *discordgo.InteractionResponseData {
	d := &discordgo.InteractionResponseData{Content: r.Content, Embeds: r.Embeds, Components: r.Components, Files: r.files(), AllowedMentions: r.mentions()}
	if r.Private {
		d.Flags = discordgo.MessageFlagsEphemeral
	}
	return d
}

// WebhookEdit is the reply as the answer to a deferred slash command.
func (r Reply) WebhookEdit() *discordgo.WebhookEdit {
	content, embeds, comps := r.Content, r.embeds(), r.components()
	return &discordgo.WebhookEdit{Content: &content, Embeds: &embeds, Components: &comps, Files: r.files(), AllowedMentions: r.mentions()}
}
