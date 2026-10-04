package main

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// reminder decides, tick by tick, when a track posts "Beba água" (each new
// round of the reminder schedule) and when it posts the day's ranking (when
// scoring closes). It only acts on changes it sees while running, so
// restarting the bot in the middle of a round, or after scoring closed, does
// not post the same thing twice.
type reminder struct {
	reminders schedule
	scoring   schedule
	interval  time.Duration
	lastRound time.Time
	rankedDay string
}

func newReminder(reminders, scoring schedule, interval time.Duration, now time.Time) *reminder {
	r := &reminder{reminders: reminders, scoring: scoring, interval: interval}
	r.lastRound, _ = reminders.roundAt(now, interval)
	if scoring.dayOver(now) {
		r.rankedDay = scoring.day(now)
	}

	return r
}

// tick reports whether a new reminder round started since the last tick, and
// whether the day's scoring just closed, which calls for the ranking.
func (r *reminder) tick(now time.Time) (remind, rank bool) {
	if round, open := r.reminders.roundAt(now, r.interval); open && !round.Equal(r.lastRound) {
		r.lastRound = round
		remind = true
	}

	if r.scoring.dayOver(now) && r.rankedDay != r.scoring.day(now) {
		r.rankedDay = r.scoring.day(now)
		rank = true
	}

	return remind, rank
}

// runReminders posts track's reminders and daily ranking until ctx is done.
// It looks at the clock every second instead of sleeping five minutes at a
// time: a 5-minute ticker would count from whenever the bot started (10:03,
// 10:08...) and know nothing about weekends.
func (wf *waterFeature) runReminders(ctx context.Context, s *discordgo.Session, track waterTrack) {
	r := newReminder(wf.reminders, wf.scoring, track.interval, wf.now())

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		now := wf.now()
		remind, rank := r.tick(now)

		if remind {
			if _, err := s.ChannelMessageSend(track.channelID, "💧 Beba água!"); err != nil {
				log.Printf("water reminder %s: %v", track.name, err)
			}
		}

		if rank {
			wf.postRanking(s, track, wf.scoring.day(now))
		}
	}
}

func (wf *waterFeature) postRanking(s *discordgo.Session, track waterTrack, day string) {
	ranking, err := wf.store.DailyRanking(track.name, day)
	if err != nil {
		log.Printf("water ranking %s: %v", track.name, err)
		return
	}

	_, err = s.ChannelMessageSendComplex(track.channelID, &discordgo.MessageSend{
		Embeds:          []*discordgo.MessageEmbed{wf.rankingEmbed(track, day, ranking)},
		AllowedMentions: noMentions,
	})
	if err != nil {
		log.Printf("water ranking %s: %v", track.name, err)
	}
}
