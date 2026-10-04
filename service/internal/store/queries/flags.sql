-- name: InsertFlag :one
INSERT INTO flags (sentence_id, word_index, lemma, pos, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: ListFlags :many
SELECT flags.id, flags.sentence_id, flags.word_index, flags.lemma, flags.pos, flags.comment, flags.created_at,
       sentences.text AS sentence_text, sentences.tree AS sentence_tree
FROM flags
JOIN sentences ON sentences.id = flags.sentence_id
ORDER BY flags.created_at DESC, flags.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: ListFlaggedWords :many
SELECT lemma, pos, count(*) AS count
FROM flags
WHERE lemma IS NOT NULL
GROUP BY lemma, pos
ORDER BY count DESC, lemma, pos
LIMIT @page_limit OFFSET @page_offset;
