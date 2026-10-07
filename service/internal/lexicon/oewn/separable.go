package oewn

import (
	"context"
	"fmt"
	"io"

	"github.com/BurntSushi/toml"
	"github.com/jameynakama/randsense/internal/store"
)

// MarkSeparable reads r as data/lexicon/separable_verbs.toml and flags its
// verbs separable, returning how many rows it flagged. It runs after Ingest,
// whose upsert clears the flags.
func MarkSeparable(ctx context.Context, db store.DBTX, r io.Reader) (int64, error) {
	var f struct {
		Lemmas []string `toml:"lemmas"`
	}
	if _, err := toml.NewDecoder(r).Decode(&f); err != nil {
		return 0, fmt.Errorf("MarkSeparable: %w", err)
	}
	n, err := store.New(db).SetSeparableVerbs(ctx, f.Lemmas)
	if err != nil {
		return 0, fmt.Errorf("MarkSeparable: %w", err)
	}
	return n, nil
}
