package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

// OpenAI Image Generation & Edit Request
type openAIImageRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt"`
	Image          string `json:"image,omitempty"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	Style          string `json:"style,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type openAIImageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		B64JSON       string `json:"b64_json,omitempty"`
		URL           string `json:"url,omitempty"`
		RevisedPrompt string `json:"revised_prompt,omitempty"`
	} `json:"data"`
}

// Gemini 原生 generateContent 生图请求 (多模态 text-to-image / image-to-image)
type geminiImageGenerateRequest struct {
	Contents []struct {
		Role  string       `json:"role,omitempty"`
		Parts []geminiPart `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		ResponseModalities []string `json:"responseModalities,omitempty"`
		ResponseMimeType   string   `json:"responseMimeType,omitempty"`
		NumberOfCandidates int      `json:"numberOfCandidates,omitempty"`
		Temperature        *float64 `json:"temperature,omitempty"`
	} `json:"generationConfig,omitempty"`
	SafetySettings []geminiSafetySetting `json:"safetySettings,omitempty"`
}

// GenerateImage 执行生图（文生图与图生图），严格要求显式传入模型名称，禁止默认回退。
func (r *ProviderRouter) GenerateImage(ctx context.Context, req domain.ImageGenerationRequest) (domain.ImageGenerationResponse, error) {
	if r == nil || !r.isLiveAPI {
		return domain.ImageGenerationResponse{}, domain.ErrProviderNotConfigured
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		return domain.ImageGenerationResponse{}, platformerr.New(http.StatusBadRequest, "llm_model_required", "llm.invalid_request")
	}

	target, targetErr := r.resolveUpstream(ctx, model)
	if targetErr != nil {
		return domain.ImageGenerationResponse{}, targetErr
	}

	protocol := normalizeProtocol(target.protocol)
	if protocol == "" {
		protocol = normalizeProtocol(r.provider)
	}
	if strings.Contains(strings.ToLower(model), "imagen") {
		return domain.ImageGenerationResponse{}, unsupportedImageProtocol("Imagen 模型已关闭，不能调用 Gemini :predict")
	}
	if req.Mode == "image_to_image" && len(req.ReferenceImages) == 0 {
		return domain.ImageGenerationResponse{}, unsupportedImageProtocol("图生图至少需要一张参考图")
	}

	switch protocol {
	case protocolOpenAI:
		return r.generateImageOpenAI(ctx, target, req, model)
	case protocolGemini:
		return r.generateImageGeminiNative(ctx, target, req, model)
	case protocolGeminiInteractions:
		return domain.ImageGenerationResponse{}, unsupportedImageProtocol("Gemini Interactions 图片适配器尚未实现")
	case protocolGeminiOpenAI:
		return domain.ImageGenerationResponse{}, unsupportedImageProtocol("Gemini OpenAI 兼容 transport 未验收图片能力")
	default:
		return domain.ImageGenerationResponse{}, unsupportedImageProtocol("当前渠道 transport 未验收图片能力")
	}
}

func (r *ProviderRouter) generateImageOpenAI(ctx context.Context, target upstreamTarget, req domain.ImageGenerationRequest, model string) (domain.ImageGenerationResponse, error) {
	n := req.N
	if n <= 0 {
		n = 1
	}

	format := strings.TrimSpace(req.OutputFormat)
	if format != "url" {
		format = "b64_json"
	}

	if req.Mode == "image_to_image" && len(req.ReferenceImages) > 0 {
		if req.ReferenceStrength != nil {
			return domain.ImageGenerationResponse{}, unsupportedImageProtocol("OpenAI 图片编辑不支持 reference_strength 参数")
		}
		return r.generateImageOpenAIEdit(ctx, target, req, model, n)
	}
	payload := openAIImageRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              n,
		Size:           req.Size,
		Quality:        req.Quality,
		Style:          req.Style,
		ResponseFormat: format,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}

	reqURL := strings.TrimRight(target.baseURL, "/") + "/images/generations"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		parsedErr := parseUpstreamHTTPError(resp)
		r.recordUpstream(target, parsedErr)
		return domain.ImageGenerationResponse{}, platformerr.Wrap(parsedErr, http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	var decoded openAIImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}
	r.recordUpstream(target, nil)

	images := make([]domain.GeneratedImage, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		mimeType := "image/png"
		if req.OutputFormat == "image/jpeg" {
			mimeType = "image/jpeg"
		}
		images = append(images, domain.GeneratedImage{
			B64JSON:       item.B64JSON,
			URL:           item.URL,
			RevisedPrompt: item.RevisedPrompt,
			MimeType:      mimeType,
		})
	}

	return domain.ImageGenerationResponse{
		Images: images,
	}, nil
}

// generateImageOpenAIEdit sends real image data to the official multipart
// Images edits endpoint. A persisted file reference is deliberately rejected:
// this adapter has no authorized file resolver and must not fabricate an edit.
func (r *ProviderRouter) generateImageOpenAIEdit(ctx context.Context, target upstreamTarget, req domain.ImageGenerationRequest, model string, n int) (domain.ImageGenerationResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{"model": model, "prompt": req.Prompt}
	if n > 0 {
		fields["n"] = fmt.Sprintf("%d", n)
	}
	if value := strings.TrimSpace(req.Size); value != "" {
		fields["size"] = value
	}
	if value := strings.TrimSpace(req.Quality); value != "" {
		fields["quality"] = value
	}
	if value := strings.TrimSpace(req.OutputFormat); value == "b64_json" || value == "url" {
		fields["response_format"] = value
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return domain.ImageGenerationResponse{}, platformerr.Internal("llm_multipart_failed", "llm.generation_failed", err.Error())
		}
	}

	for index, rawReference := range req.ReferenceImages {
		contents, mimeType, err := decodeImageReference(rawReference)
		if err != nil {
			return domain.ImageGenerationResponse{}, err
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="reference-%d%s"`, index+1, imageFilenameExtension(mimeType)))
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return domain.ImageGenerationResponse{}, platformerr.Internal("llm_multipart_failed", "llm.generation_failed", err.Error())
		}
		if _, err = part.Write(contents); err != nil {
			return domain.ImageGenerationResponse{}, platformerr.Internal("llm_multipart_failed", "llm.generation_failed", err.Error())
		}
	}
	if err := writer.Close(); err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_multipart_failed", "llm.generation_failed", err.Error())
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(target.baseURL, "/")+"/images/edits", &body)
	if err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+target.apiKey)
	resp, err := r.client.Do(httpReq)
	if err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		parsedErr := parseUpstreamHTTPError(resp)
		r.recordUpstream(target, parsedErr)
		return domain.ImageGenerationResponse{}, platformerr.Wrap(parsedErr, http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	var decoded openAIImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}
	r.recordUpstream(target, nil)
	images := make([]domain.GeneratedImage, 0, len(decoded.Data))
	for _, item := range decoded.Data {
		images = append(images, domain.GeneratedImage{B64JSON: item.B64JSON, URL: item.URL, RevisedPrompt: item.RevisedPrompt})
	}
	return domain.ImageGenerationResponse{Images: images}, nil
}

