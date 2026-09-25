package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type game struct {
	Winner string    `json:"winner"`
	Loser  string    `json:"loser"`
	At     time.Time `json:"at,omitzero"`
}

type scores struct {
	Matchups map[string]matchupScore `json:"matchups"`
	Games    []game                  `json:"games"`
}

func (s scores) matchupFor(a, b string) (matchupScore, bool) {
	m := matchupScore{Players: map[string]int{a: 0, b: 0}}
	played := false
	want := matchupKey(a, b)
	for _, g := range s.Games {
		if matchupKey(g.Winner, g.Loser) == want {
			m.Players[g.Winner]++
			played = true
		}
	}

	return m, played
}

func (s scores) allMatchups() map[string]matchupScore {
	result := make(map[string]matchupScore)

	for _, g := range s.Games {
		key := matchupKey(g.Winner, g.Loser)

		m, exists := result[key]
		if !exists {
			m = matchupScore{Players: map[string]int{g.Winner: 0, g.Loser: 0}}
		}

		m.Players[g.Winner]++
		result[key] = m
	}

	return result
}

type matchupScore struct {
	Players map[string]int `json:"players"`
}

var errNoMatchupPlayers = errors.New("no matchup for players")
var errNoMatchup = errors.New("no matchup")

func matchupKey(player1ID, player2ID string) string {
	players := []string{player1ID, player2ID}
	sort.Strings(players)
	return strings.Join(players, "|")
}

type Store struct {
	mu   sync.Mutex
	path string
}

func (st *Store) load() (scores, error) {
	currentScores := scores{
		Matchups: make(map[string]matchupScore),
	}

	data, err := os.ReadFile(st.path)
	if err != nil {
		if os.IsNotExist(err) {
			return currentScores, nil
		}
		return scores{}, err
	}

	if len(data) == 0 {
		return currentScores, nil
	}

	if err := json.Unmarshal(data, &currentScores); err != nil {
		return scores{}, err
	}

	if currentScores.Matchups == nil {
		currentScores.Matchups = make(map[string]matchupScore)
	}

	return currentScores, nil
}

func (st *Store) save(currentScores scores) error {
	data, err := json.MarshalIndent(currentScores, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(st.path, data)
}

// writeFileAtomic writes data to path.tmp, flushes it to disk and renames it
// over path, so path always holds either the old content or the new one,
// never a partial write. Both Store and WaterStore save through it.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

func (st *Store) RecordWin(winnerID, loserID string) (matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return matchupScore{}, err
	}

	current.Games = append(current.Games, game{
		Winner: winnerID,
		Loser:  loserID,
		At:     time.Now().UTC(),
	})

	if err := st.save(current); err != nil {
		return matchupScore{}, err
	}

	matchup, _ := current.matchupFor(winnerID, loserID)
	return matchup, nil
}

func (st *Store) UndoLast(aID, bID string) (game, matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return game{}, matchupScore{}, err
	}

	want := matchupKey(aID, bID)
	for i := len(current.Games) - 1; i >= 0; i-- {
		g := current.Games[i]
		if matchupKey(g.Winner, g.Loser) != want {
			continue
		}

		current.Games = slices.Delete(current.Games, i, i+1)
		if err := st.save(current); err != nil {
			return game{}, matchupScore{}, err
		}

		matchup, _ := current.matchupFor(aID, bID)
		return g, matchup, nil
	}

	return game{}, matchupScore{}, errNoMatchupPlayers
}

func (st *Store) Matchup(a, b string) (matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return matchupScore{}, err
	}

	m, played := current.matchupFor(a, b)
	if !played {
		return matchupScore{}, errNoMatchupPlayers
	}

	return m, nil
}

func (st *Store) AllMatchups() (map[string]matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	result := make(map[string]matchupScore)

	current, err := st.load()
	if err != nil {
		return result, err
	}

	all := current.allMatchups()
	if len(all) == 0 {
		return result, errNoMatchup
	}

	return all, nil
}

func (st *Store) Migrate() (matchupsFound, gamesAdded int, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return 0, 0, err
	}

	if len(current.Matchups) == 0 {
		return 0, 0, nil
	}

	if err := st.backupLegacy(); err != nil {
		return 0, 0, err
	}

	migrated, err := migrateLegacy(current)
	if err != nil {
		return 0, 0, err
	}

	if err := st.save(migrated); err != nil {
		return 0, 0, err
	}

	return len(current.Matchups), len(migrated.Games) - len(current.Games), nil
}

func (st *Store) backupLegacy() error {
	backupPath := st.path + ".legacy.bak"

	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(st.path)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, data, 0644)
}

func migrateLegacy(s scores) (scores, error) {
	if len(s.Matchups) == 0 {
		return s, nil
	}

	keys := make([]string, 0, len(s.Matchups))
	for key := range s.Matchups {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var migrated []game
	for _, key := range keys {
		ids := strings.Split(key, "|")
		players := s.Matchups[key].Players
		if len(ids) != 2 || ids[0] == ids[1] || len(players) == 0 {
			continue
		}

		for i, winner := range ids {
			loser := ids[1-i]
			for n := players[winner]; n > 0; n-- {
				migrated = append(migrated, game{Winner: winner, Loser: loser})
			}
		}
	}

	out := scores{Games: append(migrated, s.Games...)}

	oldScore := scores{Games: s.Games}.allMatchups()
	newScore := out.allMatchups()

	for _, key := range keys {
		ids := strings.Split(key, "|")
		players := s.Matchups[key].Players

		if len(ids) != 2 || ids[0] == ids[1] || len(players) == 0 {
			continue
		}

		for id, wantWins := range players {
			got := newScore[key].Players[id] - oldScore[key].Players[id]
			if got != wantWins {
				return scores{}, fmt.Errorf("Erro de migração da key %v usuario %v, legacy era: %v, novo ficou %v", key, id, wantWins, got)
			}
		}

	}

	return out, nil
}

func NewStore(path string) *Store {
	return &Store{path: path}
}
