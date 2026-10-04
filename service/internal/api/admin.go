package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/jameynakama/randsense/internal/auth"
	"github.com/jameynakama/randsense/internal/store"
)

// loginFailureDelay slows password guessing.
const loginFailureDelay = time.Second

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body)
	if err != nil || bcrypt.CompareHashAndPassword(h.admin.PasswordHash, []byte(body.Password)) != nil {
		time.Sleep(loginFailureDelay)
		writeError(w, http.StatusUnauthorized, "wrong password")
		return
	}
	value := auth.NewSession(h.admin.SessionSecret, time.Now().Add(auth.SessionTTL))
	http.SetCookie(w, h.sessionCookie(value, int(auth.SessionTTL.Seconds())))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !h.admin.InsecureCookies,
		SameSite: http.SameSiteStrictMode,
	}
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.CookieName)
		if err != nil || !auth.ValidSession(h.admin.SessionSecret, c.Value, time.Now()) {
			writeError(w, http.StatusUnauthorized, "admin login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type FlaggedSentence struct {
	ID   string          `json:"id"`
	Text string          `json:"text"`
	Tree json.RawMessage `json:"tree"`
}

type FlagResponse struct {
	ID        int64           `json:"id"`
	WordIndex *int32          `json:"word_index"`
	Lemma     *string         `json:"lemma"`
	POS       *string         `json:"pos"`
	Comment   string          `json:"comment"`
	CreatedAt time.Time       `json:"created_at"`
	Sentence  FlaggedSentence `json:"sentence"`
}

type FlaggedWordResponse struct {
	Lemma string `json:"lemma"`
	POS   string `json:"pos"`
	Count int64  `json:"count"`
}

func nullInt(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func nullText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListFlags(r.Context(), store.ListFlagsParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listFlags: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	resp := make([]FlagResponse, len(rows))
	for i, f := range rows {
		resp[i] = FlagResponse{
			ID: f.ID, WordIndex: nullInt(f.WordIndex), Lemma: nullText(f.Lemma), POS: nullText(f.Pos),
			Comment: f.Comment, CreatedAt: f.CreatedAt.Time,
			Sentence: FlaggedSentence{ID: f.SentenceID, Text: f.SentenceText, Tree: f.SentenceTree},
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listFlaggedWords(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListFlaggedWords(r.Context(), store.ListFlaggedWordsParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listFlaggedWords: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	// The query skips flags without a lemma, so Lemma and Pos are set.
	resp := make([]FlaggedWordResponse, len(rows))
	for i, fw := range rows {
		resp[i] = FlaggedWordResponse{Lemma: fw.Lemma.String, POS: fw.Pos.String, Count: fw.Count}
	}
	writeJSON(w, http.StatusOK, resp)
}
