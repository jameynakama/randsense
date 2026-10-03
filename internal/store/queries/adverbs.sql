-- name: InsertAdverb :exec
INSERT INTO adverbs (lemma, inflections, source)
VALUES ($1, $2, $3)
ON CONFLICT (lemma, source) DO NOTHING;

-- name: TruncateAdverbs :exec
TRUNCATE adverbs RESTART IDENTITY CASCADE;

-- name: CountAdverbs :one
SELECT COUNT(*) FROM adverbs;

-- name: GetAdverbByLemma :one
SELECT * FROM adverbs
WHERE lemma = $1;

-- name: GetRandomAdverb :one
SELECT * FROM adverbs
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: SetAdverbFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE adverbs SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE adverbs.lemma = f.word;
