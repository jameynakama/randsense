package main

import (
	"compress/gzip"
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jameynakama/randsense/internal/lexicon/closedclass"
	"github.com/jameynakama/randsense/internal/lexicon/oewn"
	"github.com/jameynakama/randsense/internal/lexicon/subtlex"
)

const (
	defaultDataPath = "data/oewn-2025/english-wordnet-2025.xml.gz"
	closedClassPath = "data/lexicon/closed_class.toml"
	separablePath   = "data/lexicon/separable_verbs.toml"
	subtlexPath     = "data/subtlex-us/subtlex-us-pos.tsv.gz"
)

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

	pf, err := os.Open(separablePath)
	if err != nil {
		log.Fatalf("open %s: %v", separablePath, err)
	}
	defer pf.Close()

	log.Printf("marking %s ...", separablePath)
	separable, err := oewn.MarkSeparable(ctx, db, pf)
	if err != nil {
		log.Fatalf("separable: %v", err)
	}
	log.Printf("done: separable verbs=%d", separable)

	sf, err := os.Open(subtlexPath)
	if err != nil {
		log.Fatalf("open %s: %v", subtlexPath, err)
	}
	defer sf.Close()

	sgz, err := gzip.NewReader(sf)
	if err != nil {
		log.Fatalf("gzip: %v", err)
	}
	defer sgz.Close()

	log.Printf("applying %s ...", subtlexPath)
	sstats, err := subtlex.Apply(ctx, db, sgz)
	if err != nil {
		log.Fatalf("subtlex: %v", err)
	}

	log.Printf("done: frequencies for nouns=%d verbs=%d adjectives=%d adverbs=%d",
		sstats.Nouns, sstats.Verbs, sstats.Adjectives, sstats.Adverbs)

	cf, err := os.Open(closedClassPath)
	if err != nil {
		log.Fatalf("open %s: %v", closedClassPath, err)
	}
	defer cf.Close()

	log.Printf("seeding %s ...", closedClassPath)
	cstats, err := closedclass.Seed(ctx, db, cf)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	log.Printf("done: determiners=%d prepositions=%d pronouns=%d conjunctions=%d",
		cstats.Determiners, cstats.Prepositions, cstats.Pronouns, cstats.Conjunctions)
}
