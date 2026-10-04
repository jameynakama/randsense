set dotenv-load

alias mu := migrate-up
alias md := migrate-down

default: test

# Run all tests
test args="": (test-be args) test-fe

# Start everything with hot reload
run:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    just run-be & just run-fe & wait

# Build everything
build: build-be build-fe

# Run the Go tests
[working-directory: 'service']
test-be args="":
    gotestsum ./... -- {{ args }}

[working-directory: 'service']
gotest args="":
    go test ./... {{ args }}

[working-directory: 'service']
cover:
    go test -coverprofile=coverage.out -coverpkg=./internal/api/...,./internal/grammar/...,./internal/lexicon/...,./internal/morph/...,./internal/sentence/...,./internal/live/...,./internal/auth/... ./... && go tool cover -func=coverage.out

# Start the Go server with hot reload
[working-directory: 'service']
run-be:
    air

# Build the Go server binary
[working-directory: 'service']
build-be:
    go build -o bin/randsense ./cmd/server

# Start the SvelteKit dev server
[working-directory: 'web']
run-fe:
    npm run dev

# Type-check, lint and run the web unit, component and e2e tests
[working-directory: 'web']
test-fe: build-fe
    npm run check
    npm run lint
    npx vitest --run
    npx playwright test

# Build the SvelteKit app
[working-directory: 'web']
build-fe:
    npm run build

# Run pending migrations
[working-directory: 'service']
migrate-up:
    migrate -path migrations -database "$DATABASE_URL" up

# Roll back one migration
[working-directory: 'service']
migrate-down num="1":
    migrate -path migrations -database "$DATABASE_URL" down {{ num }}

# Regenerate sqlc types after query changes
[working-directory: 'service']
generate:
    rm -f internal/store/*.sql.go
    sqlc generate

# Ingest the OEWN lexicon and the curated closed-class words into the database
[working-directory: 'service']
ingest:
    go run ./cmd/ingest

# Print a bcrypt hash of a typed password, for ADMIN_PASSWORD_HASH
[working-directory: 'service']
hash-password:
    go run ./cmd/hashpassword

@get num="1" commonness="0":
    for i in $(seq {{ num }}); do printf '%s: ' "$i"; http http://localhost:8080/api/v1/sentences/random?commonness={{ commonness }} | jq .text; done
