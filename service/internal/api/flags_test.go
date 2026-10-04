package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func flagBody(comment, wordIndex string) string {
	c, _ := json.Marshal(comment)
	if wordIndex == "" {
		return fmt.Sprintf(`{"comment": %s}`, c)
	}
	return fmt.Sprintf(`{"comment": %s, "word_index": %s}`, c, wordIndex)
}

func postFlag(t *testing.T, srv *httptest.Server, id, body string) *http.Response {
	t.Helper()
	return call(t, srv, http.MethodPost, "/api/v1/sentences/"+id+"/flags", body, http.Header{"Content-Type": {"application/json"}})
}

type savedFlag struct {
	wordIndex  *int32
	lemma, pos *string
	comment    string
}

func getFlag(t *testing.T, id int64) savedFlag {
	t.Helper()
	var f savedFlag
	err := testPool.QueryRow(context.Background(), "SELECT word_index, lemma, pos, comment FROM flags WHERE id = $1", id).
		Scan(&f.wordIndex, &f.lemma, &f.pos, &f.comment)
	if err != nil {
		t.Fatalf("flag %d: %v", id, err)
	}
	return f
}

func TestFlagSentenceOrWord(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()
	const comment = "this makes no sense at all"

	tests := []struct {
		name, wordIndex    string
		wantLemma, wantPOS string // empty means null
	}{
		{"whole sentence", "", "", ""},
		{"determiner", "0", "this", "Determiner"},
		{"noun", "1", "goose", "Noun"},
		{"framed verb", "3", "devour", "Verb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body struct {
				ID int64 `json:"id"`
			}
			decode(t, postFlag(t, srv, "aaaaaaaa", flagBody(comment, tc.wordIndex)), http.StatusCreated, &body)

			f := getFlag(t, body.ID)
			deref := func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			}
			if deref(f.lemma) != tc.wantLemma || deref(f.pos) != tc.wantPOS || f.comment != comment {
				t.Errorf("flag: got lemma %q pos %q comment %q, want %q %q %q",
					deref(f.lemma), deref(f.pos), f.comment, tc.wantLemma, tc.wantPOS, comment)
			}
			if (tc.wordIndex == "") != (f.wordIndex == nil) {
				t.Errorf("word_index: got %v for %q", f.wordIndex, tc.wordIndex)
			}
		})
	}
}

func TestFlagRejectsBadWordIndexes(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	for _, idx := range []string{"2", "-1", "4", "1.5", `"1"`, "null"} {
		t.Run(idx, func(t *testing.T) {
			resp := postFlag(t, srv, "aaaaaaaa", flagBody("this makes no sense at all", idx))
			// null is no index: the whole sentence.
			want := http.StatusBadRequest
			if idx == "null" {
				want = http.StatusCreated
			}
			decode(t, resp, want, nil)
		})
	}
}

// Characters after trimming, not bytes.
func TestFlagCommentLength(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	tests := []struct {
		name, comment string
		want          int
	}{
		{"exactly 10 after trimming", "   0123456789   ", http.StatusCreated},
		{"9 after trimming", "          012345678          ", http.StatusBadRequest},
		{"1000 two-byte characters", strings.Repeat("é", 1000), http.StatusCreated},
		{"1001 characters", strings.Repeat("a", 1001), http.StatusBadRequest},
		{"empty", "", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decode(t, postFlag(t, srv, "aaaaaaaa", flagBody(tc.comment, "")), tc.want, nil)
		})
	}
}

func TestFlagRejectsBadRequests(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	decode(t, postFlag(t, srv, "zzzzzzzz", flagBody("this makes no sense at all", "")), http.StatusNotFound, nil)
	decode(t, postFlag(t, srv, "aaaaaaaa", "not json"), http.StatusBadRequest, nil)
}
