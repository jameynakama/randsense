ALTER TABLE nouns DROP COLUMN plural;
ALTER TABLE nouns DROP COLUMN plural_override;
ALTER TABLE nouns RENAME COLUMN plural_guess TO plural;
