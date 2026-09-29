package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	runtimegateway "InkLore/internal/llm/domain/gateway"
	"InkLore/internal/platform/nulltype"
)

var (
	ErrNoAvailableKey = errors.New("没有可用的API密钥")
)

const (
	ChannelStatusActive   = "active"
	ChannelStatusDisabled = "disabled"
	ChannelStatusCooldown = "cooldown"

	ModelStatusActive   = "active"
	ModelStatusDisabled = "disabled"

	ModelTypeText       = "text"
	ModelTypeImage      = "image"
	ModelTypeMultimodal = "multimodal"
	ModelTypeEmbedding  = "embedding"
	ModelTypeRerank     = "rerank"
	ModelTypeAudio      = "audio"
	ModelTypeTTS        = "tts"

	ImagePricingModePerImage = "per_image"
	ImagePricingModeToken    = "token"
)

// NormalizeModelType 将历史/别名模型类型归一化为数据库标准类型
func NormalizeModelType(modelType string) string {
	switch strings.ToLower(strings.TrimSpace(modelType)) {
	case ModelTypeImage, "image-gen", "image_gen":
		return ModelTypeImage
	case ModelTypeText, "chat":
		return ModelTypeText
	case ModelTypeAudio, "speech-to-text", "speech_to_text", "stt":
		return ModelTypeAudio
	case ModelTypeTTS, "text-to-speech", "text_to_speech":
		return ModelTypeTTS
	case ModelTypeMultimodal:
		return ModelTypeMultimodal
	case ModelTypeEmbedding:
		return ModelTypeEmbedding
	case ModelTypeRerank:
		return ModelTypeRerank
	default:
		return strings.ToLower(strings.TrimSpace(modelType))
	}
}

// IsImageModelType 判断是否为图片生成/处理模型
func IsImageModelType(modelType string) bool {
	return NormalizeModelType(modelType) == ModelTypeImage
}

// IsTextModelType 判断是否为文本/对话类模型
func IsTextModelType(modelType string) bool {
	t := NormalizeModelType(modelType)
	return t == ModelTypeText || t == ModelTypeMultimodal
}

// Channel is the modelruntime routing view of an API channel.
type Channel struct {
	ExtraConfig                 nulltype.String
	BaseURL                     string
	Region                      string
	Provider                    string
	Protocol                    string
	Status                      string
	Name                        string
	SelectedUpstreamModel       string
	SelectedParamOverride       string
	SelectedProfileHash         string
	SelectedPricingSnapshotHash string
	SelectedProtocolFingerprint string
	ID                          int64
	SelectedMappingID           int64
	Priority                    int
	Weight                      int
	TimeoutMS                   int
}

// ChannelKey is the modelruntime routing view of a channel credential.
type ChannelKey struct {
	KeyName          string
	KeyValue         string
	Status           string
	ID               int64
	ChannelID        int64
	Weight           int
	DailyUsed        int
	DailyLimit       int
	ConcurrencyLimit int
	RateLimit        int
}

// ChannelModelMapping is the routing-only view of channel/model binding data.
type ChannelModelMapping struct {
	Weight              *int
	Priority            *int
	ParamOverride       nulltype.String
	CapabilityStatus    string
	RouteProfileHash    string
	PricingSnapshotHash string
	UpstreamModel       string
	Status              string
	ID                  int64
	ChannelID           int64
	ModelID             int64
}

// ChannelWithStats is the routing-only view of channel stats.
type ChannelWithStats struct {
	Channel
	KeyCount       int
	ActiveKeyCount int
	ModelCount     int
}

// Model is the modelruntime routing view of an admin-configured AI model.
type Model struct {
	MaxTokens            nulltype.Int64
	ContextWindow        nulltype.Int64
	ModelType            string
	DisplayName          string
	Name                 string
	ImagePricingMode     string
	Status               string
	Vendor               string
	ImageInputPolicyJSON string
	InputPrice           float64
	OutputPrice          float64
	CacheWritePrice      float64
	CacheReadPrice       float64
	ImagePrice           float64
	ImagePrice1K         float64
	ImagePrice2K         float64
	ImagePrice4K         float64
	ID                   int64
	SearchPrice          float64
	IsDefault            bool
	SupportFunctionCall  bool
	SupportToolCalls     bool
	SupportStreaming     bool
	SupportVision        bool
}

// KeyWrapper tracks runtime scheduling metadata for a channel key.
type KeyWrapper struct {
	LastUsed   time.Time
	Cooldown   time.Time
	Key        *ChannelKey
	ErrorCount int
	InUse      bool
}

type RuntimeRouter interface {
	Initialize(ctx context.Context) error
	GetChannelKey(providerName string) (*Channel, string, int64, error)
	GetChannelKeyWithRouteParams(providerName string, params map[string]interface{}) (*Channel, string, int64, error)
	GetChannelKeyContext(ctx context.Context, providerName string) (*Channel, string, int64, error)
	RecordKeyCall(keyID int64, success bool, err error)
	GetModelByDBID(dbID int64) (*Model, error)
	GetAllModels() ([]*Model, error)
	GetMappingByChannelAndModel(channelID, modelID int64) (*ChannelModelMapping, error)
	GetMappingsByChannelAndModels(channelID int64, modelIDs []int64) (map[int64]*ChannelModelMapping, error)
}

