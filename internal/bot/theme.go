// Package bot is the Discord side of the 501st War Room: replies, planet
// search, the war-room messages and the slash commands.
package bot

// What every reply shares, so the bot looks the same everywhere: its colors,
// its logo, and one icon per feature. Each icon is used in that feature's
// titles and in /help.
//
// Conventions for replies, the same in Aurebesh Droid and Sabacc Droid:
//   - Titles are "<icon> <Title Case>".
//   - Every embed has a colored bar and the logo as its thumbnail.
//   - Buttons have an icon and a Title Case label.
//   - Notices only the user sees are "❌ " (or "✅ ") and a full sentence.
//   - Text uses straight quotes and apostrophes.
//   - /help ends with the credits (War Data), More Star Wars, and Made By
//     fields, link buttons to add the bot, the Datapad app, and GitHub, and
//     the disclaimer below as its footer.

const (
	ColorSuperEarth = 0x4EA3FF // the bot's color, Super Earth blue
	ColorOrder      = 0xF2C230 // Major Orders, gold

	IconBriefing = "📡"
	IconOrder    = "🎯"
	IconFronts   = "⚔️"
	IconMap      = "🗺️"
	IconPlanet   = "🪐"
	IconNewOrder = "📣"
	IconHelp     = "📖"
	IconCommands = "🧭"
	IconWarData  = "🛰️"
	IconMadeBy   = "🛠️"
	IconAddBot   = "➕"
	IconApp      = "📱"
	IconGitHub   = "📂"

	GitHubLink = "https://github.com/TheAbubakrAbu/501st-War-Room"
	repoFiles  = "https://raw.githubusercontent.com/TheAbubakrAbu/501st-War-Room/main/"
	// Logo is every embed's thumbnail: logo.png in the repository, drawn by
	// tools/logo.
	Logo = repoFiles + "logo.png"
	// View Channel, Send Messages, Embed Links, Attach Files and Read
	// Message History.
	invitePermissions = "117760"

	// Shared word for word with Sabacc Droid's and Aurebesh Droid's /help.
	Creator           = "[Abubakr Elmallah](https://abubakrelmallah.com)"
	SabaccDroidLink   = "https://discord.ly/sabaac-droid"
	AurebeshDroidLink = "https://discord.ly/aurebesh-droid"
	DatapadApp        = "https://apps.apple.com/us/app/datapad-aurebesh-translator/id6450498054?platform=iphone"
	TranslatorApp     = "https://apps.apple.com/us/app/aurebesh-translator/id6670201513?platform=iphone"
	moreStarWars      = "- **[Sabacc Droid](" + SabaccDroidLink + ")**: Play Sabacc, the Star Wars card game, with up to 8 players.\n" +
		"- **[Aurebesh Droid](" + AurebeshDroidLink + ")**: Translate text into Aurebesh and 13 more Star Wars scripts.\n" +
		"- **[Datapad | Aurebesh Translator](" + DatapadApp + ")**: An immersive Aurebesh translator with a themed keyboard.\n" +
		"- **[Aurebesh Translator](" + TranslatorApp + ")**: A free, offline, ad-free translator."

	Disclaimer = "Fan-made and not affiliated with or endorsed by Arrowhead Game Studios or Sony Interactive Entertainment. For Super Earth!"
	Presence   = "🪐 the Galactic War"
)

// InviteLink adds the bot with appID to a server, with the permissions the
// war room needs.
func InviteLink(appID string) string {
	return "https://discord.com/oauth2/authorize?client_id=" + appID + "&permissions=" + invitePermissions + "&scope=bot+applications.commands"
}

// Notices (spec 4.3 and 11).
const (
	NoticeGathering = "❌ Gathering war intel. Try again in a minute."
	NoticeBroken    = "❌ Something went wrong. Try again in a moment."
	NoticeNoMap     = "❌ The war map isn't drawn yet. Try again in a few minutes."
	NoticeNoPlanet  = "❌ There's no planet called \"%s\". Pick one from the list as you type."
	NoticeNoAccess  = "❌ I can't post in %s. Give me View Channel, Send Messages, Embed Links and Attach Files there, then run /warroom set again."
	NoticeSet       = "✅ The war room is now in %s. It updates every 5 minutes."
	// NoticeSetRole and NoticeRoleHeadsUp are appended to NoticeSet.
	NoticeSetRole     = " New Major Orders ping %s."
	NoticeRoleHeadsUp = " Heads up: %s can only be pinged if it allows anyone to mention it, or if I have the Mention All Roles permission."
	NoticeCleared     = "✅ The war room is off. Run /warroom set to turn it back on."
	NoticeNoWarRoom   = "❌ This server has no war room. Run /warroom set to make one."
)

// Texts the replies share.
const (
	textNoOrder   = "No Major Order right now. High Command will issue new orders soon."
	textGathering = "Gathering war intel. This message fills in within a few minutes."
	textNoFronts  = "No active battles right now."
	textNewOrders = "New orders from Super Earth."
	textUpdates   = "Updates every 5 minutes"
	textBackup    = "Backup feed"
	textLiveOrder = "Use /major-order for live progress"
	// The war room's footer when announcements are refused.
	textAlertsBlocked = "⚠️ I can't post Major Order alerts in this channel. Give me Send Messages and Embed Links here. I'll try again every hour, or run /warroom set to try now."
	mapFile           = "map.png"
	maxDescription    = 4096
	maxEmbedsTotal    = 6000 // characters across all of a message's embeds
	maxFieldValue     = 1024
	maxBriefOrders    = 3
	maxAnnounceTask   = 9
)

func titled(icon, text string) string { return icon + " " + text }
