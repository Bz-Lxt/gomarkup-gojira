package auth

import (
	"strings"
	"testing"
	"time"

	"gojira/internal/domain"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("Admin@123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Admin@123" || hash == "" {
		t.Fatal("hash must not equal plaintext")
	}
	if err := CheckPassword(hash, "Admin@123"); err != nil {
		t.Fatalf("check good password: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestValidatePasswordTable(t *testing.T) {
	tests := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"too short", "Ab1", true},
		{"no upper", "abcdefg1", true},
		{"no lower", "ABCDEFG1", true},
		{"no digit", "Abcdefgh", true},
		{"empty", "", true},
		{"ok seed admin", "Admin@123", false},
		{"ok seed pm", "Pm@123456", false},
		{"ok seed dev", "Dev@123456", false},
		{"ok seed qa", "Qa@123456", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw)
			if tt.wantErr && err == nil {
				t.Fatal("expected reject")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
		})
	}
}

func TestSignParseRoundTrip(t *testing.T) {
	s := &Service{Secret: []byte("gojira-dev-secret-change-in-prod-32b")}
	u := &domain.User{ID: 7, Username: "dev", Role: domain.RoleDev}
	tok, err := s.Sign(u, "access", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != 7 || c.Username != "dev" || c.Role != domain.RoleDev || c.Kind != "access" {
		t.Fatalf("claims %+v", c)
	}
}

func TestParseRejectsTampered(t *testing.T) {
	s := &Service{Secret: []byte("gojira-dev-secret-change-in-prod-32b")}
	u := &domain.User{ID: 1, Username: "admin", Role: domain.RoleAdmin}
	tok, err := s.Sign(u, "access", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bad := tok + "x"
	if _, err := s.Parse(bad); err == nil {
		t.Fatal("tampered token should fail")
	}
	other := &Service{Secret: []byte(strings.Repeat("z", 32))}
	if _, err := other.Parse(tok); err == nil {
		t.Fatal("wrong secret should fail")
	}
}

func TestParseExpired(t *testing.T) {
	s := &Service{Secret: []byte("gojira-dev-secret-change-in-prod-32b")}
	u := &domain.User{ID: 1, Username: "admin", Role: domain.RoleAdmin}
	tok, err := s.Sign(u, "access", -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Parse(tok); err == nil {
		t.Fatal("expired token should fail")
	}
}
