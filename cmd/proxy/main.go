package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Sonic-12/LLM-GUARD/pkg/client"
	"github.com/Sonic-12/LLM-GUARD/pkg/rules"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, statusCode int, message, errType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	resp := ErrorResponse{}
	resp.Error.Message = message
	resp.Error.Type = errType
	resp.Error.Code = statusCode

	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	upstreamURL := os.Getenv("UPSTREAM_LLM_URL")
	if upstreamURL == "" {
		upstreamURL = "http://127.0.0.1:11434" // Default local LLM fallback (e.g. Ollama)
	}

	firewallURL := os.Getenv("FIREWALL_SERVICE_URL")
	if firewallURL == "" {
		firewallURL = "http://127.0.0.1:5001"
	}

	firewallClient := client.NewFirewallClient(firewallURL)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error")
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error")
			return
		}

		var chatReq ChatCompletionRequest
		if err := json.Unmarshal(bodyBytes, &chatReq); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload", "invalid_request_error")
			return
		}

		// Extract user prompt and system prompt
		var systemPrompt string
		var userPrompt string
		var userMsgIndex = -1

		for i, msg := range chatReq.Messages {
			if strings.EqualFold(msg.Role, "system") {
				systemPrompt = msg.Content
			} else if strings.EqualFold(msg.Role, "user") {
				userPrompt = msg.Content
				userMsgIndex = i
			}
		}

		if userMsgIndex == -1 || strings.TrimSpace(userPrompt) == "" {
			writeJSONError(w, http.StatusBadRequest, "Missing user query content", "invalid_request_error")
			return
		}

		// 1. Run Guardrail Evaluation (Tiers 1-3)
		decision := rules.EvaluatePrompt(userPrompt, 4000, firewallClient)
		if !decision.Allowed {
			log.Printf("[BLOCK] Rejected prompt. Reason: %s | Score: %.2f", decision.Reason, decision.JailbreakScore)
			writeJSONError(w, decision.StatusCode, "Request blocked by LLM-GUARD: "+decision.Reason, "security_violation")
			return
		}

		// 2. Delimiter Sandboxing & System Enforcer (Tier 2 encapsulation)
		chatReq.Messages[userMsgIndex].Content = rules.EnforceSystemInstructions(systemPrompt, userPrompt)

		// 3. Forward to Upstream LLM
		modifiedBody, err := json.Marshal(chatReq)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to format upstream request", "internal_error")
			return
		}

		forwardReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL+"/v1/chat/completions", bytes.NewReader(modifiedBody))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to create upstream request", "internal_error")
			return
		}

		forwardReq.Header = r.Header.Clone()
		forwardReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(forwardReq)
		if err != nil {
			log.Printf("[ERROR] Upstream forwarding failed: %v", err)
			writeJSONError(w, http.StatusBadGateway, "Upstream LLM backend unavailable", "gateway_error")
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	log.Printf("[*] LLM-GUARD reverse proxy running on :%s (Upstream: %s)", port, upstreamURL)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
