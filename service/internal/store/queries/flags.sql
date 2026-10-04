-- name: InsertFlag :one
INSERT INTO flags (sentence_id, word_index, lemma, pos, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
