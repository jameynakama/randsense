package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/auth"
	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// savedTree is a stored tree, "this goose , devours", with its comma leaf
// at index 2.
const savedTree = `{"symbol": "S", "children": [
	{"symbol": "NP", "children": [
		{"symbol": "Determiner", "lemma": "this", "word": "this"},
		{"symbol": "Noun", "lemma": "goose", "word": "goose"}
	]},
	{"symbol": "Comma", "lemma": ",", "word": ","},
	{"symbol": "Verb:intransitive", "lemma": "devour", "word": "devours"}
]}`

var idSeq atomic.Int64

// nextID is a fresh sentence ID for test rows.
func nextID() string {
	return fmt.Sprintf("t%07d", idSeq.Add(1))
}

func mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// resetSentences empties sentences, and with them stars and flags.
func resetSentences(t *testing.T) {
	t.Helper()
	mustExec(t, "TRUNCATE sentences CASCADE")
}

// insertSentence saves savedTree under id, created at createdAt.
func insertSentence(t *testing.T, id string, createdAt time.Time) {
	t.Helper()
	mustExec(t, "INSERT INTO sentences (id, text, tree, commonness, created_at) VALUES ($1, 'This goose, devours.', $2, 0, $3)",
		id, savedTree, createdAt)
}

// call sends a request with an optional body and headers. The response
// body closes when the test ends.
func call(t *testing.T, srv *httptest.Server, method, path, body string, header http.Header) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	maps.Copy(req.Header, header)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// decode checks resp's status and decodes its JSON body into v, unless v
// is nil.
func decode(t *testing.T, resp *http.Response, status int, v any) {
	t.Helper()
	if resp.StatusCode != status {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want %d: %s", resp.StatusCode, status, b)
	}
	if v == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// newServer serves the API with cfg, filling in the test grammar and verbs,
// and testPool unless cfg has its own queries.
func newServer(t *testing.T, cfg api.RouterConfig) *httptest.Server {
	t.Helper()
	g, err := grammar.Load(strings.NewReader(testGrammar))
	if err != nil {
		t.Fatalf("grammar.Load: %v", err)
	}
	v, err := morph.LoadVerbs(strings.NewReader(""))
	if err != nil {
		t.Fatalf("morph.LoadVerbs: %v", err)
	}
	if cfg.Queries == nil {
		cfg.Queries = store.New(testPool)
	}
	if cfg.Admin.PasswordHash == nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		cfg.Admin = api.AdminConfig{PasswordHash: hash, SessionSecret: testSecret, InsecureCookies: cfg.Admin.InsecureCookies}
	}
	cfg.Grammar, cfg.Verbs = g, v
	return httptest.NewServer(api.NewRouter(cfg))
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newServer(t, api.RouterConfig{})
}

const testPassword = "correct horse battery"

var testSecret = []byte("test-session-secret-32-bytes-xxx")

// adminCookie is a session cookie, as a Cookie header, that's valid until
// expires.
func adminCookie(expires time.Time) http.Header {
	c := &http.Cookie{Name: auth.CookieName, Value: auth.NewSession(testSecret, expires)}
	return http.Header{"Cookie": {c.String()}}
}
