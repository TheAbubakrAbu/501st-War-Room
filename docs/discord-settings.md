# Discord Settings

Everything to set in the [Discord Developer Portal](https://discord.com/developers/applications) for **501st War Room** (application ID `1574183300782886962`), page by page. It is written so a browser assistant such as Claude in Chrome can follow it, and a person can follow it just as well. The text on each page is also recorded in [`discord-listing.md`](discord-listing.md).

## Rules for the Assistant

- Only change the application named **501st War Room**, application ID `1574183300782886962`. Check the app picker at the top left on every page. Never change Sabacc Droid, Aurebesh Droid or any other application.
- Never click **Reset Token**, **Reset Secret** or **Delete App**. Never copy, read out, or paste a token or secret anywhere. The owner resets the bot token themselves.
- Click **Save Changes** at the bottom of each page before going to the next one, and check that the "unsaved changes" bar is gone.
- If a step needs a file uploaded and you cannot pick files, stop and ask the owner to upload it. Both files are in the `501st-War-Room` folder: `logo.png` and `banner.png`.
- If a page does not match this file (a field is missing, or Discord shows an error), stop and report what you see instead of guessing.
- Leave everything this file does not mention as it is.

## 1. General Information

- **App Icon:** `logo.png`
- **Name:** `501st War Room`
- **Description:** replace whatever is there with exactly this, keeping the blank lines:

  ```
  Follow the Helldivers 2 Galactic War right in your server. Get a live galaxy map, the current Major Order, and the busiest fronts in one war room post that updates itself every 5 minutes.

  Look up any planet, get pinged when a new Major Order drops, and dive where you're needed most. For Super Earth!

  Created by Abubakr Elmallah
  contact@abubakrelmallah.com
  https://abubakrelmallah.com/
  ```

- **Tags:** exactly these five, typing each one and pressing Enter, and removing any others: `Helldivers 2`, `Galactic War`, `Map`, `Major Order`, `501st`
- Leave **Interactions Endpoint URL**, **Linked Roles Verification URL**, **Terms of Service URL** and **Privacy Policy URL** empty.

Save.

## 2. Installation

- **Installation Contexts:** uncheck **User Install**. Keep **Guild Install** checked. The bot's commands only work in servers.
- **Install Link:** **Discord Provided Link**
- **Guild Install, Scopes:** `applications.commands` and `bot`. Add `bot` from the dropdown if it is missing.
- **Guild Install, Permissions** (the box appears once `bot` is added): exactly these five, removing any others:
  - View Channels
  - Send Messages
  - Embed Links
  - Attach Files
  - Read Message History

Save. If Discord refuses because the application is private, turn on **Public Bot** in section 4 first, then come back and save this page.

## 3. OAuth2

Nothing to change. There are no redirects, and **Public Client** stays off.

## 4. Bot

- **Icon:** `logo.png`
- **Banner:** `banner.png`
- **Username:** `501st War Room`
- **Token:** skip. The owner does this one (see below).
- **Public Bot:** on
- **Requires OAuth2 Code Grant:** off
- **Privileged Gateway Intents:** **Presence Intent**, **Server Members Intent** and **Message Content Intent** all off
- **Bot Permissions:** leave every box unchecked. It is only a calculator and saves nothing.

Save.

## 5. Leave Alone

Emojis, Webhooks, Rich Presence, App Testers, App Verification, Games, Activities and Premium Apps.

## 6. Later: Discovery

This application's menu has no **Discovery** page yet (Sabacc Droid's does). When it appears, it takes the short description, the long description and the directory tags in [`discord-listing.md`](discord-listing.md#app-directory).

## For the Owner, After the Assistant

1. **Bot** page: click **Reset Token**, copy the token, and paste it into `501st-War-Room/.env` as `DISCORD_TOKEN=...`. Never paste it into a chat.
2. Add the bot to a test server with the link at the top of the [README](../README.md).
3. Run `./run.sh` in the `501st-War-Room` folder.
