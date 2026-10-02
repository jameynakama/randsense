-- name: InsertDeterminer :exec
INSERT INTO determiners (lemma, type, number)
VALUES ($1, $2, $3);

-- name: TruncateDeterminers :exec
TRUNCATE determiners RESTART IDENTITY CASCADE;

-- name: ListDeterminers :many
SELECT * FROM determiners
ORDER BY lemma;