// generateImageGeminiNative 复用 Google 原生 generateContent 进行多模态生图与图生图 (Gemini 2.5 / 3.0 / Flash Image)
func (r *ProviderRouter) generateImageGeminiNative(ctx context.Context, target upstreamTarget, req domain.ImageGenerationRequest, model string) (domain.ImageGenerationResponse, error) {
	parts := make([]geminiPart, 0)

	// 图生图：注入参考图作为用户多模态输入 Part
	if req.Mode == "image_to_image" && len(req.ReferenceImages) > 0 {
		for _, ref := range req.ReferenceImages {
			refData, refMime := extractBase64DataAndMime(ref)
			if refData != "" {
				parts = append(parts, geminiPart{
					InlineData: &geminiInlineData{
						MimeType: refMime,
						Data:     refData,
					},
				})
			}
		}
	}

	// 注入文字提示词
	parts = append(parts, geminiPart{Text: req.Prompt})

	var payload geminiImageGenerateRequest
	payload.Contents = []struct {
		Role  string       `json:"role,omitempty"`
		Parts []geminiPart `json:"parts"`
	}{
		{
			Role:  "user",
			Parts: parts,
		},
	}

	outMime := "image/jpeg"
	if req.OutputFormat == "image/png" {
		outMime = "image/png"
	}
	payload.GenerationConfig.ResponseModalities = []string{"IMAGE"}
	payload.GenerationConfig.ResponseMimeType = outMime
	if req.N > 0 {
		payload.GenerationConfig.NumberOfCandidates = req.N
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_marshal_failed", "llm.generation_failed", err.Error())
	}

	endpoint := geminiImageEndpoint(target.baseURL, model, ":generateContent", target.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_request_failed", "llm.generation_failed", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", target.apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_call_failed", "llm.generation_failed", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		parsedErr := parseUpstreamHTTPError(resp)
		r.recordUpstream(target, parsedErr)
		return domain.ImageGenerationResponse{}, platformerr.Wrap(parsedErr, http.StatusInternalServerError, "upstream_error", "llm.generation_failed", resp.StatusCode)
	}

	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text,omitempty"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData,omitempty"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason,omitempty"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		r.recordUpstream(target, err)
		return domain.ImageGenerationResponse{}, platformerr.Internal("llm_decode_failed", "llm.generation_failed", err.Error())
	}
	r.recordUpstream(target, nil)

	images := make([]domain.GeneratedImage, 0)
	revisedPrompt := ""
	for _, cand := range decoded.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" && revisedPrompt == "" {
				revisedPrompt = part.Text
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				images = append(images, domain.GeneratedImage{
					B64JSON:       part.InlineData.Data,
					MimeType:      part.InlineData.MimeType,
					RevisedPrompt: revisedPrompt,
				})
			}
		}
	}

	if len(images) == 0 {
		finishReason := ""
		if len(decoded.Candidates) > 0 && decoded.Candidates[0].FinishReason != "" {
			finishReason = " (finish reason: " + decoded.Candidates[0].FinishReason + ")"
		}
		return domain.ImageGenerationResponse{}, platformerr.New(http.StatusInternalServerError, "upstream_no_image", "Gemini did not return any image"+finishReason)
	}

	return domain.ImageGenerationResponse{
		Images: images,
	}, nil
}

