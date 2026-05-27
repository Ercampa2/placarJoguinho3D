# Discord Game Score Bot

A Go Discord bot for recording match results and showing scoreboards for 1v1 and 2v2 games.

The bot uses Discord slash commands, stores results in a local `scores.json` file, and keeps scores by player matchup or team matchup.

## Features

- Record 1v1 match results.
- Create named 2-player teams.
- Record 2v2 match results between saved teams.
- Import multiple results at once.
- Show head-to-head scoreboards.
- Show all recorded scoreboards.
- Persist data locally in JSON.

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

The command definitions in `main.go` currently register these Portuguese command names:

| Command | Options | Description |
| --- | --- | --- |
| `/salva_jogo` | `jogador1`, `jogador2`, `vencedor` | Saves a 1v1 result. |
| `/salva_jogo_2v2` | `time1`, `time2`, `vencedor` | Saves a 2v2 result between saved teams. |
| `/criar_time` | `nome`, `jogador1`, `jogador2` | Creates or updates a named team. |
| `/times` | None | Lists saved teams. |
| `/grava_em_massa` | `results` | Imports multiple results from text. |
| `/placar` | `jogador1`, `jogador2` | Shows a 1v1 scoreboard. |
| `/placar2v2` | `time1`, `time2` | Shows a 2v2 scoreboard. |
| `/todos_placares` | None | Shows all saved scoreboards that fit in one Discord message. |

The dispatcher accepts the Portuguese command names above. It also still accepts the previous English command names for compatibility while old global Discord commands expire from cache.

## Bulk Import Format

The `/grava_em_massa` command accepts one result per line. Semicolons are also treated as line breaks.

1v1 examples:

```text
<@player1> vs <@player2> winner <@player1>
3x <@player1> vs <@player2> winner <@player2>
```

2v2 examples:

```text
2v2 <@team1a> + <@team1b> vs <@team2a> + <@team2b> winner team1
2x 2v2 <@team1a> + <@team1b> vs <@team2a> + <@team2b> winner team2
```

Rules:

- Use Discord user mentions, such as `<@123456789>`.
- Add a count prefix like `3x` to record the same result multiple times.
- For 1v1, the winner must be one of the two players.
- For 2v2, use `winner team1` or `winner team2`.
- A 2v2 game cannot include the same player more than once.

## Data Storage

Scores are stored in `scores.json` in the working directory.

The top-level structure is:

```json
{
  "matchups": {},
  "registered_teams": {}
}
```

Matchup keys are deterministic:

- 1v1 keys use sorted player IDs joined by `|`.
- 2v2 team keys use sorted player IDs joined by `+`.
- 2v2 matchup keys use sorted team keys joined by `|`.

The bot reads and writes the full file for each change. Keep a backup of `scores.json` before editing it manually.

## Development

Format the code:

```sh
gofmt -w main.go
```

Build the project:

```sh
go build .
```

Run compile checks:

```sh
go test ./...
```

## Project Structure

```text
.
├── go.mod
├── go.sum
├── main.go
├── scores.json
└── README.md
```

## Operational Notes

- `scores.json` is local to the process. Running multiple bot instances against the same file can lose updates.
- `scores.json` is not encrypted. Do not store secrets in it.
- `DISCORD_TOKEN` should be provided through environment variables or secret management, not committed to the repository.
- Commands are registered on every startup.
