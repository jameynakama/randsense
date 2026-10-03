# randsense

A random sentence generator that produces grammatically sound nonsense. It picks words from a
weighted lexicon, expands a probabilistic context-free grammar, and inflects the result into
something that parses correctly but means nothing in particular.

Built in Go. Postgres for storage, WordNet (OEWN) as the primary lexicon, and
[SUBTLEX-US](https://www.ugent.be/pp/experimentele-psychologie/en/research/documents/subtlexus)
for word frequency.

## Stack

- **[chi](https://github.com/go-chi/chi)** -- HTTP router
- **[pgx/v5](https://github.com/jackc/pgx)** -- Postgres driver + connection pool
- **[sqlc](https://sqlc.dev)** -- type-safe Go from SQL queries
- **[golang-migrate](https://github.com/golang-migrate/migrate)** -- versioned migrations
- **[docker-compose](https://docs.docker.com/compose/)** -- local Postgres
- **[just](https://github.com/casey/just)** -- task runner
- **[air](https://github.com/air-verse/air)** -- hot reload for dev

## Prerequisites

- Go 1.26+
- Docker Desktop (or equivalent)
- `just`, `sqlc`, `golang-migrate`, `air`

## Setup

```bash
cp .env.example .env  # edit as needed
docker compose up -d
just migrate-up
just ingest   # load OEWN, SUBTLEX-US frequencies and the closed-class word lists
just run
```

Server starts on `http://localhost:8080` (or `PORT` from `.env`).

## Commands

| Command                         | Description                              |
| ------------------------------- | ---------------------------------------- |
| `just`                          | Run tests (default)                      |
| `just run`                      | Start dev server with hot reload         |
| `just build`                    | Build binary to `bin/randsense`          |
| `just migrate-up`               | Apply pending migrations                 |
| `just migrate-down [n]`         | Roll back n migrations (default 1)       |
| `just generate`                 | Regenerate sqlc types after query changes|
| `just ingest`                   | Load OEWN, SUBTLEX-US and `data/lexicon/closed_class.toml` |

## API

```
GET /health
GET /api/v1/words/random?pos=noun|verb|adjective|adverb[&commonness=N]
GET /api/v1/sentences/random[?commonness=N]   -> {text, tree}
POST /api/v1/sentences/realize[?commonness=N] -> {text, tree}
```

`realize` takes a tree in the shape `random` returns and fills it with fresh words, so a specific
construction can be checked without fishing for it. Leaves must be parts of speech, optionally
qualified as in `grammar.toml`. Agreement depends on the `NP`, `VP`, `InfVP` and `GerVP` symbols,
as it does there. Bodies are capped at 64 KiB. A slot no word fits, such as a frame with no verbs
above the floor, returns 422.

```bash
echo '{"symbol": "S", "children": [
  {"symbol": "NP", "children": [{"symbol": "Pronoun"}]},
  {"symbol": "VP", "children": [{"symbol": "Verb:transitive"}, {"symbol": "Pronoun:reflexive"}]}
]}' | http POST :8080/api/v1/sentences/realize | jq .text
```

`commonness` (0 to 7, default 0) limits nouns, verbs, adjectives and adverbs to words at least that
common in that part of speech, on the Zipf scale (log10 occurrences per billion words of
subtitles). "baby" clears 5 as a noun but has no frequency as a verb. Words SUBTLEX-US lacks,
including every multiword lemma, count as 0, so any floor above 0 drops them.

| commonness | nouns that clear it, roughly                  |
| ---------- | --------------------------------------------- |
| 0          | anything, including goffer and tintinnabulate |
| 2          | druggist, matzah, coefficient and up          |
| 3          | veil, gateway, ballot and up                  |
| 4          | lake, industry, warrior and up                |
| 5          | brother, door, baby and up; about 200 nouns   |

`data/subtlex-us/subtlex-us-pos.tsv.gz` is derived from the SUBTLEX-US part-of-speech workbook by
`data/subtlex-us/convert.py`, which documents how to regenerate it.

The server loads `data/grammar/grammar.toml` and `data/lexicon/verb_morphology.toml` at startup
and refuses to start if either is invalid.

## Tests

Integration tests hit a real ephemeral database. Set `TEST_DATABASE_URL` in `.env` pointing at
the same Postgres instance -- the suite creates and drops the test DB automatically.

```bash
just test
```
