// Command proxy starts the LLM-Guard reverse proxy.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"llmguard/proxy/internal/config"
	"llmguard/proxy/internal/jailbreak"
	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/proxy"
	"llmguard/proxy/internal/rules"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rulesCfg, err := rules.LoadConfig("rules.yaml")
	if err != nil {
		log.Fatalf("failed to load rules.yaml: %v", err)
	}

	chain := middleware.NewChain()
	chain.UsePre(rules.New(rulesCfg))
	chain.UsePre(jailbreak.New(jailbreak.Config{
		OllamaURL:      rulesCfg.Jailbreak.OllamaURL,
		Model:          rulesCfg.Jailbreak.Model,
		Threshold:      rulesCfg.Jailbreak.Threshold,
		SystemPrompt:   rulesCfg.Jailbreak.SystemPrompt,
		RequestTimeout: 45 * time.Second,
	}))
	chain.UsePre(middleware.Passthrough{})
	chain.UsePost(middleware.Passthrough{})

	srv := proxy.New(cfg, chain)

	log.Printf("LLM-Guard proxy listening on %s -> upstream[%s] %s",
		cfg.ListenAddr, cfg.Upstream.Type, cfg.Upstream.BaseURL)

	if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
		log.Fatalf("proxy server failed: %v", err)
	}
}
