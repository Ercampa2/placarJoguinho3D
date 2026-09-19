package main

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
)

type scores struct {
	Matchups map[string]matchupScore `json:"matchups"`
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

	key := matchupKey(winnerID, loserID)
	matchup := current.Matchups[key]
	if matchup.Players == nil {
		matchup.Players = map[string]int{winnerID: 0, loserID: 0}
	}

	matchup.Players[winnerID]++
	current.Matchups[key] = matchup

	if err := st.save(current); err != nil {
		return matchupScore{}, err
	}

	return matchup, nil
}

func (st *Store) UndoWin(winnerID, loserID string) (matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return matchupScore{}, err
	}

	key := matchupKey(winnerID, loserID)
	matchup := current.Matchups[key]

	if matchup.Players[winnerID] < 1 {
		return matchupScore{}, errNoWins
	}

	matchup.Players[winnerID]--
	current.Matchups[key] = matchup

	if err := st.save(current); err != nil {
		return matchupScore{}, err
	}

	return matchup, nil
}

func (st *Store) Matchup(a, b string) (matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	current, err := st.load()
	if err != nil {
		return matchupScore{}, err
	}

	key := matchupKey(a, b)

	matchup := current.Matchups[key]
	if matchup.Players == nil {
		return matchupScore{}, errNoMatchupPlayers
	}

	return matchup, nil
}

func (st *Store) AllMatchups() (map[string]matchupScore, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	result := make(map[string]matchupScore)

	current, err := st.load()
	if err != nil {
		return result, err
	}

	if len(current.Matchups) == 0 {
		return result, errNoMatchup
	}

	return current.Matchups, nil
}

func NewStore(path string) *Store {
	return &Store{path: path}
}
