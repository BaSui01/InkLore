package domain

import (
	"regexp"
	"strings"
)

var environmentSecretName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidateSecretRef accepts references to deployment-managed credentials only.
// Credential values and legacy encrypted locators cannot become new metadata.
func ValidateSecretRef(ref SecretRef) error {
	if len(ref.Locator) < 1 || len(ref.Locator) > 256 || len(ref.Version) < 1 || len(ref.Version) > 64 ||
		strings.TrimSpace(ref.Version) != ref.Version || strings.Contains(ref.Locator, "..") ||
		strings.IndexFunc(ref.Locator, func(r rune) bool { return r <= 32 || r == '\\' }) >= 0 {
		return ErrInvalid
	}
	switch ref.Kind {
	case "env":
		if !environmentSecretName.MatchString(ref.Locator) {
			return ErrInvalid
		}
	case "secret_store":
		if !strings.HasPrefix(ref.Locator, "secret://") || len(ref.Locator) <= len("secret://") {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
