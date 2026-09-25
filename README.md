# Discord Game Score Bot

A Go Discord bot for recording 1v1 match results and showing scoreboards.

The bot uses Discord slash commands, stores results in a local `scores.json` file, and keeps a full history of games so scoreboards and undo work per matchup.

## Features

- Record 1v1 match results.
- Undo the most recent game between two specific players.
- Show head-to-head scoreboards.
- Show all recorded scoreboards.
- Persist a full game history locally in JSON, written atomically so a crash cannot corrupt the file.
- Migrate older `scores.json` files (win counters only, no history) automatically on startup.
- Optional: remind a channel to drink water on a schedule, count who drank through a keyboard shortcut, and rank everyone each day (see "Water reminders").

## Requirements

- Go `1.26.3` or compatible with the version in `go.mod`.
- A Discord application with a bot user.
- A bot token in `DISCORD_TOKEN`.
- The bot invited to a Discord server with permission to create and respond to slash commands.

## Setup

1. Install dependencies:

   ```sh
   go mod download
   ```

2. Set the required Discord bot token:

   ```sh
   export DISCORD_TOKEN="your-discord-bot-token"
   ```

3. Optional: register commands for one server only while developing:

   ```sh
   export GUILD_ID="your-discord-server-id"
   ```

   If `GUILD_ID` is not set, commands are registered globally. Global slash commands can take longer to appear in Discord.

4. Run the bot:

   ```sh
   go run .
   ```

The bot logs in, migrates `scores.json` if it is still in the old format, registers its slash commands, and keeps running until you stop it with `Ctrl+C`.

## Environment Variables

| Variable | Required | Description |
| --- | --- | --- |
| `DISCORD_TOKEN` | Yes | Discord bot token used to connect to the Gateway and register commands. |
| `GUILD_ID` | No | Discord server ID for guild-scoped command registration. Useful during development. |
| `AGUA_*` | No | Turn on and configure the water reminders. See "Water reminders". |

## Slash Commands

The command definitions in `commands.go` register these Portuguese command names:

| Command | Options | Description |
| --- | --- | --- |
| `/salva_jogo` | `vencedor`, `perdedor` | Records one win for `vencedor` against `perdedor`. |
| `/desfazer_jogo` | `jogador1`, `jogador2` | Removes the most recent game between these two players. It does not matter who won; the bot finds their last game and undoes it. |
| `/placar` | `jogador1`, `jogador2` | Shows the scoreboard between two players. |
| `/todos_placares` | None | Shows all saved scoreboards that fit in one Discord message. |
| `/agua_token` | None | Only when the water reminders are on. See "Water reminders". |
| `/ranking_agua` | `trilha`, optional `data` | Only when the water reminders are on. See "Water reminders". |

`/desfazer_jogo` only ever removes a game between the two named players. Games between other players are never affected, however recent they are.

`/todos_placares` looks up every player's display name through the Discord API, which can be slow. The bot acknowledges the command immediately (Discord shows "thinking...") and fills in the answer when it is ready, so it does not hit Discord's 3-second response limit.

On every startup the bot overwrites the full command list in Discord, so any command removed from `commands.go` is also removed from Discord.

## Water reminders

An optional drink-water game, on when at least one `AGUA_CANAL_*` variable is set. The bot then:

- posts "💧 Beba água!" in a channel on a schedule: Monday to Friday, 09:00–12:30 and 13:30–16:00, São Paulo time. There are two independent **tracks**: `5min` posts every 5 minutes in one channel, and `10min` posts every 10 minutes in another. Each has its own scores.
- listens on the local network for "I drank" requests, sent by a small script that each player binds to a keyboard shortcut (see "Sip scripts"), so nobody has to switch to Discord.
- posts each track's ranking for the day in its channel at 16:00, and answers `/ranking_agua` for any day.

### Rules

