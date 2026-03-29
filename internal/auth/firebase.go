package auth

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
)

// FirebaseAuth wraps the Firebase Admin SDK for token verification.
type FirebaseAuth struct {
	client *firebaseauth.Client
}

// NewFirebaseAuth initializes Firebase Admin SDK.
// Uses GOOGLE_APPLICATION_CREDENTIALS or Application Default Credentials.
func NewFirebaseAuth(ctx context.Context) (*FirebaseAuth, error) {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing firebase app: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initializing firebase auth client: %w", err)
	}

	return &FirebaseAuth{client: client}, nil
}

// VerifyToken validates a Firebase ID token and returns the Firebase UID.
func (fa *FirebaseAuth) VerifyToken(ctx context.Context, idToken string) (string, error) {
	token, err := fa.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return "", fmt.Errorf("verifying firebase token: %w", err)
	}
	return token.UID, nil
}
