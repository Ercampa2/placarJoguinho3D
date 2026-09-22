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

func seed(t *testing.T, st *Store, games ...game) {
	t.Helper()
	if err := st.save(scores{Games: games}); err != nil {
		t.Fatal(err)
	}
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
		{"no wins to undo", 0, errNoMatchupPlayers, 0},
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

			_, matchup, err := store.UndoLast("a", "b")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Undo win error: %v, want %v", err, tc.wantErr)
			}

			if tc.wantErr == nil && matchup.Players["a"] != tc.wantWins {
				t.Errorf("a want %v wins, got %v", tc.wantWins, matchup.Players["a"])
			}

		})
	}

}

func TestUndoLastWin(t *testing.T) {
	st := newTestStore(t)
	seed(t, st, game{Winner: "a", Loser: "b"}, game{Winner: "b", Loser: "a"}, game{Winner: "c", Loser: "a"})

	removed, matchup, err := st.UndoLast("b", "a")

	if err != nil {
		t.Fatal(err)
	}

	if removed.Winner != "b" {
		t.Errorf("Removido %v, esperado remover b", removed.Winner)
	}

	if matchup.Players["a"] != 1 || matchup.Players["b"] != 0 {
		t.Errorf("Pontuação a: %v, b: %v, esperado a: 1, b: 0", matchup.Players["a"], matchup.Players["b"])
	}

	s, _ := st.load()
	if len(s.Games) != 2 {
		t.Errorf("Esperava ter 2 partida, achadas: %v", len(s.Games))
	}

	if _, _, err := st.UndoLast("b", "c"); !errors.Is(err, errNoMatchupPlayers) {
		t.Errorf("Desfez uma partida que não deveria existir, err: %v", err)
	}

}

func TestMigrateLegacy(t *testing.T) {
	fixture := scores{Matchups: map[string]matchupScore{
		"111|222": {Players: map[string]int{"111": 58, "222": 52}}, // ~110 games
		"111|333": {Players: map[string]int{"111": 7, "333": 0}},   // 333 never won
		"222|333": {Players: map[string]int{"222": 1, "333": 4}},
		"444|555": {Players: map[string]int{"444": 3, "555": 3}},
		"111|444": {Players: map[string]int{"111": 9, "444": 6}},
		"a+b|c+d": {Players: nil}, // old 2v2 entry: must be skipped, not migrated
	}}

	out, err := migrateLegacy(fixture)

	if err != nil {
		t.Fatalf("Erro inesperado: %v", err)
	}

	got := out.allMatchups()
	want := map[string]map[string]int{
		"111|222": {"111": 58, "222": 52},
		"111|333": {"111": 7, "333": 0},
		"222|333": {"222": 1, "333": 4},
		"444|555": {"444": 3, "555": 3},
		"111|444": {"111": 9, "444": 6},
	}

	for key, players := range want {
		for id, wantWins := range players {
			if gotWins := got[key].Players[id]; wantWins != gotWins {
				t.Errorf("%s/%s = %d wins, want %d", key, id, gotWins, wantWins)
			}
		}
	}

	if _, stillThere := got["a+b|c+d"]; stillThere {
		t.Errorf("the old 2v2 entry should have been skipped, not migrated")
	}

	wantTotal := 58 + 52 + 7 + 0 + 1 + 4 + 3 + 3 + 9 + 6
	if len(out.Games) != wantTotal {
		t.Errorf("total games = %d, want %d", len(out.Games), wantTotal)
	}
}

func TestMigrateLegacyNoLegacyData(t *testing.T) {
	out, err := migrateLegacy(scores{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Games) != 0 {
		t.Errorf("got %d games from an empty store, want 0", len(out.Games))
	}
}

func TestMigrateLegacyKeepsExistingGames(t *testing.T) {
	s := scores{
		Matchups: map[string]matchupScore{"111|222": {Players: map[string]int{"111": 58, "222": 52}}},
		Games:    []game{{Winner: "111", Loser: "222"}}, // one game already recorded post-migration
	}

	out, err := migrateLegacy(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := out.allMatchups()["111|222"].Players["111"]; got != 59 {
		t.Errorf("111 wins = %d, want 59 (58 migrated + 1 pre-existing)", got)
	}
	if last := out.Games[len(out.Games)-1]; last != s.Games[0] {
		t.Errorf("the pre-existing game should stay last, got %+v", last)
	}
}
