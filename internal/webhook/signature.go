package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMissingSignature  = errors.New("missing X-Hub-Signature-256 header")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrSignatureMismatch = errors.New("signature mismatch")
)

func ValidateSignature(payload []byte, signature string, secret []byte) error {
	if signature == "" {
		return ErrMissingSignature
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return ErrInvalidSignature
	}

	signatureHex := strings.TrimPrefix(signature, "sha256=")
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)

	if !hmac.Equal(expected, signatureBytes) {
		// Debug: log expected vs received
		fmt.Printf("DEBUG: expected=%x received=%x\n", expected, signatureBytes)
		return ErrSignatureMismatch
	}

	return nil
}
