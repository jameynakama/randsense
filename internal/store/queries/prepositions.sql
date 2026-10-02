-- name: InsertPreposition :exec
INSERT INTO prepositions (lemma)
VALUES ($1);

-- name: TruncatePrepositions :exec
TRUNCATE prepositions RESTART IDENTITY CASCADE;

-- name: ListPrepositions :many
SELECT * FROM prepositions
ORDER BY lemma;
