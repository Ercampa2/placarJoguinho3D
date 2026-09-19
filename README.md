# Discord Game Score Bot

A Go Discord bot for recording 1v1 match results and showing scoreboards.

The bot uses Discord slash commands, stores results in a local `scores.json` file, and keeps scores by player matchup.

## Features

- Record 1v1 match results.
- Undo a recorded win.
- Show head-to-head scoreboards.
- Show all recorded scoreboards.
- Persist data locally in JSON, written atomically so a crash cannot corrupt the file.

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

The bot logs in, registers its slash commands, and keeps running until you stop it with `Ctrl+C`.

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
| `/desfazer_jogo` | `vencedor`, `perdedor` | Removes one win from `vencedor` in that matchup. It does nothing if they have no wins there. |
| `/placar` | `jogador1`, `jogador2` | Shows the scoreboard between two players. |
| `/todos_placares` | None | Shows all saved scoreboards that fit in one Discord message. |

`/desfazer_jogo` removes a win from the player you name. It does not track which game happened last, so name the winner of the game you are undoing.

`/todos_placares` looks up every player's display name through the Discord API, which can be slow. The bot acknowledges the command immediately (Discord shows "thinking...") and fills in the answer when it is ready, so it does not hit Discord's 3-second response limit.

On every startup the bot overwrites the full command list in Discord, so any command removed from `commands.go` is also removed from Discord.

## Data Storage

Scores are stored in `scores.json` in the working directory. The file is listed in `.gitignore`, so it is not committed.

The top-level structure is:

```json
{
  "matchups": {
    "111111111111111111|222222222222222222": {
      "players": {
        "111111111111111111": 3,
        "222222222222222222": 2
      }
    }
  }
}
```

Matchup keys are the two player IDs, sorted and joined by `|`, so `A|B` and `B|A` share one scoreboard.

The bot reads and writes the full file for each change. Writes go to `scores.json.tmp`, are flushed to disk, and are then renamed over `scores.json`, so the file is always either the old version or the new one. Keep a backup of `scores.json` before editing it manually.

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

The tests in `store_test.go` cover `Store` and `matchupKey`. Each test uses a temporary directory, so they never touch your real `scores.json`. The Discord command handlers are not covered by tests.

## Project Structure

```text
.
├── commands.go     slash command definitions
├── main.go         startup, interaction handler, command handlers
├── players.go      player options, name lookup, scoreboard formatting
├── store.go        Store: loading, saving and updating scores.json
├── store_test.go   tests for Store and matchupKey
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

`scores.json` is created at runtime and is not part of the repository.

## Operational Notes

- `scores.json` is local to the process. The lock in `Store` only protects a single bot process, so running multiple instances against the same file can lose updates.
- Because `scores.json` is not in git, back it up separately if the scores matter to you.
- `scores.json` is not encrypted. Do not store secrets in it.
- `DISCORD_TOKEN` should be provided through environment variables or secret management, not committed to the repository.
- Commands are synced with Discord on every startup.
