package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// colorWater is the accent of the water ranking embed.
const colorWater = 0x3498DB

// waterTrack is one series of reminders, with its own channel, interval and
// ranking. name is what goes in the sip URL (/gole/5min) and in agua.json.
type waterTrack struct {
	name      string
	label     string
	channelID string
	interval  time.Duration
}

// waterFeature is the drink-water game: timed reminders in Discord, the HTTP
// endpoint that records sips, and the /agua_token and /ranking_agua commands.
type waterFeature struct {
	store   *WaterStore
	sched   schedule
	tracks  []waterTrack
	addr    string           // where the sip endpoint listens, like ":8080"
	baseURL string           // how players reach it; only shown by /agua_token
	now     func() time.Time // time.Now, replaced in tests
}

// waterEnvVars are the variables newWaterFromEnv reads.
var waterEnvVars = []string{"AGUA_CANAL_5MIN", "AGUA_CANAL_10MIN", "AGUA_HTTP_ADDR", "AGUA_URL", "AGUA_TESTE"}

// unknownWaterVars returns the AGUA_* names in environ (as from os.Environ)
// that the bot does not read. They are most likely typos, like
// AGUA_CANAL_5_MIN for AGUA_CANAL_5MIN, which would otherwise be ignored
// without a word.
func unknownWaterVars(environ []string) []string {
	var unknown []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "AGUA_") && !slices.Contains(waterEnvVars, name) {
			unknown = append(unknown, name)
		}
	}

	return unknown
}

// newWaterFromEnv reads the AGUA_* variables. It returns nil, and no error,
// when no reminder channel is set: the feature is then off.
func newWaterFromEnv() (*waterFeature, error) {
	for _, name := range unknownWaterVars(os.Environ()) {
		log.Printf("unknown variable %s ignored; the water reminders read %s", name, strings.Join(waterEnvVars, ", "))
	}

	testMode := os.Getenv("AGUA_TESTE") == "1"

	available := []struct {
		env   string
		track waterTrack
	}{
		{"AGUA_CANAL_5MIN", waterTrack{name: "5min", label: "5 min", interval: 5 * time.Minute}},
		{"AGUA_CANAL_10MIN", waterTrack{name: "10min", label: "10 min", interval: 10 * time.Minute}},
	}

	var tracks []waterTrack
	for _, a := range available {
		track := a.track
		track.channelID = os.Getenv(a.env)
		if track.channelID == "" {
			continue
		}
		if testMode {
			track.interval /= 5 // 1 and 2 minutes
		}
		tracks = append(tracks, track)
	}

	if len(tracks) == 0 {
		log.Println("water reminders off: set AGUA_CANAL_5MIN and/or AGUA_CANAL_10MIN to turn them on and register /agua_token and /ranking_agua")
		return nil, nil
	}

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, err
	}

	addr := os.Getenv("AGUA_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	wf := &waterFeature{
		store:   NewWaterStore("agua.json"),
		sched:   workSchedule(loc),
		tracks:  tracks,
		addr:    addr,
		baseURL: os.Getenv("AGUA_URL"),
		now:     time.Now,
	}

	if testMode {
		wf.sched = testSchedule(loc, wf.now())
		w := wf.sched.windows[0]
		log.Printf("AGUA_TESTE: reminders every day from %02d:%02d to %02d:%02d", w.from/60, w.from%60, w.to/60, w.to%60)
	}

	return wf, nil
}

// start listens for sips and starts one reminder loop per track. The loops
// stop when ctx is done; the returned function closes the HTTP server.
func (wf *waterFeature) start(ctx context.Context, s *discordgo.Session) (func(), error) {
	// Listening here rather than inside the goroutine makes a busy port an
	// error at startup instead of a log line nobody reads.
	listener, err := net.Listen("tcp", wf.addr)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Handler:           wf.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("water http: %v", err)
		}
	}()

	for _, track := range wf.tracks {
		// Checking the channel now reports a bad ID or a missing permission at
		// startup, instead of at the first reminder, which can be hours away.
		channel, err := s.Channel(track.channelID)
		if err != nil {
			log.Printf("water reminders: track %s cannot use channel %s: %v. Check the ID, and give the bot View Channel, Send Messages and Embed Links there.", track.name, track.channelID, err)
		} else {
			log.Printf("water reminders: track %s in #%s, every %v", track.name, channel.Name, track.interval)
		}

		go wf.runReminders(ctx, s, track)
	}

	log.Printf("water sips: listening on %s", wf.addr)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("water http shutdown: %v", err)
		}
	}

	return shutdown, nil
}

func (wf *waterFeature) track(name string) (waterTrack, bool) {
	for _, t := range wf.tracks {
		if t.name == name {
			return t, true
		}
	}

	return waterTrack{}, false
}

func (wf *waterFeature) trackNames() string {
	names := make([]string, 0, len(wf.tracks))
	for _, t := range wf.tracks {
		names = append(names, t.name)
	}

	return strings.Join(names, ", ")
}

