package main

import (
	"time"
	_ "time/tzdata" // lets LoadLocation work even where the OS has no time zone files
)

// window is a stretch of the day, in minutes since midnight. from is
// included and to is not: {hm(9, 0), hm(16, 0)} covers 09:00 up to, but not
// including, 16:00.
type window struct {
	from, to int
}

func hm(hour, minute int) int {
	return hour*60 + minute
}

// schedule is a set of windows on some days of the week. The water feature
// uses two: one says when reminders are posted, the other when sips score.
type schedule struct {
	loc      *time.Location
	windows  []window // in order, not overlapping
	weekends bool     // also open on Saturday and Sunday
}

// reminderSchedule is when "Beba água" is posted: all day, Monday to Friday.
func reminderSchedule(loc *time.Location) schedule {
	return schedule{
		loc:     loc,
		windows: []window{{hm(0, 0), hm(24, 0)}},
	}
}

// scoringSchedule is when sips count: Monday to Friday, 09:00–16:00. The
// daily ranking is posted when it closes.
func scoringSchedule(loc *time.Location) schedule {
	return schedule{
		loc:     loc,
		windows: []window{{hm(9, 0), hm(16, 0)}},
	}
}

// testSchedule is open for ten minutes from the next full minute after now,
// on any day, so scoring and the ranking can be tried out at any hour.
func testSchedule(loc *time.Location, now time.Time) schedule {
	now = now.In(loc)
	start := hm(now.Hour(), now.Minute()+1)

	return schedule{
		loc:      loc,
		windows:  []window{{start, start + 10}},
		weekends: true,
	}
}

func (s schedule) runsOn(t time.Time) bool {
	switch t.In(s.loc).Weekday() {
	case time.Saturday, time.Sunday:
		return s.weekends
	default:
		return true
	}
}

// at returns the time that is minutes after midnight on t's day. time.Date
// carries the extra minutes over into hours, so at(t, hm(9, 30)) is 09:30 and
// at(t, hm(24, 0)) is the next midnight. Building the time this way, instead
// of adding a Duration to midnight, stays right on days when the clock
// changes.
func (s schedule) at(t time.Time, minutes int) time.Time {
	t = t.In(s.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, minutes, 0, 0, s.loc)
}

// windowAt returns the start of the window open at t, or false when none is.
func (s schedule) windowAt(t time.Time) (time.Time, bool) {
	if !s.runsOn(t) {
		return time.Time{}, false
	}

	for _, w := range s.windows {
		from, to := s.at(t, w.from), s.at(t, w.to)
		if !t.Before(from) && t.Before(to) {
			return from, true
		}
	}

	return time.Time{}, false
}

// isOpen reports whether t falls in one of the windows.
func (s schedule) isOpen(t time.Time) bool {
	_, open := s.windowAt(t)
	return open
}

// roundAt returns the start of the current round when something happens
// every interval from the start of each window, like the reminders: 00:00,
// 00:05, 00:10... It returns false when no window is open.
func (s schedule) roundAt(t time.Time, interval time.Duration) (time.Time, bool) {
	from, open := s.windowAt(t)
	if !open {
		return time.Time{}, false
	}

	// A Duration is an integer number of nanoseconds, so this division drops
	// the remainder: it counts the whole rounds since from.
	return from.Add(t.Sub(from) / interval * interval), true
}

// dayOver reports whether t is on an open day, after its last window.
func (s schedule) dayOver(t time.Time) bool {
	if !s.runsOn(t) || len(s.windows) == 0 {
		return false
	}

	return !t.Before(s.at(t, s.windows[len(s.windows)-1].to))
}

// day is t's date where the bot runs, like "2026-09-24". Sips are grouped by
// it, which is what makes each day's ranking start from zero.
func (s schedule) day(t time.Time) string {
	return t.In(s.loc).Format(time.DateOnly)
}
