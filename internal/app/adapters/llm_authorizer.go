package adapters

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"

	llmd "InkLore/internal/llm/domain"
	llmi "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/identity"
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
		h := sha256.Sum256([]byte("inklore_llm_secret_pepper:" + os.Getenv("ENCRYPTION_KEY")))
		pepperBytes = h[:]
	}

	return llmi.NewSecrets(llmi.SecretOptions{
		Pepper:    pepperBytes,
		Endpoints: endpoints,
		Resolve: func(ctx context.Context, b llmi.SecretBinding) ([]byte, error) {
			ref := b.Ref
			switch ref.Kind {
			case "plain":
				val := strings.TrimSpace(ref.Locator)
				if val == "" {
					return nil, llmd.ErrSecret
				}
				if llmi.IsEncryptedSecret(val) {
					decrypted, err := llmi.DecryptSecret(val)
					if err == nil && decrypted != "" {
						return []byte(decrypted), nil
					}
				}
				return []byte(val), nil
			case "env":
				if val := os.Getenv(ref.Locator); strings.TrimSpace(val) != "" {
					return []byte(strings.TrimSpace(val)), nil
				}
				loc := strings.TrimSpace(ref.Locator)
				if loc != "" && !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(loc) {
					if llmi.IsEncryptedSecret(loc) {
						decrypted, err := llmi.DecryptSecret(loc)
						if err == nil && decrypted != "" {
							return []byte(decrypted), nil
						}
					}
					return []byte(loc), nil
				}
				return nil, llmd.ErrSecret
			case "secret_store":
				return nil, llmd.ErrSecret
			default:
				if strings.TrimSpace(ref.Locator) != "" {
					return []byte(strings.TrimSpace(ref.Locator)), nil
				}
				return nil, llmd.ErrSecret
			}
		},
	})
}
