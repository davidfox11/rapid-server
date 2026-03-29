package auth

import (
	"context"
	"fmt"
	"strings"
)

type DevTokenVerifier struct{}

func (d *DevTokenVerifier) VerifyToken(_ context.Context, idToken string) (string, error) {
	if idToken == "" {
		return "", fmt.Errorf("empty token")
	}
	if uid, ok := strings.CutPrefix(idToken, "dev-"); ok {
		return uid, nil
	}
	return idToken, nil
}
