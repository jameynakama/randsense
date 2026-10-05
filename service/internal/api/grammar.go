package api

import "net/http"

// grammarCacheSeconds is how long clients may reuse the grammar. It only
// changes on deploy.
const grammarCacheSeconds = "300"

// getGrammar serves the grammar's rules and labels, so the frontend never
// hardcodes either.
func (h *Handler) getGrammar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age="+grammarCacheSeconds)
	writeJSON(w, http.StatusOK, h.grammar.Describe())
}
