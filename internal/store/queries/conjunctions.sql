-- name: InsertConjunction :exec
INSERT INTO conjunctions (lemma, type, joins_nps)
VALUES ($1, $2, $3);

-- name: TruncateConjunctions :exec
TRUNCATE conjunctions RESTART IDENTITY CASCADE;

-- name: ListConjunctions :many
SELECT * FROM conjunctions
ORDER BY lemma, type;

-- name: GetRandomConjunction :one
SELECT * FROM conjunctions
WHERE active
ORDER BY random()
LIMIT 1;

-- name: GetRandomConjunctionOfType :one
SELECT * FROM conjunctions
WHERE active AND type = $1
ORDER BY random()
LIMIT 1;

-- name: GetRandomNPConjunction :one
SELECT * FROM conjunctions
WHERE active AND joins_nps
ORDER BY random()
LIMIT 1;
