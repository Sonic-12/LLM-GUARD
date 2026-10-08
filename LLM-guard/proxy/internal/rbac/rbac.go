package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"llmguard/proxy/internal/middleware"
)

type RoleConfig struct {
	AllowedModels map[string]bool
	MaxChars      int
}

type Config struct {
	Enabled bool

	IssuerURL string
	JWKSURL   string

	ClientID string

	Roles map[string]RoleConfig

	DefaultModel string
	PremiumModel string
}

type BlockedError struct {
	StatusCode int
	Body       []byte
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked by rbac: status=%d", e.StatusCode)
}

func blocked(status int, reason string) error {
	payload, _ := json.Marshal(map[string]any{"allowed": false, "reason": reason})
	return &BlockedError{StatusCode: status, Body: payload}
}

type Hook struct {
	cfg  Config
	jwks *jwksCache
}

func New(cfg Config) *Hook {
	return &Hook{
		cfg:  cfg,
		jwks: newJWKSCache(cfg.JWKSURL, 10*time.Second),
	}
}

func (h *Hook) Name() string { return "rbac" }

const MetadataAuthHeader = "auth.header"

const MetadataRole = "rbac.role"

func (h *Hook) HandleRequest(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	if !h.cfg.Enabled {
		return body, nil
	}
	if h.cfg.IssuerURL == "" || h.cfg.JWKSURL == "" {
		return nil, blocked(503, "RBAC_MISCONFIGURED_NO_IDP")
	}

	const prefix = "Bearer "
	header, _ := rc.Metadata[MetadataAuthHeader].(string)
	if !strings.HasPrefix(header, prefix) {
		return nil, blocked(401, "MISSING_TOKEN")
	}
	token := strings.TrimPrefix(header, prefix)

	claims, err := h.jwks.VerifyRS256Token(ctx, token, h.cfg.IssuerURL, h.cfg.ClientID)
	if err != nil {
		return nil, blocked(401, rs256ErrorReason(err))
	}

	role, roleName, ok := h.resolveRole(claims.RealmAccess.Roles)
	if !ok {
		return nil, blocked(403, "UNKNOWN_ROLE")
	}

	rc.UserID = firstNonEmpty(claims.PreferredUsername, claims.Subject)
	rc.Metadata[MetadataRole] = roleName

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {

		return body, nil
	}

	if reqModel, ok := parsed["model"].(string); ok {
		tier := "default"
		if h.cfg.PremiumModel != "" && reqModel == h.cfg.PremiumModel {
			tier = "premium"
		}
		if !role.AllowedModels[tier] {
			return nil, blocked(403, "MODEL_NOT_ALLOWED_FOR_ROLE")
		}
	}

	rawMessages, _ := parsed["messages"].([]any)
	for _, raw := range rawMessages {
		msgMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if msgRole, _ := msgMap["role"].(string); msgRole != "user" {
			continue
		}
		content, _ := msgMap["content"].(string)
		if role.MaxChars > 0 && utf8.RuneCountInString(content) > role.MaxChars {
			return nil, blocked(403, "MAX_LENGTH_EXCEEDED_FOR_ROLE")
		}
	}

	return body, nil
}

func (h *Hook) resolveRole(realmRoles []string) (RoleConfig, string, bool) {
	for _, r := range realmRoles {
		if cfg, ok := h.cfg.Roles[r]; ok {
			return cfg, r, true
		}
	}
	return RoleConfig{}, "", false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func rs256ErrorReason(err error) string {
	switch err {
	case ErrRS256Expired:
		return "TOKEN_EXPIRED"
	case ErrRS256BadIssuer:
		return "INVALID_ISSUER"
	case ErrRS256BadAudience:
		return "INVALID_CLIENT"
	default:
		return "INVALID_TOKEN"
	}
}
