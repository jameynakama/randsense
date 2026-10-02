-- name: InsertConjunction :exec
INSERT INTO conjunctions (lemma, type)
VALUES ($1, $2);

-- name: TruncateConjunctions :exec
TRUNCATE conjunctions RESTART IDENTITY CASCADE;

-- name: ListConjunctions :many
SELECT * FROM conjunctions
ORDER BY lemma, type;
