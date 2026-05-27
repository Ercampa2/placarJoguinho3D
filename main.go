package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

const scoresFile = "scores.json"
const maxDiscordMessageLength = 2000
const scoreboardTooLongMessage = "More games exist, but the scoreboard is too long for one Discord message."

var userMentionPattern = regexp.MustCompile(`<@!?([0-9]+)>`)

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "salva_jogo",
		Description: "Salva o resultado de uma partida",
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
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "vencedor",
				Description: "Vencedor da partida",
				Required:    true,
			},
		},
	},
	{
		Name:        "salva_jogo_2v2",
		Description: "Salva o resultado de uma partida 2v2",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time1",
				Description: "Primeiro time",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time2",
				Description: "Segundo time",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "vencedor",
				Description: "Time vencedor (time1 ou time2)",
				Required:    true,
			},
		},
	},
	{
		Name:        "criar_time",
		Description: "Cria ou altera um time",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "nome",
				Description: "Nome do time",
				Required:    true,
			},
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
		Name:        "times",
		Description: "Lista os times",
	},
	{
		Name:        "grava_em_massa",
		Description: "Grava resultados em massa",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "results",
				Description: "Um resultado por linha. Use menções, opcional 3x prefix, e vencedor.",
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
		Name:        "placar2v2",
		Description: "Mostra o placar entre 2 times",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time1",
				Description: "Primeiro time",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time2",
				Description: "Segundo time",
				Required:    true,
			},
		},
	},
	{
		Name:        "todos_placares",
		Description: "Mostra o placar entre todos os jogadores",
	},
}

type scores struct {
	Matchups        map[string]matchupScore `json:"matchups"`
	RegisteredTeams map[string]savedTeam    `json:"registered_teams,omitempty"`
}

type matchupScore struct {
	Players map[string]int `json:"players"`
	Teams   map[string]int `json:"teams,omitempty"`
}

type playerOption struct {
	ID          string
	DisplayName string
}

type teamOption struct {
	Name    string
	Players []playerOption
}

type savedTeam struct {
	Name    string   `json:"name"`
	Players []string `json:"players"`
}

type bulkRecord struct {
	Line       int
	Count      int
	Player1ID  string
	Player2ID  string
	Team1IDs   []string
	Team2IDs   []string
	WinnerID   string
	WinnerTeam string
}

var scoresMu sync.Mutex

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

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("logged in as %s", r.User.String())
	})

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		commandName := i.ApplicationCommandData().Name
		response, err := handleCommand(s, i)
		if err != nil {
			log.Printf("handle /%s: %v", commandName, err)
			response = "Sorry, I could not handle that command."
		}

		if response == "" {
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})
		if err != nil {
			log.Printf("respond to /%s: %v", commandName, err)
		}
	})

	if err := session.Open(); err != nil {
		log.Fatalf("open Discord session: %v", err)
	}
	defer session.Close()

	for _, commandConfig := range commands {
		command, err := session.ApplicationCommandCreate(session.State.User.ID, guildID, commandConfig)
		if err != nil {
			log.Fatalf("register /%s command: %v", commandConfig.Name, err)
		}

		if guildID == "" {
			log.Printf("registered global /%s command. It can take time to appear in Discord.", command.Name)
		} else {
			log.Printf("registered /%s command in guild %s", command.Name, guildID)
		}
	}

	log.Println("bot is running. Press Ctrl+C to stop.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
}

func handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (string, error) {
	if i.GuildID == "" {
		return "Please use this command in a Discord server.", nil
	}

	data := i.ApplicationCommandData()
	commandName := data.Name

	switch commandName {
	case "salva_jogo", "recordgame":
		return recordGame(data)
	case "salva_jogo_2v2", "recordgame2v2":
		return recordGame2v2(data)
	case "criar_time", "createteam":
		return createTeam(data)
	case "times", "teams":
		return listTeams()
	case "grava_em_massa", "recordbulk":
		return recordBulk(data)
	case "placar", "scoreboard":
		return showScoreboard(data)
	case "placar2v2", "scoreboard2v2":
		return showScoreboard2v2(data)
	case "todos_placares", "scoreboardall":
		return showScoreboardAll(s, i.GuildID)
	default:
		return fmt.Sprintf("Unknown command: /%s.", commandName), nil
	}
}

