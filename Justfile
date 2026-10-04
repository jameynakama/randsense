set dotenv-load

alias mu := migrate-up
alias md := migrate-down

default: test

# Run all tests
test args="": (test-be args)

# Start everything with hot reload
run: run-be

# Build everything
build: build-be

# Run the Go tests
[working-directory: 'service']
test-be args="":
    gotestsum ./... -- {{ args }}

[working-directory: 'service']
gotest args="":
    go test ./... {{ args }}

[working-directory: 'service']
cover:
    go test -coverprofile=coverage.out -coverpkg=./internal/api/...,./internal/grammar/...,./internal/lexicon/...,./internal/morph/...,./internal/sentence/...,./internal/live/... ./... && go tool cover -func=coverage.out

# Start the Go server with hot reload
[working-directory: 'service']
run-be:
    air

# Build the Go server binary
[working-directory: 'service']
build-be:
    go build -o bin/randsense ./cmd/server

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

@get num="1" commonness="0":
    for i in $(seq {{ num }}); do printf '%s: ' "$i"; http http://localhost:8080/api/v1/sentences/random?commonness={{ commonness }} | jq .text; done
