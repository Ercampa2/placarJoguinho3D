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

func TestRoundAt(t *testing.T) {
	loc := saoPaulo(t)
	sched := workSchedule(loc)
	at := func(hour, minute, second int) time.Time { return monday(loc, hour, minute, second) }
	var closed time.Time

	tests := []struct {
		name          string
		now           time.Time
		want5, want10 time.Time
	}{
		{"before 9", at(8, 59, 59), closed, closed},
		{"9 sharp", at(9, 0, 0), at(9, 0, 0), at(9, 0, 0)},
		{"inside a round", at(9, 7, 30), at(9, 5, 0), at(9, 0, 0)},
		{"last round before lunch", at(12, 29, 59), at(12, 25, 0), at(12, 20, 0)},
		{"lunch starts", at(12, 30, 0), closed, closed},
		{"lunch ends", at(13, 29, 59), closed, closed},
		{"after lunch", at(13, 30, 0), at(13, 30, 0), at(13, 30, 0)},
		{"last round of the day", at(15, 59, 59), at(15, 55, 0), at(15, 50, 0)},
		{"day over", at(16, 0, 0), closed, closed},
		{"saturday", time.Date(2026, 9, 26, 10, 0, 0, 0, loc), closed, closed},
		{"sunday", time.Date(2026, 9, 27, 10, 0, 0, 0, loc), closed, closed},
		{"time given in UTC", time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), at(9, 0, 0), at(9, 0, 0)},
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
				got, open := sched.roundAt(tt.now, c.interval)
				if open != !c.want.IsZero() || (open && !got.Equal(c.want)) {
					t.Errorf("roundAt(%v, %v) = %v, %v; want %v", tt.now, c.interval, got, open, c.want)
				}
			}
		})
	}
}

func TestDayOver(t *testing.T) {
	loc := saoPaulo(t)
	sched := workSchedule(loc)

	tests := []struct {
		now  time.Time
		want bool
	}{
		{monday(loc, 9, 0, 0), false},
		{monday(loc, 15, 59, 59), false},
		{monday(loc, 16, 0, 0), true},
		{monday(loc, 23, 59, 59), true},
		{time.Date(2026, 9, 26, 17, 0, 0, 0, loc), false}, // Saturday: no reminder day
	}

	for _, tt := range tests {
		if got := sched.dayOver(tt.now); got != tt.want {
			t.Errorf("dayOver(%v) = %v, want %v", tt.now, got, tt.want)
		}
	}
}

func TestRoundsPerDay(t *testing.T) {
	loc := saoPaulo(t)
	sched := workSchedule(loc)

	if got := sched.roundsPerDay(5 * time.Minute); got != 72 {
		t.Errorf("roundsPerDay(5m) = %d, want 72", got)
	}
	if got := sched.roundsPerDay(10 * time.Minute); got != 36 {
		t.Errorf("roundsPerDay(10m) = %d, want 36", got)
	}
}

func TestTestSchedule(t *testing.T) {
	loc := saoPaulo(t)
	saturday := func(hour, minute, second int) time.Time {
		return time.Date(2026, 9, 26, hour, minute, second, 0, loc)
	}
	sched := testSchedule(loc, saturday(21, 15, 20))

	if _, open := sched.roundAt(saturday(21, 15, 59), time.Minute); open {
		t.Error("round open before the test window")
	}
	if got, open := sched.roundAt(saturday(21, 16, 0), time.Minute); !open || !got.Equal(saturday(21, 16, 0)) {
		t.Errorf("first round = %v, %v; want 21:16", got, open)
	}
	if got, open := sched.roundAt(saturday(21, 25, 59), 2*time.Minute); !open || !got.Equal(saturday(21, 24, 0)) {
		t.Errorf("last 2-minute round = %v, %v; want 21:24", got, open)
	}
	if !sched.dayOver(saturday(21, 26, 0)) {
		t.Error("test day not over at 21:26")
	}
	if got := sched.roundsPerDay(time.Minute); got != 10 {
		t.Errorf("roundsPerDay(1m) = %d, want 10", got)
	}
}

func TestDay(t *testing.T) {
	loc := saoPaulo(t)
	sched := workSchedule(loc)

	// 01:30 UTC is still the previous evening in São Paulo.
	if got := sched.day(time.Date(2026, 9, 22, 1, 30, 0, 0, time.UTC)); got != "2026-09-21" {
		t.Errorf("day = %q, want 2026-09-21", got)
	}
}
