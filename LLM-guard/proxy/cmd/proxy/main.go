// Command proxy starts the LLM-Guard reverse proxy.
package main

import (
	"flag"
	"log"
	"net/http"

	"llmguard/proxy/internal/config"
	"llmguard/proxy/internal/dlp"
	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	chain := middleware.NewChain()
	dlpHook := dlp.New(dlp.Config{
		BaseURL: cfg.DLP.BaseURL,
		Enabled: cfg.DLP.Enabled,
	})
	chain.UsePre(dlpHook)
	chain.UsePre(middleware.Passthrough{})
	chain.UsePost(middleware.Passthrough{})
	chain.UsePost(dlpHook)

	srv := proxy.New(cfg, chain)

	log.Printf("LLM-Guard proxy listening on %s -> upstream[%s] %s",
		cfg.ListenAddr, cfg.Upstream.Type, cfg.Upstream.BaseURL)

	if err := http.ListenAndServe(cfg.ListenAddr, srv); err != nil {
		log.Fatalf("proxy server failed: %v", err)
	}
}
