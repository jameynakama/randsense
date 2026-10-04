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

	"github.com/jackc/pgx/v5"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/sentence"
	"github.com/jameynakama/randsense/internal/store"
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
	saved, err := insertWithNewID(newSentenceID, func(id string) (store.Sentence, error) {
		return h.queries.InsertSentence(r.Context(), store.InsertSentenceParams{ID: id, Text: s.Text, Tree: tree, Commonness: c})
	})
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
// with words. Any words and features already in the tree are replaced.
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
	if err := tree.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clearWords(&tree)

	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	s, err := sentence.Realize(r.Context(), h.queries, &tree, h.verbs, rng, c)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusUnprocessableEntity, "no word in the lexicon fits a slot in this tree at this commonness")
		return
	}
	if err != nil {
		log.Printf("realizeSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func clearWords(n *grammar.Node) {
	n.Lemma, n.Word, n.Display, n.Features = "", "", "", grammar.Features{}
	for _, c := range n.Children {
		clearWords(c)
	}
}
