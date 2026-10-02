# randsense

A random sentence generator that produces grammatically sound nonsense. It picks words from a
weighted lexicon, expands a probabilistic context-free grammar, and inflects the result into
something that parses correctly but means nothing in particular.

Built in Go. Postgres for storage, WordNet (OEWN) as the primary lexicon.

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
just ingest   # load OEWN and the closed-class word lists
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
| `just ingest`                   | Load OEWN and `data/lexicon/closed_class.toml` |

## API

```
GET /health
GET /api/v1/words/random?pos=noun|verb|adjective|adverb
GET /api/v1/sentences/random   -> {text, tree}
```

The server loads `data/grammar/grammar.toml` and `data/lexicon/verb_morphology.toml` at startup
and refuses to start if either is invalid.

## Tests

Integration tests hit a real ephemeral database. Set `TEST_DATABASE_URL` in `.env` pointing at
the same Postgres instance -- the suite creates and drops the test DB automatically.

```bash
just test
```
