-- name: InsertAdjective :exec
INSERT INTO adjectives (lemma, inflections, source)
VALUES ($1, $2, $3)
ON CONFLICT (lemma, source) DO NOTHING;

-- name: TruncateAdjectives :exec
TRUNCATE adjectives RESTART IDENTITY CASCADE;

-- name: CountAdjectives :one
SELECT COUNT(*) FROM adjectives;

-- name: GetAdjectiveByLemma :one
SELECT * FROM adjectives
WHERE lemma = $1;

-- name: GetRandomAdjective :one
SELECT * FROM adjectives
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: SetAdjectiveFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE adjectives SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE adjectives.lemma = f.word;
