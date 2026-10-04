package main

import (
	"testing"
	"time"
)

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// monday returns a time on Monday 2026-09-21 in São Paulo.
func monday(loc *time.Location, hour, minute, second int) time.Time {
	return time.Date(2026, 9, 21, hour, minute, second, 0, loc)
}

func TestScoringIsOpen(t *testing.T) {
	loc := saoPaulo(t)
	scoring := scoringSchedule(loc)

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before 9", monday(loc, 8, 59, 59), false},
		{"9 sharp", monday(loc, 9, 0, 0), true},
		{"lunch", monday(loc, 12, 45, 0), true},
		{"last second", monday(loc, 15, 59, 59), true},
		{"16 sharp", monday(loc, 16, 0, 0), false},
		{"saturday", time.Date(2026, 9, 26, 10, 0, 0, 0, loc), false},
		{"sunday", time.Date(2026, 9, 27, 10, 0, 0, 0, loc), false},
		{"time given in UTC", time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), true}, // 09:00 in São Paulo
	}

	for _, tt := range tests {
		if got := scoring.isOpen(tt.now); got != tt.want {
			t.Errorf("%s: isOpen(%v) = %v, want %v", tt.name, tt.now, got, tt.want)
		}
	}
}

func TestReminderRoundAt(t *testing.T) {
	loc := saoPaulo(t)
	reminders := reminderSchedule(loc)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }
	var closed time.Time

	tests := []struct {
		name          string
		now           time.Time
		want5, want10 time.Time
	}{
		{"midnight", at(0, 0, 0), at(0, 0, 0), at(0, 0, 0)},
		{"early morning", at(6, 7, 30), at(6, 5, 0), at(6, 0, 0)},
		{"lunch", at(12, 47, 0), at(12, 45, 0), at(12, 40, 0)},
		{"evening", at(23, 59, 59), at(23, 55, 0), at(23, 50, 0)},
		{"saturday", time.Date(2026, 9, 26, 10, 0, 0, 0, loc), closed, closed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range []struct {
				interval time.Duration
				want     time.Time
			}{
				{5 * time.Minute, tt.want5},
				{10 * time.Minute, tt.want10},
			} {
				got, open := reminders.roundAt(tt.now, c.interval)
				if open != !c.want.IsZero() || (open && !got.Equal(c.want)) {
					t.Errorf("roundAt(%v, %v) = %v, %v; want %v", tt.now, c.interval, got, open, c.want)
				}
			}
		})
	}
}

func TestDayOver(t *testing.T) {
	loc := saoPaulo(t)
	scoring := scoringSchedule(loc)

	tests := []struct {
		now  time.Time
		want bool
	}{
		{monday(loc, 9, 0, 0), false},
		{monday(loc, 15, 59, 59), false},
		{monday(loc, 16, 0, 0), true},
		{monday(loc, 23, 59, 59), true},
		{time.Date(2026, 9, 26, 17, 0, 0, 0, loc), false}, // Saturday: no scoring day
	}

	for _, tt := range tests {
		if got := scoring.dayOver(tt.now); got != tt.want {
			t.Errorf("dayOver(%v) = %v, want %v", tt.now, got, tt.want)
		}
	}
}

func TestTestSchedule(t *testing.T) {
	loc := saoPaulo(t)
	saturday := func(hour, minute, second int) time.Time {
		return time.Date(2026, 9, 26, hour, minute, second, 0, loc)
	}
	sched := testSchedule(loc, saturday(21, 15, 20))

	if sched.isOpen(saturday(21, 15, 59)) {
		t.Error("open before the test window")
	}
	if !sched.isOpen(saturday(21, 16, 0)) || !sched.isOpen(saturday(21, 25, 59)) {
		t.Error("closed inside the test window")
	}
	if sched.isOpen(saturday(21, 26, 0)) || !sched.dayOver(saturday(21, 26, 0)) {
		t.Error("test day not over at 21:26")
	}
}

func TestDay(t *testing.T) {
	loc := saoPaulo(t)
	scoring := scoringSchedule(loc)

	// 01:30 UTC is still the previous evening in São Paulo.
	if got := scoring.day(time.Date(2026, 9, 22, 1, 30, 0, 0, time.UTC)); got != "2026-09-21" {
		t.Errorf("day = %q, want 2026-09-21", got)
	}
}
