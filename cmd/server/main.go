package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/store"
)

const grammarPath = "data/grammar/grammar.toml"

type config struct {
	databaseURL string
	port        string
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
		databaseURL: required("DATABASE_URL"),
		port:        withDefault("PORT", "8080"),
	}
}

func main() {
	cfg := loadConfig()

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
	if err != nil {
		log.Fatalf("load %s: %v", grammarPath, err)
	}

	routerCfg := api.RouterConfig{Queries: store.New(db), Grammar: g}
	r := api.NewRouter(routerCfg)

	log.Printf("starting server at http://localhost:%s", cfg.port)
	if err := http.ListenAndServe(":"+cfg.port, r); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
