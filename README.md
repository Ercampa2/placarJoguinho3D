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

## Slash Commands

The command definitions in `commands.go` register these Portuguese command names:

| Command | Options | Description |
| --- | --- | --- |
| `/salva_jogo` | `vencedor`, `perdedor` | Records one win for `vencedor` against `perdedor`. |
| `/desfazer_jogo` | `jogador1`, `jogador2` | Removes the most recent game between these two players. It does not matter who won; the bot finds their last game and undoes it. |
| `/placar` | `jogador1`, `jogador2` | Shows the scoreboard between two players. |
| `/todos_placares` | None | Shows all saved scoreboards that fit in one Discord message. |

`/desfazer_jogo` only ever removes a game between the two named players. Games between other players are never affected, however recent they are.

`/todos_placares` looks up every player's display name through the Discord API, which can be slow. The bot acknowledges the command immediately (Discord shows "thinking...") and fills in the answer when it is ready, so it does not hit Discord's 3-second response limit.

On every startup the bot overwrites the full command list in Discord, so any command removed from `commands.go` is also removed from Discord.

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

`scores.json` is not tracked in git, so it needs its own backup whenever you deploy a new version of the bot or move it to another machine:

1. Stop the bot.
2. Copy the file *outside* the project folder, e.g. `cp scores.json ~/scores.backup-$(date +%F).json`. Keep this copy somewhere separate from the machine running the bot, so a single disk failure cannot take out both.
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

## Project Structure

```text
.
├── commands.go     slash command definitions and command handlers
├── main.go         startup: Discord session, migration, interaction handler
├── players.go      player options and Discord name lookup
├── format.go       scoreboard text formatting
├── store.go        Store: game history, scoreboards, undo, and legacy migration
├── store_test.go   tests for Store, matchupKey, and migrateLegacy
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

`scores.json` is created at runtime and is not part of the repository.

## Operational Notes

- `scores.json` is local to the process. The lock in `Store` only protects a single bot process, so running multiple instances against the same file can lose updates.
- Because `scores.json` is not in git, back it up separately if the scores matter to you (see "Backups" above).
- `scores.json` is not encrypted. Do not store secrets in it.
- `DISCORD_TOKEN` should be provided through environment variables or secret management, not committed to the repository.
- Commands are synced with Discord on every startup.
