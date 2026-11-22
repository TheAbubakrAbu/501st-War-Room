# 501st War Room: Design Spec

Date: 2026-11-21
Status: design approved by Mand'alor (design page, parts 1 and 2); this spec awaits review
Design page: https://claude.ai/artifact/EJDDMy7r3WcrbaAZwU6vA3

## 1. Goal

A Discord bot for the 501st, a Helldivers 2 community run by Col. Mand'alor, that keeps the game's Galactic War map inside their server. The community is boycotting a fan-made companion app, so the bot has to give them what they used it for: where the war stands, what the current Major Order is, and how it is going.

Success means:

- One war-room message in a channel the admins choose, edited every 5 minutes with a fresh map picture, the Major Order and the busiest fronts. The channel never fills up.
- A ping when a new Major Order arrives, posted once per order, never twice.
- `/map`, `/major-order` and `/planet` answer instantly from memory.
- Old data never passes as fresh: when the feeds are down, every reply says how old its numbers are.
- It runs on the shared 1 GB Oracle VM in about 30 MB, beside Sabacc Droid and Aurebesh Droid, and ships the same way Aurebesh Droid does.

The user does not play Helldivers, so section 3 explains the game terms the spec uses.

## 2. Scope

**In v1:** the war-room message, the new Major Order announcement with an optional role ping, `/map`, `/major-order`, `/planet`, `/warroom set` and `/warroom clear`, `/help`, the community API as the main data source with Arrowhead's official API as an automatic backup, `--self-test`, `--preview`, and deployment to the Oracle VM.

**Not in v1** (listed on the design page as ideas for later): alerts when an enemy attacks a Super Earth planet, the game's news dispatches, war-wide statistics. The data layer is built so these can be added without restructuring.

**Not planned:** game art of any kind (the map is drawn by the bot), user-installed (DM) use, buttons or other interactive components.

## 3. Game terms

| Term | Meaning |
| --- | --- |
| Helldivers | The players. They fight for Super Earth. |
| Galactic War | One shared war that every player fights in. Planets change hands based on everyone's missions. |
| Factions | Super Earth (the players, called "Humans" in the data), Terminids (bugs), Automatons (robots), Illuminate (aliens). Data uses 1, 2, 3, 4 in that order. |
| Campaign | An active battle on a planet. Either a liberation (taking an enemy planet) or a defense. |
| Liberation % | How close the players are to taking an enemy planet. At 100% it flips to Super Earth. |
| Defense | An enemy attack on a Super Earth planet, on a timer. The players have to hold it or lose it. |
| Major Order | A mission from the developers for every player at once, usually a few days long, with a reward (usually Medals). The data calls it an assignment. |
| Supply lines | Links between planets (the data calls them waypoints). Battles spread along them. |

## 4. What the 501st sees

All replies follow the conventions Sabacc Droid and Aurebesh Droid share: titles are "<icon> <Title Case>", every embed has a colored bar and the logo as its thumbnail, notices only the user sees are "❌ " (or "✅ ") and a full sentence, text uses straight quotes and apostrophes, and nothing uses an em dash. Times inside embeds use Discord timestamps (`<t:UNIX:R>` for "in 1 day", `<t:UNIX:f>` for a date), which Discord shows in each reader's own time zone. Timestamps do not render in titles, field names or footers, so they only go in descriptions and field values.

Faction colors (the same on the map, embed bars and legend):

| Faction | Color | Square emoji |
| --- | --- | --- |
| Super Earth | `#4EA3FF` | 🟦 |
| Terminids | `#F2C230` | 🟨 |
| Automatons | `#FF5A4E` | 🟥 |
| Illuminate | `#B07BFF` | 🟪 |

The Major Order color is gold `#F2C230`.

### 4.1 The war-room message

One message per server, in the channel `/warroom set` picks. After every poll round the bot edits it in place, replacing the map attachment.

