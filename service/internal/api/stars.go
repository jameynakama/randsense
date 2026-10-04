package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

// voterPattern matches the random UUID a browser keeps to star with.
var voterPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// voter is the X-Voter header, lowercased so its case doesn't matter.
func voter(r *http.Request) (string, bool) {
	v := r.Header.Get("X-Voter")
	return strings.ToLower(v), voterPattern.MatchString(v)
}

const badVoter = "X-Voter header must be a UUID"

func (h *Handler) addStar(w http.ResponseWriter, r *http.Request) {
	h.changeStar(w, r, func(ctx context.Context, id, voter string) (int32, error) {
		return h.queries.AddStar(ctx, store.AddStarParams{SentenceID: id, Voter: voter})
	})
}

func (h *Handler) removeStar(w http.ResponseWriter, r *http.Request) {
	h.changeStar(w, r, func(ctx context.Context, id, voter string) (int32, error) {
		return h.queries.RemoveStar(ctx, store.RemoveStarParams{SentenceID: id, Voter: voter})
	})
}

// changeStar stars or unstars a sentence with change and answers with its
// new count. Both are idempotent.
func (h *Handler) changeStar(w http.ResponseWriter, r *http.Request, change func(ctx context.Context, id, voter string) (int32, error)) {
	v, ok := voter(r)
	if !ok {
		writeError(w, http.StatusBadRequest, badVoter)
		return
	}
	count, err := change(r.Context(), chi.URLParam(r, "id"), v)
	var pgErr *pgconn.PgError
	if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("changeStar: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int32{"count": count})
}

func (h *Handler) listStars(w http.ResponseWriter, r *http.Request) {
	v, ok := voter(r)
	if !ok {
		writeError(w, http.StatusBadRequest, badVoter)
		return
	}
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListStarredSentences(r.Context(), store.ListStarredSentencesParams{Voter: v, PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listStars: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponses(rows))
}
