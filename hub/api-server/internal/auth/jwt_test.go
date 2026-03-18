package auth_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/idiey/api-server/internal/auth"
)

func TestIssueAndValidate(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")
	tok, err := auth.Issue(42, "alice")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	claims, err := auth.Validate(req)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestValidate_MissingCookie(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")
	if _, err := auth.Validate(httptest.NewRequest(http.MethodGet, "/", nil)); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	defer os.Unsetenv("JWT_SECRET")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "not.a.jwt"})
	if _, err := auth.Validate(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestIssue_NoSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	if _, err := auth.Issue(1, "user"); err == nil {
		t.Fatal("expected error")
	}
}
