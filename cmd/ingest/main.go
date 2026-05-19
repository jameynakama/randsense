package main

import (
	"compress/gzip"
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/randsense/internal/lexicon/oewn"
)

const defaultDataPath = "data/oewn-2025/english-wordnet-2025.xml.gz"

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	path := defaultDataPath
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		log.Fatalf("gzip: %v", err)
	}
	defer gz.Close()

	log.Printf("ingesting %s ...", path)
	stats, err := oewn.Ingest(ctx, db, gz)
	if err != nil {
		log.Fatalf("ingest: %v", err)
	}

	log.Printf("done: nouns=%d verbs=%d adjectives=%d adverbs=%d skipped=%d",
		stats.Nouns, stats.Verbs, stats.Adjectives, stats.Adverbs, stats.Skipped)
}
