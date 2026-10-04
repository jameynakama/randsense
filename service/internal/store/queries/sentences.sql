-- name: InsertSentence :one
INSERT INTO sentences (id, text, tree, commonness)
VALUES (@id, @text, @tree, @commonness::float8)
RETURNING *;

-- name: GetSentence :one
SELECT * FROM sentences
WHERE id = $1;
