package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"llmguard/proxy/internal/rbac"
)

func main() {
	sub := flag.String("sub", "test-user", "subject (user id) to embed in the token")
	role := flag.String("role", "employee", "role to embed in the token")
	ttl := flag.Duration("ttl", time.Hour, "token lifetime")
	secretEnv := flag.String("secret-env", "RBAC_JWT_SECRET", "env var holding the HMAC secret")
	flag.Parse()

	secret := os.Getenv(*secretEnv)
	if secret == "" {
		log.Fatalf("gentoken: env var %s is not set", *secretEnv)
	}

	tok, err := rbac.NewToken(secret, *sub, *role, *ttl)
	if err != nil {
		log.Fatalf("gentoken: %v", err)
	}
	fmt.Println(tok)
}
