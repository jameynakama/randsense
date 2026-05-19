package api

import (
	"log"
	"net/http"
	"slices"
)

func (h *Handler) randomWord(w http.ResponseWriter, r *http.Request) {
	posChoices := []string{"noun", "verb", "adjective", "adverb"}
	pos := r.URL.Query().Get("pos")
	if pos == "" || !slices.Contains(posChoices, pos) {
		writeError(w, http.StatusBadRequest, "Please provide a pos query param from [noun, verb, adjective, adverb]")
		return
	}
	switch pos {
	case "noun":
		n, err := h.queries.GetRandomNoun(r.Context())
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getNounRespFromStoreShape(n))
	case "verb":
		v, err := h.queries.GetRandomVerb(r.Context())
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getVerbRespFromStoreShape(v))
	case "adjective":
		a, err := h.queries.GetRandomAdjective(r.Context())
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getAdjectiveRespFromStoreShape(a))
	case "adverb":
		a, err := h.queries.GetRandomAdverb(r.Context())
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getAdverbRespFromStoreShape(a))
	}
}