func recordGame(data discordgo.ApplicationCommandInteractionData) (string, error) {
	player1, player2, winner := gameOptions(data)

	if player1.ID == "" || player2.ID == "" || winner.ID == "" {
		return "Please provide player1, player2, and winner.", nil
	}

	if player1.ID == player2.ID {
		return "Player 1 and player 2 must be different people.", nil
	}

	winnerPlayer, ok := matchingPlayer(winner, player1, player2)
	if !ok {
		return "Winner must be either player1 or player2.", nil
	}

	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	key := matchupKey(player1.ID, player2.ID)
	matchup := currentScores.Matchups[key]
	if matchup.Players == nil {
		matchup.Players = map[string]int{
			player1.ID: 0,
			player2.ID: 0,
		}
	}
	matchup.Players[winnerPlayer.ID]++
	currentScores.Matchups[key] = matchup

	if err := saveScores(currentScores); err != nil {
		return "", err
	}

	return formatScoreboard(player1, player2, matchup, fmt.Sprintf("Recorded win for %s.", winnerPlayer.DisplayName)), nil
}

func showScoreboard(data discordgo.ApplicationCommandInteractionData) (string, error) {
	player1, player2, _ := gameOptions(data)

	if player1.ID == "" || player2.ID == "" {
		return "Please provide player1 and player2.", nil
	}

	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	matchup := currentScores.Matchups[matchupKey(player1.ID, player2.ID)]
	if matchup.Players == nil {
		return fmt.Sprintf("No games recorded yet for %s vs %s.", player1.DisplayName, player2.DisplayName), nil
	}

	return formatScoreboard(player1, player2, matchup, ""), nil
}

func recordGame2v2(data discordgo.ApplicationCommandInteractionData) (string, error) {
	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	team1, team2, err := namedTeamOptions(data, currentScores)
	if err != nil {
		return err.Error(), nil
	}

	winnerTeam, err := winningNamedTeam(data, team1, team2)
	if err != nil {
		return err.Error(), nil
	}

	key := teamMatchupKey(team1, team2)
	matchup := currentScores.Matchups[key]
	if matchup.Teams == nil {
		matchup.Teams = map[string]int{
			teamKey(team1): 0,
			teamKey(team2): 0,
		}
	}
	matchup.Teams[teamKey(winnerTeam)]++
	currentScores.Matchups[key] = matchup

	if err := saveScores(currentScores); err != nil {
		return "", err
	}

	return formatTeamScoreboard(team1, team2, matchup, fmt.Sprintf("Recorded win for %s.", teamDisplayName(winnerTeam))), nil
}

func showScoreboard2v2(data discordgo.ApplicationCommandInteractionData) (string, error) {
	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	team1, team2, err := namedTeamOptions(data, currentScores)
	if err != nil {
		return err.Error(), nil
	}

	matchup := currentScores.Matchups[teamMatchupKey(team1, team2)]
	if matchup.Teams == nil {
		return fmt.Sprintf("No games recorded yet for %s vs %s.", teamDisplayName(team1), teamDisplayName(team2)), nil
	}

	return formatTeamScoreboard(team1, team2, matchup, ""), nil
}

func createTeam(data discordgo.ApplicationCommandInteractionData) (string, error) {
	name := strings.TrimSpace(firstStringOption(data, "nome", "name"))
	player1, player2, _ := gameOptions(data)

	if name == "" || player1.ID == "" || player2.ID == "" {
		return "Please provide a team name and two players.", nil
	}
	if reservedTeamName(name) {
		return "Team name cannot be team1 or team2.", nil
	}
	if player1.ID == player2.ID {
		return "A team must have two different players.", nil
	}

	team := teamOption{
		Name:    name,
		Players: []playerOption{player1, player2},
	}

	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	for key, saved := range currentScores.RegisteredTeams {
		if teamKey(teamOption{Players: playersFromIDs(saved.Players)}) == teamKey(team) {
			delete(currentScores.RegisteredTeams, key)
		}
	}

	currentScores.RegisteredTeams[normalizeTeamName(name)] = savedTeam{
		Name:    name,
		Players: teamPlayerIDs(team),
	}

	if err := saveScores(currentScores); err != nil {
		return "", err
	}

	return fmt.Sprintf("Saved team %s: %s.", name, teamDisplayName(team)), nil
}

