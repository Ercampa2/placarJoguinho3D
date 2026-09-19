package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func formatScoreboard(player1, player2 playerOption, matchup matchupScore, prefix string) string {
	player1Wins := matchup.Players[player1.ID]
	player2Wins := matchup.Players[player2.ID]

	var builder strings.Builder
	if prefix != "" {
		builder.WriteString(prefix)
		builder.WriteString("\n")
	}
	builder.WriteString(fmt.Sprintf("```\n%s vs %s\n%s: %d\n%s: %d\n```", player1.DisplayName, player2.DisplayName, player1.DisplayName, player1Wins, player2.DisplayName, player2Wins))

	return builder.String()
}

func formatStoredScoreboard(s *discordgo.Session, guildID, key string, matchup matchupScore) string {
	players := matchupPlayers(key, matchup)
	if len(players) != 2 {
		return ""
	}

	return formatScoreboard(
		playerFromID(s, guildID, players[0]),
		playerFromID(s, guildID, players[1]),
		matchup,
		"",
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
