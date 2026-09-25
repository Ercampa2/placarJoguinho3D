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

	// The bot's clock decides the round, never the client: a sip can only
	// ever count for the round open right now.
	now := wf.now()
	round, open := wf.sched.roundAt(now, track.interval)
	if !open {
		writeText(w, http.StatusConflict, "Nenhuma rodada aberta agora: fora do horário dos lembretes.")
		return
	}

	roundTime := round.Format("15:04")
	today, err := wf.store.RecordSip(sip{
		Player: player,
		Track:  track.name,
		Day:    wf.sched.day(round),
		Round:  round.UTC(),
		At:     now.UTC(),
	})
	if errors.Is(err, errAlreadySipped) {
		writeText(w, http.StatusConflict, fmt.Sprintf("Você já registrou um gole na rodada das %s (%s).", roundTime, track.label))
		return
	}
	if err != nil {
		log.Printf("water sip: record: %v", err)
		writeText(w, http.StatusInternalServerError, "Erro no bot ao salvar o gole.")
		return
	}

	log.Printf("water sip: %s on %s, round %s", player, track.name, roundTime)
	writeText(w, http.StatusCreated, fmt.Sprintf("Gole registrado na rodada das %s (%s). Hoje: %d.", roundTime, track.label, today))
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}
