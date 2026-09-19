package main

import (
	"github.com/bwmarrin/discordgo"
)

const maxDiscordMessageLength = 2000
const scoreboardTooLongMessage = "More games exist, but the scoreboard is too long for one Discord message."

type playerOption struct {
	ID          string
	DisplayName string
}

func playerFromID(s *discordgo.Session, guildID, userID string) playerOption {
	player := playerOption{
		ID:          userID,
		DisplayName: userMention(userID),
	}

	member, err := s.GuildMember(guildID, userID)
	if err == nil && member != nil {
		switch {
		case member.Nick != "":
			player.DisplayName = member.Nick
		case member.User != nil && member.User.GlobalName != "":
			player.DisplayName = member.User.GlobalName
		case member.User != nil && member.User.Username != "":
			player.DisplayName = member.User.Username
		}
		return player
	}

	user, err := s.User(userID)
	if err != nil {
		return player
	}

	switch {
	case user.GlobalName != "":
		player.DisplayName = user.GlobalName
	case user.Username != "":
		player.DisplayName = user.Username
	}

	return player
}

func userOptions(data discordgo.ApplicationCommandInteractionData) map[string]playerOption {
	values := make(map[string]playerOption)

	for _, option := range data.Options {
		if option.Type != discordgo.ApplicationCommandOptionUser {
			continue
		}
		userID, _ := option.Value.(string)
		values[option.Name] = playerOption{
			ID:          userID,
			DisplayName: userDisplayName(userID, data.Resolved),
		}
	}
	return values
}

func gameOptions(data discordgo.ApplicationCommandInteractionData) (playerOption, playerOption) {
	values := userOptions(data)
	return values["jogador1"], values["jogador2"]
}

func winnerLoserOptions(data discordgo.ApplicationCommandInteractionData) (playerOption, playerOption) {
	values := userOptions(data)
	return values["vencedor"], values["perdedor"]
}
