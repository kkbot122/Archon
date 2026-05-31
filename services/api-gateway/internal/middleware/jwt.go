package middleware

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// verifyToken parses and validates a raw JWT string.
// Validates: structure, algorithm (RS256 only), signature, expiry, issuer, audience.
func verifyToken(
	rawToken string,
	cache *JWKSCache,
	expectedIssuer string,
	expectedAudience string,
) (*JWTClaims, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed token")
	}

	// Decode header to get kid and alg
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("malformed token header")
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("malformed token header")
	}

	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Fetch the public key
	pubKey, err := cache.getKey(header.Kid)
	if err != nil {
		return nil, err
	}

	// Verify signature: RS256 = RSASSA-PKCS1-v1_5 + SHA-256
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("malformed token signature")
	}

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sig); err != nil {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("malformed token claims")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("malformed token claims")
	}

	// Validate expiry
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	// Validate issuer (skip if not configured — allows test environments)
	if expectedIssuer != "" && claims.Iss != expectedIssuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Validate audience (skip if not configured)
	if expectedAudience != "" {
		found := false
		for _, a := range claims.Aud {
			if a == expectedAudience {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid audience")
		}
	}

	if claims.Sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	return &claims, nil
}

// SignTestToken signs a JWT with the given RSA key — for use in tests only.
func SignTestToken(key *rsa.PrivateKey, kid, sub, issuer, audience string, exp time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid}
	hJSON, _ := json.Marshal(header)

	claims := map[string]any{
		"sub": sub,
		"iss": issuer,
		"aud": audience,
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	}
	cJSON, _ := json.Marshal(claims)

	signingInput := base64.RawURLEncoding.EncodeToString(hJSON) + "." +
		base64.RawURLEncoding.EncodeToString(cJSON)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}