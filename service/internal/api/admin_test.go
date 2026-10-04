package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/auth"
)

func login(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	return call(t, srv, http.MethodPost, "/api/v1/admin/login", body, http.Header{"Content-Type": {"application/json"}})
}

func sessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("expected a session cookie; got none")
	return nil
}

func TestLoginSetsASessionCookie(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := login(t, srv, `{"password": "`+testPassword+`"}`)
	decode(t, resp, http.StatusNoContent, nil)

	c := sessionCookie(t, resp)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != int(auth.SessionTTL.Seconds()) {
		t.Errorf("cookie: got %+v, want HttpOnly, Secure, SameSite=Strict, Path=/, 14 days", c)
	}
	if !auth.ValidSession(testSecret, c.Value, time.Now()) {
		t.Errorf("cookie value %q isn't a valid session", c.Value)
	}
}

func TestLoginCookieIsInsecureInDevelopment(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Admin: api.AdminConfig{InsecureCookies: true}})
	defer srv.Close()

	resp := login(t, srv, `{"password": "`+testPassword+`"}`)
	decode(t, resp, http.StatusNoContent, nil)

	if sessionCookie(t, resp).Secure {
		t.Error("cookie: got Secure, want plain http to work")
	}
}

func TestLoginFailuresAreSlowAndSetNoCookie(t *testing.T) {
	srv := newTestServer(t)
	t.Cleanup(srv.Close)

	for name, body := range map[string]string{
		"wrong password": `{"password": "wrong"}`,
		"no password":    `{}`,
		"not JSON":       `password`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			resp := login(t, srv, body)
			decode(t, resp, http.StatusUnauthorized, nil)

			if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
				t.Errorf("answered after %v, want about a second", elapsed)
			}
			if len(resp.Cookies()) != 0 {
				t.Errorf("cookies: got %v, want none", resp.Cookies())
			}
		})
	}
}

func TestAdminRoutesNeedAValidSession(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	tampered := adminCookie(time.Now().Add(time.Hour))
	tampered.Set("Cookie", tampered.Get("Cookie")+"x")
	other := &http.Cookie{Name: auth.CookieName, Value: auth.NewSession([]byte("another-secret-another-secret-xx"), time.Now().Add(time.Hour))}

	sessions := map[string]http.Header{
		"no cookie":    nil,
		"tampered":     tampered,
		"expired":      adminCookie(time.Now().Add(-time.Second)),
		"other secret": {"Cookie": {other.String()}},
	}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/logout"},
		{http.MethodGet, "/api/v1/admin/flags"},
		{http.MethodGet, "/api/v1/admin/flagged-words"},
	}
	for name, header := range sessions {
		for _, r := range routes {
			t.Run(name+" "+r.path, func(t *testing.T) {
				decode(t, call(t, srv, r.method, r.path, "", header), http.StatusUnauthorized, nil)
			})
		}
	}
}

func TestLogoutClearsTheCookie(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := call(t, srv, http.MethodPost, "/api/v1/admin/logout", "", adminCookie(time.Now().Add(time.Hour)))
	decode(t, resp, http.StatusNoContent, nil)

	if c := sessionCookie(t, resp); c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cookie: got %+v, want it cleared", c)
	}
}

func TestAdminListsFlagsNewestFirst(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	t0 := time.Now().Add(-time.Hour)
	mustExec(t, "INSERT INTO flags (sentence_id, comment, created_at) VALUES ('aaaaaaaa', 'older whole sentence', $1)", t0)
	mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment, created_at) VALUES ('aaaaaaaa', 1, 'goose', 'Noun', 'newer goose', $1)", t0.Add(time.Minute))
	srv := newTestServer(t)
	defer srv.Close()

	var body []struct {
		WordIndex *int32  `json:"word_index"`
		Lemma     *string `json:"lemma"`
		Comment   string  `json:"comment"`
		Sentence  struct {
			ID   string          `json:"id"`
			Text string          `json:"text"`
			Tree json.RawMessage `json:"tree"`
		} `json:"sentence"`
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/admin/flags", "", adminCookie(time.Now().Add(time.Hour))), http.StatusOK, &body)

	if len(body) != 2 || body[0].Comment != "newer goose" || body[1].Comment != "older whole sentence" {
		t.Fatalf("flags: got %+v, want newer goose then older whole sentence", body)
	}
	if body[0].WordIndex == nil || *body[0].WordIndex != 1 || body[0].Lemma == nil || *body[0].Lemma != "goose" {
		t.Errorf("word flag: got %+v, want index 1, lemma goose", body[0])
	}
	if body[1].WordIndex != nil || body[1].Lemma != nil {
		t.Errorf("sentence flag: got %+v, want null index and lemma", body[1])
	}
	if s := body[0].Sentence; s.ID != "aaaaaaaa" || s.Text != "This goose, devours." || len(s.Tree) == 0 {
		t.Errorf("sentence: got %+v, want aaaaaaaa with its text and tree", s)
	}
}

func TestAdminListsMostFlaggedWords(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	for _, f := range []struct {
		idx        int
		lemma, pos string
	}{{1, "goose", "Noun"}, {3, "devour", "Verb"}, {1, "goose", "Noun"}} {
		mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment) VALUES ('aaaaaaaa', $1, $2, $3, 'a long enough comment')", f.idx, f.lemma, f.pos)
	}
	mustExec(t, "INSERT INTO flags (sentence_id, comment) VALUES ('aaaaaaaa', 'the whole thing is off')")
	srv := newTestServer(t)
	defer srv.Close()

	var body []struct {
		Lemma string `json:"lemma"`
		POS   string `json:"pos"`
		Count int64  `json:"count"`
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/admin/flagged-words", "", adminCookie(time.Now().Add(time.Hour))), http.StatusOK, &body)

	got := make([]string, len(body))
	for i, w := range body {
		got[i] = fmt.Sprintf("%s/%s/%d", w.Lemma, w.POS, w.Count)
	}
	if want := []string{"goose/Noun/2", "devour/Verb/1"}; !slices.Equal(got, want) {
		t.Errorf("flagged words: got %v, want %v", got, want)
	}
}
