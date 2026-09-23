package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"llmguard/proxy/internal/middleware"
)

type RoleConfig struct {
	AllowedModels map[string]bool
	MaxChars      int
}

type Config struct {
	Enabled bool
	Secret  string
	Roles   map[string]RoleConfig

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
	cfg Config
}

func New(cfg Config) *Hook {
	return &Hook{cfg: cfg}
}

func (h *Hook) Name() string { return "rbac" }

const MetadataAuthHeader = "auth.header"

const MetadataRole = "rbac.role"

func (h *Hook) HandleRequest(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	if !h.cfg.Enabled {
		return body, nil
	}
	if h.cfg.Secret == "" {
		return nil, blocked(503, "RBAC_MISCONFIGURED_NO_SECRET")
	}

	const prefix = "Bearer "
	header, _ := rc.Metadata[MetadataAuthHeader].(string)
	if !strings.HasPrefix(header, prefix) {
		return nil, blocked(401, "MISSING_TOKEN")
	}
	token := strings.TrimPrefix(header, prefix)

	claims, err := VerifyToken(h.cfg.Secret, token)
	if err != nil {
		return nil, blocked(401, "INVALID_TOKEN")
	}

	role, ok := h.cfg.Roles[claims.Role]
	if !ok {
		return nil, blocked(403, "UNKNOWN_ROLE")
	}

	rc.UserID = claims.Subject
	rc.Metadata[MetadataRole] = claims.Role

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
