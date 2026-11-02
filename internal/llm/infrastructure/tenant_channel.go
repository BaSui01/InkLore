package infrastructure

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"

	"InkLore/internal/platform/uow"
)

// TenantChannelTarget is the resolved route for one authorized tenant/model pair.
type TenantChannelTarget struct {
	BaseURL       string
	APIKey        string
	UpstreamModel string
	Protocol      string
}

type TenantChannelResolver func(context.Context, string, string) (TenantChannelTarget, error)

// NewTenantChannelResolver only reads channels bound to the exact tenant and model.
// No environment-default or other-tenant fallback is permitted for this path.
func NewTenantChannelResolver(manager *uow.Manager) (TenantChannelResolver, error) {
	if manager == nil {
		return nil, errors.New("llm tenant channel: database required")
	}
	return func(ctx context.Context, tenantID, modelID string) (TenantChannelTarget, error) {
		if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(modelID) == "" {
			return TenantChannelTarget{}, errors.New("llm tenant channel: tenant and model required")
		}
		db, err := manager.DB(ctx)
		if err != nil {
			return TenantChannelTarget{}, err
		}
		var row struct {
			BaseURL       string
			UpstreamModel string
			ModelName     string
			Protocol      string
			SecretKind    string
			SecretLocator string
		}
		result := db.Raw(`SELECT c.base_url, cm.upstream_model, m.upstream_name AS model_name,
				pr.protocol, k.secret_kind, k.secret_locator
			FROM llm_channel_models cm
			JOIN llm_channels c ON c.tenant_id=cm.tenant_id AND c.id=cm.channel_id
			JOIN llm_models m ON m.tenant_id=cm.tenant_id AND m.id=cm.model_id
			JOIN llm_providers pr ON pr.tenant_id=c.tenant_id AND pr.id=c.provider_id
			JOIN llm_keys k ON k.tenant_id=c.tenant_id AND k.channel_id=c.id
			WHERE cm.tenant_id=? AND cm.model_id=?
			  AND cm.enabled=TRUE AND cm.deleted_at IS NULL
			  AND c.enabled=TRUE AND c.deleted_at IS NULL
			  AND m.enabled=TRUE AND m.deleted_at IS NULL
			  AND pr.enabled=TRUE AND pr.deleted_at IS NULL
			  AND k.enabled=TRUE AND k.deleted_at IS NULL
			ORDER BY cm.priority, cm.weight DESC, k.weight DESC, k.created_at DESC
			LIMIT 1`, tenantID, modelID).Scan(&row)
		if result.Error != nil {
			return TenantChannelTarget{}, result.Error
		}
		if result.RowsAffected != 1 {
			return TenantChannelTarget{}, errors.New("llm tenant channel: no enabled route")
		}
		protocol := strings.ToLower(strings.TrimSpace(row.Protocol))
		switch protocol {
		case "openai", "deepseek", "anthropic", "gemini", "ollama":
		default:
			return TenantChannelTarget{}, errors.New("llm tenant channel: unsupported provider protocol")
		}
		endpoint := strings.TrimRight(strings.TrimSpace(row.BaseURL), "/")
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Scheme != "https" && !(protocol == "ollama" && parsed.Scheme == "http" && isLocalOrPrivateHost(parsed.Host))) {
			return TenantChannelTarget{}, errors.New("llm tenant channel: invalid endpoint")
		}
		modelName := strings.TrimSpace(row.UpstreamModel)
		if modelName == "" {
			modelName = strings.TrimSpace(row.ModelName)
		}
		if modelName == "" {
			return TenantChannelTarget{}, errors.New("llm tenant channel: missing upstream model")
		}
		var apiKey string
		switch {
		case row.SecretKind == "env" && IsEncryptedSecret(row.SecretLocator):
			apiKey, err = DecryptSecret(row.SecretLocator)
		case row.SecretKind == "env" && envNameRegex.MatchString(row.SecretLocator):
			apiKey = os.Getenv(row.SecretLocator)
		default:
			return TenantChannelTarget{}, errors.New("llm tenant channel: secret provider unavailable")
		}
		if err != nil || strings.TrimSpace(apiKey) == "" {
			return TenantChannelTarget{}, errors.New("llm tenant channel: secret unavailable")
		}
		return TenantChannelTarget{
			BaseURL: endpoint, APIKey: strings.TrimSpace(apiKey),
			UpstreamModel: modelName, Protocol: protocol,
		}, nil
	}, nil
}
