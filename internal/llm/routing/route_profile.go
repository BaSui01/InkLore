package routing

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const MaxRouteProfileBytes = 64 * 1024

// CompiledRouteProfile 是管理配置写入和严格 readiness 共用的档案编译结果。
// 编译只负责结构与指纹，不根据 provider/model 名称猜测能力；真实证据仍由管理员提供。
type CompiledRouteProfile struct {
	CanonicalJSON       string
	CapabilityStatus    CapabilityStatus
	ProfileHash         string
	PricingSnapshotHash string
	ProtocolFingerprint string
}

// CompileRouteProfile 校验 RouteProfile 文档的基础结构，并返回 canonical JSON 与派生 hash。
// 未声明能力段、状态非法或 JSON 不是对象时直接失败，防止 readiness 只靠字符串状态放行。
func CompileRouteProfile(raw string) (CompiledRouteProfile, error) {
	if len(strings.TrimSpace(raw)) > MaxRouteProfileBytes {
		return CompiledRouteProfile{}, fmt.Errorf("route profile exceeds %d bytes", MaxRouteProfileBytes)
	}
	canonical, ok := canonicalProfileJSON(raw)
	if !ok {
		return CompiledRouteProfile{}, fmt.Errorf("route profile must be a valid JSON object")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(canonical), &object); err != nil || object == nil {
		return CompiledRouteProfile{}, fmt.Errorf("route profile must be a JSON object")
	}
	var envelope routeProfileEnvelope
	if err := json.Unmarshal([]byte(canonical), &envelope); err != nil {
		return CompiledRouteProfile{}, fmt.Errorf("decode route profile: %w", err)
	}
	status := CapabilityStatus(strings.ToLower(strings.TrimSpace(string(envelope.CapabilityStatus))))
	if status != "" && status != CapabilityStatusVerified && status != CapabilityStatusInferred && status != CapabilityStatusUnknown && status != CapabilityStatusDisabled {
		return CompiledRouteProfile{}, fmt.Errorf("invalid capability status: %s", status)
	}
	if !hasRouteCapabilitySection(object) {
		return CompiledRouteProfile{}, fmt.Errorf("route profile capability section is missing")
	}
	return CompiledRouteProfile{
		CanonicalJSON:       canonical,
		CapabilityStatus:    status,
		ProfileHash:         RouteProfileHash(canonical),
		PricingSnapshotHash: RoutePricingSnapshotHash(canonical),
		ProtocolFingerprint: RouteProtocolFingerprint(canonical),
	}, nil
}

func hasRouteCapabilitySection(object map[string]json.RawMessage) bool {
	for _, key := range []string{"capability", "image_input_policy", "image_capabilities", "image_output"} {
		if value, ok := object[key]; ok && strings.TrimSpace(string(value)) != "" && string(value) != "null" {
			return true
		}
	}
	return false
}

// GenerationRouteError 保留规划阶段的字段级排除结果，供 preflight 展示“为什么
// 没有候选”；Error 文本只做日志摘要，调用方应通过 errors.As 读取 Candidates。
type GenerationRouteError struct {
	Code       string
	Candidates []PlannedCandidate
}

// RouteProfileHash 生成 mapping 能力档案的 canonical hash，供规划快照与执行期漂移
// 检查共同使用，避免不同层各自实现导致同一档案产生不同指纹。
func RouteProfileHash(raw string) string {
	canonical, ok := canonicalProfileJSON(raw)
	if !ok {
		canonical = strings.TrimSpace(raw)
	}
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum[:])
}

// RoutePricingSnapshotHash 对 profile 中实际参与计费的字段生成稳定指纹。
// 没有任何 pricing 字段时返回空串，调用方可以把“未配置价格”和“价格 hash 已计算”区分开。
func RoutePricingSnapshotHash(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil || payload == nil {
		return ""
	}
	pricing := make(map[string]any)
	for _, key := range []string{"billing_required", "provider_pricing", "customer_pricing", "pricing", "margin_policy"} {
		if value, exists := payload[key]; exists {
			pricing[key] = value
		}
	}
	if len(pricing) == 0 {
		return ""
	}
	encoded, err := json.Marshal(pricing)
	if err != nil {
		return ""
	}
	return RouteProfileHash(string(encoded))
}

