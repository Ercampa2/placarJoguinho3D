package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// handler serves the sip endpoint:
//
//	POST /gole/{track}
//	Authorization: Bearer <token from /agua_token>
//
// Every answer is one line of plain text meant for people, which the scripts
// show as a desktop notification.
func (wf *waterFeature) handler() http.Handler {
	mux := http.NewServeMux()
	// The method in the pattern makes the mux answer 405 to anything but POST.
	mux.HandleFunc("POST /gole/{track}", wf.handleSip)
	return mux
}

// handleSip runs in its own goroutine for each request, at the same time as
// other requests, the reminder loops and the slash commands. The shared state
// is all in WaterStore, behind its mutex.
func (wf *waterFeature) handleSip(w http.ResponseWriter, r *http.Request) {
	track, ok := wf.track(r.PathValue("track"))
	if !ok {
		writeText(w, http.StatusNotFound, fmt.Sprintf("Trilha desconhecida. Use: %s.", wf.trackNames()))
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	player, err := wf.store.PlayerForToken(token)
	if errors.Is(err, errUnknownToken) {
		writeText(w, http.StatusUnauthorized, "Token inválido. Gere o seu com /agua_token no Discord.")
		return
	}
	if err != nil {
		log.Printf("water sip: read tokens: %v", err)
		writeText(w, http.StatusInternalServerError, "Erro no bot ao ler os tokens.")
		return
	}

	// The bot's clock decides whether the sip scores, never the client's.
	now := wf.now()
	if !wf.scoring.isOpen(now) {
		writeText(w, http.StatusConflict, "Fora do horário de pontuação: o gole não conta agora.")
		return
	}

	today, err := wf.store.RecordSip(sip{
		Player: player,
		Track:  track.name,
		Day:    wf.scoring.day(now),
		At:     now.UTC(),
	})
	if err != nil {
		log.Printf("water sip: record: %v", err)
		writeText(w, http.StatusInternalServerError, "Erro no bot ao salvar o gole.")
		return
	}

	log.Printf("water sip: %s on %s", player, track.name)
	writeText(w, http.StatusCreated, fmt.Sprintf("Gole registrado (%s). Hoje: %d.", track.label, today))
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}