- Each reminder opens a **round** that lasts until the next reminder time. The rounds are cut at lunch and at the end of the day: the 12:25 round closes at 12:30, not after lunch. The 5-minute track has 72 rounds a day and the 10-minute track has 36.
- A sip counts for the round that is open when the request reaches the bot. The bot's clock decides, and requests do not name a round, so there is no way to answer an older reminder. A request outside the rounds is refused.
- Each player counts at most one sip per round and track.
- After a restart, the bot does not post the reminder of the round in progress again (nor the ranking, after 16:00); it posts at the next round. Sips keep working meanwhile, since rounds come from the clock.
- Scores are per track and per day; the next day starts from zero. Every sip is kept in `agua.json`, so past days can still be shown.
- Nothing can tell whether someone really drank: the game runs on trust. The token only stops people from recording sips for someone else.

`schedule.roundAt` (in `water_schedule.go`) is the single place that turns a time into a round; the reminder loop and the HTTP endpoint both use it.

### Configuration

| Variable | Required | Description |
| --- | --- | --- |
| `AGUA_CANAL_5MIN` | No | Channel ID for the 5-minute track. Unset: that track is off. |
| `AGUA_CANAL_10MIN` | No | Channel ID for the 10-minute track. Unset: that track is off. |
| `AGUA_HTTP_ADDR` | No | Address the sip endpoint listens on. Default `:8080`, which accepts connections from other machines; `localhost:8080` would not. |
| `AGUA_URL` | No | How players reach the bot, like `http://192.168.0.10:8080`. Only used to show it in `/agua_token` replies. |
| `AGUA_TESTE` | No | `1` replaces the schedule with a 10-minute test run, starting on the next full minute and on any day, with 1- and 2-minute rounds and the ranking at the end. |

Channel IDs are copied from Discord with Developer Mode on (right-click the channel, Copy Channel ID).

At startup the log names the channel of each track, and warns about `AGUA_*` variables the bot does not read (usually typos, like `AGUA_CANAL_5_MIN`) and about channels the bot cannot see. With no channel set, it says `water reminders off`.

On the machine that runs the bot:

- The bot needs **View Channel**, **Send Messages** and **Embed Links** (the daily ranking is an embed) in the reminder channels. Answering slash commands does not need these permissions, but posting on its own does; without them the log shows Discord errors 50001 or 50013. Private channels, where @everyone cannot view the channel, need the bot added to the channel's permissions.
- Allow the HTTP port through the firewall. On Windows, prefer a rule for the port over approving the program: `go run` builds the program in a different temporary folder each time.
- Give the machine a fixed LAN address, such as a DHCP reservation in the router, so the players' scripts keep working.
- Discord only makes noise for plain messages in channels set to notify on **All Messages**; each player sets that on the reminder channel.

To try it on a development machine, use `AGUA_TESTE=1` and a **test channel**. If the development bot uses the real channels, both bots will post reminders there.

### Commands

| Command | Options | Description |
| --- | --- | --- |
| `/agua_token` | None | Generates your token for the sip scripts and shows it only to you, with the bot's address when `AGUA_URL` is set. Generating a new token revokes the old one. |
| `/ranking_agua` | `trilha`, optional `data` | Shows a track's ranking for a day: `24/09`, `24/09/2026` or `2026-09-24`; today by default. Players with the same count share a place. |

### Sip endpoint

```text
POST /gole/{track}
Authorization: Bearer <token from /agua_token>
```

`track` is `5min` or `10min`. The request has no body. Every answer is one line of plain text meant for people, which the scripts show as a notification:

| Status | When |
| --- | --- |
| 201 | Sip recorded. The text says the round and how many sips the player has on that track today. |
| 409 | Already recorded in this round, or no round is open right now. |
| 401 | Missing or unknown token. |
| 404 | Unknown track, or one that is off. |
| 405 | Anything but `POST`. |

The endpoint is plain HTTP, so tokens cross the local network unencrypted. That is acceptable only on a trusted network.

### Sip scripts

`scripts/` has one script per system. Each player sets three values at its top: the bot's address, their token from `/agua_token`, and which tracks they play (one or both; a single key press then records a sip on each).

