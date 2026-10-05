package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/sentence"
)

// buildTTL is how long a realized tree can wait to be kept.
const buildTTL = 24 * time.Hour

// maxKeepBytes caps a keep request. A realized tree is bigger than the one
// posted to realize: it carries words and features.
const maxKeepBytes = 256 << 10

// signTree signs a realized tree's canonical JSON and when it was realized,
// as "unix.mac". The server keeps nothing between realize and keep.
func signTree(secret, tree []byte, issued time.Time) string {
	at := strconv.FormatInt(issued.Unix(), 10)
	return at + "." + base64.RawURLEncoding.EncodeToString(treeMAC(secret, at, tree))
}

// validTree reports whether sig signs tree and was issued less than
// buildTTL before now.
func validTree(secret, tree []byte, sig string, now time.Time) bool {
	at, mac, ok := strings.Cut(sig, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(mac)
	return err == nil && hmac.Equal(got, treeMAC(secret, at, tree)) && now.Before(time.Unix(unix, 0).Add(buildTTL))
}

func treeMAC(secret []byte, at string, tree []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(at + "."))
	m.Write(tree)
	return m.Sum(nil)
}

// realizeResponse is a realized sentence and the signature Keep checks.
type realizeResponse struct {
	*sentence.Sentence
	Signature string `json:"signature"`
}

type keepRequest struct {
	Tree      grammar.Node `json:"tree"`
	Signature string       `json:"signature"`
}

// keepSentence saves a tree realize returned, under its signature, so
// nobody can save words of their own under RandSense's name. The text is
// written from the tree, never taken from the client.
func (h *Handler) keepSentence(w http.ResponseWriter, r *http.Request) {
	var req keepRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxKeepBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("body must be JSON {tree, signature} of at most %d bytes: %v", maxKeepBytes, err))
		return
	}
	// Marshaling the decoded tree gives the same bytes realize signed.
	tree, err := json.Marshal(&req.Tree)
	if err != nil {
		log.Printf("keepSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if !validTree(h.buildSecret, tree, req.Signature, time.Now()) {
		writeError(w, http.StatusBadRequest, "the signature doesn't match this tree, or is a day old: reroll, then keep")
		return
	}
	// Realize always sets the root's floor, and the signature vouches for
	// the tree.
	saved, err := h.save(r.Context(), sentence.Text(&req.Tree), tree, *req.Tree.Features.Commonness, originBuilt)
	if err != nil {
		log.Printf("keepSentence: save: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	resp := sentenceResponse(saved)
	h.publish("sentence", resp)
	writeJSON(w, http.StatusCreated, resp)
}
