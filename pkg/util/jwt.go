package util

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrNotAJWT reports a token that does not have a JWT's shape at all, as
// opposed to one whose payload did not unmarshal into the caller's claims.
var ErrNotAJWT = errors.New("this does not look like a JWT")

// DecodeJWTClaims reads a JWT's payload without verifying its signature, for
// callers that only need to inspect a claim; verifying stays with the token's
// issuer.
func DecodeJWTClaims(token string, into any) error {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return ErrNotAJWT
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ErrNotAJWT
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return fmt.Errorf("could not read the token's claims: %w", err)
	}
	return nil
}