func extractBase64DataAndMime(raw string) (string, string) {
	if strings.HasPrefix(raw, "data:") {
		parts := strings.SplitN(raw, ",", 2)
		if len(parts) == 2 {
			mime := "image/jpeg"
			meta := strings.TrimPrefix(parts[0], "data:")
			if strings.Contains(meta, ";") {
				mime = strings.SplitN(meta, ";", 2)[0]
			}
			return parts[1], mime
		}
	}
	return raw, "image/jpeg"
}

func unsupportedImageProtocol(_ string) error {
	return platformerr.New(http.StatusUnprocessableEntity, "llm.capability_mismatch", "llm.capability_mismatch")
}

func decodeImageReference(raw string) ([]byte, string, error) {
	encoded, mimeType := extractBase64DataAndMime(strings.TrimSpace(raw))
	mimeType = strings.ToLower(mimeType)
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return nil, "", unsupportedImageProtocol("OpenAI 图片编辑参考图格式不受支持")
	}
	contents, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		contents, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(encoded))
	}
	if err != nil || len(contents) == 0 {
		return nil, "", unsupportedImageProtocol("OpenAI 图片编辑参考图不是有效 base64 数据")
	}
	return contents, mimeType, nil
}

func imageFilenameExtension(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func geminiImageEndpoint(baseURL, model, action, apiKey string) string {
	base := strings.TrimRight(baseURL, "/")
	prefix := ""
	if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
		prefix = "/v1beta"
	}
	modelPath := strings.TrimPrefix(model, "models/")
	return fmt.Sprintf("%s%s/models/%s%s?key=%s", base, prefix, url.PathEscape(modelPath), action, url.QueryEscape(apiKey))
}
