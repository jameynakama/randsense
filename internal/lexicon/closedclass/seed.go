// Package closedclass seeds the hand-curated closed-class word tables
// (determiners, prepositions, pronouns, conjunctions) from a TOML file.
// WordNet only covers the open classes, so these lists live in the repo.
package closedclass

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jameynakama/randsense/internal/store"
)

// Stats reports per-table row counts from a successful Seed run.
type Stats struct {
	Determiners  int
	Prepositions int
	Pronouns     int
	Conjunctions int
}

type determiner struct {
	Lemma  string `toml:"lemma"`
	Type   string `toml:"type"`
	Number string `toml:"number"`
}

type preposition struct {
	Lemma string `toml:"lemma"`
}

type pronoun struct {
	Lemma  string `toml:"lemma"`
	Case   string `toml:"case"`
	Person int16  `toml:"person"`
	Number string `toml:"number"`
	Gender string `toml:"gender"`
}

type conjunction struct {
	Lemma    string `toml:"lemma"`
	Type     string `toml:"type"`
	JoinsNPs bool   `toml:"joins_nps"`
}

type file struct {
	Determiners  []determiner  `toml:"determiner"`
	Prepositions []preposition `toml:"preposition"`
	Pronouns     []pronoun     `toml:"pronoun"`
	Conjunctions []conjunction `toml:"conjunction"`
}

var (
	determinerTypes   = []string{"indefinite", "definite", "demonstrative", "possessive", "quantifier"}
	determinerNumbers = []string{"singular", "plural", "either"}
	pronounCases      = []string{"nominative", "accusative", "genitive", "reflexive"}
	pronounPersons    = []int16{1, 2, 3}
	pronounNumbers    = []string{"singular", "plural"}
	pronounGenders    = []string{"masc", "fem", "neuter", "epicene"}
	conjunctionTypes  = []string{"coordinating", "subordinating"}
)

// Seed validates r as closed-class TOML, then replaces the contents of all
// four tables in one transaction. Invalid input writes nothing.
func Seed(ctx context.Context, pool *pgxpool.Pool, r io.Reader) (Stats, error) {
	var f file
	if _, err := toml.NewDecoder(r).Decode(&f); err != nil {
		return Stats{}, fmt.Errorf("Seed, decode toml: %w", err)
	}
	if err := f.validate(); err != nil {
		return Stats{}, fmt.Errorf("Seed: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("Seed, pool.Begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := store.New(tx)

	for _, tr := range []struct {
		kind string
		op   func(context.Context) error
	}{
		{"Determiners", q.TruncateDeterminers},
		{"Prepositions", q.TruncatePrepositions},
		{"Pronouns", q.TruncatePronouns},
		{"Conjunctions", q.TruncateConjunctions},
	} {
		if err := tr.op(ctx); err != nil {
			return Stats{}, fmt.Errorf("Seed, Truncate%s: %w", tr.kind, err)
		}
	}

	for _, d := range f.Determiners {
		err := q.InsertDeterminer(ctx, store.InsertDeterminerParams{Lemma: d.Lemma, Type: d.Type, Number: d.Number})
		if err != nil {
			return Stats{}, fmt.Errorf("Seed, determiner %q: %w", d.Lemma, err)
		}
	}
	for _, p := range f.Prepositions {
		if err := q.InsertPreposition(ctx, p.Lemma); err != nil {
			return Stats{}, fmt.Errorf("Seed, preposition %q: %w", p.Lemma, err)
		}
	}
	for _, p := range f.Pronouns {
		err := q.InsertPronoun(ctx, store.InsertPronounParams{
			Lemma: p.Lemma, Case: p.Case, Person: p.Person, Number: p.Number, Gender: p.Gender,
		})
		if err != nil {
			return Stats{}, fmt.Errorf("Seed, pronoun %q: %w", p.Lemma, err)
		}
	}
	for _, c := range f.Conjunctions {
		err := q.InsertConjunction(ctx, store.InsertConjunctionParams{Lemma: c.Lemma, Type: c.Type, JoinsNps: c.JoinsNPs})
		if err != nil {
			return Stats{}, fmt.Errorf("Seed, conjunction %q: %w", c.Lemma, err)
		}
	}

	stats := Stats{
		Determiners:  len(f.Determiners),
		Prepositions: len(f.Prepositions),
		Pronouns:     len(f.Pronouns),
		Conjunctions: len(f.Conjunctions),
	}
	return stats, tx.Commit(ctx)
}

func (f file) validate() error {
	var lemmas []string
	for _, d := range f.Determiners {
		lemmas = append(lemmas, d.Lemma)
		if !slices.Contains(determinerTypes, d.Type) || !slices.Contains(determinerNumbers, d.Number) {
			return fmt.Errorf("determiner %q: type must be one of %v and number one of %v",
				d.Lemma, determinerTypes, determinerNumbers)
		}
	}
	for _, p := range f.Prepositions {
		lemmas = append(lemmas, p.Lemma)
	}
	for _, p := range f.Pronouns {
		lemmas = append(lemmas, p.Lemma)
		if !slices.Contains(pronounCases, p.Case) || !slices.Contains(pronounPersons, p.Person) ||
			!slices.Contains(pronounNumbers, p.Number) || !slices.Contains(pronounGenders, p.Gender) {
			return fmt.Errorf("pronoun %q: case must be one of %v, person one of %v, number one of %v, gender one of %v",
				p.Lemma, pronounCases, pronounPersons, pronounNumbers, pronounGenders)
		}
	}
	for _, c := range f.Conjunctions {
		lemmas = append(lemmas, c.Lemma)
		if !slices.Contains(conjunctionTypes, c.Type) {
			return fmt.Errorf("conjunction %q: type must be one of %v", c.Lemma, conjunctionTypes)
		}
	}
	if slices.Contains(lemmas, "") {
		return fmt.Errorf("entry with an empty lemma")
	}
	return nil
}
