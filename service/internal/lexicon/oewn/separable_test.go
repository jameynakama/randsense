package oewn_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
	"github.com/jameynakama/randsense/internal/store"
)

func TestMarkSeparable(t *testing.T) {
	truncateLexicon(t)
	ctx := context.Background()
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	if _, err := oewn.Ingest(ctx, testPool, f, nil); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	n, err := oewn.MarkSeparable(ctx, testPool, strings.NewReader(`lemmas = ["loiter", "not a verb here"]`))
	if err != nil {
		t.Fatalf("MarkSeparable: %v", err)
	}
	if n != 1 {
		t.Errorf("flagged: got %d, want 1", n)
	}

	q := store.New(testPool)
	for lemma, want := range map[string]bool{"loiter": true, "devour": false} {
		v, err := q.GetVerbByLemma(ctx, lemma)
		if err != nil {
			t.Fatalf("GetVerbByLemma(%s): %v", lemma, err)
		}
		if v.Separable != want {
			t.Errorf("%s separable: got %v, want %v", lemma, v.Separable, want)
		}
	}
}

func TestMarkSeparableRejectsBadTOML(t *testing.T) {
	if _, err := oewn.MarkSeparable(context.Background(), testPool, strings.NewReader("lemmas = [")); err == nil {
		t.Error("expected an error")
	}
}