func (wf *waterFeature) commands() []*discordgo.ApplicationCommand {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(wf.tracks))
	for _, t := range wf.tracks {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: t.label, Value: t.name})
	}

	return []*discordgo.ApplicationCommand{
		{
			Name:        "agua_token",
			Description: "Gera o seu token para o script de água (o token anterior para de funcionar)",
		},
		{
			Name:        "ranking_agua",
			Description: "Mostra o ranking de água de um dia",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "trilha",
					Description: "Qual lembrete",
					Required:    true,
					Choices:     choices,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "data",
					Description: "Dia, como 24/09 ou 24/09/2026 (padrão: hoje)",
				},
			},
		},
	}
}

func (wf *waterFeature) handleCommand(i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) (reply, error) {
	switch data.Name {
	case "agua_token":
		return wf.newToken(interactionUserID(i))
	case "ranking_agua":
		return wf.showRanking(data)
	default:
		return textReply(fmt.Sprintf("Unknown command: /%s.", data.Name)), nil
	}
}

func (wf *waterFeature) newToken(playerID string) (reply, error) {
	if playerID == "" {
		return textReply("Não consegui identificar quem pediu o token."), nil
	}

	token, err := wf.store.NewToken(playerID)
	if err != nil {
		return reply{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🔑 Seu token para o script de água (não compartilhe):\n`%s`\n", token)
	if wf.baseURL != "" {
		fmt.Fprintf(&b, "Endereço do bot: `%s`\n", wf.baseURL)
	}
	fmt.Fprintf(&b, "Trilhas: `%s`\n", wf.trackNames())
	b.WriteString("Cole no `agua.sh` (Linux) ou no `agua.ps1` (Windows). Se gerar outro token, este para de funcionar.")

	return reply{content: b.String(), ephemeral: true}, nil
}

func (wf *waterFeature) showRanking(data discordgo.ApplicationCommandInteractionData) (reply, error) {
	var trackName, dateText string
	for _, option := range data.Options {
		switch option.Name {
		case "trilha":
			trackName = option.StringValue()
		case "data":
			dateText = option.StringValue()
		}
	}

	track, ok := wf.track(trackName)
	if !ok {
		return textReply(fmt.Sprintf("Trilha desconhecida. Use: %s.", wf.trackNames())), nil
	}

	now := wf.now()
	day := wf.sched.day(now)
	if dateText != "" {
		parsed, err := parseDay(dateText, now.In(wf.sched.loc))
		if err != nil {
			return textReply("Data inválida. Use 24/09, 24/09/2026 ou 2026-09-24."), nil
		}
		day = parsed
	}

	ranking, err := wf.store.DailyRanking(track.name, day)
	if err != nil {
		return reply{}, err
	}

	return embedReply(wf.rankingEmbed(track, day, ranking)), nil
}

// parseDay reads a date typed in /ranking_agua and returns it as
// "2006-01-02". A date without a year is in today's year.
func parseDay(text string, today time.Time) (string, error) {
	text = strings.TrimSpace(text)

	// Go layouts spell dates with a reference date instead of letters like
	// PHP's "d/m/Y": "2" is the day and "1" the month, one or two digits each.
	for _, layout := range []string{"2/1/2006", time.DateOnly} {
		if t, err := time.Parse(layout, text); err == nil {
			return t.Format(time.DateOnly), nil
		}
	}

	t, err := time.Parse("2/1", text)
	if err != nil {
		return "", err
	}

	return time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format(time.DateOnly), nil
}

// rankingEmbed lists who drank the most on day. Players with the same count
// share a place, and the next place skips accordingly (1, 1, 3).
func (wf *waterFeature) rankingEmbed(track waterTrack, day string, ranking []rankEntry) *discordgo.MessageEmbed {
	date := day
	if t, err := time.Parse(time.DateOnly, day); err == nil {
		date = t.Format("02/01/2006")
	}

	embed := &discordgo.MessageEmbed{
		Title: "🏆 Ranking de água — " + track.label,
		Color: colorWater,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%s · %d lembretes no dia", date, wf.sched.roundsPerDay(track.interval)),
		},
	}

	if len(ranking) == 0 {
		embed.Description = "Ninguém registrou gole neste dia."
		return embed
	}

	var b strings.Builder
	place := 0
	for i, entry := range ranking {
		if i == 0 || entry.Sips < ranking[i-1].Sips {
			place = i + 1
		}
		// A mention shows the player's name; embeds never notify anyone.
		fmt.Fprintf(&b, "%s %s — %d\n", medal(place), userMention(entry.Player), entry.Sips)
	}
	embed.Description = b.String()

	return embed
}

func medal(place int) string {
	switch place {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return fmt.Sprintf("%d.", place)
	}
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	switch {
	case i.Member != nil && i.Member.User != nil:
		return i.Member.User.ID
	case i.User != nil:
		return i.User.ID
	default:
		return ""
	}
}
