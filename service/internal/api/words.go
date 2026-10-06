package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// definitionsCacheSeconds is how long clients may reuse a word's
// definitions. They only change on reingest.
const definitionsCacheSeconds = "86400"

type definitionsResponse struct {
	Definitions json.RawMessage `json:"definitions"`
}

// getDefinitions serves a content word's OEWN glosses in sense order.
// Closed-class parts of speech have none, so they're a 404 like an unknown
// lemma.
func (h *Handler) getDefinitions(w http.ResponseWriter, r *http.Request) {
	lookup := map[string]func(context.Context, string) ([]byte, error){
		"noun":      h.queries.GetNounDefinitions,
		"verb":      h.queries.GetVerbDefinitions,
		"adjective": h.queries.GetAdjectiveDefinitions,
		"adverb":    h.queries.GetAdverbDefinitions,
	}[chi.URLParam(r, "pos")]
	if lookup == nil {
		writeError(w, http.StatusNotFound, "no such word")
		return
	}

	// chi hands over the raw path segment when the client escapes a character
	// Go wouldn't ("A%2FC"), so the lemma comes through still escaped. That
	// can't happen while AllowLemma admits only letters, apostrophes, spaces
	// and hyphens; widen it and this needs url.PathUnescape.
	defs, err := lookup(r.Context(), chi.URLParam(r, "lemma"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "no such word")
		return
	}
	if err != nil {
		log.Printf("getDefinitions: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age="+definitionsCacheSeconds)
	writeJSON(w, http.StatusOK, definitionsResponse{Definitions: defs})
}