// RouteProtocolFingerprint 对协议选择相关字段生成稳定指纹。显式 fingerprint
// 优先沿用管理员提供的值；没有显式值时由当前 profile 的协议字段派生。
func RouteProtocolFingerprint(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil || payload == nil {
		return ""
	}
	if explicit, ok := payload["protocol_fingerprint"].(string); ok && strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	protocol := make(map[string]any)
	for _, key := range []string{"image_protocol", "gemini_interactions", "stream", "streaming", "response_modalities", "supports_streaming"} {
		if value, exists := payload[key]; exists {
			protocol[key] = value
		}
	}
	if len(protocol) == 0 {
		return ""
	}
	encoded, err := json.Marshal(protocol)
	if err != nil {
		return ""
	}
	return RouteProfileHash(string(encoded))
}

func canonicalProfileJSON(raw string) (string, bool) {
	var payload any
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		return "", false
	}
	normalized := normalizeCanonicalProfileValue(payload)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// profile 数组在能力/协议档案中表示集合而非执行顺序；对标量集合排序，
// 使管理员调整 JSON 字段顺序不会无意义地触发 profile/protocol 漂移。
func normalizeCanonicalProfileValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			typed[key] = normalizeCanonicalProfileValue(item)
		}
		return typed
	case []any:
		allScalar := true
		for index, item := range typed {
			typed[index] = normalizeCanonicalProfileValue(item)
			if _, ok := typed[index].(map[string]any); ok {
				allScalar = false
			}
			if _, ok := typed[index].([]any); ok {
				allScalar = false
			}
		}
		if allScalar {
			sort.SliceStable(typed, func(i, j int) bool {
				left, _ := json.Marshal(typed[i])
				right, _ := json.Marshal(typed[j])
				return string(left) < string(right)
			})
		}
		return typed
	default:
		return value
	}
}

func (e *GenerationRouteError) Error() string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return "generation route planning failed"
	}
	return e.Code
}

type routeProfileEnvelope struct {
	CapabilityStatus  CapabilityStatus   `json:"capability_status"`
	Capability        *routeCapability   `json:"capability"`
	ImageInputPolicy  *ImageInputPolicy  `json:"image_input_policy"`
	ImageCapabilities *imageCapabilities `json:"image_capabilities"`
}

type routeCapability struct {
	MediaTypes  []string           `json:"media_types"`
	Modes       []string           `json:"modes"`
	Image       *ImageInputPolicy  `json:"image"`
	ImageOutput *imageCapabilities `json:"image_output"`
}

type imageCapabilities struct {
	Resolutions  []string `json:"resolutions"`
	AspectRatios []string `json:"aspect_ratios"`
	Qualities    []string `json:"qualities"`
	Formats      []string `json:"formats"`
	MaxOutputs   int      `json:"max_outputs"`
	MinWidth     int      `json:"min_width"`
	MaxWidth     int      `json:"max_width"`
	MinHeight    int      `json:"min_height"`
	MaxHeight    int      `json:"max_height"`
}

