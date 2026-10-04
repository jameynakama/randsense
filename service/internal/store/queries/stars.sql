-- name: AddStar :one
-- One statement, so star_count and stars change together. Starring twice
-- inserts nothing and adds 0. An unknown sentence fails the foreign key.
WITH added AS (
    INSERT INTO stars (sentence_id, voter)
    VALUES (@sentence_id, @voter)
    ON CONFLICT DO NOTHING
    RETURNING 1
)
UPDATE sentences SET star_count = star_count + (SELECT count(*) FROM added)
WHERE id = @sentence_id
RETURNING star_count;

-- name: RemoveStar :one
-- An unknown sentence returns no rows.
WITH removed AS (
    DELETE FROM stars
    WHERE sentence_id = @sentence_id AND voter = @voter
    RETURNING 1
)
UPDATE sentences SET star_count = star_count - (SELECT count(*) FROM removed)
WHERE id = @sentence_id
RETURNING star_count;

-- name: ListStarredSentences :many
SELECT sentences.* FROM sentences
JOIN stars ON stars.sentence_id = sentences.id
WHERE stars.voter = @voter
ORDER BY stars.created_at DESC, sentences.id DESC
LIMIT @page_limit OFFSET @page_offset;
