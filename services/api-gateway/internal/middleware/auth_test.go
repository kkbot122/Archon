package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/api-gateway/internal/middleware"
)

func TestAuthMiddleware_MissingToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/query", nil)
	rec := httptest.NewRecorder()

	// A dummy handler that represents our GraphQL endpoint
	mockGraphQLHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Auth(mockGraphQLHandler)
	handler.ServeHTTP(rec, req)

	// It should block the request
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing token")
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Basic some-base64-string")
	rec := httptest.NewRecorder()

	mockGraphQLHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Auth(mockGraphQLHandler)
	handler.ServeHTTP(rec, req)

	// It should block the request
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token format")
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer valid-mock-token")
	rec := httptest.NewRecorder()

	var extractedUserID string
	mockGraphQLHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the user ID from the context exactly as the resolvers will
		val := r.Context().Value(middleware.UserContextKey)
		if val != nil {
			extractedUserID = val.(string)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Auth(mockGraphQLHandler)
	handler.ServeHTTP(rec, req)

	// It should allow the request and properly inject the User ID
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", extractedUserID)
}