// EvaluateRouteProfile evaluates legacy param_override and RouteProfileV1 envelopes.
// StrictCapability=true 时，能力档案缺失或状态未核验不会被默认为支持。
func EvaluateRouteProfile(raw string, req GenerationRouteRequirement) RouteProfileEvaluation {
	result := RouteProfileEvaluation{Compatible: true, Known: true}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		if req.StrictCapability && requiresCapabilityDeclaration(req) {
			return incompatibleUnknown("capability_profile_missing")
		}
		return result
	}

	var envelope routeProfileEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		if req.StrictCapability && requiresCapabilityDeclaration(req) {
			return incompatibleUnknown("capability_profile_invalid")
		}
		return result
	}
	envelope.CapabilityStatus = CapabilityStatus(strings.ToLower(strings.TrimSpace(string(envelope.CapabilityStatus))))
	if envelope.CapabilityStatus == CapabilityStatusDisabled {
		return incompatible("capability_disabled", "", nil, nil)
	}
	if req.StrictCapability && envelope.CapabilityStatus != CapabilityStatusVerified {
		return incompatibleUnknown("capability_profile_unverified")
	}

	imagePolicy := envelope.ImageInputPolicy
	imageCaps := envelope.ImageCapabilities
	if envelope.Capability != nil {
		if imagePolicy == nil {
			imagePolicy = envelope.Capability.Image
		}
		if imageCaps == nil {
			imageCaps = envelope.Capability.ImageOutput
		}
		if len(envelope.Capability.Modes) > 0 && req.Mode != "" &&
			!isModeCompatible(envelope.Capability.Modes, req.Mode) {
			return incompatible("mode_unsupported", "mode", req.Mode, envelope.Capability.Modes)
		}
		if len(envelope.Capability.MediaTypes) > 0 && req.MediaType != "" &&
			!containsFold(envelope.Capability.MediaTypes, req.MediaType) {
			return incompatible("media_type_unsupported", "media_type", req.MediaType, envelope.Capability.MediaTypes)
		}
	}

	switch strings.ToLower(strings.TrimSpace(req.MediaType)) {
	case "image", "image-gen", "image_gen":
		if violation := evaluateImageCapabilities(imageCaps, req); violation != nil {
			return *violation
		}
		if req.InputImageCount <= 0 {
			return result
		}
		if imagePolicy == nil {
			if req.StrictCapability {
				return incompatibleUnknown("image_input_policy_missing")
			}
			return result
		}
		if imagePolicy.Enabled != nil && !*imagePolicy.Enabled {
			return incompatible("image_input_disabled", "input_image_count", req.InputImageCount, 0)
		}
		if imagePolicy.MaxImages > 0 && req.InputImageCount > imagePolicy.MaxImages {
			return incompatible("input_image_count_exceeded", "input_image_count", req.InputImageCount, imagePolicy.MaxImages)
		}
		if imagePolicy.MaxTotalBytes > 0 && req.InputImageCount > 0 && !inputTotalBytesKnown(req) {
			// 常规生图链路未探测到参考图总大小时宽容放行，仅在严格核验模式下拦截，避免误杀可用渠道
			if req.StrictCapability {
				return incompatibleUnknown("input_total_bytes_unknown")
			}
		}
		if imagePolicy.MaxTotalBytes > 0 && req.InputTotalBytes > imagePolicy.MaxTotalBytes {
			return incompatible("input_total_bytes_exceeded", "input_total_bytes", req.InputTotalBytes, imagePolicy.MaxTotalBytes)
		}
		if violation := evaluateImageInputMetadata(imagePolicy, req); violation != nil {
			return *violation
		}
		return result
	default:
		if req.StrictCapability && requiresCapabilityDeclaration(req) && envelope.Capability == nil {
			return incompatibleUnknown("capability_media_type_missing")
		}
		return result
	}
}

func evaluateImageInputMetadata(policy *ImageInputPolicy, req GenerationRouteRequirement) *RouteProfileEvaluation {
	if policy == nil {
		return nil
	}
	if policy.MaxSingleBytes > 0 && req.InputImageCount > 0 && !inputSingleBytesKnown(req) {
		if req.StrictCapability {
			result := incompatibleUnknown("input_file_bytes_unknown")
			return &result
		}
	}
	if policy.MaxSingleBytes > 0 && inputSingleBytesKnown(req) && req.MaxSingleInputBytes > policy.MaxSingleBytes {
		result := incompatible("input_file_bytes_exceeded", "max_single_input_bytes", req.MaxSingleInputBytes, policy.MaxSingleBytes)
		return &result
	}
	if policy.MaxTotalBytes > 0 && req.InputImageCount > 0 && !inputTotalBytesKnown(req) {
		if req.StrictCapability {
			result := incompatibleUnknown("input_total_bytes_unknown")
			return &result
		}
	}
	if policy.MaxTotalBytes > 0 && inputTotalBytesKnown(req) && req.InputTotalBytes > policy.MaxTotalBytes {
		result := incompatible("input_total_bytes_exceeded", "input_total_bytes", req.InputTotalBytes, policy.MaxTotalBytes)
		return &result
	}
	// 像素配置与拦截已完全移除：仅保留单图体积与总体积/容量拦截，生图参考图由模型运行时自适应缩放
	return evaluateAllowedInputMetadata(policy.AllowedMimeTypes, policy.AllowedRoles, req)
}

