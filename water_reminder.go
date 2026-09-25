package main

import (
	"context"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
)

// reminder decides, tick by tick, when a track posts "Beba água" and when it
// posts the day's ranking. It only acts on changes it sees while running, so
// restarting the bot in the middle of a round, or after the day ended, does
// not post the same thing twice.
type reminder struct {
	sched     schedule
	interval  time.Duration
	lastRound time.Time
	rankedDay string
}

func newReminder(sched schedule, interval time.Duration, now time.Time) *reminder {
	r := &reminder{sched: sched, interval: interval}
	r.lastRound, _ = sched.roundAt(now, interval)
	if sched.dayOver(now) {
		r.rankedDay = sched.day(now)
	}

	return r
}

// tick reports whether a new round started since the last tick, which calls
// for a reminder, and whether the reminder day just ended, which calls for
// the ranking.
func (r *reminder) tick(now time.Time) (remind, rank bool) {
	if round, open := r.sched.roundAt(now, r.interval); open && !round.Equal(r.lastRound) {
		r.lastRound = round
		remind = true
	}

	if r.sched.dayOver(now) && r.rankedDay != r.sched.day(now) {
		r.rankedDay = r.sched.day(now)
		rank = true
	}

	return remind, rank
}

// runReminders posts track's reminders and daily ranking until ctx is done.
// It looks at the clock every second instead of sleeping five minutes at a
// time: a 5-minute ticker would count from whenever the bot started (10:03,
// 10:08...) and know nothing about lunch or weekends.
func (wf *waterFeature) runReminders(ctx context.Context, s *discordgo.Session, track waterTrack) {
	r := newReminder(wf.sched, track.interval, wf.now())

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
			wf.postRanking(s, track, wf.sched.day(now))
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