- Title: `📡 Galactic War Briefing`, Super Earth blue bar.
- Description: the Major Order (section 7 sets the task lines), as `**🎯 Major Order** · ends <t:X:R>`, then the briefing, then one line per task. With several orders, each gets its own block, up to 3. With none: "No Major Order right now. High Command will issue new orders soon."
- Field `⚔️ Top Fronts`: the 5 campaigns with the most Helldivers, ties broken by liberation %, one line each: `🟪 **Zea Rugosia** · 86.7% · 12,308 Helldivers`. A defense reads `🟪 **Alderidge Cove** · Defense 3.6% · 1,204 Helldivers`, using the attacker's color.
- Image: `attachment://map.png`, the latest map (section 8).
- Footer: `21,738 Helldivers deployed · Super Earth holds 212 of 273 planets · Updates every 5 minutes`, with the embed timestamp set to when the data was fetched.
- When the data came from the backup source, the footer adds ` · Backup feed`.
- When the data is more than 15 minutes old, the footer becomes `⚠️ War data from 25 minutes ago. The war feeds aren't answering.` (the minutes are filled in).
- Before the first successful poll, the message is a placeholder: `📡 Galactic War Briefing` with "Gathering war intel. This message fills in within a few minutes." and no image.

### 4.2 New Major Order announcement

After a successful poll round, for each server with a war room that is not paused (section 11), every Major Order whose ID is not in that server's announced list is posted as a new message in the war-room channel:

- Content: `<@&ROLE> New orders from Super Earth.` when a role is set, otherwise `New orders from Super Earth.` Allowed mentions are limited to that one role, so nothing else can ever ping.
- Embed `📣 New Major Order`, gold bar: the briefing as the description; one inline field per task (`Task 1`, `Task 2`, ... up to 9) with the task text; a `Reward` field ("40 Medals"); an `Ends` field (`<t:X:f>, <t:X:R>`).
- Footer: `Use /major-order for live progress`.

The order's ID is added to the announced list (the newest 20 IDs are kept) and saved before the next order is posted. When `/warroom set` first sets up a server, every order running at that moment goes straight into the announced list, so setting up the war room never announces an order that is already running. An order that started while the bot was offline is announced when it comes back, as long as the order is still running.

### 4.3 Commands

Registered globally with one bulk overwrite at every start (idempotent). Guild install only; the commands are not offered in DMs.

| Command | Who | Reply |
| --- | --- | --- |
| `/map` | Anyone | Public. Embed `🗺️ Galactic War Map` with the latest map as its image and the same footer as the war room. |
| `/major-order` | Anyone | Public. One embed per order, `🎯 Major Order`, gold bar: briefing, one field per task with its progress line, `Reward`, `Ends`. With none, the "No Major Order right now" text. |
| `/planet name` | Anyone | Public. See below. |
| `/warroom set channel [role]` | Manage Server | Private. See below. |
| `/warroom clear` | Manage Server | Private. See below. |
| `/help` | Anyone | In the channel, like the other two bots' `/help`. Embed `📖 501st War Room`: what the bot does, a `📡 War Room` field for `/warroom set` and `clear`, a `🧭 More Commands` field, then the same ending as the other two bots: a `🛰️ War Data` credits field ([Helldivers 2 Community API](https://github.com/helldivers-2/api), Arrowhead's official feed, helldivers-2/json), `🌌 More Star Wars` (Sabacc Droid, Aurebesh Droid and the two apps, word for word), `🛠️ Made By` with the same creator link, link buttons `➕ Add 501st War Room` (an invite for the bot that answered), `📱 Datapad App` and `📂 GitHub`, and the footer "Fan-made and not affiliated with or endorsed by Arrowhead Game Studios or Sony Interactive Entertainment. For Super Earth!" |

`/warroom` gets Discord's default member permission Manage Server, so Discord hides and blocks it for everyone else. Server admins can still grant it to other roles in Server Settings, which is intended.

Until the first successful poll, `/map`, `/major-order` and `/planet` reply privately: "❌ Gathering war intel. Try again in a minute."

**`/planet name`** has autocomplete. As the user types, it offers up to 25 planets whose name contains the text (case-insensitive): names starting with the text first, then the rest, each group ordered by Helldivers on the planet, then by name. Each choice shows the planet's name, and its value is the planet index. If the user submits text that is not a choice, the bot takes an exact name match, then the first name that starts with the text, and otherwise replies privately: "❌ There's no planet called "Xyz". Pick one from the list as you type." The reply embed, with the owner's color as the bar:

