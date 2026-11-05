package adapters

import (
	"context"
	"strings"
	"testing"

	d "InkLore/internal/llm/domain"
)

func TestLLMSecretsRequireConfiguredPepperAndReferences(t *testing.T) {
	const provider = "11111111-1111-4111-8111-111111111111"
	t.Setenv("LLM_SECRET_BINDINGS", `[{"provider_id":"`+provider+`","ref":{"kind":"env","locator":"INKLORE_SECRET_ADAPTER_TEST","version":"v1"}}]`)
	t.Setenv("LLM_SECRET_PEPPER", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("INKLORE_SECRET_ADAPTER_TEST", "test-value-must-not-be-returned")
	ref := d.SecretRef{Kind: "env", Locator: "INKLORE_SECRET_ADAPTER_TEST", Version: "v1"}
	secrets, err := NewLLMSecrets()
	if err != nil {
		t.Fatalf("missing pepper prevented API configuration startup: %v", err)
	}
	if _, err := secrets.Fingerprint(context.Background(), provider, ref); err == nil {
		t.Fatal("missing pepper produced a predictable fingerprint")
	}
	t.Setenv("LLM_SECRET_PEPPER", "short")
	secrets, err = NewLLMSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secrets.Fingerprint(context.Background(), provider, ref); err == nil {
		t.Fatal("short pepper was accepted")
	}
	t.Setenv("LLM_SECRET_PEPPER", strings.Repeat("p", 32))
	secrets, err = NewLLMSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if hash, err := secrets.Fingerprint(context.Background(), provider, ref); err != nil || len(hash) != 64 {
		t.Fatalf("configured environment reference failed: %v", err)
	}
	for _, ref := range []d.SecretRef{
		{Kind: "plain", Locator: "UPPERCASE_SECRET", Version: "v1"},
		{Kind: "env", Locator: "sk-literal-secret", Version: "v1"},
		{Kind: "env", Locator: "enc:v1:legacy", Version: "v1"},
		{Kind: "unknown", Locator: "sk-literal-secret", Version: "v1"},
	} {
		if _, err := secrets.Fingerprint(context.Background(), provider, ref); err == nil {
			t.Fatal("credential contents were accepted as a reference")
		}
	}
	for _, config := range []string{"", "[]", "{invalid-json}", `[{"provider_id":"` + provider + `","unexpected":true}]`} {
		t.Setenv("LLM_SECRET_BINDINGS", config)
		secrets, err := NewLLMSecrets()
		if err != nil {
			t.Fatalf("unavailable bindings prevented API configuration startup: %v", err)
		}
		if _, err := secrets.Fingerprint(context.Background(), provider, ref); err == nil {
			t.Fatal("missing or malformed bindings authorized a credential")
		}
	}
}
