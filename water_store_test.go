package main

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newTestWaterStore(t *testing.T) *WaterStore {
	t.Helper()
	return NewWaterStore(filepath.Join(t.TempDir(), "agua.json"))
}

func sipAt(player, track string, at time.Time) sip {
	return sip{Player: player, Track: track, Day: at.Format(time.DateOnly), At: at}
}

func recordSip(t *testing.T, ws *WaterStore, s sip) int {
	t.Helper()
	today, err := ws.RecordSip(s)
	if err != nil {
		t.Fatalf("RecordSip(%+v): %v", s, err)
	}
	return today
}

func TestRecordSipCountsEverySip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agua.json")
	ws := NewWaterStore(path)
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	if got := recordSip(t, ws, sipAt("a", "5min", now)); got != 1 {
		t.Errorf("first sip today = %d, want 1", got)
	}

	// No limit: a sip a second later counts too.
	if got := recordSip(t, ws, sipAt("a", "5min", now.Add(time.Second))); got != 2 {
		t.Errorf("second sip today = %d, want 2", got)
	}

	// Other tracks, players and days have their own counts.
	if got := recordSip(t, ws, sipAt("a", "10min", now)); got != 1 {
		t.Errorf("sip on another track today = %d, want 1", got)
	}
	if got := recordSip(t, ws, sipAt("b", "5min", now)); got != 1 {
		t.Errorf("another player's sip today = %d, want 1", got)
	}
	if got := recordSip(t, ws, sipAt("a", "5min", now.AddDate(0, 0, 1))); got != 1 {
		t.Errorf("first sip the next day = %d, want 1", got)
	}

	// The count survives reading the file again.
	if got := recordSip(t, NewWaterStore(path), sipAt("a", "5min", now.Add(time.Minute))); got != 3 {
		t.Errorf("sip after reload = %d, want 3", got)
	}
}

func TestDailyRanking(t *testing.T) {
	ws := newTestWaterStore(t)
	day1 := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	day2 := day1.AddDate(0, 0, 1)
	minutes := func(base time.Time, n int) time.Time { return base.Add(time.Duration(n) * 5 * time.Minute) }

	for _, s := range []sip{
		sipAt("b", "5min", minutes(day1, 0)),
		sipAt("b", "5min", minutes(day1, 1)),
		sipAt("c", "5min", minutes(day1, 0)),
		sipAt("c", "5min", minutes(day1, 1)),
		sipAt("a", "5min", minutes(day1, 0)),
		sipAt("a", "10min", minutes(day1, 0)), // other track
		sipAt("a", "5min", minutes(day2, 0)),  // other day
		sipAt("a", "5min", minutes(day2, 1)),
	} {
		recordSip(t, ws, s)
	}

	got, err := ws.DailyRanking("5min", "2026-09-21")
	if err != nil {
		t.Fatalf("DailyRanking: %v", err)
	}

	want := []rankEntry{{"b", 2}, {"c", 2}, {"a", 1}}
	if !slices.Equal(got, want) {
		t.Errorf("ranking = %v, want %v", got, want)
	}

	empty, err := ws.DailyRanking("5min", "2026-09-23")
	if err != nil {
		t.Fatalf("DailyRanking: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ranking of a day without sips = %v, want empty", empty)
	}
}

func TestTokens(t *testing.T) {
	ws := newTestWaterStore(t)

	if _, err := ws.PlayerForToken(""); !errors.Is(err, errUnknownToken) {
		t.Errorf("empty token: err = %v, want errUnknownToken", err)
	}

	first, err := ws.NewToken("a")
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	player, err := ws.PlayerForToken(first)
	if err != nil || player != "a" {
		t.Errorf("PlayerForToken(first) = %q, %v; want a", player, err)
	}

	second, err := ws.NewToken("a")
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if second == first {
		t.Fatal("new token equals the old one")
	}

	if _, err := ws.PlayerForToken(first); !errors.Is(err, errUnknownToken) {
		t.Errorf("replaced token: err = %v, want errUnknownToken", err)
	}

	player, err = ws.PlayerForToken(second)
	if err != nil || player != "a" {
		t.Errorf("PlayerForToken(second) = %q, %v; want a", player, err)
	}
}
