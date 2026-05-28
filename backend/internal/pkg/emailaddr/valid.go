package emailaddr

import (
	"net/mail"
	"regexp"
	"strings"
)

// localLoginEmail matches user@host when host has no dot (e.g. admin@localhost for desktop).
var localLoginEmail = regexp.MustCompile(`^[^\s@]+@[a-zA-Z0-9_-]+$`)

// ValidForLogin accepts standard RFC 5322 addresses and single-label local hosts used by the desktop build.
func ValidForLogin(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 255 {
		return false
	}
	if _, err := mail.ParseAddress(email); err == nil {
		return true
	}
	return localLoginEmail.MatchString(email)
}
