package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
)

// realized is a realize response for realizeTree.
type realized struct {
	Text      string          `json:"text"`
	Tree      json.RawMessage `json:"tree"`
	Signature string          `json:"signature"`
}

func realizeForKeep(t *testing.T, srv *httptest.Server) realized {
	t.Helper()
	resp := postTree(t, srv, "", realizeTree)
	defer resp.Body.Close()
	var r realized
	decode(t, resp, http.StatusOK, &r)
	if r.Signature == "" {
		t.Fatal("realize returned no signature")
	}
	return r
}

func keepBody(tree json.RawMessage, signature string) string {
	return fmt.Sprintf(`{"tree": %s, "signature": %q}`, tree, signature)
}

func TestKeepSavesABuiltSentence(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)
	r := realizeForKeep(t, srv)

	var kept api.SentenceResponse
	decode(t, call(t, srv, http.MethodPost, "/api/v1/sentences", keepBody(r.Tree, r.Signature), nil), http.StatusCreated, &kept)

	if kept.Origin != "built" || kept.Text != r.Text {
		t.Errorf("kept: got %q %q, want built %q", kept.Origin, kept.Text, r.Text)
	}
	var got api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/"+kept.ID, "", nil), http.StatusOK, &got)
	if got.Origin != "built" {
		t.Errorf("saved origin: got %q, want built", got.Origin)
	}
	if name, data := nextEvent(t, events); name != "sentence" || !strings.Contains(data, kept.ID) {
		t.Errorf("event: got %s %s, want the kept sentence", name, data)
	}
}

func TestKeepRejectsTreesItDidNotSign(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()
	r := realizeForKeep(t, srv)
	tampered := json.RawMessage(strings.ReplaceAll(string(r.Tree), "goose", "gander"))

	tests := []struct{ name, body string }{
		{"tampered tree", keepBody(tampered, r.Signature)},
		{"no signature", keepBody(r.Tree, "")},
		{"garbage signature", keepBody(r.Tree, "x.y")},
		{"not JSON", "keep it"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodPost, "/api/v1/sentences", tc.body, nil), http.StatusBadRequest, nil)
		})
	}
}