type ChannelKeyCandidate struct {
	Channel             *Channel
	ParamOverride       string
	CapabilityStatus    string
	ProfileHash         string
	PricingSnapshotHash string
	ProtocolFingerprint string
	UpstreamModel       string
	Keys                []*ChannelKey
	MappingID           int64
	EffectivePriority   int
	EffectiveWeight     int
	RuntimeWeight       float64
	EstimatedCost       float64
	WeightDisabled      bool
	CostKnown           bool
	FromRule            bool
	Preferred           bool
}

type ChannelProvider interface {
	GetActiveChannels() ([]*Channel, error)
	GetChannelKeys(channelID int64) ([]*ChannelKey, error)
	SelectChannel(modelOrProvider string, region string) (*Channel, *ChannelKey, error)
	ListChannelKeyCandidates(ctx context.Context, modelOrProvider string, region string, params map[string]interface{}) ([]*ChannelKeyCandidate, error)
	DecryptChannelKey(encryptedKey string) (string, error)
	RecordKeyCall(keyID int64, outcome KeyCallOutcome) error
	GetProviders() ([]string, error)
	ResetDailyUsage(ctx context.Context) error
	GetChannelStats() ([]*ChannelWithStats, error)
	TestChannel(channelID int64) error
}

type ModelProvider interface {
	GetModel(id int64) (*Model, error)
	GetActiveModels() ([]*Model, error)
	GetModelByName(name string) (*Model, error)
	GetMappingByChannelAndModel(channelID, modelID int64) (*ChannelModelMapping, error)
	GetMappingsByChannelAndModels(channelID int64, modelIDs []int64) (map[int64]*ChannelModelMapping, error)
}

// ConcurrencyLimitError is returned when every candidate channel key is at its in-flight concurrency_limit.
type ConcurrencyLimitError struct {
	KeyID int64
	Limit int
	Cause error
}

func (e *ConcurrencyLimitError) Error() string {
	if e == nil {
		return "all keys at concurrency limit"
	}
	if e.Cause != nil {
		return fmt.Sprintf("all keys at concurrency limit: %v", e.Cause)
	}
	if e.KeyID > 0 {
		return fmt.Sprintf("all keys at concurrency limit: key %d at concurrency limit %d", e.KeyID, e.Limit)
	}
	return "all keys at concurrency limit"
}

func (e *ConcurrencyLimitError) CapacityLimited() bool { return true }

func (e *ConcurrencyLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// RateLimitError is returned when every candidate channel key is at its rate limit.
type RateLimitError struct {
	KeyID int64
	Limit int
	Cause error
}

func (e *RateLimitError) Error() string {
	if e == nil {
		return "all keys at rate limit"
	}
	if e.Cause != nil {
		return fmt.Sprintf("all keys at rate limit: %v", e.Cause)
	}
	if e.KeyID > 0 {
		return fmt.Sprintf("all keys at rate limit: key %d at rate limit %d rpm", e.KeyID, e.Limit)
	}
	return "all keys at rate limit"
}

func (e *RateLimitError) CapacityLimited() bool { return true }

func (e *RateLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

const (
	WaitReasonKeyCapacity  = "key_capacity"
	WaitReasonKeyRateLimit = "key_rate_limit"
	WaitReasonWorkerBusy   = "worker_busy"
	WaitReasonProvider     = "provider_rate_limited"
)

// HTTPStatusError 携带上游 HTTP 状态码的最小契约。
type HTTPStatusError interface {
	error
	ProviderHTTPStatus() int
}

// StreamHandler 流式回调函数签名。
type StreamHandler func(chunk string) error

const (
	KeyCallFailureUnknown         = "unknown"
	KeyCallFailureProviderAuth    = "provider_auth_failed"
	KeyCallFailureProviderRate    = "provider_rate_limited"
	KeyCallFailureProviderInvalid = "provider_invalid_request"
	KeyCallFailureProviderDown    = "provider_unavailable"
	KeyCallFailureTransport       = "transport_error"
)

// KeyCallOutcome is the narrow runtime-to-admin contract for one credential call.
type KeyCallOutcome struct {
	FailureKind string
	HTTPStatus  int
	Success     bool
}

func classifyKeyCallOutcome(success bool, err error) KeyCallOutcome {
	outcome := KeyCallOutcome{Success: success}
	if success || err == nil {
		return outcome
	}

	var statusErr HTTPStatusError
	if errors.As(err, &statusErr) && statusErr != nil {
		outcome.HTTPStatus = statusErr.ProviderHTTPStatus()
	}

	switch runtimegateway.ClassifyExecutionError(err) {
	case runtimegateway.ErrorTypeProviderAuthFailed:
		outcome.FailureKind = KeyCallFailureProviderAuth
	case runtimegateway.ErrorTypeProviderRateLimited:
		outcome.FailureKind = KeyCallFailureProviderRate
	case runtimegateway.ErrorTypeProviderInvalidRequest, runtimegateway.ErrorTypeSchemaValidationFailed:
		outcome.FailureKind = KeyCallFailureProviderInvalid
	case runtimegateway.ErrorTypeRetryable:
		if outcome.HTTPStatus >= http.StatusInternalServerError {
			outcome.FailureKind = KeyCallFailureProviderDown
		} else {
			outcome.FailureKind = KeyCallFailureTransport
		}
	default:
		outcome.FailureKind = KeyCallFailureUnknown
	}
	return outcome
}
