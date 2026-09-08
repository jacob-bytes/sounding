package auth

import (
	"testing"
	"time"
)

func TestIssueVerify(t *testing.T) {
	tok, err := Issue("secret", "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Verify("secret", tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Sub != "admin" {
		t.Fatalf("sub: %s", claims.Sub)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	tok, _ := Issue("secret", "admin", time.Hour)
	if _, err := Verify("other", tok); err == nil {
		t.Fatal("wrong secret should fail")
	}
}

func TestVerifyExpired(t *testing.T) {
	tok, _ := Issue("secret", "admin", -time.Hour)
	if _, err := Verify("secret", tok); err == nil {
		t.Fatal("expired token should fail")
	}
}
