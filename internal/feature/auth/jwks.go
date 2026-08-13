package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksPath       = "/auth/v1/.well-known/jwks.json"
	jwksTTL        = 10 * time.Minute
	jwksEmptyTTL   = 30 * time.Second
	jwksMaxBody    = 1 << 20
	jwtParseLeeway = 30 * time.Second
)

var jwksAlgs = []string{"ES256", "ES384", "ES512", "RS256", "RS384", "RS512", "EdDSA"}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (a *Auth) keyForToken(ctx context.Context, token *jwt.Token) (any, error) {
	if a.config.Keyfunc != nil {
		return a.config.Keyfunc(token)
	}
	kid, _ := token.Header["kid"].(string)
	key, err := a.lookupJWK(ctx, kid, false)
	if err != nil {
		return nil, err
	}
	if key != nil {
		return key, nil
	}
	key, err = a.lookupJWK(ctx, kid, true)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("unknown jwt kid %q", kid)
	}
	return key, nil
}

func (a *Auth) lookupJWK(ctx context.Context, kid string, force bool) (any, error) {
	a.jwksMu.Lock()
	defer a.jwksMu.Unlock()
	ttl := jwksTTL
	if len(a.jwksKeys) == 0 {
		ttl = jwksEmptyTTL
	}
	stale := a.jwksAt.IsZero() || time.Since(a.jwksAt) > ttl
	if force || stale {
		keys, err := a.fetchJWKS(ctx)
		if err != nil {
			if len(a.jwksKeys) == 0 {
				return nil, err
			}
		} else {
			a.jwksKeys = keys
			a.jwksAt = time.Now()
		}
	}
	if kid == "" && len(a.jwksKeys) == 1 {
		for _, key := range a.jwksKeys {
			return key, nil
		}
	}
	if key, ok := a.jwksKeys[kid]; ok {
		return key, nil
	}
	return nil, nil
}

func (a *Auth) fetchJWKS(ctx context.Context) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.config.SupabaseURL+jwksPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", a.config.PublishableKey)
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, jwksMaxBody))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("jwks: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	keys := make(map[string]any, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.publicKey()
		if err != nil {
			return nil, err
		}
		kid := k.Kid
		if kid == "" {
			kid = "_"
		}
		keys[kid] = pub
	}
	return keys, nil
}

func (k jwk) publicKey() (any, error) {
	switch strings.ToUpper(k.Kty) {
	case "EC":
		return k.ecPublicKey()
	case "RSA":
		return k.rsaPublicKey()
	case "OKP":
		return k.okpPublicKey()
	default:
		return nil, fmt.Errorf("jwks: unsupported kty %q", k.Kty)
	}
}

func (k jwk) ecPublicKey() (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("jwks: unsupported crv %q", k.Crv)
	}
	x, err := parseJWKInt(k.X)
	if err != nil {
		return nil, err
	}
	y, err := parseJWKInt(k.Y)
	if err != nil {
		return nil, err
	}
	if !curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("jwks: point not on curve")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

func (k jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	n, err := parseJWKInt(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := b64(k.E)
	if err != nil {
		return nil, err
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() {
		return nil, fmt.Errorf("jwks: rsa exponent overflow")
	}
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func (k jwk) okpPublicKey() (ed25519.PublicKey, error) {
	if k.Crv != "Ed25519" {
		return nil, fmt.Errorf("jwks: unsupported okp crv %q", k.Crv)
	}
	x, err := b64(k.X)
	if err != nil {
		return nil, err
	}
	if len(x) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("jwks: invalid ed25519 key")
	}
	return ed25519.PublicKey(x), nil
}

func parseJWKInt(s string) (*big.Int, error) {
	raw, err := b64(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(raw), nil
}

func b64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func parseJWT(raw string, keyfunc jwt.Keyfunc) (supabaseClaims, error) {
	token, err := jwt.ParseWithClaims(raw, &supabaseClaims{}, keyfunc,
		jwt.WithLeeway(jwtParseLeeway),
		jwt.WithValidMethods(jwksAlgs),
	)
	if err != nil {
		return supabaseClaims{}, err
	}
	claims, ok := token.Claims.(*supabaseClaims)
	if !ok || !token.Valid {
		return supabaseClaims{}, fmt.Errorf("invalid token")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return supabaseClaims{}, fmt.Errorf("missing sub")
	}
	return *claims, nil
}
