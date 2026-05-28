package emailaddr

import "testing"

func TestValidForLogin(t *testing.T) {
	tests := []struct {
		email string
		ok    bool
	}{
		{"admin@localhost", true},
		{"user@example.com", true},
		{"", false},
		{"not-an-email", false},
		{"@localhost", false},
	}
	for _, tc := range tests {
		if got := ValidForLogin(tc.email); got != tc.ok {
			t.Errorf("ValidForLogin(%q) = %v, want %v", tc.email, got, tc.ok)
		}
	}
}
