package api

import (
	"strings"
	"testing"
	"time"
)

func TestValidTree(t *testing.T) {
	secret := []byte("build-secret-for-tests-32-bytes-x")
	tree := []byte(`{"symbol":"S"}`)
	issued := time.Unix(1_800_000_000, 0)
	sig := signTree(secret, tree, issued)
	at, mac, _ := strings.Cut(sig, ".")

	tests := []struct {
		name         string
		secret, tree []byte
		sig          string
		now          time.Time
		want         bool
	}{
		{"fresh", secret, tree, sig, issued.Add(time.Hour), true},
		{"a day old", secret, tree, sig, issued.Add(buildTTL), false},
		{"another tree", secret, []byte(`{"symbol":"NP"}`), sig, issued, false},
		{"another secret", []byte("another-build-secret-32-bytes-xx"), tree, sig, issued, false},
		{"another time", secret, tree, "1800000001." + mac, issued, false},
		{"no dot", secret, tree, at + mac, issued, false},
		{"time that isn't a number", secret, tree, "soon." + mac, issued, false},
		{"mac that isn't base64", secret, tree, at + ".!!", issued, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validTree(tc.secret, tc.tree, tc.sig, tc.now); got != tc.want {
				t.Errorf("validTree: got %v, want %v", got, tc.want)
			}
		})
	}
}
