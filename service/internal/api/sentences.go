package api

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

const (
	idAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	idLength   = 8
	// maxIDAttempts bounds retries on an ID collision, which takes a vast
	// table to happen even once.
	maxIDAttempts = 5
)

// newSentenceID is a random base-62 ID.
func newSentenceID() string {
	b := make([]byte, idLength)
	for i := range b {
		b[i] = idAlphabet[rand.IntN(len(idAlphabet))]
	}
	return string(b)
}

// insertWithNewID calls insert with IDs from newID until one isn't taken.
func insertWithNewID(newID func() string, insert func(id string) (store.Sentence, error)) (store.Sentence, error) {
	var s store.Sentence
	var err error
	for range maxIDAttempts {
		s, err = insert(newID())
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
			return s, err
		}
	}
	return s, err
}

const (
	defaultPageLimit = 30
	maxPageLimit     = 100
)

// page parses a list endpoint's limit and offset query params.
func page(r *http.Request) (limit, offset int32, err error) {
	limit = defaultPageLimit
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 || n > maxPageLimit {
			return 0, 0, fmt.Errorf("limit must be a whole number from 1 to %d", maxPageLimit)
		}
		limit = int32(n)
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			return 0, 0, errors.New("offset must be a whole number from 0")
		}
		offset = int32(n)
	}
	return limit, offset, nil
}

// sentenceResponses never returns nil, so an empty list encodes as [].
func sentenceResponses(rows []store.Sentence) []SentenceResponse {
	resp := make([]SentenceResponse, len(rows))
	for i, s := range rows {
		resp[i] = sentenceResponse(s)
	}
	return resp
}

func writeNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "no sentence with that id")
}

func (h *Handler) listSentences(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListSentences(r.Context(), store.ListSentencesParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listSentences: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponses(rows))
}

func (h *Handler) getSentence(w http.ResponseWriter, r *http.Request) {
	s, err := h.queries.GetSentence(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("getSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponse(s))
}
