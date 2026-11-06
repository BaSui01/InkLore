package application

import (
	"encoding/json"
	"os"
	"testing"

	d "InkLore/internal/llm/domain"
)

func TestSecretReferenceInputContractRejectsPlain(t *testing.T) {
	common, err := os.ReadFile("../../../docs/system-design/api/common.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("../../../docs/system-design/api/llm-schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	validate, err := NewInputValidator(common, schema)
	if err != nil {
		t.Fatal(err)
	}
	in := d.KeyInput{ChannelID: "11111111-1111-4111-8111-111111111111", Title: "Reference", SecretRef: d.SecretRef{Kind: "env", Locator: "TEST_API_KEY", Version: "v1"}, Weight: 1, RPM: 1, Concurrency: 1, Enabled: true}
	raw, _ := json.Marshal(in)
	if err := validate(d.Keys, raw); err != nil {
		t.Fatal(err)
	}
	in.SecretRef.Kind = "plain"
	raw, _ = json.Marshal(in)
	if err := validate(d.Keys, raw); err == nil {
		t.Fatal("KeyInput still advertises writable plain credentials")
	}
}
