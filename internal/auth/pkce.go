// pkce.go는 RFC 7636 PKCE (Proof Key for Code Exchange) 지원을 구현합니다.
// Device Authorization Grant Flow에서 인증 코드 가로채기 공격을 방지합니다.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEPair holds a code_verifier and its corresponding code_challenge.
type PKCEPair struct {
	CodeVerifier  string
	CodeChallenge string
	Method        string // always "S256"
}

// GeneratePKCE creates a new PKCE code_verifier (43-128 chars, URL-safe)
// and its SHA-256 code_challenge per RFC 7636.
func GeneratePKCE() (*PKCEPair, error) {
	// 32 random bytes -> base64url encoding produces 43 characters
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)

	// code_challenge = BASE64URL(SHA256(code_verifier))
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	return &PKCEPair{
		CodeVerifier:  verifier,
		CodeChallenge: challenge,
		Method:        "S256",
	}, nil
}
