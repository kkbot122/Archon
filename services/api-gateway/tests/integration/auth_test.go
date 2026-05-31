package integration_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kisna/archon/services/api-gateway/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKeys holds a keypair and the kid used to sign test tokens.
type testKeys struct {
	priv *rsa.PrivateKey
	kid  string
}

func generateTestKeys(t *testing.T) *testKeys {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &testKeys{priv: priv, kid: "test-key-1"}
}

// startJWKSServer spins up an httptest server that serves a real JWKS.
// Calling rotate() replaces the key and updates the served JWKS.
func startJWKSServer(t *testing.T, initial *testKeys) (serverURL string, rotate func(*testKeys)) {
	t.Helper()

	var mu sync.RWMutex
	current := initial

	jwksHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		k := current
		mu.RUnlock()

		pub := k.priv.Public().(*rsa.PublicKey)
		nBytes := pub.N.Bytes()
		eBytes := big.NewInt(int64(pub.E)).Bytes()

		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": k.kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(nBytes),
					"e":   base64.RawURLEncoding.EncodeToString(eBytes),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(jwksHandler)
	t.Cleanup(srv.Close)

	rotateFn := func(newKeys *testKeys) {
		mu.Lock()
		current = newKeys
		mu.Unlock()
	}

	return srv.URL, rotateFn
}

func buildCache(t *testing.T, jwksURL string) *middleware.JWKSCache {
	t.Helper()
	// Temporarily set env var so NewJWKSCache works
	os.Setenv("ARCHON_JWKS_URL", jwksURL)
	cache, err := middleware.NewJWKSCache(jwksURL)
	require.NoError(t, err)
	return cache
}

// helper: sign a valid token expiring in 1 minute
func validToken(t *testing.T, keys *testKeys, sub string) string {
	t.Helper()
	tok, err := middleware.SignTestToken(
		keys.priv, keys.kid, sub,
		"https://test.issuer", "archon",
		time.Now().Add(time.Minute),
	)
	require.NoError(t, err)
	return tok
}

// ---------- unit-level middleware tests ----------

func TestJWKSMiddleware_ValidToken(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)

	os.Setenv("ARCHON_JWT_ISSUER", "https://test.issuer")
	os.Setenv("ARCHON_JWT_AUDIENCE", "archon")

	var capturedSub string
	handler := middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSub, _ = middleware.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+validToken(t, keys, "user-abc"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-abc", capturedSub)
}

func TestJWKSMiddleware_ExpiredToken(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)

	tok, _ := middleware.SignTestToken(
		keys.priv, keys.kid, "user-abc",
		"https://test.issuer", "archon",
		time.Now().Add(-time.Minute), // expired
	)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "token expired")
}

func TestJWKSMiddleware_WrongIssuer(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)
	os.Setenv("ARCHON_JWT_ISSUER", "https://expected.issuer")

	tok, _ := middleware.SignTestToken(
		keys.priv, keys.kid, "user-abc",
		"https://wrong.issuer", "archon",
		time.Now().Add(time.Minute),
	)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid issuer")
}

func TestJWKSMiddleware_WrongAudience(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)
	os.Setenv("ARCHON_JWT_ISSUER", "https://test.issuer")
	os.Setenv("ARCHON_JWT_AUDIENCE", "archon")

	tok, _ := middleware.SignTestToken(
		keys.priv, keys.kid, "user-abc",
		"https://test.issuer", "wrong-audience",
		time.Now().Add(time.Minute),
	)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid audience")
}

func TestJWKSMiddleware_TamperedSignature(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)

	tok := validToken(t, keys, "user-abc")
	// flip last byte of signature
	tampered := tok[:len(tok)-2] + "XX"

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tampered)
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJWKSMiddleware_MissingHeader(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)

	req := httptest.NewRequest("GET", "/query", nil)
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing token")
}

func TestJWKSMiddleware_MalformedBearerFormat(t *testing.T) {
	keys := generateTestKeys(t)
	jwksURL, _ := startJWKSServer(t, keys)
	cache := buildCache(t, jwksURL)

	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Token abc123") // wrong scheme
	rec := httptest.NewRecorder()
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token format")
}

func TestJWKSMiddleware_KidMissTriggersRotation(t *testing.T) {
	// Start server with key1
	key1 := generateTestKeys(t)
	key1.kid = "key-v1"
	jwksURL, rotateFn := startJWKSServer(t, key1)
	cache := buildCache(t, jwksURL)
	os.Setenv("ARCHON_JWT_ISSUER", "https://test.issuer")
	os.Setenv("ARCHON_JWT_AUDIENCE", "archon")

	// Rotate to key2 at the server
	key2 := generateTestKeys(t)
	key2.kid = "key-v2"
	rotateFn(key2)

	// Token signed with key2 (kid not in cache yet) → should trigger rotation → 200
	tok := validToken(t, key2, "user-xyz")
	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	var capturedSub string
	middleware.Auth(cache)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSub, _ = middleware.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-xyz", capturedSub)
}