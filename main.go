package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// noMentions stops Discord from pinging anyone from text this bot builds,
// in case a display name ever falls back to a raw mention.
var noMentions = &discordgo.MessageAllowedMentions{}

func main() {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("missing DISCORD_TOKEN environment variable")
	}

	guildID := os.Getenv("GUILD_ID")

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("create Discord session: %v", err)
	}

	store := NewStore("scores.json")

	matchups, games, err := store.Migrate()
	if err != nil {
		log.Fatalf("migrate score %v", err)
	}

	if matchups > 0 {
		log.Printf("Migrados %d jogos legacy para %d jogos novos", matchups, games)
	}

	water, err := newWaterFromEnv()
	if err != nil {
		log.Fatalf("configure water reminders: %v", err)
	}

	// ctx is cancelled on Ctrl+C or SIGTERM, which stops the water goroutines.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("logged in as %s", r.User.String())
	})

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		commandName := i.ApplicationCommandData().Name
		slowMode := commandName == "todos_placares"

		if slowMode {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})

			if err != nil {
				log.Printf("defer /%s: %v", commandName, err)
				return
			}
		}

		resp, err := handleCommand(s, i, store, water)
		if err != nil {
			log.Printf("handle /%s: %v", commandName, err)
			resp = textReply("Desculpe, não pude tratar este comando.")
		}

		if resp.content == "" && resp.embed == nil {
			resp = textReply("Feito.")
		}

		var embeds []*discordgo.MessageEmbed
		if resp.embed != nil {
			embeds = []*discordgo.MessageEmbed{resp.embed}
		}

		if slowMode {
			_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content:         &resp.content,
				Embeds:          &embeds,
				AllowedMentions: noMentions,
			})

			if err != nil {
				log.Printf("edit response to /%s: %v", commandName, err)
			}
		} else {
			data := &discordgo.InteractionResponseData{
				Content:         resp.content,
				Embeds:          embeds,
				AllowedMentions: noMentions,
			}
			if resp.ephemeral {
				data.Flags = discordgo.MessageFlagsEphemeral
			}

			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: data,
			})

			if err != nil {
				log.Printf("respond to /%s: %v", commandName, err)
			}
		}
	})

	if err := session.Open(); err != nil {
		log.Fatalf("open Discord session: %v", err)
	}
	defer session.Close()

	allCommands := commands
	if water != nil {
		allCommands = slices.Concat(commands, water.commands())
	}

	// Bulk overwrite replaces the whole command list, so commands removed from
	// the code (like the old team and bulk ones) also disappear from Discord.
	registered, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, guildID, allCommands)
	if err != nil {
		log.Fatalf("register commands: %v", err)
	}

	for _, command := range registered {
		if guildID == "" {
			log.Printf("registered global /%s command. It can take time to appear in Discord.", command.Name)
		} else {
			log.Printf("registered /%s command in guild %s", command.Name, guildID)
		}
	}

	if water != nil {
		shutdownWater, err := water.start(ctx, session)
		if err != nil {
			log.Fatalf("start water reminders: %v", err)
		}
		defer shutdownWater()
	}

	log.Println("bot is running. Press Ctrl+C to stop.")

	<-ctx.Done()

	log.Println("shutting down")
}

func userMention(userID string) string {
	if userID == "" {
		return ""
	}

	return (&discordgo.User{ID: userID}).Mention()
}

func userDisplayName(userID string, resolved *discordgo.ApplicationCommandInteractionDataResolved) string {
	if resolved != nil {
		if member := resolved.Members[userID]; member != nil && member.Nick != "" {
			return member.Nick
		}
		if user := resolved.Users[userID]; user != nil {
			switch {
			case user.GlobalName != "":
				return user.GlobalName
			case user.Username != "":
				return user.Username
			}
		}
	}

	return userMention(userID)
}