**Linux** (`scripts/agua.sh`, needs `curl`; `notify-send` from libnotify shows the answer):

1. Copy it somewhere, like `~/agua.sh`, set the three values and run `chmod +x ~/agua.sh`.
2. Run it once from a terminal to check the answer.
3. Bind it to a shortcut:
   - GNOME: Settings, Keyboard, Custom Shortcuts, command `/home/<you>/agua.sh`.
   - KDE Plasma: System Settings, Shortcuts, Add New, Command or Script.
   - i3 or sway: `bindsym $mod+Shift+a exec --no-startup-id ~/agua.sh`.
   - Hyprland: `bind = SUPER SHIFT, A, exec, ~/agua.sh`.

**Windows 10** (`scripts/agua.ps1` and `scripts/instalar-atalho.ps1`, using the `curl.exe` that ships with Windows 10 1803 and later):

1. Put both files in a folder that will not move, like `C:\agua`, and set the three values at the top of `agua.ps1` with Notepad.
2. Right-click `instalar-atalho.ps1` and choose **Run with PowerShell**. It creates a Start menu shortcut triggered by **Ctrl+Alt+A**; edit `$Tecla` in it and run it again for another key.
3. Press the key. The answer shows up as a Windows notification after a second or two. The shortcut starts PowerShell minimized and hidden, so it does not take the focus from the window in use.

If Windows refuses to run files that came from the internet, right-click each one, Properties, and tick **Unblock**. The `.ps1` files are kept ASCII-only on purpose: Windows PowerShell 5.1 reads files without a byte order mark as ANSI and would garble accented letters.

## Data Storage

Scores are stored in `scores.json` in the working directory. The file is listed in `.gitignore`, so it is not committed; back it up separately (see "Backups" below).

The top-level structure is:

```json
{
  "games": [
    { "winner": "111111111111111111", "loser": "222222222222222222", "at": "2026-09-19T14:32:00Z" }
  ]
}
```

Every recorded game is kept, in order. Scoreboards and undo are both computed from this list rather than from a stored total, which is what lets `/desfazer_jogo` remove the right game instead of just decrementing a counter. `matchupKey` (the two player IDs, sorted and joined by `|`) is used to group games into a matchup so that, for example, a game recorded as `A` beating `B` and one recorded as `B` beating `A` land in the same scoreboard.

The bot reads and writes the full file for each change. Writes go to `scores.json.tmp`, are flushed to disk, and are then renamed over `scores.json`, so the file is always either the old version or the new one, never a partial write.

The water reminders keep their own file, `agua.json`, written the same way and also left out of git. It holds each player's token and every sip:

```json
{
  "tokens": { "111111111111111111": "B6VPR5RD6HLJR7KMSXGQC6Q7YI" },
  "sips": [
    { "player": "111111111111111111", "track": "5min", "day": "2026-09-24",
      "round": "2026-09-24T13:05:00Z", "at": "2026-09-24T13:05:42Z" }
  ]
}
```

`round` is the start of the round and identifies the reminder; `day` is its date in São Paulo, which rankings group by.

### Migrating from the old format

Older versions of this bot stored only a running win count per matchup:

```json
{ "matchups": { "111|222": { "players": { "111": 58, "222": 52 } } } }
```

On startup, `Store.Migrate` (in `store.go`) detects this `matchups` field and converts each counter into that many individual games, so a `58`/`52` counter becomes 110 games with no recorded time (`at` is left empty for these). Team-based (2v2) entries from even older versions have no `players` map and are dropped during migration, since 2v2 play was removed from the bot. Before writing anything, migration copies the untouched original file to `scores.json.legacy.bak` and verifies, matchup by matchup and player by player, that every migrated game count matches the original counter; if anything does not match, it aborts with an error and leaves `scores.json` untouched. The bot logs a line like:

```
Migrados 5 jogos legacy para 312 jogos novos
```

when a migration actually happens. Seeing this line, and the numbers in it, is the quickest sanity check that a deploy went well. Migration is a no-op on a file that has already been converted, so it is safe to run on every startup.

