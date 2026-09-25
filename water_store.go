package main

import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"time"
)

// sip is one "I drank water" counted for a round.
type sip struct {
	Player string    `json:"player"`
	Track  string    `json:"track"`
	Day    string    `json:"day"`   // local date of the round, "2006-01-02"
	Round  time.Time `json:"round"` // start of the round, which identifies the reminder
	At     time.Time `json:"at"`    // when the request arrived
}

type waterData struct {
	Tokens map[string]string `json:"tokens"` // player ID -> token
	Sips   []sip             `json:"sips"`
}

type rankEntry struct {
	Player string
	Sips   int
}

var errAlreadySipped = errors.New("already sipped in this round")
var errUnknownToken = errors.New("unknown token")

// WaterStore keeps sips and tokens in a JSON file, the same way Store keeps
// games: every call reads the file, and every change rewrites it atomically.
type WaterStore struct {
	mu   sync.Mutex
	path string
}

func NewWaterStore(path string) *WaterStore {
	return &WaterStore{path: path}
}

func (ws *WaterStore) load() (waterData, error) {
	current := waterData{Tokens: make(map[string]string)}

	data, err := os.ReadFile(ws.path)
	if err != nil {
		if os.IsNotExist(err) {
			return current, nil
		}
		return waterData{}, err
	}

	if len(data) == 0 {
		return current, nil
	}

	if err := json.Unmarshal(data, &current); err != nil {
		return waterData{}, err
	}

	if current.Tokens == nil {
		current.Tokens = make(map[string]string)
	}

	return current, nil
}

func (ws *WaterStore) save(current waterData) error {
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(ws.path, data)
}

// RecordSip saves s, unless its player already has a sip in the same round of
// the same track (errAlreadySipped). It returns how many sips that player has
// on that track and day, counting this one.
func (ws *WaterStore) RecordSip(s sip) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	current, err := ws.load()
	if err != nil {
		return 0, err
	}

	today := 1
	for _, old := range current.Sips {
		if old.Player != s.Player || old.Track != s.Track {
			continue
		}

		// Equal, not ==: == also compares the Location, and a time read back
		// from JSON does not carry the same one as a time just computed.
		if old.Round.Equal(s.Round) {
			return 0, errAlreadySipped
		}

		if old.Day == s.Day {
			today++
		}
	}

	current.Sips = append(current.Sips, s)
	if err := ws.save(current); err != nil {
		return 0, err
	}

	return today, nil
}

// NewToken gives player a new random token. It replaces the one they had,
// which stops working.
func (ws *WaterStore) NewToken(player string) (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	current, err := ws.load()
	if err != nil {
		return "", err
	}

	token := rand.Text()
	current.Tokens[player] = token
	if err := ws.save(current); err != nil {
		return "", err
	}

	return token, nil
}

// PlayerForToken returns whose token this is, or errUnknownToken.
func (ws *WaterStore) PlayerForToken(token string) (string, error) {
	if token == "" {
		return "", errUnknownToken
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	current, err := ws.load()
	if err != nil {
		return "", err
	}

	for player, t := range current.Tokens {
		if t == token {
			return player, nil
		}
	}

	return "", errUnknownToken
}

// DailyRanking counts each player's sips on track during day ("2006-01-02"),
// most sips first. Players with the same count are ordered by ID, so the
// order does not change between calls.
func (ws *WaterStore) DailyRanking(track, day string) ([]rankEntry, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	current, err := ws.load()
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, s := range current.Sips {
		if s.Track == track && s.Day == day {
			counts[s.Player]++
		}
	}

	ranking := make([]rankEntry, 0, len(counts))
	for player, n := range counts {
		ranking = append(ranking, rankEntry{Player: player, Sips: n})
	}

	slices.SortFunc(ranking, func(a, b rankEntry) int {
		if c := cmp.Compare(b.Sips, a.Sips); c != 0 {
			return c
		}
		return cmp.Compare(a.Player, b.Player)
	})

	return ranking, nil
}
