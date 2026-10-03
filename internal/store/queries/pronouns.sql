-- name: InsertPronoun :exec
INSERT INTO pronouns (lemma, case_, person, number, gender)
VALUES ($1, $2, $3, $4, $5);

-- name: TruncatePronouns :exec
TRUNCATE pronouns RESTART IDENTITY CASCADE;

-- name: ListPronouns :many
SELECT * FROM pronouns
ORDER BY lemma, case_, person, number, gender;

-- name: GetRandomPronounWithCase :one
SELECT * FROM pronouns
WHERE active AND case_ = $1
ORDER BY random()
LIMIT 1;

-- name: GetRandomPronounWithAgreement :one
-- An empty gender matches any.
SELECT * FROM pronouns
WHERE active AND case_ = @case_ AND person = @person AND number = @number
  AND (@gender::text = '' OR gender = @gender)
ORDER BY random()
LIMIT 1;
