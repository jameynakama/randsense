CREATE TABLE nouns (
    id           BIGSERIAL    PRIMARY KEY,
    lemma        TEXT         NOT NULL,
    inflections  JSONB        NOT NULL DEFAULT '{}',
    source       TEXT         NOT NULL,
    source_id    TEXT,
    register     TEXT,
    frequency    NUMERIC,
    active       BOOLEAN      NOT NULL DEFAULT TRUE,
    vote_count   INTEGER      NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    update_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (lemma, source)
);
CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON nouns FOR EACH ROW EXECUTE FUNCTION set_update_time();
-- index for the hot path (random word selection):
CREATE INDEX nouns_active_idx ON nouns (active) WHERE active;

CREATE TABLE verbs (
    id           BIGSERIAL    PRIMARY KEY,
    lemma        TEXT         NOT NULL,
    inflections  JSONB        NOT NULL DEFAULT '{}',
    frames       JSONB        NOT NULL DEFAULT '[]',
    source       TEXT         NOT NULL,
    source_id    TEXT,
    register     TEXT,
    frequency    NUMERIC,
    active       BOOLEAN      NOT NULL DEFAULT TRUE,
    vote_count   INTEGER      NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    update_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (lemma, source)
);
CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON verbs FOR EACH ROW EXECUTE FUNCTION set_update_time();
CREATE INDEX verbs_active_idx ON verbs (active) WHERE active;

CREATE TABLE adjectives (
    id           BIGSERIAL    PRIMARY KEY,
    lemma        TEXT         NOT NULL,
    inflections  JSONB        NOT NULL DEFAULT '{}',
    source       TEXT         NOT NULL,
    source_id    TEXT,
    register     TEXT,
    frequency    NUMERIC,
    active       BOOLEAN      NOT NULL DEFAULT TRUE,
    vote_count   INTEGER      NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    update_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (lemma, source)
);
CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON adjectives FOR EACH ROW EXECUTE FUNCTION set_update_time();
CREATE INDEX adjectives_active_idx ON adjectives (active) WHERE active;

CREATE TABLE adverbs (
    id           BIGSERIAL    PRIMARY KEY,
    lemma        TEXT         NOT NULL,
    inflections  JSONB        NOT NULL DEFAULT '{}',
    source       TEXT         NOT NULL,
    source_id    TEXT,
    register     TEXT,
    frequency    NUMERIC,
    active       BOOLEAN      NOT NULL DEFAULT TRUE,
    vote_count   INTEGER      NOT NULL DEFAULT 0,
    create_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    update_time  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (lemma, source)
);
CREATE TRIGGER set_update_time_before_update
    BEFORE UPDATE ON adverbs FOR EACH ROW EXECUTE FUNCTION set_update_time();
CREATE INDEX adverbs_active_idx ON adverbs (active) WHERE active;
