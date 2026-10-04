package api

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

func TestNewSentenceIDIsEightBase62Characters(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9A-Za-z]{8}$`)
	for range 100 {
		if id := newSentenceID(); !pattern.MatchString(id) {
			t.Fatalf("id: got %q, want 8 base-62 characters", id)
		}
	}
}

// insertTaken fails with a unique violation for IDs starting "taken" and
// records every ID it was given.
func insertTaken(tried *[]string) func(string) (store.Sentence, error) {
	return func(id string) (store.Sentence, error) {
		*tried = append(*tried, id)
		if strings.HasPrefix(id, "taken") {
			return store.Sentence{}, &pgconn.PgError{Code: pgerrcode.UniqueViolation}
		}
		return store.Sentence{ID: id}, nil
	}
}

func TestInsertWithNewIDRetriesTakenIDs(t *testing.T) {
	ids := []string{"taken111", "taken222", "free3333"}
	var tried []string

	s, err := insertWithNewID(func() string { return ids[len(tried)] }, insertTaken(&tried))

	if err != nil || s.ID != "free3333" {
		t.Errorf("got %q, %v; want free3333, nil", s.ID, err)
	}
	if len(tried) != 3 {
		t.Errorf("attempts: got %d, want 3", len(tried))
	}
}

func TestInsertWithNewIDGivesUp(t *testing.T) {
	var tried []string

	_, err := insertWithNewID(func() string { return "takenxxx" }, insertTaken(&tried))

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
		t.Errorf("err: got %v, want a unique violation", err)
	}
	if len(tried) != maxIDAttempts {
		t.Errorf("attempts: got %d, want %d", len(tried), maxIDAttempts)
	}
}

func TestInsertWithNewIDDoesNotRetryOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	attempts := 0

	_, err := insertWithNewID(newSentenceID, func(string) (store.Sentence, error) {
		attempts++
		return store.Sentence{}, boom
	})

	if !errors.Is(err, boom) || attempts != 1 {
		t.Errorf("got %v after %d attempts; want boom after 1", err, attempts)
	}
}
