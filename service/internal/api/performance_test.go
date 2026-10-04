package api_test

import (
	"context"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/store"
)

// queryCounter counts the queries a pool runs.
type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func countingServer(t *testing.T) (*httptest.Server, *queryCounter) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(testDBURL)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	c := &queryCounter{}
	cfg.ConnConfig.Tracer = c
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	srv := newServer(t, api.RouterConfig{Queries: store.New(pool)})
	t.Cleanup(srv.Close)
	return srv, c
}

// listEndpoints are checked for N+1 queries. seed adds n more rows the
// endpoint lists.
var listEndpoints = []struct {
	name   string
	path   string
	header http.Header
	seed   func(t *testing.T, n int)
}{
	{"sentences", "/api/v1/sentences", nil, func(t *testing.T, n int) {
		for range n {
			insertSentence(t, nextID(), time.Now())
		}
	}},
	{"starred sentences", "/api/v1/stars", http.Header{"X-Voter": {voterA}}, func(t *testing.T, n int) {
		for range n {
			id := nextID()
			insertSentence(t, id, time.Now())
			mustExec(t, "INSERT INTO stars (sentence_id, voter) VALUES ($1, $2)", id, voterA)
		}
	}},
	{"admin flags", "/api/v1/admin/flags", adminCookie(time.Now().Add(time.Hour)), func(t *testing.T, n int) {
		for range n {
			id := nextID()
			insertSentence(t, id, time.Now())
			mustExec(t, "INSERT INTO flags (sentence_id, comment) VALUES ($1, 'a long enough comment')", id)
		}
	}},
	{"flagged words", "/api/v1/admin/flagged-words", adminCookie(time.Now().Add(time.Hour)), func(t *testing.T, n int) {
		id := nextID()
		insertSentence(t, id, time.Now())
		for range n {
			mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment) VALUES ($1, 1, $2, 'Noun', 'a long enough comment')", id, nextID())
		}
	}},
}

func TestListEndpointsRunOneQueryWhateverTheRowCount(t *testing.T) {
	for _, e := range listEndpoints {
		t.Run(e.name, func(t *testing.T) {
			resetSentences(t)
			srv, c := countingServer(t)
			queries := func() int64 {
				c.n.Store(0)
				decode(t, call(t, srv, http.MethodGet, e.path, "", e.header), http.StatusOK, nil)
				return c.n.Load()
			}

			e.seed(t, 2+rand.IntN(5))
			first := queries()
			e.seed(t, 2+rand.IntN(5))
			second := queries()

			if first != 1 || second != 1 {
				t.Errorf("queries: got %d then %d, want 1 both times", first, second)
			}
		})
	}
}

func TestListEndpointsRejectBadPages(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	for _, e := range listEndpoints {
		t.Run(e.name, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodGet, e.path+"?limit=0", "", e.header), http.StatusBadRequest, nil)
		})
	}
}
