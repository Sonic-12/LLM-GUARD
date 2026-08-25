package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
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
			Type:      flat["upstream.type"],
			BaseURL:   flat["upstream.base_url"],
			APIKeyEnv: flat["upstream.api_key_env"],
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

	return &cfg, nil
}
func parseYAML(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	section := ""

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// Strip comments.
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