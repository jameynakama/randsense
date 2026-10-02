-- name: InsertNoun :exec
INSERT INTO nouns (lemma, inflections, source)
VALUES ($1, $2, $3)
ON CONFLICT (lemma, source) DO NOTHING;

-- name: TruncateNouns :exec
TRUNCATE nouns RESTART IDENTITY CASCADE;

-- name: CountNouns :one
SELECT COUNT(*) FROM nouns;

-- name: GetNounByLemma :one
SELECT * FROM nouns
WHERE lemma = $1;

-- name: GetRandomNoun :one
SELECT * FROM nouns
WHERE active
ORDER BY random()
LIMIT 1;

-- name: MarkPluralNouns :execrows
-- A lemma is plural if it ends in -s and its singular is also a lemma
-- ("Rastas"/"Rasta"). Short words and -ss/-us/-is endings ("Ms", "Mass",
-- "Pus") are left singular.
UPDATE nouns p SET plural = TRUE
WHERE p.lemma ~ 's$'
  AND length(p.lemma) > 3
  AND p.lemma !~ '(ss|us|is)$'
  AND EXISTS (SELECT 1 FROM nouns n WHERE n.lemma = left(p.lemma, -1));
