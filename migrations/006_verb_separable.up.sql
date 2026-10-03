-- A separable phrasal verb puts a pronoun object before its particle: "look
-- it up", never "look up it".
ALTER TABLE verbs ADD COLUMN separable BOOLEAN NOT NULL DEFAULT FALSE;
