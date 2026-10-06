package api

import (
	"cmp"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/live"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// defaultKeepalive is how often an idle stream sends a comment, so proxies
// don't close it.
const defaultKeepalive = 25 * time.Second

// AdminConfig is how the single admin logs in.
type AdminConfig struct {
	PasswordHash  []byte // bcrypt
	SessionSecret []byte // HMAC key for session cookies
	// InsecureCookies lets the session cookie work over plain http, for
	// development.
	InsecureCookies bool
}

type RouterConfig struct {
	Queries *store.Queries
	Grammar *grammar.Grammar
	Verbs   *morph.Verbs
	Admin   AdminConfig
	// Keepalive overrides defaultKeepalive.
	Keepalive time.Duration
	// BuildSecret signs realized trees, so Keep saves only what realize made.
	BuildSecret []byte
}

type Handler struct {
	queries     *store.Queries
	grammar     *grammar.Grammar
	verbs       *morph.Verbs
	admin       AdminConfig
	hub         *live.Hub
	keepalive   time.Duration
	buildSecret []byte
}

func NewRouter(cfg RouterConfig) http.Handler {
	hub := live.NewHub()
	go hub.Run()
	h := &Handler{
		queries:     cfg.Queries,
		grammar:     cfg.Grammar,
		verbs:       cfg.Verbs,
		admin:       cfg.Admin,
		hub:         hub,
		buildSecret: cfg.BuildSecret,
		keepalive:   cmp.Or(cfg.Keepalive, defaultKeepalive),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", h.healthCheck)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/words/random", h.randomWord)
		r.Get("/words/{pos}/{lemma}", h.getDefinitions)
		r.Get("/grammar", h.getGrammar)
		r.Get("/sentences", h.listSentences)
		r.Post("/sentences", h.keepSentence)
		r.Get("/sentences/random", h.randomSentence)
		r.Get("/sentences/stream", h.stream)
		r.Post("/sentences/realize", h.realizeSentence)
		r.Get("/sentences/{id}", h.getSentence)
		r.Post("/sentences/{id}/stars", h.addStar)
		r.Delete("/sentences/{id}/stars", h.removeStar)
		r.Post("/sentences/{id}/flags", h.flagSentence)
		r.Get("/stars", h.listStars)
		r.Route("/admin", func(r chi.Router) {
			r.Post("/login", h.login)
			r.Group(func(r chi.Router) {
				r.Use(h.requireAdmin)
				r.Post("/logout", h.logout)
				r.Get("/flags", h.listFlags)
				r.Get("/flagged-words", h.listFlaggedWords)
			})
		})
	})

	return r
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
