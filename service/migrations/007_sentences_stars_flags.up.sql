-- id is 8 random base-62 characters made in Go: unlike a sequence, it
-- doesn't reveal how many sentences exist.
CREATE TABLE sentences (
    id          TEXT        PRIMARY KEY,
    text        TEXT        NOT NULL,
    tree        JSONB       NOT NULL,
    commonness  NUMERIC     NOT NULL,
    -- Kept in step with stars in the same statement, so lists never count rows.
    star_count  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX sentences_created_at_idx ON sentences (created_at DESC, id DESC);

-- voter is a random token the browser keeps, not an account.
CREATE TABLE stars (
    sentence_id TEXT        NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    voter       TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sentence_id, voter)
);
CREATE INDEX stars_voter_idx ON stars (voter, created_at DESC);

-- A null word_index flags the whole sentence. lemma and pos are copied from
-- the tree so the most-flagged words are a plain GROUP BY.
CREATE TABLE flags (
    id          BIGSERIAL   PRIMARY KEY,
    sentence_id TEXT        NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    word_index  INTEGER,
    lemma       TEXT,
    pos         TEXT,
    comment     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX flags_created_at_idx ON flags (created_at DESC, id DESC);