func evaluateImageCapabilities(caps *imageCapabilities, req GenerationRouteRequirement) *RouteProfileEvaluation {
	if caps == nil {
		return nil
	}
	// 分辨率限制已按需求移除：不再对生图分辨率进行排他过滤，直接放行透传给上游渠道处理，避免误杀 mapping_filtered
	if req.AspectRatio != "" && len(caps.AspectRatios) > 0 && !containsFold(caps.AspectRatios, req.AspectRatio) {
		result := incompatible("image_aspect_ratio_unsupported", "aspect_ratio", req.AspectRatio, caps.AspectRatios)
		return &result
	}
	// quality="auto" 表示由服务端按渠道实际档位决定，不作为路由过滤条件，
	// 避免渠道能力声明未包含 auto 档位时全部映射被误杀为 mapping_filtered。
	reqQuality := strings.TrimSpace(req.Quality)
	if reqQuality != "" && !strings.EqualFold(reqQuality, "auto") && len(caps.Qualities) > 0 && !containsFold(caps.Qualities, reqQuality) {
		result := incompatible("image_quality_unsupported", "quality", req.Quality, caps.Qualities)
		return &result
	}
	if req.OutputFormat != "" && len(caps.Formats) > 0 && !containsFold(caps.Formats, req.OutputFormat) {
		result := incompatible("image_format_unsupported", "output_format", req.OutputFormat, caps.Formats)
		return &result
	}
	if caps.MaxOutputs > 0 && req.OutputCount > caps.MaxOutputs {
		result := incompatible("image_output_count_exceeded", "output_count", req.OutputCount, caps.MaxOutputs)
		return &result
	}
	if caps.MinWidth > 0 && req.Width > 0 && req.Width < caps.MinWidth {
		result := incompatible("image_width_below_minimum", "width", req.Width, caps.MinWidth)
		return &result
	}
	if caps.MaxWidth > 0 && req.Width > caps.MaxWidth {
		result := incompatible("image_width_exceeded", "width", req.Width, caps.MaxWidth)
		return &result
	}
	if caps.MinHeight > 0 && req.Height > 0 && req.Height < caps.MinHeight {
		result := incompatible("image_height_below_minimum", "height", req.Height, caps.MinHeight)
		return &result
	}
	if caps.MaxHeight > 0 && req.Height > caps.MaxHeight {
		result := incompatible("image_height_exceeded", "height", req.Height, caps.MaxHeight)
		return &result
	}
	return nil
}

func evaluateAllowedInputMetadata(allowedMimes, allowedRoles []string, req GenerationRouteRequirement) *RouteProfileEvaluation {
	if len(allowedMimes) > 0 {
		if len(req.InputMimeTypes) == 0 || !req.InputMimeTypesKnown {
			if req.StrictCapability {
				result := incompatibleUnknown("input_mime_types_unknown")
				return &result
			}
		} else {
			for _, mimeType := range req.InputMimeTypes {
				if strings.TrimSpace(mimeType) != "" && !containsFold(allowedMimes, mimeType) {
					result := incompatible("input_mime_type_unsupported", "mime_type", mimeType, allowedMimes)
					return &result
				}
			}
		}
	}
	if len(allowedRoles) > 0 {
		if len(req.InputRoles) == 0 || !req.InputRolesKnown {
			if req.StrictCapability {
				result := incompatibleUnknown("input_roles_unknown")
				return &result
			}
		} else {
			for _, role := range req.InputRoles {
				if strings.TrimSpace(role) != "" && !containsFold(allowedRoles, role) {
					result := incompatible("input_role_unsupported", "role", role, allowedRoles)
					return &result
				}
			}
		}
	}
	return nil
}

func inputTotalBytesKnown(req GenerationRouteRequirement) bool {
	return req.InputImageCount+req.InputAudioCount == 0 || req.InputTotalBytesKnown || req.InputTotalBytes > 0
}

func inputSingleBytesKnown(req GenerationRouteRequirement) bool {
	return req.InputImageCount+req.InputAudioCount == 0 || req.MaxSingleInputBytesKnown || req.MaxSingleInputBytes > 0
}

func inputPixelsKnown(req GenerationRouteRequirement) bool {
	return req.InputImageCount == 0 || req.MaxInputPixelsKnown || req.MaxInputPixels > 0
}

func requiresCapabilityDeclaration(req GenerationRouteRequirement) bool {
	return req.InputImageCount > 0 || req.InputAudioCount > 0 ||
		req.Resolution != "" || req.AspectRatio != "" || req.Quality != "" || req.DurationSeconds > 0 ||
		strings.TrimSpace(req.Mode) != ""
}