func listTeams() (string, error) {
	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	if len(currentScores.RegisteredTeams) == 0 {
		return "No teams created yet.", nil
	}

	keys := make([]string, 0, len(currentScores.RegisteredTeams))
	for key := range currentScores.RegisteredTeams {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("```")
	for _, key := range keys {
		team := currentScores.RegisteredTeams[key]
		builder.WriteString("\n")
		builder.WriteString(team.Name)
		builder.WriteString(": ")
		builder.WriteString(strings.Join(userMentions(team.Players), " + "))
	}
	builder.WriteString("\n```")

	return builder.String(), nil
}

func recordBulk(data discordgo.ApplicationCommandInteractionData) (string, error) {
	results := stringOption(data, "results")
	if strings.TrimSpace(results) == "" {
		return bulkImportHelp(), nil
	}

	records, err := parseBulkRecords(results)
	if err != nil {
		return err.Error() + "\n\n" + bulkImportHelp(), nil
	}

	scoresMu.Lock()
	defer scoresMu.Unlock()

	currentScores, err := loadScores()
	if err != nil {
		return "", err
	}

	total1v1 := 0
	total2v2 := 0
	for _, record := range records {
		if record.is2v2() {
			team1 := teamFromIDs(record.Team1IDs)
			team2 := teamFromIDs(record.Team2IDs)
			winnerTeam := team1
			if record.WinnerTeam == "team2" {
				winnerTeam = team2
			}

			key := teamMatchupKey(team1, team2)
			matchup := currentScores.Matchups[key]
			if matchup.Teams == nil {
				matchup.Teams = map[string]int{
					teamKey(team1): 0,
					teamKey(team2): 0,
				}
			}
			matchup.Teams[teamKey(winnerTeam)] += record.Count
			currentScores.Matchups[key] = matchup
			total2v2 += record.Count
			continue
		}

		player1 := playerOption{ID: record.Player1ID}
		player2 := playerOption{ID: record.Player2ID}
		winner := playerOption{ID: record.WinnerID}
		winnerPlayer, ok := matchingPlayer(winner, player1, player2)
		if !ok {
			return fmt.Sprintf("Line %d: winner must be one of the two 1v1 players.", record.Line), nil
		}

		key := matchupKey(player1.ID, player2.ID)
		matchup := currentScores.Matchups[key]
		if matchup.Players == nil {
			matchup.Players = map[string]int{
				player1.ID: 0,
				player2.ID: 0,
			}
		}
		matchup.Players[winnerPlayer.ID] += record.Count
		currentScores.Matchups[key] = matchup
		total1v1 += record.Count
	}

	if err := saveScores(currentScores); err != nil {
		return "", err
	}

	return fmt.Sprintf("Imported %d result(s): %d 1v1, %d 2v2.", total1v1+total2v2, total1v1, total2v2), nil
}

func showScoreboardAll(s *discordgo.Session, guildID string) (string, error) {
	scoresMu.Lock()
	currentScores, err := loadScores()
	scoresMu.Unlock()
	if err != nil {
		return "", err
	}

	if len(currentScores.Matchups) == 0 {
		return "No games recorded yet.", nil
	}

	keys := make([]string, 0, len(currentScores.Matchups))
	for key := range currentScores.Matchups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		matchup := currentScores.Matchups[key]
		scoreboard := formatStoredScoreboard(s, guildID, key, matchup, currentScores)
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
		return "No valid games recorded yet.", nil
	}

	return builder.String(), nil
}

func formatStoredScoreboard(s *discordgo.Session, guildID, key string, matchup matchupScore, currentScores scores) string {
	if matchup.Teams != nil {
		teams := strings.Split(key, "|")
		if len(teams) != 2 {
			return ""
		}

		team1 := teamFromKey(s, guildID, teams[0], currentScores)
		team2 := teamFromKey(s, guildID, teams[1], currentScores)
		if len(team1.Players) != 2 || len(team2.Players) != 2 {
			return ""
		}

		return formatTeamScoreboard(team1, team2, matchup, "")
	}

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

func teamFromKey(s *discordgo.Session, guildID, key string, currentScores scores) teamOption {
	userIDs := strings.Split(key, "+")
	team := teamOption{
		Name:    registeredTeamNameByKey(key, currentScores),
		Players: make([]playerOption, 0, len(userIDs)),
	}
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		team.Players = append(team.Players, playerFromID(s, guildID, userID))
	}
	return team
}

func teamFromIDs(userIDs []string) teamOption {
	team := teamOption{
		Players: make([]playerOption, 0, len(userIDs)),
	}
	for _, userID := range userIDs {
		team.Players = append(team.Players, playerOption{ID: userID, DisplayName: userMention(userID)})
	}
	return team
}

