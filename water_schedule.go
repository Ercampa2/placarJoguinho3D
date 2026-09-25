package main

import (
	"time"
	_ "time/tzdata" // lets LoadLocation work even where the OS has no time zone files
)

// window is a stretch of the day when reminders run, in minutes since
// midnight. from is included and to is not: {hm(9, 0), hm(12, 30)} covers
// 09:00 up to, but not including, 12:30.
type window struct {
	from, to int
}

func hm(hour, minute int) int {
	return hour*60 + minute
}

// schedule says when reminder rounds happen. Rounds start at the beginning
// of each window and then every interval, and each one lasts until the next
// one starts or the window ends. A round is identified by its start time
// alone, so the reminder loop and the sip endpoint agree on which round is
// open just by asking roundAt, with no shared state between them.
type schedule struct {
	loc      *time.Location
	windows  []window // in order, not overlapping
	weekends bool     // also run on Saturday and Sunday
}

// workSchedule is the real one: Monday to Friday, 09:00–12:30 and
// 13:30–16:00.
func workSchedule(loc *time.Location) schedule {
	return schedule{
		loc: loc,
		windows: []window{
			{hm(9, 0), hm(12, 30)},
			{hm(13, 30), hm(16, 0)},
		},
	}
}

// testSchedule runs for ten minutes from the next full minute after now, on
// any day, so reminders and the ranking can be tried out at any hour.
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
// carries the extra minutes over into hours, so at(t, hm(9, 30)) is 09:30.
// Building the time this way, instead of adding a Duration to midnight, stays
// right on days when the clock changes.
func (s schedule) at(t time.Time, minutes int) time.Time {
	t = t.In(s.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, minutes, 0, 0, s.loc)
}

// roundAt returns the start of the round open at t, for reminders every
// interval. It returns false when no round is open: outside the windows or
// on a day off.
func (s schedule) roundAt(t time.Time, interval time.Duration) (time.Time, bool) {
	if !s.runsOn(t) {
		return time.Time{}, false
	}

	for _, w := range s.windows {
		from, to := s.at(t, w.from), s.at(t, w.to)
		if t.Before(from) || !t.Before(to) {
			continue
		}

		// A Duration is an integer number of nanoseconds, so this division
		// drops the remainder: it counts the whole rounds since from.
		return from.Add(t.Sub(from) / interval * interval), true
	}

	return time.Time{}, false
}

// dayOver reports whether t is on a reminder day, after its last window. The
// day's ranking is due from then on.
func (s schedule) dayOver(t time.Time) bool {
	if !s.runsOn(t) || len(s.windows) == 0 {
		return false
	}

	return !t.Before(s.at(t, s.windows[len(s.windows)-1].to))
}

// roundsPerDay is how many rounds a reminder day has at this interval.
func (s schedule) roundsPerDay(interval time.Duration) int {
	total := 0
	for _, w := range s.windows {
		length := time.Duration(w.to-w.from) * time.Minute
		total += int((length + interval - 1) / interval)
	}

	return total
}

// day is t's date where the reminders run, like "2026-09-24". Sips are grouped
// by it, which is what makes each day's ranking start from zero.
func (s schedule) day(t time.Time) string {
	return t.In(s.loc).Format(time.DateOnly)
}