func incompatible(code, field string, requested, supported any) RouteProfileEvaluation {
	return RouteProfileEvaluation{
		Compatible: false,
		Known:      true,
		Reasons: []CapabilityViolation{{
			ReasonCode: code,
			Field:      field,
			Requested:  requested,
			Supported:  supported,
		}},
	}
}

func incompatibleUnknown(code string) RouteProfileEvaluation {
	return RouteProfileEvaluation{
		Compatible: false,
		Known:      false,
		Reasons:    []CapabilityViolation{{ReasonCode: code}},
	}
}

func lookupFoldedDurations(values map[string][]string, wanted string) ([]string, bool) {
	for key, durations := range values {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(wanted)) {
			return durations, true
		}
	}
	return nil, false
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

// isModeCompatible 判定请求模式与渠道声明的模式列表是否兼容。
// 适配不同前端、工作流与不同模型提供商对生图/编辑模式的同义称谓：
// - "image_generation" <=> "text_to_image" / "text-to-image" / "t2i"
// - "image_edit" <=> "image_to_image" / "image-to-image" / "i2i"
func isModeCompatible(supportedModes []string, requestedMode string) bool {
	if containsFold(supportedModes, requestedMode) {
		return true
	}
	req := strings.ToLower(strings.TrimSpace(requestedMode))
	var aliases []string
	switch req {
	case "image_generation", "image-generation", "generation":
		aliases = []string{"text_to_image", "text-to-image", "t2i", "image_generation", "generation"}
	case "text_to_image", "text-to-image", "t2i":
		aliases = []string{"image_generation", "image-generation", "generation", "text_to_image", "text-to-image", "t2i"}
	case "image_edit", "image-edit", "edit":
		aliases = []string{"image_to_image", "image-to-image", "i2i", "image_edit", "image-edit", "edit"}
	case "image_to_image", "image-to-image", "i2i":
		aliases = []string{"image_edit", "image-edit", "edit", "image_to_image", "image-to-image", "i2i"}
	}
	for _, alias := range aliases {
		if containsFold(supportedModes, alias) {
			return true
		}
	}
	return false
}

// IsImageResolutionCompatible 评估渠道或模型声明的分辨率集合与请求分辨率是否兼容。
// 适配不同模型厂商的协议差异：
// 1. 文本精确/大小写匹配（如 "1K", "4K", "1024x1024"）；
// 2. 档位互通映射（如 Gemini 传 1K/2K/4K/512 + 比例，OpenAI/Flux 传 1024x1024/4096x4096 等具体尺寸）：
//   - "512" <=> "512x512" (约 26 万像素)
//   - "1K"  <=> "1024x1024", "1248x1248", "1024x1536", "1536x1024", "768x1024", "1024x768" 等 (约 70万~180万像素)
//   - "2K"  <=> "2048x2048", "2560x1440", "1440x2560" 等 (约 200万~500万像素)
//   - "4K"  <=> "4096x4096", "2880x2880", "3840x2160", "2160x3840" 等 (约 500万~2000万像素)
func IsImageResolutionCompatible(supportedResolutions []string, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" || len(supportedResolutions) == 0 {
		return true
	}
	// 1. 优先直接匹配（大小写不敏感）
	if containsFold(supportedResolutions, requested) {
		return true
	}

	// 2. 归一化档位比对
	reqTier := NormalizeResolutionTier(requested)
	for _, supported := range supportedResolutions {
		suppTier := NormalizeResolutionTier(supported)
		if reqTier != "" && suppTier != "" && strings.EqualFold(reqTier, suppTier) {
			return true
		}
	}
	return false
}

// isImageResolutionCompatible 保持包内兼容别名。
func isImageResolutionCompatible(supportedResolutions []string, requested string) bool {
	return IsImageResolutionCompatible(supportedResolutions, requested)
}

// NormalizeResolutionTier 将尺寸字符串或档位名称归一化为标准的 512, 1K, 2K, 4K 档位。
func NormalizeResolutionTier(res string) string {
	res = strings.TrimSpace(strings.ToUpper(res))
	if res == "" {
		return ""
	}
	switch res {
	case "512", "512X512":
		return "512"
	case "1K", "1024", "1024X1024", "1248X1248", "1024X1536", "1536X1024", "768X1024", "1024X768", "1920X1080", "1080X1920", "1024X1792", "1792X1024":
		return "1K"
	case "2K", "2048", "2048X2048", "2048X1152", "1152X2048", "2560X1440", "1440X2560":
		return "2K"
	case "4K", "4096", "4096X4096", "2880X2880", "3840X2160", "2160X3840", "8K":
		return "4K"
	}

	// 如果传入的是 WxH 格式，按像素乘积自动判定档位
	if parts := strings.Split(res, "X"); len(parts) == 2 {
		w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errW == nil && errH == nil && w > 0 && h > 0 {
			pixels := int64(w) * int64(h)
			if pixels <= 400_000 {
				return "512"
			}
			if pixels <= 1_800_000 {
				return "1K"
			}
			if pixels <= 5_000_000 {
				return "2K"
			}
			return "4K"
		}
	}
	return ""
}

