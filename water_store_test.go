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

func sipAt(player, track string, round time.Time) sip {
	return sip{Player: player, Track: track, Day: round.Format(time.DateOnly), Round: round, At: round}
}

func recordSip(t *testing.T, ws *WaterStore, s sip) int {
	t.Helper()
	today, err := ws.RecordSip(s)
	if err != nil {
		t.Fatalf("RecordSip(%+v): %v", s, err)
	}
	return today
}

func TestRecordSipOncePerRound(t *testing.T) {
	ws := newTestWaterStore(t)
	round1 := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	round2 := round1.Add(5 * time.Minute)

	if got := recordSip(t, ws, sipAt("a", "5min", round1)); got != 1 {
		t.Errorf("first sip today = %d, want 1", got)
	}

	if _, err := ws.RecordSip(sipAt("a", "5min", round1)); !errors.Is(err, errAlreadySipped) {
		t.Errorf("second sip in the same round: err = %v, want errAlreadySipped", err)
	}

	if got := recordSip(t, ws, sipAt("a", "5min", round2)); got != 2 {
		t.Errorf("sip in the next round today = %d, want 2", got)
	}

	// Other tracks and other players have their own rounds.
	if got := recordSip(t, ws, sipAt("a", "10min", round1)); got != 1 {
		t.Errorf("sip on another track today = %d, want 1", got)
	}
	if got := recordSip(t, ws, sipAt("b", "5min", round1)); got != 1 {
		t.Errorf("another player's sip today = %d, want 1", got)
	}
}

func TestRecordSipDuplicateAfterReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agua.json")
	loc := saoPaulo(t)
	round := monday(loc, 10, 5, 0)

	recordSip(t, NewWaterStore(path), sipAt("a", "5min", round))

	// A new store reads the round back from JSON, with a different Location
	// than monday() builds; the duplicate must still be caught.
	if _, err := NewWaterStore(path).RecordSip(sipAt("a", "5min", monday(loc, 10, 5, 0))); !errors.Is(err, errAlreadySipped) {
		t.Errorf("err = %v, want errAlreadySipped", err)
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
