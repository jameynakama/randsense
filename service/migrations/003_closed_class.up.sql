CREATE TABLE determiners (
    id      BIGSERIAL PRIMARY KEY,
    lemma   TEXT      NOT NULL UNIQUE,
    type    TEXT      NOT NULL,        -- 'indefinite' | 'definite' | 'demonstrative' | 'possessive'
    number  TEXT      NOT NULL,        -- 'singular' | 'plural' | 'either'
    active  BOOLEAN   NOT NULL DEFAULT TRUE
);

CREATE TABLE prepositions (
    id     BIGSERIAL PRIMARY KEY,
    lemma  TEXT      NOT NULL UNIQUE,
    active BOOLEAN   NOT NULL DEFAULT TRUE
);

CREATE TABLE pronouns (
    id      BIGSERIAL PRIMARY KEY,
    lemma   TEXT      NOT NULL,
    case_   TEXT      NOT NULL,        -- 'nominative' | 'accusative' | 'genitive' | 'reflexive'
    person  SMALLINT  NOT NULL,        -- 1 | 2 | 3
    number  TEXT      NOT NULL,        -- 'singular' | 'plural'
    gender  TEXT      NOT NULL,        -- 'masc' | 'fem' | 'neuter' | 'epicene'
    active  BOOLEAN   NOT NULL DEFAULT TRUE,
    UNIQUE (lemma, case_, person, number, gender)
);

CREATE TABLE conjunctions (
    id     BIGSERIAL PRIMARY KEY,
    lemma  TEXT      NOT NULL,
    type   TEXT      NOT NULL,         -- 'coordinating' | 'subordinating'
    active BOOLEAN   NOT NULL DEFAULT TRUE,
    UNIQUE (lemma, type)
);
