-- name: InsertPronoun :exec
INSERT INTO pronouns (lemma, case_, person, number, gender)
VALUES ($1, $2, $3, $4, $5);

-- name: TruncatePronouns :exec
TRUNCATE pronouns RESTART IDENTITY CASCADE;

-- name: ListPronouns :many
SELECT * FROM pronouns
ORDER BY lemma, case_, person, number, gender;