func normalizeResolutionTier(res string) string {
	return NormalizeResolutionTier(res)
}

// ImageCapabilities 公开既有图片能力结构，不另建按供应商或模型名推断的能力字典。
type ImageCapabilities = imageCapabilities

// ProjectCommonImageCapabilities 投影所有已验证路由的保守交集，实际路由仍由 planner 决定。
func ProjectCommonImageCapabilities(profiles []string) (*ImageCapabilities, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("image model has no verified route profile")
	}
	var result *ImageCapabilities
	for _, raw := range profiles {
		compiled, err := CompileRouteProfile(raw)
		if err != nil {
			return nil, err
		}
		if compiled.CapabilityStatus != CapabilityStatusVerified {
			return nil, fmt.Errorf("image route profile is not verified")
		}
		var envelope routeProfileEnvelope
		if err := json.Unmarshal([]byte(compiled.CanonicalJSON), &envelope); err != nil {
			return nil, err
		}
		caps := envelope.ImageCapabilities
		if caps == nil && envelope.Capability != nil {
			caps = envelope.Capability.ImageOutput
		}
		if caps == nil {
			return nil, fmt.Errorf("verified route profile lacks image output capabilities")
		}
		if caps.MaxOutputs < 0 || caps.MinWidth < 0 || caps.MaxWidth < 0 || caps.MinHeight < 0 || caps.MaxHeight < 0 {
			return nil, fmt.Errorf("image route profile has invalid numeric limits")
		}
		if (caps.MaxWidth > 0 && caps.MaxWidth < caps.MinWidth) || (caps.MaxHeight > 0 && caps.MaxHeight < caps.MinHeight) {
			return nil, fmt.Errorf("image route profile has inverted dimension limits")
		}
		if result == nil {
			initial := *caps
			result = &initial
			continue
		}
		result.Resolutions = intersectImageOptions(result.Resolutions, caps.Resolutions)
		result.AspectRatios = intersectImageOptions(result.AspectRatios, caps.AspectRatios)
		result.Qualities = intersectImageOptions(result.Qualities, caps.Qualities)
		result.Formats = intersectImageOptions(result.Formats, caps.Formats)
		result.MaxOutputs = min(result.MaxOutputs, caps.MaxOutputs)
		result.MinWidth = max(result.MinWidth, caps.MinWidth)
		result.MinHeight = max(result.MinHeight, caps.MinHeight)
		result.MaxWidth = intersectImageMaximum(result.MaxWidth, caps.MaxWidth)
		result.MaxHeight = intersectImageMaximum(result.MaxHeight, caps.MaxHeight)
	}
	if result == nil {
		return nil, fmt.Errorf("image route profiles produced no capabilities")
	}
	if len(result.Resolutions) == 0 || len(result.AspectRatios) == 0 || len(result.Qualities) == 0 || result.MaxOutputs <= 0 {
		return nil, fmt.Errorf("image route capabilities have missing or disjoint required options")
	}
	if (result.MaxWidth > 0 && result.MaxWidth < result.MinWidth) || (result.MaxHeight > 0 && result.MaxHeight < result.MinHeight) {
		return nil, fmt.Errorf("image route dimension limits have no intersection")
	}
	return result, nil
}

func intersectImageOptions(left, right []string) []string {
	result := make([]string, 0, len(left))
	for _, item := range left {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		for _, other := range right {
			if strings.EqualFold(item, strings.TrimSpace(other)) {
				result = append(result, item)
				break
			}
		}
	}
	return result
}

func intersectImageMaximum(left, right int) int {
	if left == 0 {
		return right
	}
	if right == 0 {
		return left
	}
	return min(left, right)
}