- Title `🪐 Zea Rugosia`, description `Ferris Sector` (plus ` · 🎯 Major Order target` when a task targets it).
- `Held By`: faction name with its square emoji.
- `Liberation` (enemy planets in a campaign): a 10-cell bar and the percentage.
- `Defense` (Super Earth planets under attack): the attacker, a 10-cell bar of defense progress, and `ends <t:X:R>`.
- `Helldivers`: the count on the planet now.
- `Biome` and `Hazards`, when the data has them (the community API does, the official one does not).
- `Supply Lines`: the names of every planet linked to it, in either direction, alphabetical.
- Footer as in the war room.

**`/warroom set channel [role]`**. The channel option only accepts text and announcement channels. The bot posts the war-room message in that channel (or edits its existing one if the war room is already there), then saves the server's settings. If an older war-room message exists in a different channel, the bot deletes it (best effort, a failure is only logged). The bot does not compute permissions ahead of time; if Discord refuses the post (error 50001 or 50013), the reply is "❌ I can't post in #channel. Give me View Channel, Send Messages, Embed Links and Attach Files there, then run /warroom set again." On success: "✅ The war room is now in #channel. It updates every 5 minutes." plus " New Major Orders ping @Role." when a role is given. If that role is not mentionable, the reply adds: "Heads up: @Role can only be pinged if it allows anyone to mention it, or if I have the Mention All Roles permission." Running `/warroom set` again also clears a paused war room (section 11).

**`/warroom clear`** deletes the war-room message (best effort) and the server's settings: "✅ The war room is off. Run /warroom set to turn it back on." With no war room: "❌ This server has no war room. Run /warroom set to make one."

The bot's presence is `Watching 🪐 the Galactic War`.

## 5. Data sources

Both are free and public. Neither is the boycotted app. One background poller fetches every 5 minutes; commands never call an API.

### 5.1 Community API (main)

Base `https://api.helldivers2.dev`. Every request sends:

- `X-Super-Client: 501st-war-room`
- `X-Super-Contact`: the `SUPER_CONTACT` environment variable if set, otherwise `501st-war-room`. The user decides whether to put a real contact there; the code never contains one.
- `Accept-Language: en-US`, and `User-Agent: 501st-war-room/<version>`.

Its rate limit is 5 requests per 10 seconds (`x-ratelimit-limit: 5`, `retry-after: 10`). A round makes 3 requests, one after another, each with a 15 second timeout:

| Endpoint | Fields used |
| --- | --- |
| `/api/v1/planets` | `index`, `name` (upper case, e.g. "ZEA RUGOSIA"), `sector`, `position.x/y`, `waypoints`, `health`, `maxHealth`, `currentOwner` (string), `event` (a defense: `faction`, `health`, `maxHealth`, `endTime`), `statistics.playerCount`, `biome.name`, `hazards[].name` |
| `/api/v1/campaigns` | `planet.index`, `type` |
| `/api/v1/assignments` | `id`, `title`, `briefing`, `tasks[].type/values/valueTypes`, `progress[]`, `reward.type/amount`, `expiration` (ISO time) |

