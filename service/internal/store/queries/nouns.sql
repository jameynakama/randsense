-- name: InsertNoun :exec
-- OEWN entries that share a lemma pool their definitions.
INSERT INTO nouns (lemma, inflections, definitions, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lemma, source) DO UPDATE SET definitions = nouns.definitions || EXCLUDED.definitions;

-- name: TruncateNouns :exec
TRUNCATE nouns RESTART IDENTITY CASCADE;

-- name: CountNouns :one
SELECT COUNT(*) FROM nouns;

-- name: GetNounByLemma :one
SELECT * FROM nouns
WHERE lemma = $1;

-- name: GetRandomNoun :one
SELECT * FROM nouns
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: MarkPluralNouns :execrows
-- A lemma is plural if it ends in -s and its singular (minus -s, or minus
-- -es) is also a lemma ("Rastas"/"Rasta", "eyeglasses"/"eyeglass"). Short
-- words and -ss/-us/-is endings ("Ms", "Mass", "Pus") are left singular.
UPDATE nouns p SET plural = TRUE
WHERE p.lemma ~ 's$'
  AND length(p.lemma) > 3
  AND p.lemma !~ '(ss|us|is)$'
  AND EXISTS (
    SELECT 1 FROM nouns n
    WHERE n.lemma = left(p.lemma, -1)
       OR (p.lemma ~ 'es$' AND n.lemma = left(p.lemma, -2))
  );

-- name: SetNounFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE nouns SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE nouns.lemma = f.word;

-- name: SetProperNounFrequencies :execrows
-- Words are lowercase, so a capitalized lemma ("America") matches its
-- lowercase form. Only name frequencies go here, so the element "In" doesn't
-- pick up the preposition's.
UPDATE nouns SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE nouns.lemma <> lower(nouns.lemma) AND lower(nouns.lemma) = f.word;

-- name: GetRandomSingularNoun :one
-- For a locked singular determiner, which can't go with "Rastas".
SELECT * FROM nouns
WHERE active AND NOT plural AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: LookupNoun :one
-- A locked word: the floor doesn't apply.
SELECT * FROM nouns
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;

-- name: GetNounDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM nouns
WHERE lemma = $1
ORDER BY id
LIMIT 1;