type ImageInputPolicy struct {
	Enabled          *bool    `json:"enabled,omitempty"`
	MaxImages        int      `json:"max_images,omitempty"`
	MaxTotalBytes    int64    `json:"max_total_bytes,omitempty"`
	MaxSingleBytes   int64    `json:"max_single_bytes,omitempty"`
	AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	AllowedRoles     []string `json:"allowed_roles,omitempty"`
	Source           string   `json:"source,omitempty"`
	VerifiedAt       string   `json:"verified_at,omitempty"`
}

type imageInputPolicyOverride struct {
	ImageInputPolicy *ImageInputPolicy `json:"image_input_policy,omitempty"`
}

func ResolveImageInputPolicy(modelJSON, mappingJSON string) (ImageInputPolicy, error) {
	base, err := parseImageInputPolicy(modelJSON)
	if err != nil {
		return ImageInputPolicy{}, err
	}
	mappingJSON = strings.TrimSpace(mappingJSON)
	if mappingJSON == "" || mappingJSON == "{}" || mappingJSON == "null" {
		return base, nil
	}
	var envelope imageInputPolicyOverride
	if err := json.Unmarshal([]byte(mappingJSON), &envelope); err != nil {
		return ImageInputPolicy{}, fmt.Errorf("解析渠道图片输入策略失败: %w", err)
	}
	if envelope.ImageInputPolicy == nil {
		return base, nil
	}
	result := base
	override := *envelope.ImageInputPolicy
	if override.Enabled != nil {
		enabled := *override.Enabled
		result.Enabled = &enabled
	}
	if override.MaxImages > 0 || (override.Enabled != nil && !*override.Enabled) {
		result.MaxImages = override.MaxImages
	}
	if override.MaxTotalBytes > 0 || (override.Enabled != nil && !*override.Enabled) {
		result.MaxTotalBytes = override.MaxTotalBytes
	}
	if override.MaxSingleBytes > 0 || (override.Enabled != nil && !*override.Enabled) {
		result.MaxSingleBytes = override.MaxSingleBytes
	}
	if len(override.AllowedMimeTypes) > 0 {
		result.AllowedMimeTypes = append([]string(nil), override.AllowedMimeTypes...)
	}
	if len(override.AllowedRoles) > 0 {
		result.AllowedRoles = append([]string(nil), override.AllowedRoles...)
	}
	if strings.TrimSpace(override.Source) != "" {
		result.Source = strings.TrimSpace(override.Source)
	}
	if strings.TrimSpace(override.VerifiedAt) != "" {
		result.VerifiedAt = strings.TrimSpace(override.VerifiedAt)
	}
	return result, validateImageInputPolicy(result)
}

func parseImageInputPolicy(raw string) (ImageInputPolicy, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return ImageInputPolicy{}, nil
	}
	var policy ImageInputPolicy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return ImageInputPolicy{}, fmt.Errorf("解析模型图片输入策略失败: %w", err)
	}
	return policy, validateImageInputPolicy(policy)
}

func validateImageInputPolicy(policy ImageInputPolicy) error {
	if policy.MaxImages < 0 {
		return fmt.Errorf("图片输入上限不能为负数")
	}
	if policy.MaxTotalBytes < 0 {
		return fmt.Errorf("参考图总大小上限不能为负数")
	}
	if policy.MaxSingleBytes < 0 {
		return fmt.Errorf("图片输入单文件大小上限不能为负数")
	}
	if policy.Enabled != nil && !*policy.Enabled && (policy.MaxImages > 0 || policy.MaxTotalBytes > 0) {
		return fmt.Errorf("禁用图片输入时不能配置正数上限")
	}
	return nil
}

// CheckViolation 检查输入参考图约束，返回违规原因（空字符串表示通过）。
func (p ImageInputPolicy) CheckViolation(count int, totalBytes int64) string {
	if count <= 0 {
		return ""
	}
	if p.Enabled != nil && !*p.Enabled {
		return "当前模型不支持参考图输入"
	}
	if p.MaxImages > 0 && count > p.MaxImages {
		return fmt.Sprintf("当前模型最多支持 %d 张参考图，实际为 %d 张", p.MaxImages, count)
	}
	if p.MaxTotalBytes > 0 && totalBytes > p.MaxTotalBytes {
		return fmt.Sprintf("参考图总大小 %d 字节超过当前模型限制 %d 字节", totalBytes, p.MaxTotalBytes)
	}
	return ""
}
