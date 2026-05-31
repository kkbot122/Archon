package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kisna/archon/services/api-gateway/internal/middleware"
)

// buildTestCache creates a JWKSCache pointed at the given URL.
// Used for the simple header-format tests that never reach signature verification.
func buildTestCache(t *testing.T, url string) *middleware.JWKSCache {
	t.Helper()
	cache, err := middleware.NewJWKSCache(url)
	if err != nil {
		t.Skipf("cannot reach JWKS URL %s, skipping: %v", url, err)
	}
	return cache
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	// Use a dummy URL — the request is rejected before any key lookup
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	cache, err := middleware.NewJWKSCache(srv.URL)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/query", nil)
	rec := httptest.NewRecorder()

	handler := middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing token")
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	cache, err := middleware.NewJWKSCache(srv.URL)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Basic some-base64-string")
	rec := httptest.NewRecorder()

	handler := middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token format")
}

func TestAuthMiddleware_MalformedJWT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	cache, err := middleware.NewJWKSCache(srv.URL)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer notavalidjwt")
	rec := httptest.NewRecorder()

	handler := middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}