func namedTeamOptions(data discordgo.ApplicationCommandInteractionData, currentScores scores) (teamOption, teamOption, error) {
	team1Name := stringOption(data, "team1")
	team2Name := stringOption(data, "team2")
	team1, ok := registeredTeam(team1Name, currentScores)
	if !ok {
		return teamOption{}, teamOption{}, fmt.Errorf("Team %q does not exist. Create it with /criar_time first.", team1Name)
	}

	team2, ok := registeredTeam(team2Name, currentScores)
	if !ok {
		return teamOption{}, teamOption{}, fmt.Errorf("Team %q does not exist. Create it with /criar_time first.", team2Name)
	}

	if normalizeTeamName(team1.Name) == normalizeTeamName(team2.Name) {
		return teamOption{}, teamOption{}, fmt.Errorf("Team 1 and team 2 must be different teams.")
	}

	if duplicatePlayers(team1, team2) {
		return teamOption{}, teamOption{}, fmt.Errorf("The two teams cannot share players.")
	}

	return team1, team2, nil
}

func winningNamedTeam(data discordgo.ApplicationCommandInteractionData, team1, team2 teamOption) (teamOption, error) {
	winner := strings.TrimSpace(firstStringOption(data, "vencedor", "winner"))
	switch normalizeTeamName(winner) {
	case "team1", normalizeTeamName(team1.Name):
		return team1, nil
	case "team2", normalizeTeamName(team2.Name):
		return team2, nil
	default:
		return teamOption{}, fmt.Errorf("Winner must be %s, %s, team1, or team2.", team1.Name, team2.Name)
	}
}

func registeredTeam(name string, currentScores scores) (teamOption, bool) {
	saved, ok := currentScores.RegisteredTeams[normalizeTeamName(name)]
	if !ok || len(saved.Players) != 2 {
		return teamOption{}, false
	}

	return teamOption{
		Name:    saved.Name,
		Players: playersFromIDs(saved.Players),
	}, true
}

func registeredTeamNameByKey(key string, currentScores scores) string {
	for _, team := range currentScores.RegisteredTeams {
		if teamKey(teamOption{Players: playersFromIDs(team.Players)}) == key {
			return team.Name
		}
	}
	return ""
}

func playersFromIDs(userIDs []string) []playerOption {
	players := make([]playerOption, 0, len(userIDs))
	for _, userID := range userIDs {
		players = append(players, playerOption{ID: userID, DisplayName: userMention(userID)})
	}
	return players
}

func gameOptions(data discordgo.ApplicationCommandInteractionData) (playerOption, playerOption, playerOption) {
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

	return firstPlayerOption(values, "jogador1", "player1"),
		firstPlayerOption(values, "jogador2", "player2"),
		firstPlayerOption(values, "vencedor", "winner")
}

func teamOptions(data discordgo.ApplicationCommandInteractionData) (teamOption, teamOption) {
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

	return teamOption{
			Players: []playerOption{values["team1_player1"], values["team1_player2"]},
		}, teamOption{
			Players: []playerOption{values["team2_player1"], values["team2_player2"]},
		}
}

func stringOption(data discordgo.ApplicationCommandInteractionData, name string) string {
	for _, option := range data.Options {
		if option.Name != name || option.Type != discordgo.ApplicationCommandOptionString {
			continue
		}

		value, _ := option.Value.(string)
		return value
	}

	return ""
}

func firstStringOption(data discordgo.ApplicationCommandInteractionData, names ...string) string {
	for _, name := range names {
		if value := stringOption(data, name); value != "" {
			return value
		}
	}
	return ""
}

func firstPlayerOption(values map[string]playerOption, names ...string) playerOption {
	for _, name := range names {
		if value := values[name]; value.ID != "" {
			return value
		}
	}
	return playerOption{}
}

func parseBulkRecords(input string) ([]bulkRecord, error) {
	var records []bulkRecord
	input = strings.ReplaceAll(input, ";", "\n")
	for lineNumber, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		record, err := parseBulkRecord(lineNumber+1, line)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("No results found.")
	}

	return records, nil
}

