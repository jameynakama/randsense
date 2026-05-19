set dotenv-load

alias mu := migrate-up
alias md := migrate-down

default: test

# Run all tests
test args="":
    gotestsum ./... -- {{ args }}

gotest args="":
    go test ./... {{ args }}

cover:
    go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# Start the dev server with hot reload
run:
    air

# Build binary
build:
    go build -o bin/randsense ./cmd/server

# Run pending migrations
migrate-up:
    migrate -path migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate-down num="1":
    migrate -path migrations -database "$DATABASE_URL" down {{ num }}

# Regenerate sqlc types after query changes
generate:
    rm -f internal/store/*.sql.go
    sqlc generate

# Ingest OEW lexicon into the database
ingest:
    go run ./cmd/ingest
