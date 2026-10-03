package subtlex_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool is the package-wide pool against a freshly-created and
// migrated test database. Created in TestMain, torn down at exit.
var testPool *pgxpool.Pool

func getRequiredEnvVar(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s must be set", key)
	}
	return v
}

func getDBConn(ctx context.Context, dbURL string) *pgxpool.Pool {
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("error establishing test database connection: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("cannot ping test database %s: %v", dbURL, err)
	}
	return db
}

func TestMain(m *testing.M) {
	testDBURL := getRequiredEnvVar("TEST_DATABASE_URL")
	// Own database so this package can't drop one another package is using
	// when `go test ./...` runs packages in parallel.
	testDBURL = swapDBName(testDBURL, getDBName(testDBURL)+"_subtlex")
	testDBName := getDBName(testDBURL)

	ctx := context.Background()

	pgDB := getDBConn(ctx, swapDBName(testDBURL, "postgres"))
	pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	if _, err := pgDB.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName)); err != nil {
		log.Fatal("could not create test database")
	}

	migrateURL := strings.Replace(testDBURL, "postgres://", "pgx5://", 1)
	// Three levels up from internal/lexicon/subtlex -> repo root -> migrations.
	mig, err := migrate.New("file://../../../migrations", migrateURL)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("could not migrate test db: %v", err)
	}

	testPool = getDBConn(ctx, testDBURL)

	code := m.Run()

	testPool.Close()
	pgDB.Exec(ctx, `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = $1 AND pid <> pg_backend_pid()
	`, testDBName)
	pgDB.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName))
	pgDB.Close()

	os.Exit(code)
}

func getDBName(dbURL string) string {
	u, _ := url.Parse(dbURL)
	return u.Path[1:]
}

func swapDBName(oldDB, newDB string) string {
	u, _ := url.Parse(oldDB)
	u.Path = "/" + newDB
	return u.String()
}