The design page said 4 requests; `/api/v1/war` turned out to add nothing the bot shows (total Helldivers is the sum of the planets' player counts), so it is dropped. The API's `now` field is not a real date (it read 1972 on 2026-11-21), so the bot never uses it. A campaign's own `faction` field is not the enemy either; the enemy is the planet's `event.faction` for a defense, otherwise the planet's `currentOwner`.

### 5.2 Arrowhead's official API (backup)

Base `https://api.live.prod.thehelldiversgame.com/api`, with `Accept-Language: en-US`. It needs no keys. A round makes 3 requests, 15 second timeout each:

| Endpoint | Fields used |
| --- | --- |
| `/WarSeason/{war}/WarInfo` | `planetInfos[]`: `index`, `position`, `waypoints`, `maxHealth` |
| `/WarSeason/{war}/Status` | `time` (game clock, seconds), `planetStatus[]`: `index`, `owner` (1-4), `health`, `players`; `campaigns[].planetIndex`; `planetEvents[]`: `planetIndex`, `race`, `health`, `maxHealth`, `expireTime` |
| `/v2/Assignment/War/{war}` | `id32`, `setting.overrideTitle`, `setting.overrideBrief`, `setting.tasks`, `setting.reward`, `progress`, `expiresIn` (seconds left) |

`{war}` comes from `/WarSeason/current/WarID` (it returned `{"id":801}` on 2026-11-21). The bot asks once at startup and then once an hour, and uses 801 if that request has never succeeded.

Game times become real times relative to the fetch, which avoids converting the game clock: a defense ends at `fetched + (expireTime - status.time)` seconds, an order ends at `fetched + expiresIn` seconds.

The official API has no names, only numbers. Names come from section 5.3.

### 5.3 Planet names

A check on 2026-11-21 found that the community's names file (`helldivers-2/json`, `planets/planets.json`) has two planets swapped compared with the live game: it calls index 99 "Alderidge Cove" and index 273 "Sangis", while the community API has them the other way round. The bot follows the community API, its main source, so both sources always show the same name for the same planet:

- `internal/source/planets.json` is embedded in the binary: index, name and sector for every planet, generated from the community API by `go run ./tools/planets` and committed. That tool takes each name's capitalization from the `helldivers-2/json` file, matched by name rather than by index ("CERBERUS IIIC" becomes "Cerberus IIIc", "WIDOW'S HARBOR" becomes "Widow's Harbor").
- At run time, every successful community round refreshes the names in memory, and when anything changed, saves them to `data/planets.json`, which is loaded at startup in place of the embedded copy. So the backup source uses the newest names the bot has seen.
- A name the embedded file has never seen is title-cased word by word, keeping Roman numerals upper case ("NEW PLANET IV" becomes "New Planet IV").
- A planet index with no name at all shows as "Planet 263".

### 5.4 Fallback rules

Each round tries the community API, then the official API, and uses the first that succeeds completely. A source fails the round if any of its requests times out, returns a non-200 status (429 included), returns JSON that does not parse, or the result fails the sanity checks: at least 200 planets, every owner between 1 and 4, every position within -1.5 to 1.5. A failed source's partial data is thrown away whole, so a snapshot is never half one round and half another.

If both fail, the bot keeps the last good snapshot and the footers show its age. Rounds run every 5 minutes; until the first success after startup, they retry every 30 seconds so the "Gathering war intel" notice lasts as short as possible. Each failed source logs a `WARN` line. Going 15 minutes without good data (3 failed rounds once there is data, or about 30 quick retries at startup) logs one `ERROR` line, and at most one more per hour while it lasts, so the daily error report notices an outage without filling up. A map that fails to draw is not an outage: the new data goes out with the last good map, and the error is logged at most hourly.

## 6. Data model (`internal/war`)

Plain Go types and pure functions, no network, so all of it is unit tested:

```go
type Faction int // 1 SuperEarth, 2 Terminids, 3 Automatons, 4 Illuminate

type Planet struct {
    Index      int
    Name       string
    Sector     string
    X, Y       float64
    Owner      Faction
    Health     int64
    MaxHealth  int64
    Players    int
    Waypoints  []int
    InCampaign bool
    Defense    *Defense // nil unless under attack
    Biome      string   // empty when the source has none
    Hazards    []string
}

type Defense struct {
    Enemy     Faction
    Health    int64
    MaxHealth int64
    Ends      time.Time
}

type Order struct {
    ID       int64
    Title    string
    Briefing string
    Tasks    []Task
    Reward   string    // "40 Medals"
    Ends     time.Time
}

type Snapshot struct {
    Fetched time.Time
    Source  string // "community" or "official"
    Planets []Planet
    Orders  []Order
}
```

Functions: `(*Snapshot).Planet(index)`, `Fronts(n)` (section 4.1 ordering), `Held()` (Super Earth planets and total), `Players()` (sum over planets), `Targets()` (planet indexes any task points at), `Links(index)` (supply-line neighbors in either direction), `(Planet).Liberation()` (`100 × (1 − Health/MaxHealth)` for an enemy planet), `(Planet).DefenseProgress()` (the same formula on the defense event), `(Planet).Enemy()` (the defense attacker, otherwise the owner), and the faction names, colors and emoji.

## 7. Major Order tasks

Each task has a `type`, a list of `values` and a parallel list of `valueTypes` that says what each value means. The bot reads values by their value type, never by position:

| Value type | Meaning |
| --- | --- |
| 1 | faction (race) |
| 3 | goal (a count) |
| 4 | unit (enemy type ID, 0 means any) |
| 9 | difficulty |
| 11 | location type (1 means a planet) |
| 12 | location index (the planet's index) |

A task points at a planet when its location type is 1. Its progress is the matching entry of the order's `progress` list.

| Task type | Text | Progress line |
| --- | --- | --- |
| 3 Eradicate | `Kill 215,000,000 Voteless` (unit name), adding ` on <planet>` when it has one | bar, %, count |
| 11 Liberation | `Liberate <planet>`, or `Liberate 5 planets` | planet tasks: ✅ when held, otherwise its liberation %; count tasks: bar |
| 13 Control | `Hold <planet>`, or `Hold 5 planets` | same as Liberation |
| 12 Defense | `Defend <planet>`, or `Defend 5 planets from the Automatons` | bar or ✅ |
| 7 CompleteMissions | `Complete 500,000 missions`, adding ` against the <faction>`, ` on <planet>`, ` on Difficulty 7 or higher` when present | bar, %, count |
| 9 CompleteOperations | the same with "operations" | bar, %, count |
| 2 Extract | `Complete 1,000,000 extractions`, with the same additions | bar, %, count |
| 15 Expand | `Take 5 planets from the <faction>` | bar, %, count |
| anything else | `Objective 2 (see the briefing above)` | bar and % when it has a goal |

A bar is 10 cells, `▰` filled and `▱` empty, in a code span: `` `▰▰▰▰▰▰▰▰▰▱` 96.8% (208,156,482) ``. A finished task starts with ✅, an open one with ○.

Task types 3, 11 and 13 have been seen in live orders (2026-11-19 and 2026-11-21). The texts for 2, 7, 9, 12 and 15 follow the community's documentation (`helldivers-2/json`, `assignments/tasks`) and get corrected against real orders when they show up.

Unit names are a small built-in table, matched against live orders and their briefings: `4211847317` Voteless, `878778730` Troopers, `1153658728` Factory Striders, `1379865898` Bile Spewers. A unit not in the table reads `Kill 25,000,000 Terminid targets (see the briefing)`, and unit 0 reads as the faction (`Kill 250,000,000 Terminids`). Reward type 1 is Medals; another type shows as `40 (reward type 3)`.

Briefings are trimmed, and the game's highlight tags (`<i=1>text</i>`) become bold text. Text that would go over a Discord limit is cut at a word boundary and ends with "…".

## 8. The map picture (`internal/render`)

A 1200 × 1200 PNG drawn by the bot with Go's image packages and `golang.org/x/image` (vector shapes with anti-aliasing, and the Go fonts, which ship under a BSD license). It matches the map Mand'alor saw on the design page:

- Background: near-black space `#060A12` with a soft radial glow, a fixed scatter of faint stars (a seeded random generator, so every picture has the same sky), and faint distance rings around Super Earth.
- Supply lines: thin lines between linked planets, drawn once per pair. Lines touching an enemy planet take that enemy's color at low opacity; lines touching an active battle are brighter.
- Planets: dots in the owner's color, Super Earth planets slightly smaller and translucent so the fronts stand out. Super Earth itself is a larger blue dot with a white center.
- Active battles: a ring in the enemy's color with a soft glow, and a blue arc on top that fills clockwise from 12 o'clock with liberation % (or defense progress).
- Major Order targets: white corner brackets around the planet and a small `MAJOR ORDER` tag.
- Labels: the top 5 fronts and every Major Order target get their name and a line like `86.7% · 12,308`, white text with a dark outline so it reads over anything.
- Legend box in a corner: the four factions, BATTLE, MAJOR ORDER, and "BLUE ARC = % LIBERATED".

Positions are placed exactly as on the design page: Super Earth (0, 0) at the center, every coordinate multiplied by `(600 - 92) / m`, where `m` is the largest absolute x or y of any planet (so the farthest planet sits 92 px from the edge), and y pointing up (`pixelY = 600 - y × scale`). The picture is drawn once per round and the bytes are kept; `/map`, the war room and the self-test all use those bytes. The self-test holds it under 1 MB and under 1 second to draw on the CI runner. If a plain PNG goes over 1 MB, it switches to a 256-color palette PNG, as Aurebesh Droid does.

## 9. Architecture

One Go program, `warroom`, in six packages under `internal/`, each with one job:

| Package | Job | Depends on |
| --- | --- | --- |
| `war` | Types and math from section 6, Major Order text from section 7. | nothing |
| `source` | `Community`, `Official` and `Fallback`, each a `Source`; the planet names from 5.3. Turns both APIs into a `war.Snapshot`. | `war` |
| `render` | `Map(*war.Snapshot) ([]byte, error)`. | `war` |
| `poller` | The loop: fetch, swap, draw, notify. | `source`, `render`, `war` |
| `store` | Per-server settings on disk. | nothing |
| `bot` | Discord: commands, autocomplete, the war room, announcements. Reply builders are pure functions of the poller state, so the self-test can run them without Discord. | `poller`, `store`, `war` |

Plus `internal/selftest` and `cmd/warroom/main.go`.

```go
// source
type Source interface {
    Name() string
    Fetch(ctx context.Context) (*war.Snapshot, error)
}

// poller
type State struct {
    Snap    *war.Snapshot // nil before the first success
    Map     []byte        // PNG for Snap
    Failing int           // rounds in a row with both sources down
}
func (p *Poller) Run(ctx context.Context)  // calls onRound(*State) after every round
func (p *Poller) Current() *State          // atomic read, never blocks
```

**Each round:** 1) fetch through `Fallback`; 2) on success, draw the map, then swap in a new `State` with one atomic pointer store, so a reader never sees new data with an old map; on failure, swap in a copy with `Failing` raised; 3) call `onRound`, which edits every server's war-room message and then posts any new Major Order announcements.

