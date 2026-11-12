package infrastructure

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/scope"
)

// EnvironmentSecretBindings is deployment metadata, not a database-controlled
// list. Each provider UUID belongs to exactly one tenant. An empty or malformed
// allowlist never authorizes arbitrary environment variables.
func EnvironmentSecretBindings() ([]SecretBinding, error) {
	raw := os.Getenv("LLM_SECRET_BINDINGS")
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	if len(raw) > 1<<20 {
		return nil, d.ErrSecret
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var bindings []SecretBinding
	if err := decoder.Decode(&bindings); err != nil || len(bindings) > 1000 {
		return nil, d.ErrSecret
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, d.ErrSecret
	}
	for _, binding := range bindings {
		if !scope.ValidID(binding.ProviderID) || d.ValidateSecretRef(binding.Ref) != nil {
			return nil, d.ErrSecret
		}
	}
	return bindings, nil
}

// resolveDeploymentSecret repeats authorization at dispatch so a persisted key
// cannot retain access after deployment revocation or bypass the catalog port.
// No value is read before the exact provider, reference and version match.
func resolveDeploymentSecret(binding SecretBinding) ([]byte, error) {
	bindings, err := EnvironmentSecretBindings()
	if err != nil || d.ValidateSecretRef(binding.Ref) != nil {
		return nil, d.ErrSecret
	}
	for _, allowed := range bindings {
		if allowed != binding {
			continue
		}
		if binding.Ref.Kind == "env" {
			if value := strings.TrimSpace(os.Getenv(binding.Ref.Locator)); value != "" {
				return []byte(value), nil
			}
		}
		return nil, d.ErrSecret
	}
	return nil, d.ErrSecret
}
