package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Accent colors for scoreboard embeds, matching Discord's own brand palette.
const (
	colorRecorded = 0x57F287 // green: a win was just recorded
	colorUndone   = 0xED4245 // red: a game was just removed
	colorNeutral  = 0x5865F2 // blurple: a plain lookup, nothing changed
)

// formatScoreboardEmbed builds a rich scoreboard for a single matchup.
// description is shown under the title (e.g. "Vitória gravada para X."), and
// may be empty. Whoever is currently ahead gets a 🏆 next to their name and
// their avatar as the thumbnail; a tied matchup gets neither.
func formatScoreboardEmbed(player1, player2 playerOption, matchup matchupScore, description string, color int) *discordgo.MessageEmbed {
	player1Wins := matchup.Players[player1.ID]
	player2Wins := matchup.Players[player2.ID]

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s vs %s", player1.DisplayName, player2.DisplayName),
		Description: description,
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			scoreboardField(player1, player1Wins, player2Wins),
			scoreboardField(player2, player2Wins, player1Wins),
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d jogo(s) no total", player1Wins+player2Wins),
		},
	}

	if leader, ok := leadingPlayer(player1, player1Wins, player2, player2Wins); ok && leader.AvatarURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: leader.AvatarURL}
	}

	return embed
}

func scoreboardField(player playerOption, wins, otherWins int) *discordgo.MessageEmbedField {
	name := player.DisplayName
	if wins > otherWins {
		name = "🏆 " + name
	}
	return &discordgo.MessageEmbedField{
		Name:   name,
		Value:  fmt.Sprintf("%d", wins),
		Inline: true,
	}
}

// leadingPlayer returns whichever player has strictly more wins. The second
// return value is false on a tie, when nobody is "leading".
func leadingPlayer(player1 playerOption, player1Wins int, player2 playerOption, player2Wins int) (playerOption, bool) {
	switch {
	case player1Wins > player2Wins:
		return player1, true
	case player2Wins > player1Wins:
		return player2, true
	default:
		return playerOption{}, false
	}
}

// formatScoreboardText is the plain-text scoreboard used by /todos_placares,
// which lists many matchups in one message and could exceed Discord's
// 10-embeds-per-message limit, so it avoids embeds entirely.
func formatScoreboardText(player1, player2 playerOption, matchup matchupScore) string {
	player1Wins := matchup.Players[player1.ID]
	player2Wins := matchup.Players[player2.ID]

	return fmt.Sprintf("```\n%s vs %s\n%s: %d\n%s: %d\n```",
		player1.DisplayName, player2.DisplayName,
		player1.DisplayName, player1Wins,
		player2.DisplayName, player2Wins)
}

func formatStoredScoreboard(s *discordgo.Session, guildID, key string, matchup matchupScore) string {
	players := matchupPlayers(key, matchup)
	if len(players) != 2 {
		return ""
	}

	return formatScoreboardText(
		playerFromID(s, guildID, players[0]),
		playerFromID(s, guildID, players[1]),
		matchup,
	)
}

func matchupPlayers(key string, matchup matchupScore) []string {
	players := strings.Split(key, "|")
	if len(players) == 2 {
		return players
	}

	players = make([]string, 0, len(matchup.Players))
	for playerID := range matchup.Players {
		players = append(players, playerID)
	}
	sort.Strings(players)
	return players
}
