package api

import (
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jameynakama/randsense/internal/store"
)

func numericToFloat(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		log.Printf("error: numericToFloat: %v", err)
		return nil
	}
	v := f.Float64
	return &v
}

type WordResponse struct {
	ID          int64           `json:"id"`
	Lemma       string          `json:"lemma"`
	Inflections json.RawMessage `json:"inflections"`
	Frames      json.RawMessage `json:"frames,omitempty"`
	Source      string          `json:"source"`
	Frequency   *float64        `json:"frequency"`
	Active      bool            `json:"active"`
	VoteCount   int             `json:"vote_count"`
}

func getNounRespFromStoreShape(n store.Noun) WordResponse {
	return WordResponse{
		ID:          n.ID,
		Lemma:       n.Lemma,
		Inflections: n.Inflections,
		Source:      n.Source,
		Frequency:   numericToFloat(n.Frequency),
		Active:      n.Active,
		VoteCount:   int(n.VoteCount),
	}
}

func getVerbRespFromStoreShape(v store.Verb) WordResponse {
	return WordResponse{
		ID:          v.ID,
		Lemma:       v.Lemma,
		Inflections: v.Inflections,
		Frames:      v.Frames,
		Source:      v.Source,
		Frequency:   numericToFloat(v.Frequency),
		Active:      v.Active,
		VoteCount:   int(v.VoteCount),
	}
}

func getAdjectiveRespFromStoreShape(a store.Adjective) WordResponse {
	return WordResponse{
		ID:          a.ID,
		Lemma:       a.Lemma,
		Inflections: a.Inflections,
		Source:      a.Source,
		Frequency:   numericToFloat(a.Frequency),
		Active:      a.Active,
		VoteCount:   int(a.VoteCount),
	}
}

func getAdverbRespFromStoreShape(a store.Adverb) WordResponse {
	return WordResponse{
		ID:          a.ID,
		Lemma:       a.Lemma,
		Inflections: a.Inflections,
		Source:      a.Source,
		Frequency:   numericToFloat(a.Frequency),
		Active:      a.Active,
		VoteCount:   int(a.VoteCount),
	}
}

type SentenceResponse struct {
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Tree      json.RawMessage `json:"tree"`
	StarCount int32           `json:"star_count"`
	Origin    string          `json:"origin"`
	CreatedAt time.Time       `json:"created_at"`
}

func sentenceResponse(s store.Sentence) SentenceResponse {
	return SentenceResponse{ID: s.ID, Text: s.Text, Tree: s.Tree, StarCount: s.StarCount, Origin: s.Origin, CreatedAt: s.CreatedAt.Time}
}
