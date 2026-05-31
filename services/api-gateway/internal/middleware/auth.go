package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type contextKey string

const UserContextKey contextKey = "user_id"

type JWKSCache struct {
	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
	url  string
}

type JWTClaims struct {
	Sub string   `json:"sub"`
	Iss string   `json:"iss"`
	Aud audClaim `json:"aud"`
	Exp int64    `json:"exp"`
}

type audClaim []string

func (a *audClaim) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*a = audClaim{s}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	*a = audClaim(arr)
	return nil
}

type jwksResponse struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func NewJWKSCache(jwksURL string) (*JWKSCache, error) {
	c := &JWKSCache{
		keys: make(map[string]*rsa.PublicKey),
		url:  jwksURL,
	}
	if err := c.fetch(); err != nil {
		return nil, fmt.Errorf("initial JWKS fetch failed: %w", err)
	}
	return c, nil
}

func (c *JWKSCache) fetch() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(c.url)
	if err != nil {
		return fmt.Errorf("fetching JWKS from %s: %w", c.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS response: %w", err)
	}

	fresh := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			return fmt.Errorf("parsing key kid=%s: %w", k.Kid, err)
		}
		fresh[k.Kid] = pub
	}

	c.mu.Lock()
	c.keys = fresh
	c.mu.Unlock()
	return nil
}

func (c *JWKSCache) getKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	k, ok := c.keys[kid]
	c.mu.RUnlock()
	if ok {
		return k, nil
	}

	slog.Info("JWKS kid miss, rotating cache", "kid", kid)
	if err := c.fetch(); err != nil {
		return nil, fmt.Errorf("JWKS rotation failed: %w", err)
	}

	c.mu.RLock()
	k, ok = c.keys[kid]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown kid: %s", kid)
	}
	return k, nil
}

func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func InitAuth() (*JWKSCache, error) {
	url := os.Getenv("ARCHON_JWKS_URL")
	if url == "" {
		return nil, fmt.Errorf("ARCHON_JWKS_URL is not set")
	}
	return NewJWKSCache(url)
}

func Auth(cache *JWKSCache) func(http.Handler) http.Handler {
	expectedIssuer := os.Getenv("ARCHON_JWT_ISSUER")
	expectedAudience := os.Getenv("ARCHON_JWT_AUDIENCE")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Unauthorized: invalid token format", http.StatusUnauthorized)
				return
			}

			claims, err := verifyToken(parts[1], cache, expectedIssuer, expectedAudience)
			if err != nil {
				slog.Warn("JWT verification failed", "error", err)
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims.Sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(UserContextKey).(string)
	return v, ok && v != ""
}