-- name: InsertAdverb :exec
-- OEWN entries that share a lemma pool their definitions.
INSERT INTO adverbs (lemma, inflections, definitions, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lemma, source) DO UPDATE SET definitions = adverbs.definitions || EXCLUDED.definitions;

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

-- name: LookupAdverb :one
-- A locked word: the floor doesn't apply.
SELECT * FROM adverbs
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;

-- name: GetAdverbDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM adverbs
WHERE lemma = $1
ORDER BY id
LIMIT 1;
