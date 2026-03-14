// Package middleware provides HTTP middleware for the bookshelf API.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "userID"

// JWTClaims are the custom claims embedded in every issued token.
type JWTClaims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// ErrUnauthorized is returned by GetRequiredUserID when no authenticated user
// is present in the context.
var ErrUnauthorized = errors.New("authentication required")

// SetAuth returns a middleware that parses the Bearer JWT from the
// Authorization header and stores the user ID in the request context when
// valid. Requests with a missing or invalid token are not rejected — routes
// that require authentication should call GetRequiredUserID.
func SetAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header != "" && strings.HasPrefix(header, "Bearer ") {
				tokenStr := strings.TrimPrefix(header, "Bearer ")
				claims := &JWTClaims{}
				token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(secret), nil
				})
				if err == nil && token.Valid {
					ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID retrieves the authenticated user's ID from the context.
// Returns 0 if no valid JWT was present on the request.
func GetUserID(ctx context.Context) uint {
	v, _ := ctx.Value(userIDKey).(uint)
	return v
}

// GetRequiredUserID returns the authenticated user ID from ctx.
// Returns 0 and ErrUnauthorized if no authenticated user is present.
func GetRequiredUserID(ctx context.Context) (uint, error) {
	v, _ := ctx.Value(userIDKey).(uint)
	if v == 0 {
		return 0, ErrUnauthorized
	}
	return v, nil
}
