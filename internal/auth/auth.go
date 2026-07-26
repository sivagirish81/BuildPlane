package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func CheckBearer(r *http.Request, token string) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	got := strings.TrimPrefix(header, "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
