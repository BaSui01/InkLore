package infrastructure

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	d "InkLore/internal/llm/domain"
)

func TestSecretsDoNotUseDefaultKeysOrDecryptForDisplay(t *testing.T) {
	t.Setenv("LLM_SECRET_PEPPER", "")
	t.Setenv("ENCRYPTION_KEY", "")
	if _, err := EncryptSecret("test-credential"); err == nil {
		t.Fatal("encryption used a fixed default key")
	}
	if _, err := DecryptSecret("UPPERCASE_PLAINTEXT"); err == nil {
		t.Fatal("plaintext input was returned as decrypted data")
	}
	t.Setenv("LLM_SECRET_PEPPER", strings.Repeat("p", 32))
	ciphertext, err := EncryptSecret("test-credential")
	if err != nil {
		t.Fatal(err)
	}
	if value, err := DecryptSecret(ciphertext); err != nil || value != "test-credential" {
		t.Fatalf("explicit legacy key could not decrypt its ciphertext: %v", err)
	}
	t.Setenv("LLM_SECRET_PEPPER", "")
	if _, err := DecryptSecret(ciphertext); err == nil {
		t.Fatal("legacy decryption used a fixed default key")
	}
	for _, value := range []string{"UPPERCASE_PLAINTEXT", "sk-literal-secret", ciphertext, "secret://vault/sensitive/path"} {
		if MaskSecret(value) != "****" {
			t.Fatal("secret display exposed credential contents")
		}
	}
	for _, kind := range []string{"plain", "secret_store", "unknown"} {
		raw, err := normalize(d.Keys, []byte(`{"secret_ref":{"kind":"`+kind+`","locator":"UPPERCASE_PLAINTEXT","version":"v1"}}`))
		if err != nil || strings.Contains(string(raw), "UPPERCASE_PLAINTEXT") {
			t.Fatalf("typed metadata was guessed from its contents: %s %v", raw, err)
		}
		var obj struct {
			SecretRef d.SecretRef `json:"secret_ref"`
		}
		if json.Unmarshal(raw, &obj) != nil || obj.SecretRef.Locator != "****" {
			t.Fatal("sensitive locator did not use a fixed mask")
		}
	}
}

func TestSecretBindingCannotBeBypassedWithPlainKind(t *testing.T) {
	const provider = "11111111-1111-4111-8111-111111111111"
	called := false
	ref := d.SecretRef{Kind: "env", Locator: "TEST_ALLOWED_REFERENCE", Version: "v1"}
	secrets, err := NewSecrets(SecretOptions{
		Pepper: []byte(strings.Repeat("p", 32)), Bindings: []SecretBinding{{ProviderID: provider, Ref: ref}},
		Resolve: func(context.Context, SecretBinding) ([]byte, error) {
			called = true
			return []byte("unit-test-value"), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ref.Kind = "plain"
	if _, err := secrets.Fingerprint(context.Background(), provider, ref); err == nil || called {
		t.Fatal("plain kind bypassed the configured binding")
	}
	if err := secrets.WithSecret(context.Background(), provider, ref, func([]byte) error { return nil }); err == nil || called {
		t.Fatal("plain kind reached the credential resolver")
	}
}
