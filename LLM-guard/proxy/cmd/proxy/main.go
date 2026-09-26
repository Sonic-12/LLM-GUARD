package main

import (
	"flag"
	"log"
	"net/http"

	"llmguard/proxy/internal/config"
	"llmguard/proxy/internal/dlp"
	"llmguard/proxy/internal/hardening"
	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/outputguard"
	"llmguard/proxy/internal/proxy"
	"llmguard/proxy/internal/rbac"
	"llmguard/proxy/internal/rules"
)

func rbacRoles(in map[string]config.RoleConfig) map[string]rbac.RoleConfig {
	out := make(map[string]rbac.RoleConfig, len(in))
	for name, r := range in {
		out[name] = rbac.RoleConfig{AllowedModels: r.AllowedModels, MaxChars: r.MaxChars}
	}
	return out
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	chain := middleware.NewChain()
	rulesHook := rules.New(rules.Config{
		Enabled:         cfg.Rules.Enabled,
		MaxChars:        cfg.Rules.MaxChars,
		FirewallURL:     cfg.Rules.FirewallURL,
		DecisionLogPath: cfg.Rules.DecisionLogPath,
	})
	dlpHook := dlp.New(dlp.Config{
		BaseURL: cfg.DLP.BaseURL,
		Enabled: cfg.DLP.Enabled,
	})
	hardeningHook := hardening.New(hardening.Config{Enabled: cfg.Rules.HardenSystemPrompt})
	rbacHook := rbac.New(rbac.Config{
		Enabled:      cfg.RBAC.Enabled,
		IssuerURL:    cfg.RBAC.IssuerURL,
		JWKSURL:      cfg.RBAC.JWKSURL,
		ClientID:     cfg.RBAC.ClientID,
		Roles:        rbacRoles(cfg.RBAC.Roles),
		DefaultModel: cfg.Upstream.DefaultModel,
		PremiumModel: cfg.Upstream.PremiumModel,
	})
	outputGuardHook := outputguard.New(outputguard.Config{
		Enabled:     cfg.OutputGuard.Enabled,
		DLPBaseURL:  cfg.DLP.BaseURL,
		FlagLogPath: cfg.OutputGuard.FlagLogPath,
	})

	chain.UsePre(rbacHook)
	chain.UsePre(rulesHook)
	chain.UsePre(dlpHook)
	chain.UsePre(hardeningHook)
	chain.UsePre(middleware.Passthrough{})

	chain.UsePost(outputGuardHook)
	chain.UsePost(middleware.Passthrough{})
	chain.UsePost(dlpHook)

	srv := proxy.New(cfg, chain)

	log.Printf("LLM-Guard proxy listening on %s -> upstream[%s] %s",
		cfg.ListenAddr, cfg.Upstream.Type, cfg.Upstream.BaseURL)

	if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
		log.Fatalf("proxy server failed: %v", err)
	}
}
