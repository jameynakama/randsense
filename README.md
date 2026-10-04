# randsense

A random sentence generator that produces grammatically sound nonsense. It picks words from a
weighted lexicon, expands a probabilistic context-free grammar, and inflects the result into
something that parses correctly but means nothing in particular.

Built in Go. Postgres for storage, WordNet (OEWN) as the primary lexicon, and
[SUBTLEX-US](https://www.ugent.be/pp/experimentele-psychologie/en/research/documents/subtlexus)
for word frequency.

The Go service lives in `service/`; run the `just` recipes from the repository root.

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
just ingest   # load OEWN, separable-verb labels, SUBTLEX-US frequencies and closed-class words
just run
```

Server starts on `http://localhost:8080` (or `PORT` from `.env`).

## Commands

| Command                         | Description                              |
| ------------------------------- | ---------------------------------------- |
| `just`                          | Run tests (default)                      |
| `just run`                      | Start the app with hot reload            |
| `just run-be`                   | Start the Go server with hot reload      |
| `just test-be`                  | Run the Go tests                         |
| `just build-be`                 | Build binary to `service/bin/randsense`  |
| `just migrate-up`               | Apply pending migrations                 |
| `just migrate-down [n]`         | Roll back n migrations (default 1)       |
| `just generate`                 | Regenerate sqlc types after query changes|
| `just hash-password`            | Print a bcrypt hash for `ADMIN_PASSWORD_HASH` |
| `just ingest`                   | Load OEWN, SUBTLEX-US and the `service/data/lexicon/` lists |

## API

```
GET /health
GET /api/v1/words/random?pos=noun|verb|adjective|adverb[&commonness=N]
GET /api/v1/sentences/random[?commonness=N]   -> {id, text, tree, star_count, created_at}
POST /api/v1/sentences/realize[?commonness=N] -> {text, tree}
GET /api/v1/sentences[?limit=30&offset=0]     -> [{id, text, tree, star_count, created_at}]
GET /api/v1/sentences/{id}                    -> {id, text, tree, star_count, created_at}
POST   /api/v1/sentences/{id}/stars           -> {count}   (X-Voter: <uuid>)
DELETE /api/v1/sentences/{id}/stars           -> {count}   (X-Voter: <uuid>)
GET    /api/v1/stars[?limit&offset]           -> [sentence] starred by X-Voter, newest star first
POST   /api/v1/sentences/{id}/flags           {comment, word_index?} -> 201 {id}
GET    /api/v1/sentences/stream               Server-Sent Events: sentence, stars
POST   /api/v1/admin/login                    {password} -> 204 + session cookie
POST   /api/v1/admin/logout                   -> 204
GET    /api/v1/admin/flags[?limit&offset]     -> [{id, word_index, lemma, pos, comment, created_at, sentence: {id, text, tree}}]
GET    /api/v1/admin/flagged-words[?limit&offset] -> [{lemma, pos, count}]
```

The stream sends `sentence` (the full sentence) whenever `random` saves one and `stars`
(`{id, count}`) after every star or unstar, even one that leaves the count unchanged, with a `:`
keepalive every 25 seconds. It doesn't replay missed events.

There's one admin and no user table. `ADMIN_PASSWORD_HASH` holds a bcrypt hash (`just
hash-password` makes one) and `SESSION_SECRET` (32+ bytes) signs the 14-day session cookie, so
changing either and restarting logs everyone out. Every `/admin` route but `login` needs the
cookie. `INSECURE_COOKIES=true` drops the cookie's `Secure` flag for plain-http development.

The voter token is a random UUID the browser keeps. Starring and unstarring are idempotent.

A flag's comment is 10 to 1,000 characters after trimming. `word_index` counts the tree's leaves
from 0, commas included; a comma can't be flagged, and without an index the flag is for the
whole sentence.

List endpoints return newest first, `limit` 1 to 100 (default 30).

`random` saves each sentence it returns and allows any origin, so other sites can embed it.
`realize` saves nothing.

`realize` takes a tree in the shape `random` returns and fills it with fresh words, so a specific
construction can be checked without fishing for it. Leaves must be parts of speech, optionally
qualified as in `grammar.toml`. Agreement depends on the symbol names its header lists, as it does
there. Bodies are capped at 64 KiB. A slot no word fits, such as a frame with no verbs
above the floor, returns 422.

Every node in a returned tree may carry a `features` object with what generation worked out:
the root's `tense` and `commonness`; an NP's `person` and `number`; a noun's `number`; a verb's
`form` (`finite`, `base` or `gerund`), `frames` (every frame its lemma has) and `separable`,
plus `tense`, `person` and `number` when it's finite; a pronoun's `case`, `person`, `number`
and `gender`; a determiner's `type` and `number`. Content words carry their Zipf `frequency`
when SUBTLEX-US has it. Empty fields are omitted. A leaf whose written form differs from `word`,
such as a separable verb split around its object ("looked her up"), carries `display`.

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

`service/data/subtlex-us/subtlex-us-pos.tsv.gz` is derived from the SUBTLEX-US part-of-speech
workbook by `service/data/subtlex-us/convert.py`, which documents how to regenerate it.

The server loads `service/data/grammar/grammar.toml` and the two
`service/data/lexicon/verb_morphology*.toml` files at startup and refuses to start if any is
invalid.

## Tests

Integration tests hit real ephemeral databases. Set `TEST_DATABASE_URL` in `.env` pointing at
the same Postgres instance -- each package creates and drops its own database, named from it with a
package suffix (`randsense_test_api`), so packages can run in parallel.

```bash
just test
```
