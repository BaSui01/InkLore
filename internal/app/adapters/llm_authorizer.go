package adapters

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"os"
	"strings"

	llmd "InkLore/internal/llm/domain"
	llmi "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/logger"
)

// LLMAuthorizer keeps the LLM admin audience and role policy at the app boundary.
type LLMAuthorizer struct{ Service *identity.Service }

func (a LLMAuthorizer) Authorize(ctx context.Context, p identity.Principal, audience string, roles ...string) (identity.Principal, error) {
	if a.Service == nil || p.Audience != audience {
		return identity.Principal{}, errors.New("llm authorization unavailable")
	}
	return a.Service.Authorize(ctx, p, roles...)
}

// NewLLMSecrets creates a fail-closed deployment adapter. Bindings and endpoint
// allowlists are supplied by deployment; absent mappings reject key/channel writes.
func NewLLMSecrets() (*llmi.Secrets, error) {
	endpoints := map[string][]string{
		"deepseek":  {"https://api.deepseek.com", "https://api.deepseek.com/v1"},
		"openai":    {"https://api.openai.com", "https://api.openai.com/v1"},
		"anthropic": {"https://api.anthropic.com", "https://api.anthropic.com/v1"},
		"gemini": {
			"https://generativelanguage.googleapis.com",
			"https://generativelanguage.googleapis.com/v1beta",
			"https://generativelanguage.googleapis.com/v1",
		},
		"ollama": {"http://localhost:11434", "http://127.0.0.1:11434"},
	}

	if envExtra := strings.TrimSpace(os.Getenv("LLM_ALLOWED_ENDPOINTS")); envExtra != "" {
		for _, raw := range strings.Split(envExtra, ",") {
			ep := strings.TrimRight(strings.TrimSpace(raw), "/")
			if ep != "" {
				if u, err := url.Parse(ep); err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") {
					for p := range endpoints {
						endpoints[p] = append(endpoints[p], ep)
					}
				}
			}
		}
	}

	pepperBytes := []byte(os.Getenv("LLM_SECRET_PEPPER"))
	if len(pepperBytes) < 32 {
		pepperBytes = nil
		if encryptionKey := os.Getenv("ENCRYPTION_KEY"); len(encryptionKey) >= 32 {
			h := sha256.Sum256([]byte("inklore_llm_secret_pepper:" + encryptionKey))
			pepperBytes = h[:]
		}
	}

	bindings, err := llmi.EnvironmentSecretBindings()
	if err != nil {
		// Keep catalog configuration available, but leave every credential
		// unauthorized. Never log the malformed configuration or its contents.
		logger.Warn("LLM credential bindings are invalid; credential access is disabled")
		bindings = nil
	}
	return llmi.NewSecrets(llmi.SecretOptions{
		Bindings:  bindings,
		Pepper:    pepperBytes,
		Endpoints: endpoints,
		Resolve: func(ctx context.Context, b llmi.SecretBinding) ([]byte, error) {
			ref := b.Ref
			if llmd.ValidateSecretRef(ref) != nil {
				return nil, llmd.ErrSecret
			}
			if ref.Kind == "env" {
				if val := os.Getenv(ref.Locator); strings.TrimSpace(val) != "" {
					return []byte(strings.TrimSpace(val)), nil
				}
			}
			return nil, llmd.ErrSecret
		},
	})
}
