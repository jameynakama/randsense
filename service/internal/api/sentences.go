package api

import (
	"errors"
	"math/rand/v2"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

const (
	idAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	idLength   = 8
	// maxIDAttempts bounds retries on an ID collision, which takes a vast
	// table to happen even once.
	maxIDAttempts = 5
)

// newSentenceID is a random base-62 ID.
func newSentenceID() string {
	b := make([]byte, idLength)
	for i := range b {
		b[i] = idAlphabet[rand.IntN(len(idAlphabet))]
	}
	return string(b)
}

// insertWithNewID calls insert with IDs from newID until one isn't taken.
func insertWithNewID(newID func() string, insert func(id string) (store.Sentence, error)) (store.Sentence, error) {
	var s store.Sentence
	var err error
	for range maxIDAttempts {
		s, err = insert(newID())
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
			return s, err
		}
	}
	return s, err
}
