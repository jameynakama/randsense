package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/store"
)

const (
	minCommentLen = 10
	maxCommentLen = 1000
	maxFlagBytes  = 16 << 10
)

// flagSentence records a complaint about a sentence, or one word in it.
// word_index counts the tree's leaves in order, commas included, but a
// comma can't be flagged.
func (h *Handler) flagSentence(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Comment   string `json:"comment"`
		WordIndex *int   `json:"word_index"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFlagBytes)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "body must be JSON with a comment and an optional whole-number word_index")
		return
	}
	comment := strings.TrimSpace(body.Comment)
	if n := utf8.RuneCountInString(comment); n < minCommentLen || n > maxCommentLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("comment must be %d to %d characters", minCommentLen, maxCommentLen))
		return
	}

	s, err := h.queries.GetSentence(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("flagSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	params := store.InsertFlagParams{SentenceID: s.ID, Comment: comment}
	if body.WordIndex != nil {
		var tree grammar.Node
		if err := json.Unmarshal(s.Tree, &tree); err != nil {
			log.Printf("flagSentence: tree of %s: %v", s.ID, err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		leaves := tree.LeafNodes()
		i := *body.WordIndex
		if i < 0 || i >= len(leaves) || leaves[i].POS() == grammar.Comma {
			writeError(w, http.StatusBadRequest, "word_index must point at a word in the sentence")
			return
		}
		params.WordIndex = pgtype.Int4{Int32: int32(i), Valid: true}
		params.Lemma = pgtype.Text{String: leaves[i].Lemma, Valid: true}
		params.Pos = pgtype.Text{String: string(leaves[i].POS()), Valid: true}
	}

	id, err := h.queries.InsertFlag(r.Context(), params)
	if err != nil {
		log.Printf("flagSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}
