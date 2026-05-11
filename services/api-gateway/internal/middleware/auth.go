package middleware

import (
	"context"
	"net/http"
	"strings"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

const UserContextKey contextKey = "user_id"

// Auth middleware validates the Authorization header before hitting GraphQL.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Unauthorized: invalid token format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		
		// Note: In a production environment, you would verify a JWT signature here.
		// For this implementation, we validate a mock token and inject the dummy user ID.
		if token != "valid-mock-token" {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Inject the authenticated User ID into the request context
		ctx := context.WithValue(r.Context(), UserContextKey, "11111111-1111-1111-1111-111111111111")
		
		// Pass the new context to the next handler (the GraphQL server)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}