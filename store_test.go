package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "scores.json"))
}

func TestRecordWinCounts(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.RecordWin("a", "b"); err != nil {
		t.Fatalf("RecordWin: %v", err)
	}

	matchup, err := store.RecordWin("a", "b")
	if err != nil {
		t.Fatalf("RecordWin: %v", err)
	}

	if got := matchup.Players["a"]; got != 2 {
		t.Errorf("a wins = %d, want 2", got)
	}

	if got := matchup.Players["b"]; got != 0 {
		t.Errorf("b wins = %d, want 0", got)
	}
}

func TestNoMacthupPlayer(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.Matchup("a", "b"); !errors.Is(err, errNoMatchupPlayers) {
		t.Errorf("Should get matchup error")
	}

	if _, err := store.RecordWin("a", "b"); err != nil {
		t.Fatalf("RecordWin: %v", err)
	}

	if _, err := store.Matchup("a", "b"); errors.Is(err, errNoMatchupPlayers) {
		t.Errorf("Should have found a matchup")
	}
}

func TestNoMacthupAll(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.AllMatchups(); !errors.Is(err, errNoMatchup) {
		t.Errorf("Should get matchup error")
	}

	if _, err := store.RecordWin("a", "b"); err != nil {
		t.Fatalf("RecordWin: %v", err)
	}

	if _, err := store.AllMatchups(); errors.Is(err, errNoMatchup) {
		t.Errorf("Should have found a matchup")
	}
}

func TestCheckPersistency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	store := NewStore(path)

	if _, err := store.RecordWin("a", "b"); err != nil {
		t.Fatalf("RecordWin: %v", err)
	}

	store2 := NewStore(path)
	matchup, err := store2.Matchup("a", "b")

	if errors.Is(err, errNoMatchupPlayers) {
		t.Errorf("Should get matchup")
	}

	if got := matchup.Players["a"]; got != 1 {
		t.Errorf("a wins = %d, want 1", got)
	}
}

func TestMatchupKeyTable(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"already sorted", "a", "b", "a|b"},
		{"reversed", "b", "a", "a|b"},
		{"mock id", "309516471770284034", "353354649157238784", "309516471770284034|353354649157238784"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchupKey(tc.a, tc.b); got != tc.want {
				t.Errorf("matchupKey(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestUndoWinTable(t *testing.T) {
	tests := []struct {
		name       string
		winsBefore int
		wantErr    error
		wantWins   int
	}{
		{"no wins to undo", 0, errNoWins, 0},
		{"one win", 1, nil, 0},
		{"two win", 2, nil, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			for i := 0; i < tc.winsBefore; i++ {
				if _, err := store.RecordWin("a", "b"); err != nil {
					t.Fatalf("RecordWin: %v", err)
				}
			}

			matchup, err := store.UndoWin("a", "b")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Undo win error: %v, want %v", err, tc.wantErr)
			}

			if tc.wantErr == nil && matchup.Players["a"] != tc.wantWins {
				t.Errorf("a want %v wins, got %v", tc.wantWins, matchup.Players["a"])
			}
		})
	}

}
