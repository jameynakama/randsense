-- name: InsertSentence :one
INSERT INTO sentences (id, text, tree, commonness, origin)
VALUES (@id, @text, @tree, @commonness::float8, @origin)
RETURNING *;

-- name: GetSentence :one
SELECT * FROM sentences
WHERE id = $1;

-- name: ListSentences :many
SELECT * FROM sentences
ORDER BY created_at DESC, id DESC
LIMIT @page_limit OFFSET @page_offset;
