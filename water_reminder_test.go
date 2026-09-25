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

func TestReminderDay(t *testing.T) {
	loc := saoPaulo(t)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }

	r := newReminder(workSchedule(loc), 5*time.Minute, at(8, 0, 0))
	runTicks(t, r, []tickStep{
		{at(8, 59, 59), false, false},
		{at(9, 0, 0), true, false},
		{at(9, 0, 1), false, false},
		{at(9, 4, 59), false, false},
		{at(9, 5, 0), true, false},
		{at(12, 25, 0), true, false},
		{at(12, 30, 0), false, false},
		{at(13, 29, 59), false, false},
		{at(13, 30, 0), true, false},
		{at(15, 55, 0), true, false},
		{at(16, 0, 0), false, true},
		{at(16, 0, 1), false, false},
		{at(23, 59, 59), false, false},
		{time.Date(2026, 9, 22, 9, 0, 0, 0, loc), true, false},
	})
}

func TestReminderRestart(t *testing.T) {
	loc := saoPaulo(t)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }

	// Restarted at 10:07: the 10:05 round was announced before the restart.
	r := newReminder(workSchedule(loc), 5*time.Minute, at(10, 7, 0))
	runTicks(t, r, []tickStep{
		{at(10, 7, 1), false, false},
		{at(10, 10, 0), true, false},
	})

	// Restarted after the day ended: the ranking was already posted.
	r = newReminder(workSchedule(loc), 5*time.Minute, at(17, 0, 0))
	runTicks(t, r, []tickStep{
		{at(17, 0, 1), false, false},
	})
}

func TestReminderWeekend(t *testing.T) {
	loc := saoPaulo(t)
	saturday := func(hour, minute int) time.Time { return time.Date(2026, 9, 26, hour, minute, 0, 0, loc) }

	r := newReminder(workSchedule(loc), 5*time.Minute, saturday(8, 0))
	runTicks(t, r, []tickStep{
		{saturday(9, 0), false, false},
		{saturday(16, 0), false, false},
	})
}
