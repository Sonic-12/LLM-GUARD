package rbac

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llmguard/proxy/internal/middleware"
)

const (
	testIssuer   = "http://localhost:8081/realms/llmguard"
	testClientID = "llmguard-proxy"
	testKid      = "test-key"
)

func newTestJWKSServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}

	n := base64.RawURLEncoding.EncodeToString(priv.PublicKey.N.Bytes())
	eBytes := big.NewInt(int64(priv.PublicKey.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	doc := jwksResponse{Keys: []jwk{{Kty: "RSA", Kid: testKid, Use: "sig", N: n, E: e}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(doc)
	}))
	return srv, priv
}

func signTestToken(t *testing.T, priv *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "kid": testKid}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func testRoles() map[string]RoleConfig {
	return map[string]RoleConfig{
		"admin":    {AllowedModels: map[string]bool{"default": true, "premium": true}, MaxChars: 4000},
		"employee": {AllowedModels: map[string]bool{"default": true, "premium": true}, MaxChars: 4000},
		"guest":    {AllowedModels: map[string]bool{"default": true}, MaxChars: 1000},
	}
}

func testConfig(jwksURL string) Config {
	return Config{
		Enabled:      true,
		IssuerURL:    testIssuer,
		JWKSURL:      jwksURL,
		ClientID:     testClientID,
		DefaultModel: "llama3.2:3b",
		PremiumModel: "llama3.1:8b",
		Roles:        testRoles(),
	}
}

func chatBody(model, content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": content}},
	})
	return body
}

func newRC(header string) *middleware.RequestContext {
	return &middleware.RequestContext{
		RequestID: "test",
		Metadata:  map[string]any{MetadataAuthHeader: header},
	}
}

func validClaims(role string, ttl time.Duration) map[string]any {
	return map[string]any{
		"sub":                "11111111-2222-3333-4444-555555555555",
		"preferred_username": "testuser",
		"iss":                testIssuer,
		"azp":                testClientID,
		"exp":                time.Now().Add(ttl).Unix(),
		"realm_access":       map[string]any{"roles": []string{role}},
	}
}

func TestHandleRequest_MissingToken(t *testing.T) {
	srv, _ := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	_, err := hook.HandleRequest(context.Background(), newRC(""), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_InvalidSignature(t *testing.T) {
	srv, _ := newTestJWKSServer(t)
	defer srv.Close()
	_, otherKey := newTestJWKSServer(t) // token signed with a DIFFERENT key than the JWKS serves
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, otherKey, validClaims("admin", time.Hour))
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_ExpiredToken(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, priv, validClaims("admin", -time.Hour))
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_WrongIssuer(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	claims := validClaims("admin", time.Hour)
	claims["iss"] = "http://attacker.example/realms/fake"
	tok := signTestToken(t, priv, claims)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_WrongClient(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	claims := validClaims("admin", time.Hour)
	claims["azp"] = "some-other-app"
	tok := signTestToken(t, priv, claims)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_UnknownRole(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, priv, validClaims("superadmin", time.Hour))
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_GuestBlockedFromPremiumModel(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, priv, validClaims("guest", time.Hour))
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.1:8b", "hi"))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_GuestOverRoleCharLimit(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, priv, validClaims("guest", time.Hour))
	longPrompt := make([]byte, 1500)
	for i := range longPrompt {
		longPrompt[i] = 'a'
	}
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", string(longPrompt)))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_AdminAllowedThrough(t *testing.T) {
	srv, priv := newTestJWKSServer(t)
	defer srv.Close()
	hook := New(testConfig(srv.URL))
	tok := signTestToken(t, priv, validClaims("admin", time.Hour))
	rc := newRC("Bearer " + tok)
	out, err := hook.HandleRequest(context.Background(), rc, chatBody("llama3.1:8b", "hi"))
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
	if rc.UserID != "testuser" {
		t.Errorf("expected rc.UserID to be set from preferred_username, got %q", rc.UserID)
	}
	if rc.Metadata[MetadataRole] != "admin" {
		t.Errorf("expected rc.Metadata role to be admin, got %v", rc.Metadata[MetadataRole])
	}
}

func TestHandleRequest_DisabledPassesThrough(t *testing.T) {
	cfg := testConfig("http://unused.invalid")
	cfg.Enabled = false
	hook := New(cfg)
	out, err := hook.HandleRequest(context.Background(), newRC(""), chatBody("llama3.2:3b", "hi"))
	if err != nil {
		t.Fatalf("expected disabled hook to pass through, got error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func TestHandleRequest_MissingIdPConfigFailsClosed(t *testing.T) {
	cfg := testConfig("")
	cfg.IssuerURL = ""
	hook := New(cfg)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer whatever"), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 503)
}

func assertBlockedStatus(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected request to be blocked, got nil error")
	}
	blocked, ok := err.(*BlockedError)
	if !ok {
		t.Fatalf("expected *BlockedError, got %T: %v", err, err)
	}
	if blocked.StatusCode != want {
		t.Errorf("expected status %d, got %d", want, blocked.StatusCode)
	}
}
