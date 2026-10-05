package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/sentence"
)

// maxCommonness is about the Zipf frequency of "the", the most common word.
const maxCommonness = 7

// defaultCommonness is the floor when a request names none: it keeps out
// the rarest words, which make the weakest sentences.
const defaultCommonness = 1

// commonness parses the optional commonness query param: content words must
// be at least this common, as a Zipf frequency.
func commonness(r *http.Request) (float64, error) {
	v := r.URL.Query().Get("commonness")
	if v == "" {
		return defaultCommonness, nil
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

// randomSentence generates a sentence, saves it and returns it. Any site
// may call it: signatures on other sites embed it.
func (h *Handler) randomSentence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	tree, err := json.Marshal(s.Tree)
	if err != nil {
		log.Printf("randomSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	saved, err := h.save(r.Context(), s.Text, tree, c, originGenerated)
	if err != nil {
		log.Printf("randomSentence: save: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	resp := sentenceResponse(saved)
	h.publish("sentence", resp)
	writeJSON(w, http.StatusOK, resp)
}

// maxTreeBytes caps a realize request body, since every leaf is a lookup.
const maxTreeBytes = 64 << 10

// realizeSentence fills a posted tree, in the shape randomSentence returns,
// with words. The tree must derive from the grammar, and may have holes,
// which are expanded first. Any words and features already in it are
// replaced, except the lemmas of locked leaves. When no word fits a slot, the 422 names the slot's leaf.
func (h *Handler) realizeSentence(w http.ResponseWriter, r *http.Request) {
	c, err := commonness(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var tree grammar.Node
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTreeBytes)).Decode(&tree); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("body must be a JSON tree of at most %d bytes: %v", maxTreeBytes, err))
		return
	}
	if err := h.grammar.Check(&tree); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clearWords(&tree)

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	s, err := sentence.Realize(r.Context(), h.queries, h.grammar, &tree, h.verbs, rng, c)
	var leafErr *sentence.LeafError
	if errors.As(err, &leafErr) {
		msg := "no word in the lexicon fits this slot at this commonness"
		if errors.Is(leafErr, sentence.ErrLockMismatch) {
			msg = "the locked word doesn't fit this slot"
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": msg, "leaf": leafErr.Index})
		return
	}
	if err != nil {
		log.Printf("realizeSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	canonical, err := json.Marshal(s.Tree)
	if err != nil {
		log.Printf("realizeSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, realizeResponse{Sentence: s, Signature: signTree(h.buildSecret, canonical, time.Now())})
}

// clearWords empties every word and feature, except a locked leaf's lemma.
func clearWords(n *grammar.Node) {
	if !n.Locked {
		n.Lemma = ""
	}
	n.Word, n.Display, n.Features = "", "", grammar.Features{}
	for _, c := range n.Children {
		clearWords(c)
	}
}