## Backups

`scores.json` is not tracked in git, so it needs its own backup whenever you deploy a new version of the bot or move it to another machine. The same goes for `agua.json` when the water reminders are on:

1. Stop the bot.
2. Copy the file *outside* the project folder, e.g. `cp scores.json ~/scores.backup-$(date +%F).json` (and `cp agua.json ~/agua.backup-$(date +%F).json`). Keep this copy somewhere separate from the machine running the bot, so a single disk failure cannot take out both.
3. Update the code (see "Deploying to another machine" below).
4. Start the bot and read the startup log. If it migrated the file, check the migrated-game count against what you expect.
5. Run `/placar` for a matchup you know well, or `/todos_placares`, and confirm the numbers match what you remember.

If anything looks wrong, stop the bot and restore `scores.json` from the backup made in step 2 before doing anything else.

## Deploying to another machine

`scores.json` is untracked, so updating the code with git can interact with it in ways that are easy to get wrong:

- If `scores.json` on that machine is unmodified relative to the last commit that still tracked it, `git pull` will **delete the file** as part of applying the commit that untracked it.
- If it has been modified (which it will have, from real games), `git pull` will refuse and print an error about local changes that would be overwritten. This is the safe outcome; do not run `git stash` or `git checkout -- scores.json` to work around it without a backup first (see above), since either can discard the live file.

The safe sequence, assuming the backup from the previous section has already been made:

```sh
git checkout -- scores.json   # safe now that you have a backup
git pull
cp ~/scores.backup-<date>.json scores.json
go run .   # or however you normally start the bot; watch the migration log line
```

If you deploy by copying files instead of using git, just make sure you never copy a `scores.json` over the real one.

## Development

Format the code:

```sh
gofmt -w .
```

Check for common mistakes:

```sh
go vet ./...
```

Build the project:

```sh
go build .
```

Run the tests:

```sh
go test ./...
```

Run them with Go's data race detector, which is useful for the locking in `Store`:

```sh
go test -race ./...
```

`store_test.go` covers `Store` (recording, undoing, looking up matchups, persistence, and migration from the old format) and `matchupKey`. Each test uses a temporary directory, so they never touch your real `scores.json`. The Discord command handlers in `commands.go` are not covered by tests.

The `water_*_test.go` files cover the water reminders without Discord or a network: the schedule (`roundAt` against a table of times), `WaterStore`, the HTTP endpoint through `net/http/httptest`, when the reminder loop posts, and the ranking text. They also use temporary directories and never touch `agua.json`.

## Project Structure

```text
.
├── commands.go     slash command definitions and command handlers
├── main.go         startup: Discord session, migration, interaction handler
├── players.go      player options and Discord name lookup
├── format.go       scoreboard text formatting
├── store.go        Store: game history, scoreboards, undo, and legacy migration
├── store_test.go   tests for Store, matchupKey, and migrateLegacy
├── water.go            water reminders: AGUA_* config, startup, commands, ranking embed
├── water_schedule.go   reminder hours and roundAt, which maps a time to its round
├── water_store.go      WaterStore: sips and tokens in agua.json
├── water_http.go       POST /gole/{track}, the sip endpoint
├── water_reminder.go   the per-track loop that posts reminders and the daily ranking
├── water_*_test.go     tests for all of the above
├── scripts/
│   ├── agua.sh              Linux sip script
│   ├── agua.ps1             Windows 10 sip script
│   └── instalar-atalho.ps1  creates the Windows keyboard shortcut
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

`scores.json` and `agua.json` are created at runtime and are not part of the repository.

## Operational Notes

- `scores.json` is local to the process. The lock in `Store` only protects a single bot process, so running multiple instances against the same file can lose updates.
- Because `scores.json` is not in git, back it up separately if the scores matter to you (see "Backups" above).
- `scores.json` is not encrypted. Do not store secrets in it.
- `DISCORD_TOKEN` should be provided through environment variables or secret management, not committed to the repository.
- Commands are synced with Discord on every startup.
