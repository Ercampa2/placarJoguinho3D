package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestWater returns a feature whose clock reads *now, and a valid token for
// player "111".
func newTestWater(t *testing.T, now *time.Time) (*waterFeature, string) {
	t.Helper()

	wf := &waterFeature{
		store: newTestWaterStore(t),
		sched: workSchedule(saoPaulo(t)),
		tracks: []waterTrack{
			{name: "5min", label: "5 min", interval: 5 * time.Minute},
			{name: "10min", label: "10 min", interval: 10 * time.Minute},
		},
		now: func() time.Time { return *now },
	}

	token, err := wf.store.NewToken("111")
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	return wf, token
}

func sendSip(wf *waterFeature, method, track, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/gole/"+track, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	wf.handler().ServeHTTP(rec, req)
	return rec
}

func checkResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantText string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d (body %q)", rec.Code, wantStatus, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), wantText) {
		t.Errorf("body = %q, want it to contain %q", rec.Body.String(), wantText)
	}
}

func TestSipOncePerRound(t *testing.T) {
	loc := saoPaulo(t)
	now := monday(loc, 10, 7, 0)
	wf, token := newTestWater(t, &now)

	checkResponse(t, sendSip(wf, http.MethodPost, "5min", token), http.StatusCreated, "rodada das 10:05 (5 min). Hoje: 1.")
	checkResponse(t, sendSip(wf, http.MethodPost, "5min", token), http.StatusConflict, "já registrou")

	// The 10-minute track has its own round and score.
	checkResponse(t, sendSip(wf, http.MethodPost, "10min", token), http.StatusCreated, "rodada das 10:00 (10 min). Hoje: 1.")

	// The next reminder opens a new round.
	now = monday(loc, 10, 10, 0)
	checkResponse(t, sendSip(wf, http.MethodPost, "5min", token), http.StatusCreated, "rodada das 10:10 (5 min). Hoje: 2.")
}

func TestSipOutsideRounds(t *testing.T) {
	loc := saoPaulo(t)

	for _, now := range []time.Time{
		monday(loc, 12, 45, 0),                   // lunch
		monday(loc, 16, 0, 0),                    // day over
		time.Date(2026, 9, 26, 10, 0, 0, 0, loc), // Saturday
	} {
		wf, token := newTestWater(t, &now)
		checkResponse(t, sendSip(wf, http.MethodPost, "5min", token), http.StatusConflict, "Nenhuma rodada aberta")
	}
}

func TestSipRejected(t *testing.T) {
	now := monday(saoPaulo(t), 10, 7, 0)
	wf, token := newTestWater(t, &now)

	checkResponse(t, sendSip(wf, http.MethodPost, "5min", ""), http.StatusUnauthorized, "/agua_token")
	checkResponse(t, sendSip(wf, http.MethodPost, "5min", "wrong"), http.StatusUnauthorized, "/agua_token")
	checkResponse(t, sendSip(wf, http.MethodPost, "2min", token), http.StatusNotFound, "5min, 10min")
	checkResponse(t, sendSip(wf, http.MethodGet, "5min", token), http.StatusMethodNotAllowed, "")
}