**Concurrency:** commands run in discordgo's handler goroutines and only read `Current()`. War-room work (round updates, `/warroom set`, `/warroom clear`) runs under one mutex, so two of them never touch the same message at once. The store has its own mutex.

**Startup:** read the environment, open the store, load the names, connect to Discord (intent: Guilds only, no message content), register the commands, start the poller. **Shutdown** on SIGTERM or SIGINT: stop the poller, let a running war-room update finish (at most 20 seconds), close the Discord session. The unit's `TimeoutStopSec=30` covers it.

**Discord library:** `github.com/bwmarrin/discordgo` v0.29.0. It is the most widely used Go Discord library, with the most examples to learn from, and it covers everything v1 needs: slash commands with autocomplete, embeds, attachments on edit, and waiting out Discord's rate limits on its own. Its last release is from May 2025; it stays behind the `bot` package so it could be swapped for `disgoorg/disgo` (active, v0.20.0) without touching anything else.

## 10. Storage (`internal/store`)

One small JSON file per server, `data/servers/<guild id>.json`:

```json
{
  "guild": "123",
  "channel": "456",
  "message": "789",
  "role": "",
  "announced": [1278711574, 2890701424],
  "paused": ""
}
```

Writes go to a temporary file in the same folder, are synced to disk, then renamed over the old file, so a crash leaves either the old file or the new one, never half of one. At startup every file is loaded; a file that does not parse is renamed to `<id>.json.bad`, logged as an `ERROR`, and skipped. When the bot is removed from a server (a guild delete event that is not an outage), that server's file is deleted. Nothing about users is stored: only server, channel, message and role IDs, and order IDs.

