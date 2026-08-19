package auth

import "testing"

func TestHashPasswordRoundTrips(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := VerifyPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := VerifyPassword(hash, "wrong password"); err == nil {
		t.Error("wrong password accepted")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("identical passwords produced identical hashes; salt is not being applied")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, bad := range []string{
		"", "not-a-hash", "pbkdf2-sha256$abc$c2FsdA$aGFzaA",
		"bcrypt$1$c2FsdA$aGFzaA", "pbkdf2-sha256$1000$!!!$aGFzaA",
		"pbkdf2-sha256$1000$c2FsdA", "pbkdf2-sha256$-1$c2FsdA$aGFzaA",
	} {
		if err := VerifyPassword(bad, "anything"); err == nil {
			t.Errorf("malformed hash %q was accepted", bad)
		}
	}
}

func TestSessionTokenIsNotStoredInPlaintext(t *testing.T) {
	token, digest, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	if token == "" || digest == "" {
		t.Fatal("empty token or digest")
	}
	if token == digest {
		t.Error("digest equals the token; the stored value must not be the secret")
	}
	if hashToken(token) != digest {
		t.Error("hashToken is not reproducible for the same token")
	}

	other, _, _ := newSessionToken()
	if other == token {
		t.Error("session tokens are not unique")
	}
}
