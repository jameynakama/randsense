package auth_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/auth"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestValidSession(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	valid := auth.NewSession(secret, now.Add(time.Hour))
	exp, sig, _ := strings.Cut(valid, ".")

	tests := []struct {
		name, value string
		secret      []byte
		want        bool
	}{
		{"unexpired", valid, secret, true},
		{"expired", auth.NewSession(secret, now.Add(-time.Second)), secret, false},
		{"expiring now", auth.NewSession(secret, now), secret, false},
		{"signed with another secret", valid, []byte("another-secret-another-secret-xx"), false},
		{"expiry pushed later", fmt.Sprint(now.Add(48*time.Hour).Unix()) + "." + sig, secret, false},
		{"signature tampered", exp + "." + strings.Repeat("A", len(sig)), secret, false},
		{"signature not base64", exp + ".!!!", secret, false},
		{"expiry not a number", "soon." + sig, secret, false},
		{"no signature", exp, secret, false},
		{"empty", "", secret, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := auth.ValidSession(tc.secret, tc.value, now); got != tc.want {
				t.Errorf("expected %v; got %v", tc.want, got)
			}
		})
	}
}
