package rbac

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwksCache struct {
	url      string
	client   *http.Client
	cacheTTL time.Duration

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func newJWKSCache(url string, timeout time.Duration) *jwksCache {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &jwksCache{
		url:      url,
		client:   &http.Client{Timeout: timeout},
		cacheTTL: 10 * time.Minute,
		keys:     map[string]*rsa.PublicKey{},
	}
}

func (c *jwksCache) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, ok := c.keys[kid]
	fresh := time.Since(c.fetchedAt) < c.cacheTTL
	c.mu.RUnlock()
	if ok && fresh {
		return key, nil
	}

	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok = c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("rbac: no JWKS key found for kid %q", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("rbac: fetching JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rbac: JWKS endpoint returned status %d", resp.StatusCode)
	}

	var doc jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("rbac: decoding JWKS: %w", err)
	}

	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pub, err := jwkToRSAPublicKey(k)
		if err != nil {
			continue // skip a malformed/unsupported key rather than fail the whole set
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("rbac: JWKS endpoint returned no usable RSA signing keys")
	}

	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return nil
}

func jwkToRSAPublicKey(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}