func parseBulkRecord(lineNumber int, line string) (bulkRecord, error) {
	count, line := parseBulkCount(line)
	mentions := userMentionPattern.FindAllStringSubmatch(line, -1)
	mentionIDs := make([]string, 0, len(mentions))
	for _, mention := range mentions {
		mentionIDs = append(mentionIDs, mention[1])
	}

	if len(mentionIDs) < 3 {
		return bulkRecord{}, fmt.Errorf("Line %d: use Discord mentions for players and winner.", lineNumber)
	}

	winnerTeam := parseBulkWinnerTeam(line)
	switch {
	case isBulk2v2Line(line, mentionIDs, winnerTeam):
		return parseBulk2v2Record(lineNumber, count, mentionIDs, winnerTeam)
	default:
		return parseBulk1v1Record(lineNumber, count, mentionIDs)
	}
}

func parseBulkCount(line string) (int, string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 1, line
	}

	token := strings.TrimSuffix(strings.ToLower(fields[0]), "x")
	count, err := strconv.Atoi(token)
	if err != nil || count < 1 {
		return 1, line
	}

	return count, strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
}

func parseBulkWinnerTeam(line string) string {
	fields := strings.Fields(strings.ToLower(line))
	for index, field := range fields {
		field = strings.Trim(field, ":,;")
		if field != "winner" && field != "win" && field != "w" {
			continue
		}
		if index+1 >= len(fields) {
			return ""
		}

		return normalizeWinnerTeam(fields[index+1])
	}

	return ""
}

func normalizeWinnerTeam(value string) string {
	value = strings.Trim(strings.ToLower(value), ":,;")
	switch value {
	case "1", "t1", "team1", "team_1":
		return "team1"
	case "2", "t2", "team2", "team_2":
		return "team2"
	default:
		return ""
	}
}

func isBulk2v2Line(line string, mentionIDs []string, winnerTeam string) bool {
	line = strings.ToLower(line)
	return strings.Contains(line, "2v2") || winnerTeam != "" || len(mentionIDs) >= 4
}

func parseBulk1v1Record(lineNumber, count int, mentionIDs []string) (bulkRecord, error) {
	if len(mentionIDs) < 3 {
		return bulkRecord{}, fmt.Errorf("Line %d: 1v1 format needs player1, player2, and winner mentions.", lineNumber)
	}

	record := bulkRecord{
		Line:      lineNumber,
		Count:     count,
		Player1ID: mentionIDs[0],
		Player2ID: mentionIDs[1],
		WinnerID:  mentionIDs[2],
	}

	if record.Player1ID == record.Player2ID {
		return bulkRecord{}, fmt.Errorf("Line %d: player 1 and player 2 must be different people.", lineNumber)
	}
	if record.WinnerID != record.Player1ID && record.WinnerID != record.Player2ID {
		return bulkRecord{}, fmt.Errorf("Line %d: 1v1 winner must be player 1 or player 2.", lineNumber)
	}

	return record, nil
}

func parseBulk2v2Record(lineNumber, count int, mentionIDs []string, winnerTeam string) (bulkRecord, error) {
	if len(mentionIDs) < 4 {
		return bulkRecord{}, fmt.Errorf("Line %d: 2v2 format needs four player mentions.", lineNumber)
	}

	record := bulkRecord{
		Line:       lineNumber,
		Count:      count,
		Team1IDs:   []string{mentionIDs[0], mentionIDs[1]},
		Team2IDs:   []string{mentionIDs[2], mentionIDs[3]},
		WinnerTeam: winnerTeam,
	}

	if record.WinnerTeam == "" && len(mentionIDs) >= 5 {
		record.WinnerTeam = winnerTeamFromPlayerID(mentionIDs[4], record.Team1IDs, record.Team2IDs)
	}
	if record.WinnerTeam == "" {
		return bulkRecord{}, fmt.Errorf("Line %d: 2v2 format needs winner team1 or winner team2.", lineNumber)
	}

	if duplicatePlayerIDs(record.Team1IDs, record.Team2IDs) {
		return bulkRecord{}, fmt.Errorf("Line %d: each player can only appear once in a 2v2 game.", lineNumber)
	}

	return record, nil
}

func winnerTeamFromPlayerID(winnerID string, team1IDs, team2IDs []string) string {
	for _, playerID := range team1IDs {
		if winnerID == playerID {
			return "team1"
		}
	}
	for _, playerID := range team2IDs {
		if winnerID == playerID {
			return "team2"
		}
	}
	return ""
}

func duplicatePlayerIDs(teamIDs ...[]string) bool {
	seen := make(map[string]bool)
	for _, ids := range teamIDs {
		for _, id := range ids {
			if id == "" {
				continue
			}
			if seen[id] {
				return true
			}
			seen[id] = true
		}
	}
	return false
}

