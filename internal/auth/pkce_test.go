package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"
)

func TestGeneratePKCE_Length(t *testing.T) {
	pair, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE returned error: %v", err)
	}

	// RFC 7636 Section 4.1: code_verifier must be 43-128 characters
	vLen := len(pair.CodeVerifier)
	if vLen < 43 || vLen > 128 {
		t.Errorf("code_verifier length %d not in range [43, 128]", vLen)
	}

	// code_challenge is SHA256 base64url encoded = 43 characters
	cLen := len(pair.CodeChallenge)
	if cLen < 43 || cLen > 128 {
		t.Errorf("code_challenge length %d not in range [43, 128]", cLen)
	}
}

func TestGeneratePKCE_URLSafe(t *testing.T) {
	pair, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE returned error: %v", err)
	}

	// RFC 7636: code_verifier uses unreserved characters [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
	// base64url uses [A-Za-z0-9_-] which is a subset of unreserved characters
	urlSafe := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

	if !urlSafe.MatchString(pair.CodeVerifier) {
		t.Errorf("code_verifier contains invalid characters: %s", pair.CodeVerifier)
	}

	if !urlSafe.MatchString(pair.CodeChallenge) {
		t.Errorf("code_challenge contains invalid characters: %s", pair.CodeChallenge)
	}
}

func TestGeneratePKCE_ChallengeVerification(t *testing.T) {
	pair, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE returned error: %v", err)
	}

	// Verify: code_challenge == BASE64URL(SHA256(code_verifier))
	h := sha256.Sum256([]byte(pair.CodeVerifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	if pair.CodeChallenge != expected {
		t.Errorf("code_challenge mismatch:\n  got:  %s\n  want: %s", pair.CodeChallenge, expected)
	}
}

func TestGeneratePKCE_MethodIsS256(t *testing.T) {
	pair, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE returned error: %v", err)
	}

	if pair.Method != "S256" {
		t.Errorf("expected method 'S256', got '%s'", pair.Method)
	}
}

func TestGeneratePKCE_Uniqueness(t *testing.T) {
	pair1, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE (1) returned error: %v", err)
	}

	pair2, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE (2) returned error: %v", err)
	}

	if pair1.CodeVerifier == pair2.CodeVerifier {
		t.Error("two consecutive calls produced identical code_verifiers")
	}

	if pair1.CodeChallenge == pair2.CodeChallenge {
		t.Error("two consecutive calls produced identical code_challenges")
	}
}