`data/planets.json` (section 5.3) lives in the same folder. The data folder is `data` by default (`--data` flag); locally it is `data.nosync` so iCloud skips it.

## 11. When things break

| What happens | What the bot does |
| --- | --- |
| Community API down or slow | Uses the official API for that round; footer shows "Backup feed". |
| Both sources down | Keeps the last good data; footers show its age once it is over 15 minutes old; logging as in 5.4. |
| A source returns partial or nonsense data | That source fails the round (5.4); nothing from it is used. |
| Bot just started | Data commands reply "❌ Gathering war intel. Try again in a minute."; rounds retry every 30 seconds until the first success. |
| War-room message deleted (error 10008) | Posts a fresh one in the same channel and saves its ID. |
| Bot lost access to the channel, or the channel is gone (50001, 50013, 10003) | Logs a `WARN`, marks the server paused, and tries again once an hour (a refused try logs only `INFO`); `/warroom set` tries at once. |
| Any other edit failure (network, Discord 5xx) | Logs a `WARN` and tries again next round. |
| Bot restarts | The announced list on disk means no order is posted twice. |
| A Major Order task it can't read | The fallback text from section 7; the briefing above it always shows. |
| A planet index with no name | "Planet 263". |
| Discord rate limits | discordgo waits and retries. |
| A command handler panics | Recovered, logged as an `ERROR` with the stack, and the user gets "❌ Something went wrong. Try again in a moment." |