func (record bulkRecord) is2v2() bool {
	return len(record.Team1IDs) > 0 || len(record.Team2IDs) > 0
}

func bulkImportHelp() string {
	return "Bulk format examples:\n" +
		"`<@player1> vs <@player2> winner <@player1>`\n" +
		"`3x <@player1> vs <@player2> winner <@player2>`\n" +
		"`2v2 <@team1a> + <@team1b> vs <@team2a> + <@team2b> winner team1`\n" +
		"`2x 2v2 <@team1a> + <@team1b> vs <@team2a> + <@team2b> winner team2`"
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

func loadScores() (scores, error) {
	currentScores := scores{
		Matchups:        make(map[string]matchupScore),
		RegisteredTeams: make(map[string]savedTeam),
	}

	data, err := os.ReadFile(scoresFile)
	if err != nil {
		if os.IsNotExist(err) {
			return currentScores, nil
		}
		return scores{}, err
	}

	if len(data) == 0 {
		return currentScores, nil
	}

	if err := json.Unmarshal(data, &currentScores); err != nil {
		return scores{}, err
	}

	if currentScores.Matchups == nil {
		currentScores.Matchups = make(map[string]matchupScore)
	}
	if currentScores.RegisteredTeams == nil {
		currentScores.RegisteredTeams = make(map[string]savedTeam)
	}

	return currentScores, nil
}

func saveScores(currentScores scores) error {
	data, err := json.MarshalIndent(currentScores, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(scoresFile, data, 0644)
}

func matchupKey(player1ID, player2ID string) string {
	players := []string{player1ID, player2ID}
	sort.Strings(players)
	return strings.Join(players, "|")
}

func teamMatchupKey(team1, team2 teamOption) string {
	teams := []string{teamKey(team1), teamKey(team2)}
	sort.Strings(teams)
	return strings.Join(teams, "|")
}

func teamKey(team teamOption) string {
	playerIDs := teamPlayerIDs(team)
	sort.Strings(playerIDs)
	return strings.Join(playerIDs, "+")
}

func teamDisplayName(team teamOption) string {
	if team.Name != "" {
		return team.Name
	}

	names := make([]string, 0, len(team.Players))
	for _, player := range team.Players {
		names = append(names, player.DisplayName)
	}
	return strings.Join(names, " + ")
}

func teamPlayerIDs(team teamOption) []string {
	playerIDs := make([]string, 0, len(team.Players))
	for _, player := range team.Players {
		playerIDs = append(playerIDs, player.ID)
	}
	sort.Strings(playerIDs)
	return playerIDs
}

func normalizeTeamName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func reservedTeamName(name string) bool {
	switch normalizeTeamName(name) {
	case "team1", "team2":
		return true
	default:
		return false
	}
}

func userMentions(userIDs []string) []string {
	mentions := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		mentions = append(mentions, userMention(userID))
	}
	return mentions
}

func duplicatePlayers(teams ...teamOption) bool {
	seen := make(map[string]bool)
	for _, team := range teams {
		for _, player := range team.Players {
			if player.ID == "" {
				continue
			}
			if seen[player.ID] {
				return true
			}
			seen[player.ID] = true
		}
	}
	return false
}

func validTeam(team teamOption) bool {
	if len(team.Players) != 2 {
		return false
	}
	for _, player := range team.Players {
		if player.ID == "" {
			return false
		}
	}
	return true
}

func matchingPlayer(winner, player1, player2 playerOption) (playerOption, bool) {
	switch {
	case winner.ID == player1.ID:
		return player1, true
	case winner.ID == player2.ID:
		return player2, true
	default:
		return playerOption{}, false
	}
}

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

func formatTeamScoreboard(team1, team2 teamOption, matchup matchupScore, prefix string) string {
	team1Name := teamDisplayName(team1)
	team2Name := teamDisplayName(team2)
	team1Wins := matchup.Teams[teamKey(team1)]
	team2Wins := matchup.Teams[teamKey(team2)]

	var builder strings.Builder
	if prefix != "" {
		builder.WriteString(prefix)
		builder.WriteString("\n")
	}
	builder.WriteString(fmt.Sprintf("```\n%s vs %s\n%s: %d\n%s: %d\n```", team1Name, team2Name, team1Name, team1Wins, team2Name, team2Wins))

	return builder.String()
}
