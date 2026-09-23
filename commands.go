package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "salva_jogo",
		Description: "Salva o resultado de uma partida",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "vencedor",
				Description: "Jogador vencedor",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "perdedor",
				Description: "Jogador perdedor",
				Required:    true,
			},
		},
	},
	{
		Name:        "desfazer_jogo",
		Description: "Esquece o jogo mais recente",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "jogador1",
				Description: "Primeiro jogador",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "jogador2",
				Description: "Segundo jogador",
				Required:    true,
			},
		},
	},
	{
		Name:        "placar",
		Description: "Mostra o placar entre 2 jogadores",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "jogador1",
				Description: "Primeiro jogador",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "jogador2",
				Description: "Segundo jogador",
				Required:    true,
			},
		},
	},
	{
		Name:        "todos_placares",
		Description: "Mostra o placar entre todos os jogadores",
	},
}

// reply is what a command handler produces. main.go turns it into the
// actual Discord response: content, an embed, or both.
type reply struct {
	content string
	embed   *discordgo.MessageEmbed
}

func textReply(content string) reply {
	return reply{content: content}
}

func embedReply(embed *discordgo.MessageEmbed) reply {
	return reply{embed: embed}
}

func handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate, store *Store) (reply, error) {
	if i.GuildID == "" {
		return textReply("Please use this command in a Discord server."), nil
	}

	data := i.ApplicationCommandData()
	commandName := data.Name

	switch commandName {
	case "salva_jogo":
		return recordGame(store, data)
	case "placar":
		return showScoreboard(store, data)
	case "todos_placares":
		return showScoreboardAll(store, s, i.GuildID)
	case "desfazer_jogo":
		return undoGame(store, data)
	default:
		return textReply(fmt.Sprintf("Unknown command: /%s.", commandName)), nil
	}
}

func recordGame(store *Store, data discordgo.ApplicationCommandInteractionData) (reply, error) {
	winner, loser := winnerLoserOptions(data)

	if winner.ID == "" || loser.ID == "" {
		return textReply("Envie um vencedor e um perdedor"), nil
	}

	if winner.ID == loser.ID {
		return textReply("Os jogadores devem ser pessoas diferentes"), nil
	}

	matchup, err := store.RecordWin(winner.ID, loser.ID)
	if err != nil {
		return reply{}, err
	}

	description := fmt.Sprintf("Vitória gravada para %s.", winner.DisplayName)
	return embedReply(formatScoreboardEmbed(winner, loser, matchup, description, colorRecorded, winner)), nil
}

func undoGame(store *Store, data discordgo.ApplicationCommandInteractionData) (reply, error) {
	player1, player2 := gameOptions(data)

	if player1.ID == "" || player2.ID == "" {
		return textReply("Envie 2 jogadores"), nil
	}

	if player1.ID == player2.ID {
		return textReply("Os jogadores devem ser pessoas diferentes"), nil
	}

	removed, matchup, err := store.UndoLast(player1.ID, player2.ID)
	if err != nil {
		if errors.Is(err, errNoMatchupPlayers) {
			return textReply(fmt.Sprintf("Nenhuma partida gravada entre %v e %v", player1.DisplayName, player2.DisplayName)), nil
		}
		return reply{}, err
	}

	winnerName, loserName := player1.DisplayName, player2.DisplayName
	if removed.Winner == player2.ID {
		winnerName, loserName = loserName, winnerName
	}

	description := fmt.Sprintf("Removida vitória de %v sobre %v.", winnerName, loserName)
	return embedReply(formatScoreboardEmbed(player1, player2, matchup, description, colorUndone, playerOption{})), nil
}

func showScoreboard(store *Store, data discordgo.ApplicationCommandInteractionData) (reply, error) {
	player1, player2 := gameOptions(data)

	if player1.ID == "" || player2.ID == "" {
		return textReply("Envie 2 jogadores"), nil
	}

	matchup, err := store.Matchup(player1.ID, player2.ID)
	if err != nil {
		if errors.Is(err, errNoMatchupPlayers) {
			return textReply(fmt.Sprintf("Nenhum jogo gravado para %s vs %s.", player1.DisplayName, player2.DisplayName)), nil
		}

		return reply{}, err
	}

	return embedReply(formatScoreboardEmbed(player1, player2, matchup, "", colorNeutral, playerOption{})), nil
}

func showScoreboardAll(store *Store, s *discordgo.Session, guildID string) (reply, error) {
	current, err := store.AllMatchups()
	if err != nil {
		if errors.Is(err, errNoMatchup) {
			return textReply("Nenhuma partida registrada ainda"), nil
		}
		return reply{}, err
	}

	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		matchup := current[key]
		scoreboard := formatStoredScoreboard(s, guildID, key, matchup)
		if scoreboard == "" {
			continue
		}

		separator := ""
		if builder.Len() > 0 {
			separator = "\n"
		}
		if builder.Len()+len(separator)+len(scoreboard) > maxDiscordMessageLength {
			if builder.Len()+len(separator)+len(scoreboardTooLongMessage) <= maxDiscordMessageLength {
				builder.WriteString(separator)
				builder.WriteString(scoreboardTooLongMessage)
			}
			break
		}
		builder.WriteString(separator)
		builder.WriteString(scoreboard)
	}

	if builder.Len() == 0 {
		return textReply("Nenhum jogo válido gravado ainda"), nil
	}

	return textReply(builder.String()), nil
}
