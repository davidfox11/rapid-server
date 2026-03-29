package auth

import (
	"context"
	"fmt"
	"strings"
)

// DevTokenVerifier is a stub for local development.
// It accepts any token and extracts a user ID from it.
// Token format: "dev-<firebase_uid>" returns "<firebase_uid>".
// Any other non-empty token returns the token itself as the UID.
type DevTokenVerifier struct{}

// VerifyToken extracts a user ID from a dev token.
func (d *DevTokenVerifier) VerifyToken(_ context.Context, idToken string) (string, error) {
	if idToken == "" {
		return "", fmt.Errorf("empty token")
	}
	if uid, ok := strings.CutPrefix(idToken, "dev-"); ok {
		return uid, nil
	}
	return idToken, nil
}
