package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr  string            `yaml:"listen_addr"`
	Upstream    UpstreamConfig    `yaml:"upstream"`
	DLP         DLPConfig         `yaml:"dlp"`
	Rules       RulesConfig       `yaml:"rules"`
	RBAC        RBACConfig        `yaml:"rbac"`
	OutputGuard OutputGuardConfig `yaml:"outputguard"`
	Telemetry   TelemetryConfig   `yaml:"telemetry"`
}

type RoleConfig struct {
	AllowedModels map[string]bool
	MaxChars      int
}

type RBACConfig struct {
	Enabled   bool   `yaml:"enabled"`
	IssuerURL string `yaml:"issuer_url"`
	JWKSURL   string `yaml:"jwks_url"`
	ClientID  string `yaml:"client_id"`
	Roles     map[string]RoleConfig
}

type TelemetryConfig struct {
	Enabled      bool   `yaml:"enabled"`
	LocalLogPath string `yaml:"local_log_path"`
	SidecarURL   string `yaml:"sidecar_url"`
}

type OutputGuardConfig struct {
	Enabled     bool   `yaml:"enabled"`
	FlagLogPath string `yaml:"flag_log_path"`

	TestMode bool `yaml:"test_mode"`
}

type UpstreamConfig struct {
	Type      string `yaml:"type"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`

	DefaultModel string `yaml:"default_model"` // used for every request unless PremiumModel is explicitly requested
	PremiumModel string `yaml:"premium_model"` // opt-in only; used only when client requests this exact model name

	PremiumTimeout time.Duration `yaml:"-"` // how long we wait on PremiumModel before falling back to DefaultModel

	DefaultNumPredict  int `yaml:"-"` // cap on DefaultModel's answer (direct path, not racing anything)
	PremiumNumPredict  int `yaml:"-"` // cap on PremiumModel's answer (shortens the race itself)
	FallbackNumPredict int `yaml:"-"` // cap when PremiumModel timed out and we fell back to DefaultModel
}

type DLPConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
}

type RulesConfig struct {
	Enabled            bool   `yaml:"enabled"`
	MaxChars           int    `yaml:"max_chars"`
	FirewallURL        string `yaml:"firewall_url"`
	DecisionLogPath    string `yaml:"decision_log_path"`
	HardenSystemPrompt bool   `yaml:"harden_system_prompt"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	defer f.Close()

	flat, err := parseYAML(f)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	cfg := Config{
		ListenAddr: flat["listen_addr"],
		Upstream: UpstreamConfig{
			Type:         flat["upstream.type"],
			BaseURL:      flat["upstream.base_url"],
			APIKeyEnv:    flat["upstream.api_key_env"],
			DefaultModel: flat["upstream.default_model"],
			PremiumModel: flat["upstream.premium_model"],
		},
		DLP: DLPConfig{
			Enabled: flat["dlp.enabled"] == "true",
			BaseURL: flat["dlp.base_url"],
		},
		Rules: RulesConfig{
			Enabled:            flat["rules.enabled"] == "true",
			FirewallURL:        flat["rules.firewall_url"],
			DecisionLogPath:    flat["rules.decision_log_path"],
			HardenSystemPrompt: flat["rules.harden_system_prompt"] == "true",
		},
		RBAC: RBACConfig{
			Enabled:   flat["rbac.enabled"] == "true",
			IssuerURL: flat["rbac.issuer_url"],
			JWKSURL:   flat["rbac.jwks_url"],
			ClientID:  flat["rbac.client_id"],
			Roles:     map[string]RoleConfig{},
		},
		OutputGuard: OutputGuardConfig{
			Enabled:     flat["outputguard.enabled"] == "true",
			FlagLogPath: flat["outputguard.flag_log_path"],
			TestMode:    flat["outputguard.test_mode"] == "true",
		},
		Telemetry: TelemetryConfig{
			Enabled:      flat["telemetry.enabled"] == "true",
			LocalLogPath: flat["telemetry.local_log_path"],
			SidecarURL:   flat["telemetry.sidecar_url"],
		},
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.Upstream.Type == "" {
		cfg.Upstream.Type = "mock"
	}

	cfg.Upstream.PremiumTimeout = 20 * time.Second
	if v := flat["upstream.premium_timeout_seconds"]; v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			cfg.Upstream.PremiumTimeout = time.Duration(sec) * time.Second
		}
	}

	cfg.Upstream.DefaultNumPredict = 500
	if v := flat["upstream.default_num_predict"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Upstream.DefaultNumPredict = n
		}
	}

	cfg.Upstream.PremiumNumPredict = 400
	if v := flat["upstream.premium_num_predict"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Upstream.PremiumNumPredict = n
		}
	}

	cfg.Upstream.FallbackNumPredict = 300
	if v := flat["upstream.fallback_num_predict"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Upstream.FallbackNumPredict = n
		}
	}

	cfg.Rules.MaxChars = 4000
	if v := flat["rules.max_chars"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Rules.MaxChars = n
		}
	}

	if err := parseRoles(flat, &cfg.RBAC); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &cfg, nil
}

func parseRoles(flat map[string]string, rbac *RBACConfig) error {
	const prefix = "rbac.role_"

	for key := range flat {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := strings.TrimPrefix(key, prefix)

		var roleName, field string
		switch {
		case strings.HasSuffix(rest, "_models"):
			roleName = strings.TrimSuffix(rest, "_models")
			field = "models"
		case strings.HasSuffix(rest, "_max_chars"):
			roleName = strings.TrimSuffix(rest, "_max_chars")
			field = "max_chars"
		default:
			return fmt.Errorf("unrecognized rbac role key %q (expected _models or _max_chars suffix)", key)
		}
		if roleName == "" {
			return fmt.Errorf("rbac role key %q has no role name", key)
		}

		role := rbac.Roles[roleName]
		switch field {
		case "models":
			role.AllowedModels = map[string]bool{}
			for _, tier := range strings.Split(flat[key], ",") {
				tier = strings.TrimSpace(tier)
				if tier != "" {
					role.AllowedModels[tier] = true
				}
			}
		case "max_chars":
			n, err := strconv.Atoi(flat[key])
			if err != nil {
				return fmt.Errorf("rbac role %q max_chars: %w", roleName, err)
			}
			role.MaxChars = n
		}
		rbac.Roles[roleName] = role
	}
	return nil
}

func parseYAML(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	section := ""

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		indented := strings.HasPrefix(line, "  ")
		trimmed := strings.TrimSpace(line)

		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := ""
		if len(parts) == 2 {
			val = strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
		}

		if !indented {
			if val == "" {
				section = key
				continue
			}
			section = ""
			out[key] = val
			continue
		}

		if section == "" {
			return nil, fmt.Errorf("indented key %q with no open section", key)
		}
		out[section+"."+key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