## 12. Logging

Plain lines to standard output, which journald stores: `INFO: ...`, `WARN: ...`, `ERROR: ...`, with details as `key=value` after the message. The `ERROR:` form matches the pattern `~/bot-alerts.py` already uses for the daily error report, so the report needs no change. The first line at startup names the version (the build's commit).

## 13. Testing

**Fixtures.** Real replies saved on 2026-11-21 from both sources at nearly the same moment (community planets, campaigns, assignments; official WarInfo, Status, Assignment), plus the official Assignment reply from 2026-11-19 (a Control task and Voteless). They live in `internal/source/testdata/`.

**Unit tests** (`go test ./...`):

- Both sources parse their fixtures into snapshots that agree exactly on the planet count, positions and supply lines. Owners and campaigns are not compared, since the war can move between the two captures.
- Liberation and defense math, `Fronts` ordering and ties, `Held`, `Links` in both directions.
- Every task type in section 7, both live orders, unknown types, unknown units, reward types.
- Names: the 99/273 case, capitalization, Roman numerals, unknown indexes.
- `Fallback` against `httptest` servers: community 503, community timeout, community 429, bad JSON, failed sanity check, both down (keeps the last snapshot).
- Store: round trip, crash safety of the rename, a corrupt file renamed to `.bad`.
- Autocomplete ranking and the submit-without-choosing rules.
- Game-time conversion for the official source.

**`--self-test`** pushes the fixtures through the whole bot without Discord or the network: both sources, the map, and every reply (briefing in every state: normal, backup, stale, placeholder, no order, three orders; `/map`; `/major-order`; `/planet` for every planet; the announcement; `/help`; every notice). It checks each against Discord's limits (title 256, description 4096, 25 fields, field name 256, value 1024, footer 2048, 6000 characters per message, 10 embeds; autocomplete choice names 100 characters and 25 choices) and against the conventions (titles start with an icon, every embed has the logo as its thumbnail, straight quotes only, no em dash, user-only notices start with ❌ or ✅), checks the map's size and draw time, and ends with "All checks passed" or "N problems" (exit status 0 or 1), like the other two bots. Nothing deploys unless it passes.

**`--preview out.png`** fetches live data (or the fixtures with `--fixtures`), writes the map PNG, and prints the briefing embed as JSON, for checking by eye.

## 14. Repository

New public GitHub repo `501st-War-Room` (public so the server can download builds without a password), MIT license like the other two. Module path `github.com/TheAbubakrAbu/501st-War-Room`, Go 1.27.

```
501st-War-Room/
  cmd/warroom/main.go          flags: --self-test, --preview, --fixtures, --data
  internal/war/
  internal/source/             community.go, official.go, fallback.go, names.go, planets.json, testdata/
  internal/render/
  internal/poller/
  internal/store/
  internal/bot/                commands.go, warroom.go, announce.go, replies.go, theme.go
  internal/selftest/
  tools/planets/               regenerates internal/source/planets.json
  tools/logo/                  draws logo.png
  .github/workflows/deploy.yml
  start-worker.sh              same as Aurebesh Droid's, for this repo
  run.sh                       loads .env and runs the bot locally with data.nosync
  README.md  LICENSE  logo.png  .gitignore  go.mod  go.sum
```

`theme.go` holds the colors, icons, the logo, the creator link, the More Star Wars links and disclaimer, and the conventions comment block the other two bots carry. The logo is drawn by `render.Logo` in the map's style (1024 x 1024, everything inside a round rim since Discord crops avatars round), saved as `logo.png` at the repository root, and served to Discord from raw.githubusercontent.com like the other bots' thumbnails, so it shows once the repository is public. The README uses the same layout as the other bots (the add link and logo under the title, then Features, Commands, Getting Started, Project Layout, Hosting, More Star Wars, Privacy & Data, License, Feedback & Contact). `.gitignore` covers `.env`, `data/`, `data.nosync/`, `LinuxBinary` and any built `warroom` binary. Builds use `go run` or `-o` outside the project, so no binaries land in iCloud.

**Configuration:** `DISCORD_TOKEN` (required) and `SUPER_CONTACT` (optional) from the environment; `.env` locally (read by `run.sh`) and on the server (read by systemd).

Dependencies: `github.com/bwmarrin/discordgo` and `golang.org/x/image`. Nothing else.

## 15. Deploying

Copied from how Aurebesh Droid ships:

1. `.github/workflows/deploy.yml`, named "Test and Deploy 501st War Room", runs on every push to main and by hand. The `test` job: `actions/setup-go` with `go-version-file: go.mod`, `go vet ./...`, `go test ./...`, build with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=<sha>"`, run `--self-test`, upload the binary as an artifact. The `deploy` job: publish it as `LinuxBinary` on the `nightly` pre-release (never "latest"), then SSH to the server with the deploy key and run `warroom <tested sha>`. Same action versions and the same pinned host key line as Aurebesh's workflow.
2. On the server: `~/501st-War-Room` (a clone of the repo, for `start-worker.sh` and the deploy script), `.env` with the token, and the `data/` folder.
3. `war-room.service`: a copy of `aurebesh-droid.service` running `start-worker.sh`, plus `MemoryMax=100M` so it can never squeeze the other two bots, and `Environment=GOMEMLIMIT=60MiB` so Go's garbage collector works to stay well under that. Expected use is about 30 MB.
4. **Needs the user's yes at the time, like every server change:** a `warroom` target in `~/deploy.sh`; a `war-room` entry in `~/bot-alerts.py` (alerts sent with Aurebesh Droid's token); the `Server/` backup folder updated (`files/war-room.service`, `setup.sh` and `restore.sh` learn the third bot, the README table gets a row), then `backup.sh`.

