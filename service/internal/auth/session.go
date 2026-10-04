// Package auth signs the single admin's session: a cookie holding an
// expiry time, signed with HMAC-SHA256. There's no session table, so
// changing the secret logs every session out.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName = "randsense_admin"
	SessionTTL = 14 * 24 * time.Hour
)

// NewSession is a cookie value that's valid until expires.
func NewSession(secret []byte, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + base64.RawURLEncoding.EncodeToString(mac(secret, exp))
}

// ValidSession reports whether value was signed with secret and hasn't
// expired by now.
func ValidSession(secret []byte, value string, now time.Time) bool {
	exp, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	return err == nil && hmac.Equal(got, mac(secret, exp)) && now.Before(time.Unix(unix, 0))
}

func mac(secret []byte, msg string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return m.Sum(nil)
}
