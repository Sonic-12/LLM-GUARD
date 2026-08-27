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
	ListenAddr string         `yaml:"listen_addr"`
	Upstream   UpstreamConfig `yaml:"upstream"`
	DLP        DLPConfig      `yaml:"dlp"`
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

	return &cfg, nil
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