The repo needs the same two secrets as the others: `ORACLE_HOST` and `ORACLE_SSH_KEY`.

Invite link scopes `bot applications.commands` with permissions View Channel, Send Messages, Embed Links, Attach Files and Read Message History (`117760`). Mention All Roles is only needed if the ping role is not mentionable, and the admins can grant it separately.

## 16. Needed from the user

- A new Discord application named "501st War Room" (Mand'alor can pick another name and picture later; nothing in the code depends on them), owned by the same account or team as the other two bots so down alerts reach the same person. Its token goes straight into `501st-War-Room/.env`, never into chat.
- A private test server to try it in before it joins the 501st's server.
- The empty `501st-War-Room` GitHub repo and its two secrets, when it is time to deploy.
- A yes before the server changes in 15.4.

## 17. Changes from the design page

Small calls made while writing this spec, none of which change what Mand'alor approved:

- Titles carry icons (`📡 Galactic War Briefing`, `📣 New Major Order`) to match the other two bots' conventions. The briefing's icon moved from 🌌 to 📡 so 🌌 can head More Star Wars in `/help`, as it does in the other two bots.
- 3 requests per round instead of 4 (5.1).
- Planet names come from the community API instead of the names file, because of the 99/273 swap (5.3). The design page's map used the names file, so its labels for those two planets may be swapped.
- `/help` added, since both other bots have one and it is where the data sources get credit.
- The official source asks for the current war ID instead of assuming 801, so a new war season doesn't break the backup.
