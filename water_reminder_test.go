package main

import (
	"testing"
	"time"
)

type tickStep struct {
	now          time.Time
	remind, rank bool
}

func runTicks(t *testing.T, r *reminder, steps []tickStep) {
	t.Helper()
	for _, step := range steps {
		remind, rank := r.tick(step.now)
		if remind != step.remind || rank != step.rank {
			t.Errorf("tick(%v) = remind %v, rank %v; want %v, %v", step.now, remind, rank, step.remind, step.rank)
		}
	}
}

func newWorkReminder(loc *time.Location, now time.Time) *reminder {
	return newReminder(reminderSchedule(loc), scoringSchedule(loc), 5*time.Minute, now)
}

func TestReminderDay(t *testing.T) {
	loc := saoPaulo(t)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }

	r := newWorkReminder(loc, at(6, 0, 0))
	runTicks(t, r, []tickStep{
		{at(6, 0, 1), false, false}, // the 06:00 round was posted before the start
		{at(6, 4, 59), false, false},
		{at(6, 5, 0), true, false}, // reminders run outside scoring hours
		{at(9, 0, 0), true, false},
		{at(9, 0, 1), false, false},
		{at(12, 30, 0), true, false}, // and at lunch
		{at(15, 55, 0), true, false},
		{at(16, 0, 0), true, true}, // scoring closes: ranking
		{at(16, 0, 1), false, false},
		{at(23, 55, 0), true, false},
		{time.Date(2026, 9, 22, 0, 0, 0, 0, loc), true, false},
		{time.Date(2026, 9, 22, 16, 0, 0, 0, loc), true, true},
	})
}

func TestReminderRestart(t *testing.T) {
	loc := saoPaulo(t)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }

	// Restarted at 10:07: the 10:05 round was announced before the restart.
	r := newWorkReminder(loc, at(10, 7, 0))
	runTicks(t, r, []tickStep{
		{at(10, 7, 1), false, false},
		{at(10, 10, 0), true, false},
	})

	// Restarted after scoring closed: the ranking was already posted.
	r = newWorkReminder(loc, at(17, 2, 0))
	runTicks(t, r, []tickStep{
		{at(17, 2, 1), false, false},
		{at(17, 5, 0), true, false},
	})
}

func TestReminderWeekend(t *testing.T) {
	loc := saoPaulo(t)
	saturday := func(hour, minute int) time.Time { return time.Date(2026, 9, 26, hour, minute, 0, 0, loc) }

	r := newWorkReminder(loc, saturday(8, 0))
	runTicks(t, r, []tickStep{
		{saturday(9, 0), false, false},
		{saturday(16, 0), false, false},
	})
}
