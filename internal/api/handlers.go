package api

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"

	"github.com/jameynakama/randsense/internal/sentence"
)

// maxCommonness is about the Zipf frequency of "the", the most common word.
const maxCommonness = 7

// commonness parses the optional commonness query param: content words must
// be at least this common, as a Zipf frequency. Absent means any word.
func commonness(r *http.Request) (float64, error) {
	v := r.URL.Query().Get("commonness")
	if v == "" {
		return 0, nil
	}
	c, err := strconv.ParseFloat(v, 64)
	if err != nil || !(c >= 0 && c <= maxCommonness) {
		return 0, fmt.Errorf("commonness must be a number from 0 to %d", maxCommonness)
	}
	return c, nil
}

func (h *Handler) randomWord(w http.ResponseWriter, r *http.Request) {
	posChoices := []string{"noun", "verb", "adjective", "adverb"}
	pos := r.URL.Query().Get("pos")
	if pos == "" || !slices.Contains(posChoices, pos) {
		writeError(w, http.StatusBadRequest, "Please provide a pos query param from [noun, verb, adjective, adverb]")
		return
	}
	c, err := commonness(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch pos {
	case "noun":
		n, err := h.queries.GetRandomNoun(r.Context(), c)
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getNounRespFromStoreShape(n))
	case "verb":
		v, err := h.queries.GetRandomVerb(r.Context(), c)
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getVerbRespFromStoreShape(v))
	case "adjective":
		a, err := h.queries.GetRandomAdjective(r.Context(), c)
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getAdjectiveRespFromStoreShape(a))
	case "adverb":
		a, err := h.queries.GetRandomAdverb(r.Context(), c)
		if err != nil {
			log.Printf("randomWord: %v", err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		writeJSON(w, http.StatusOK, getAdverbRespFromStoreShape(a))
	}
}

func (h *Handler) randomSentence(w http.ResponseWriter, r *http.Request) {
	c, err := commonness(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// A *rand.Rand isn't safe for concurrent use, so each request gets its own.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	s, err := sentence.Generate(r.Context(), h.queries, h.grammar, h.verbs, rng, c)
	if err != nil {
		log.Printf("randomSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, s)
}
