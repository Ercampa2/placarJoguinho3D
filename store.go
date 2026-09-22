package main

import (
	"encoding/json"
	"errors"
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

var errNoWins = errors.New("no wins to undo")
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

	tmp := st.path + ".tmp"

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

	return os.Rename(tmp, st.path)
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

func NewStore(path string) *Store {
	return &Store{path: path}
}
