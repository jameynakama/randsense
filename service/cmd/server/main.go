package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	grammarPath = "data/grammar/grammar.toml"
	verbsPath   = "data/lexicon/verb_morphology.toml"
	curatedPath = "data/lexicon/verb_morphology_curated.toml"
)

type config struct {
	databaseURL       string
	port              string
	adminPasswordHash string
	sessionSecret     string
	buildSecret       string
	insecureCookies   bool
}

func loadConfig() config {
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			log.Fatalf("must set %s env var", key)
		}
		return v
	}

	withDefault := func(key string, defaultV string) string {
		v := os.Getenv(key)
		if v == "" {
			return defaultV
		}
		return v
	}

	return config{
		databaseURL:       required("DATABASE_URL"),
		port:              withDefault("PORT", "8080"),
		adminPasswordHash: required("ADMIN_PASSWORD_HASH"),
		sessionSecret:     required("SESSION_SECRET"),
		buildSecret:       required("BUILD_SECRET"),
		insecureCookies:   os.Getenv("INSECURE_COOKIES") == "true",
	}
}

func main() {
	cfg := loadConfig()
	if _, err := bcrypt.Cost([]byte(cfg.adminPasswordHash)); err != nil {
		log.Fatalf("ADMIN_PASSWORD_HASH must be a bcrypt hash (single-quote it in .env so its $s survive): %v", err)
	}
	if len(cfg.sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be at least 32 bytes")
	}
	if len(cfg.buildSecret) < 32 {
		log.Fatal("BUILD_SECRET must be at least 32 bytes")
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		log.Fatal("error establishing database connection")
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal("cannot ping database")
	}
	log.Println("database connected")

	f, err := os.Open(grammarPath)
	if err != nil {
		log.Fatalf("open %s: %v", grammarPath, err)
	}
	g, err := grammar.Load(f)
	f.Close()
	if err == nil {
		err = g.Labeled()
	}
	if err != nil {
		log.Fatalf("load %s: %v", grammarPath, err)
	}

	vf, err := os.Open(verbsPath)
	if err != nil {
		log.Fatalf("open %s: %v", verbsPath, err)
	}
	cf, err := os.Open(curatedPath)
	if err != nil {
		log.Fatalf("open %s: %v", curatedPath, err)
	}
	v, err := morph.LoadVerbs(vf, cf)
	vf.Close()
	cf.Close()
	if err != nil {
		log.Fatalf("load verb morphology: %v", err)
	}

	routerCfg := api.RouterConfig{
		Queries: store.New(db), Grammar: g, Verbs: v,
		Admin: api.AdminConfig{
			PasswordHash:    []byte(cfg.adminPasswordHash),
			SessionSecret:   []byte(cfg.sessionSecret),
			InsecureCookies: cfg.insecureCookies,
		},
		BuildSecret: []byte(cfg.buildSecret),
	}
	r := api.NewRouter(routerCfg)

	log.Printf("starting server at http://localhost:%s", cfg.port)
	if err := http.ListenAndServe(":"+cfg.port, r); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
