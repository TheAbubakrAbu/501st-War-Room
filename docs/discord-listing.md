# Discord Listing

The text on 501st War Room's pages in the Discord Developer Portal (application ID `1574183300782886962`), kept here so it can be restored or matched by the other bots ([Sabacc Droid](https://github.com/TheAbubakrAbu/Sabacc-Droid/blob/main/docs/discord-listing.md) and [Aurebesh Droid](https://github.com/TheAbubakrAbu/Aurebesh-Droid/blob/main/docs/discord-listing.md)).

It follows the same pattern as theirs, with two differences: the website is the main site rather than `/star-wars/`, and the tags leave out Star Wars and Droid, because this is a Helldivers 2 bot.

## General Information

**App icon:** [`logo.png`](../logo.png)

**Description** (maximum 400 characters, shown in the bot's About Me):

```
Follow the Helldivers 2 Galactic War right in your server. Get a live galaxy map, the current Major Order, and the busiest fronts in one war room post that updates itself every 5 minutes.

Look up any planet, get pinged when a new Major Order drops, and dive where you're needed most. For Super Earth!

Created by Abubakr Elmallah
contact@abubakrelmallah.com
https://abubakrelmallah.com/
```

**Tags** (maximum 5): Helldivers 2, Galactic War, Map, Major Order, 501st

## Installation

- **Installation Contexts:** Guild Install only. Unlike Sabacc Droid, the commands are registered for servers only (`internal/bot/commands.go`), and the war room needs the bot in the server.
- **Install Link:** Discord Provided Link
- **Guild Install scopes:** applications.commands, bot
- **Guild Install permissions:** View Channels, Send Messages, Embed Links, Attach Files, Read Message History (`117760`, the same as the README's invite link)

## OAuth2

Nothing set, as on Sabacc Droid.

## Bot

- **Icon:** [`logo.png`](../logo.png)
- **Banner:** [`banner.png`](../banner.png) (1360 x 480, Discord's 17:6 profile banner at twice the size). Both pictures are drawn by `go run ./tools/logo`.
- **Username:** 501st War Room
- **Public Bot:** on, so the Add button in `/help` and the Discord Provided Link work for any server
- **Requires OAuth2 Code Grant:** off
- **Privileged Gateway Intents:** all off; the bot only uses the Guilds intent (`internal/bot/bot.go`)
- **Bot Permissions:** left empty; it is only a calculator and saves nothing

## App Directory

**Short description:**

```
Follow the Helldivers 2 Galactic War on Discord! Get a live galaxy map, the current Major Order, and the busiest fronts in a war room that updates itself every 5 minutes.
```

**Long description:**

```
# 501st War Room | Your Helldivers 2 Galactic War Command Center!

Bring the Galactic War to your Discord server! Follow the fight for Super Earth without leaving the chat: see the whole galaxy on a live map, track the current Major Order, and find out where Helldivers are needed most.

📡 **Features**

• **Live War Room:** One message in the channel of your choice, updated every 5 minutes with the Major Order, the five busiest fronts, and a fresh war map. The channel never fills up.

• **Galactic War Map:** Every planet in its owner's color, supply lines, active battles ringed with their liberation progress, and Major Order targets marked.

• **Major Order Alerts:** Each new Major Order is announced once, with an optional role ping.

• **Planet Lookup:** Owner, liberation or defense progress, Helldivers on the planet, biome, hazards, and supply lines, with names suggested as you type.

• **Always On:** If the community API goes down, the bot switches to Arrowhead's official feed on its own. If both are down, it keeps the last good data and says how old it is.

• **Easy to Use:** Access everything directly through Discord slash commands.

🧭 **Commands**

`/warroom set` - Put the auto-updating war room in a channel, with an optional role to ping for new Major Orders (needs Manage Server).

`/warroom clear` - Turn the war room off.

`/map` - Post the latest war map.

`/major-order` - Show the current Major Order with each task's progress, the reward, and the time left.

`/planet` - Look up any planet.

`/help` - Learn how to use the 501st War Room.

Whether you're a seasoned Helldiver or fresh out of training, 501st War Room keeps your server briefed and ready to drop.

For Super Earth!

Created by Abubakr Elmallah

Website: https://abubakrelmallah.com/

Support: contact@abubakrelmallah.com
```

**Directory tags:** Helldivers 2, Galactic War, Gaming, Utility, Strategy
