package auth

import (
	"context"
	"net/http"
	"strings"
)

type TokenVerifier interface {
	VerifyToken(ctx context.Context, idToken string) (string, error)
}

type contextKey string

const firebaseUIDKey contextKey = "firebase_uid"

func UserIDFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(firebaseUIDKey).(string)
	return uid, ok
}

func Middleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			uid, err := verifier.VerifyToken(r.Context(), token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), firebaseUIDKey, uid)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
