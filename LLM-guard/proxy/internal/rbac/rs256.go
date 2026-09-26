package rbac

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type IDTokenClaims struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Issuer            string `json:"iss"`
	AuthorizedParty   string `json:"azp"`
	Exp               int64  `json:"exp"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

var (
	ErrRS256InvalidToken = errors.New("rbac: invalid token")
	ErrRS256Expired      = errors.New("rbac: token expired")
	ErrRS256BadIssuer    = errors.New("rbac: unexpected token issuer")
	ErrRS256BadAudience  = errors.New("rbac: token not authorized for this client")
)

type rs256Header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

func (jc *jwksCache) VerifyRS256Token(ctx context.Context, token, issuer, clientID string) (*IDTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrRS256InvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrRS256InvalidToken
	}
	var header rs256Header
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, ErrRS256InvalidToken
	}
	if header.Alg != "RS256" {
		return nil, ErrRS256InvalidToken
	}

	pubKey, err := jc.publicKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrRS256InvalidToken
	}
	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], sig); err != nil {
		return nil, ErrRS256InvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrRS256InvalidToken
	}
	var claims IDTokenClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrRS256InvalidToken
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrRS256Expired
	}
	if issuer != "" && claims.Issuer != issuer {
		return nil, ErrRS256BadIssuer
	}
	if clientID != "" && claims.AuthorizedParty != clientID {
		return nil, ErrRS256BadAudience
	}

	return &claims, nil
}
