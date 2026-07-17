package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
)

// DashScopeService is Quill's hand-written native DashScope HTTP client.
// QwenService remains the OpenAI-compatible fallback. Both implement
// LLMService, so domain services and the agent loop are wire-protocol agnostic.
type DashScopeService struct {
	client           *http.Client
	baseURL          string
	apiKey           string
	maxModel         string
	turboModel       string
	embModel         string
	embDims          int
	rerankModel      string
	fallbackModel    string
	fallbackOn429    bool
	maxSem           chan struct{}
	turboSem         chan struct{}
	budgetMgr        *ContextBudgetManager
	throttle         *QwenThrottle
	retryMaxAttempts int
	retrySleep       throttleSleep
	jitter           func(time.Duration) time.Duration

	usageMu sync.Mutex
	usage   LLMUsageSnapshot
}

// NewDashScopeService constructs a native client from the same role-based
// configuration as QwenService. QWEN_NATIVE_BASE_URL accepts either a host
// (https://dashscope-intl.aliyuncs.com) or an /api/v1 URL; the endpoint paths
// are added exactly once.
func NewDashScopeService(cfg *config.Config, budgetMgr *ContextBudgetManager) *DashScopeService {
	if cfg == nil {
		cfg = &config.Config{}
	}
	extractionModel := cfg.QwenExtractionModel
	if extractionModel == "" {
		extractionModel = cfg.QwenTurboModel
	}
	if extractionModel == "" {
		extractionModel = "qwen-turbo"
	}
	reasoningModel := cfg.QwenReasoningModel
	if reasoningModel == "" {
		reasoningModel = cfg.QwenMaxModel
	}
	if reasoningModel == "" {
		reasoningModel = "qwen-max"
	}
	nativeBase := cfg.QwenNativeBaseURL
	if nativeBase == "" {
		nativeBase = cfg.QwenBaseURL
	}
	maxConcurrency := cfg.LLMMaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = maxInt(cfg.QwenMaxConcurrency, cfg.QwenTurboConcurrency)
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	turboConcurrency := cfg.QwenTurboConcurrency
	if turboConcurrency <= 0 {
		turboConcurrency = maxConcurrency
	}
	maxModelConcurrency := cfg.QwenMaxConcurrency
	if maxModelConcurrency <= 0 {
		maxModelConcurrency = maxConcurrency
	}
	turboTPM, maxTPM, rpm, reserve, rampStep := cfg.LLMTPMTurbo, cfg.LLMTPMMax, cfg.LLMRPM, cfg.LLMInteractiveReserve, cfg.LLMRampStep
	if turboTPM <= 0 {
		turboTPM = 5_000_000
	}
	if maxTPM <= 0 {
		maxTPM = 1_000_000
	}
	if rpm <= 0 {
		rpm = 600
	}
	if reserve <= 0 || reserve >= 1 {
		reserve = 0.30
	}
	if rampStep <= 0 {
		rampStep = 1
	}
	retryAttempts := cfg.QwenRetryMaxAttempts
	if retryAttempts <= 0 {
		retryAttempts = 3
	}
	timeout := cfg.QwenAPITimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &DashScopeService{
		client:           &http.Client{Timeout: timeout},
		baseURL:          nativeAPIBaseURL(nativeBase),
		apiKey:           cfg.QwenAPIKey,
		maxModel:         reasoningModel,
		turboModel:       extractionModel,
		embModel:         cfg.QwenEmbeddingModel,
		embDims:          cfg.QwenEmbeddingDims,
		rerankModel:      cfg.QwenRerankModel,
		fallbackModel:    cfg.QwenFallbackModel,
		fallbackOn429:    cfg.QwenFallbackOn429,
		maxSem:           make(chan struct{}, maxModelConcurrency),
		turboSem:         make(chan struct{}, turboConcurrency),
		budgetMgr:        budgetMgr,
		throttle:         newQwenThrottle(turboTPM, maxTPM, rpm, reserve, maxConcurrency, rampStep),
		retryMaxAttempts: retryAttempts,
		retrySleep:       sleepWithContext,
		jitter:           defaultRetryJitter,
	}
}

func nativeAPIBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	base = strings.TrimSuffix(base, "/compatible-mode/v1")
	base = strings.TrimSuffix(base, "/api/v1")
	return base + "/api/v1"
}

func (s *DashScopeService) ContextBudget() *ContextBudgetManager {
	if s == nil {
		return nil
	}
	return s.budgetMgr
}

func (s *DashScopeService) IngestionConcurrency() int {
	if s == nil || s.throttle == nil || s.throttle.gate == nil {
		return 2
	}
	return s.throttle.gate.maxLimit()
}

// LLMUsageSnapshot returns cumulative provider-reported token counts for this
// client. The API key is deliberately absent from the snapshot and logs.
func (s *DashScopeService) UsageSnapshot() LLMUsageSnapshot {
	if s == nil {
		return LLMUsageSnapshot{}
	}
	s.usageMu.Lock()
	defer s.usageMu.Unlock()
	return s.usage
}

func (s *DashScopeService) recordUsage(usage LLMUsageSnapshot) {
	if s == nil {
		return
	}
	s.usageMu.Lock()
	s.usage.InputTokens += usage.InputTokens
	s.usage.OutputTokens += usage.OutputTokens
	s.usage.CachedTokens += usage.CachedTokens
	s.usage.CacheCreationInputTokens += usage.CacheCreationInputTokens
	s.usage.Requests++
	snapshot := s.usage
	s.usageMu.Unlock()
	log.Printf("[dashscope] usage input=%d output=%d cached=%d cache_creation=%d requests=%d", usage.InputTokens, usage.OutputTokens, usage.CachedTokens, usage.CacheCreationInputTokens, snapshot.Requests)
}

type dashScopeInput struct {
	Messages []QwenMessage `json:"messages"`
}

type dashScopeParameters struct {
	ResultFormat      string          `json:"result_format,omitempty"`
	Tools             []QwenTool      `json:"tools,omitempty"`
	ToolChoice        interface{}     `json:"tool_choice,omitempty"`
	ResponseFormat    *ResponseFormat `json:"response_format,omitempty"`
	Stream            bool            `json:"stream,omitempty"`
	IncrementalOutput bool            `json:"incremental_output,omitempty"`
	MaxTokens         int             `json:"max_tokens,omitempty"`
}

type dashScopeChatRequest struct {
	Model      string              `json:"model"`
	Input      dashScopeInput      `json:"input"`
	Parameters dashScopeParameters `json:"parameters,omitempty"`
}

type dashScopeContentBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

type dashScopeWireMessage struct {
	Role       string         `json:"role"`
	Content    interface{}    `json:"content,omitempty"`
	ToolCalls  []QwenToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// MarshalJSON maps the provider-neutral QwenMessage into DashScope's native
// content-block form only when an explicit cache marker is present. Ordinary
// messages remain strings, and the compatible client never sees this method.
func (r dashScopeChatRequest) MarshalJSON() ([]byte, error) {
	wireMessages := make([]dashScopeWireMessage, len(r.Input.Messages))
	for i, message := range r.Input.Messages {
		wire := dashScopeWireMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallID: message.ToolCallID,
		}
		if message.CacheControl != nil && message.CacheControl.Type != "" {
			wire.Content = []dashScopeContentBlock{{
				Type:         "text",
				Text:         message.Content,
				CacheControl: message.CacheControl,
			}}
		}
		wireMessages[i] = wire
	}
	return json.Marshal(struct {
		Model string `json:"model"`
		Input struct {
			Messages []dashScopeWireMessage `json:"messages"`
		} `json:"input"`
		Parameters dashScopeParameters `json:"parameters,omitempty"`
	}{
		Model: r.Model,
		Input: struct {
			Messages []dashScopeWireMessage `json:"messages"`
		}{Messages: wireMessages},
		Parameters: r.Parameters,
	})
}

func markDashScopeStablePrefix(messages []QwenMessage) []QwenMessage {
	marked := append([]QwenMessage(nil), messages...)
	for i := range marked {
		if marked[i].Role != "system" {
			break
		}
		if marked[i].CacheControl == nil {
			marked[i].CacheControl = &CacheControl{Type: "ephemeral"}
		}
	}
	return marked
}

func ensureDashScopeCacheBatchCompatible(messages []QwenMessage, batch bool) error {
	if !batch {
		return nil
	}
	for _, message := range messages {
		if message.CacheControl != nil && message.CacheControl.Type != "" {
			return fmt.Errorf("DashScope context cache cannot be combined with Batch requests")
		}
	}
	return nil
}

type dashScopeEmbeddingRequest struct {
	Model      string                   `json:"model"`
	Input      dashScopeEmbeddingInput  `json:"input"`
	Parameters dashScopeEmbeddingParams `json:"parameters,omitempty"`
}

type dashScopeEmbeddingInput struct {
	Texts []string `json:"texts"`
}

type dashScopeEmbeddingParams struct {
	TextType   string `json:"text_type,omitempty"`
	Dimension  int    `json:"dimension,omitempty"`
	OutputType string `json:"output_type,omitempty"`
}

type dashScopeChatResponse struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Output     struct {
		Text    string `json:"text"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string         `json:"role"`
				Content   string         `json:"content"`
				ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Usage struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens             int `json:"cached_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

type dashScopeEmbeddingResponse struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Output     struct {
		Embeddings []struct {
			Embedding []float32 `json:"embedding"`
			TextIndex int       `json:"text_index"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

type dashScopeRerankRequest struct {
	Model      string                `json:"model"`
	Input      dashScopeRerankInput  `json:"input"`
	Parameters dashScopeRerankParams `json:"parameters,omitempty"`
}

type dashScopeRerankInput struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type dashScopeRerankParams struct {
	TopN            int  `json:"top_n,omitempty"`
	ReturnDocuments bool `json:"return_documents"`
}

type dashScopeRerankResponse struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Output     struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

var dashScopeCanonicalEntityTypes = []string{
	"character", "place", "object", "faction", "event", "world_rule", "plot_arc",
}

var dashScopeRelationshipTypes = []string{
	// The first thirteen are the edge labels seeded by migration 014. The
	// final three preserve legacy prompt/fallback labels already accepted by
	// ingestion and existing corpora.
	"ALLY_OF", "MEMBER_OF", "LOCATED_IN", "OPPOSES", "PARENT_OF", "MENTOR_OF", "CAUSED_BY", "CONTROLS", "IMPRISONED_IN", "CONTROLLED_BY", "DEPENDS_ON", "FLOWS_THROUGH", "MAPS", "ENEMY_OF", "LOCATED_AT", "CO_OCCURS_WITH",
}

func dashScopeEnumValues(values []string) []interface{} {
	result := make([]interface{}, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func dashScopeEntityJSONSchema() map[string]interface{} {
	entity := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":        map[string]interface{}{"type": "string"},
			"aliases":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"type":        map[string]interface{}{"type": "string", "enum": dashScopeEnumValues(dashScopeCanonicalEntityTypes)},
			"status":      map[string]interface{}{"type": "string", "enum": []interface{}{"active", "archived"}},
			"confidence":  map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
			"description": map[string]interface{}{"type": "string"},
			"properties":  map[string]interface{}{"type": "object"},
		},
		"required":             []interface{}{"name", "aliases", "type", "status", "confidence", "description", "properties"},
		"additionalProperties": false,
	}
	properties := map[string]interface{}{
		"characters":        map[string]interface{}{"type": "array", "items": entity},
		"places":            map[string]interface{}{"type": "array", "items": entity},
		"objects":           map[string]interface{}{"type": "array", "items": entity},
		"events":            map[string]interface{}{"type": "array", "items": entity},
		"factions":          map[string]interface{}{"type": "array", "items": entity},
		"world_rules":       map[string]interface{}{"type": "array", "items": entity},
		"plot_developments": map[string]interface{}{"type": "array", "items": entity},
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             []interface{}{"characters", "places", "objects", "events", "factions", "world_rules", "plot_developments"},
		"additionalProperties": false,
	}
}

func dashScopeRelationshipJSONSchema() map[string]interface{} {
	relationship := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"source":     map[string]interface{}{"type": "string"},
			"target":     map[string]interface{}{"type": "string"},
			"type":       map[string]interface{}{"type": "string", "enum": dashScopeEnumValues(dashScopeRelationshipTypes)},
			"properties": map[string]interface{}{"type": "object"},
		},
		"required":             []interface{}{"source", "target", "type", "properties"},
		"additionalProperties": false,
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{"relationships": map[string]interface{}{"type": "array", "items": relationship}},
		"required":             []interface{}{"relationships"},
		"additionalProperties": false,
	}
}

func dashScopeSchemaResponse(name string, schema map[string]interface{}) *ResponseFormat {
	return &ResponseFormat{
		Type: "json_schema",
		JSONSchema: &JSONSchemaResponseFormat{
			Name:   name,
			Schema: schema,
			Strict: true,
		},
	}
}

func dashScopeEntityTypeAllowed(entityType string) bool {
	for _, allowed := range dashScopeCanonicalEntityTypes {
		if entityType == allowed {
			return true
		}
	}
	return false
}

func dashScopeRelationshipTypeAllowed(relType string) bool {
	for _, allowed := range dashScopeRelationshipTypes {
		if relType == allowed {
			return true
		}
	}
	return false
}

func validateDashScopeEntitySlice(items []ExtractedEntity, expectedType string) []ExtractedEntity {
	valid := make([]ExtractedEntity, 0, len(items))
	for _, item := range items {
		if item.Name == "" || !dashScopeEntityTypeAllowed(item.Type) || item.Type != expectedType {
			log.Printf("[dashscope] schema violation: dropping entity payload=%+v expected_type=%s", item, expectedType)
			continue
		}
		valid = append(valid, item)
	}
	return valid
}

func validateDashScopeEntities(entities *ExtractedEntities) *ExtractedEntities {
	if entities == nil {
		return entities
	}
	entities.Characters = validateDashScopeEntitySlice(entities.Characters, "character")
	entities.Places = validateDashScopeEntitySlice(entities.Places, "place")
	entities.Objects = validateDashScopeEntitySlice(entities.Objects, "object")
	entities.Events = validateDashScopeEntitySlice(entities.Events, "event")
	entities.Factions = validateDashScopeEntitySlice(entities.Factions, "faction")
	entities.WorldRules = validateDashScopeEntitySlice(entities.WorldRules, "world_rule")
	entities.PlotDevelopments = validateDashScopeEntitySlice(entities.PlotDevelopments, "plot_arc")
	return entities
}

func validateDashScopeRelationships(items []map[string]interface{}) []map[string]interface{} {
	valid := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		source, sourceOK := item["source"].(string)
		target, targetOK := item["target"].(string)
		relType, typeOK := item["type"].(string)
		if !sourceOK || !targetOK || source == "" || target == "" || !typeOK || !dashScopeRelationshipTypeAllowed(relType) {
			log.Printf("[dashscope] schema violation: dropping relationship payload=%v", item)
			continue
		}
		valid = append(valid, item)
	}
	return valid
}

func (s *DashScopeService) tierForModel(model string) qwenModelTier {
	if model == s.maxModel {
		return qwenMaxTier
	}
	return qwenTurboTier
}

func estimateDashScopeChatTokens(request dashScopeChatRequest) int {
	characters := 0
	for _, message := range request.Input.Messages {
		characters += len(message.Content)
	}
	return maxInt(1, characters/4+1024)
}

func estimateDashScopeEmbeddingTokens(request dashScopeEmbeddingRequest) int {
	characters := 0
	for _, text := range request.Input.Texts {
		characters += len(text)
	}
	return maxInt(1, characters/4)
}

func withDashScopeModel(payload interface{}, model string) interface{} {
	switch request := payload.(type) {
	case dashScopeChatRequest:
		request.Model = model
		return request
	case dashScopeEmbeddingRequest:
		request.Model = model
		return request
	default:
		return payload
	}
}

func (s *DashScopeService) acquireModelSemaphore(ctx context.Context, tier qwenModelTier) (func(), error) {
	if s == nil {
		return nil, fmt.Errorf("dashscope service is nil")
	}
	sem := s.turboSem
	if tier == qwenMaxTier {
		sem = s.maxSem
	}
	if sem == nil {
		return func() {}, nil
	}
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *DashScopeService) sendNativeRequest(ctx context.Context, tier qwenModelTier, model, method, path string, payload interface{}, tokens int, allowFallback bool) (*http.Response, func(bool), error) {
	if s == nil || s.client == nil {
		return nil, nil, fmt.Errorf("dashscope http client is not configured")
	}
	activeModel := model
	for attempt := 0; ; attempt++ {
		requestTier := tier
		if activeModel != model {
			requestTier = s.tierForModel(activeModel)
		}
		requestPayload := withDashScopeModel(payload, activeModel)
		body, err := json.Marshal(requestPayload)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal native request: %w", err)
		}
		var reader io.Reader
		if method != http.MethodGet {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.baseURL, "/")+path, reader)
		if err != nil {
			return nil, nil, fmt.Errorf("create native request: %w", err)
		}
		if method != http.MethodGet {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		if request, ok := requestPayload.(dashScopeChatRequest); ok && request.Parameters.Stream {
			req.Header.Set("X-DashScope-SSE", "enable")
		}

		modelRelease, err := s.acquireModelSemaphore(ctx, requestTier)
		if err != nil {
			return nil, nil, fmt.Errorf("acquire DashScope model concurrency: %w", err)
		}
		quotaRelease, err := s.throttle.acquire(ctx, requestTier, requestClass(ctx), tokens)
		if err != nil {
			modelRelease()
			return nil, nil, fmt.Errorf("acquire DashScope quota: %w", err)
		}
		release := func(success bool) {
			quotaRelease(success)
			modelRelease()
		}
		resp, err := s.client.Do(req)
		if err != nil {
			release(false)
			return nil, nil, fmt.Errorf("call DashScope API: %w", err)
		}
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return resp, release, nil
		}
		responseBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		release(false)
		if resp.StatusCode != http.StatusTooManyRequests {
			return nil, nil, &qwenHTTPError{status: resp.StatusCode, body: string(responseBody)}
		}
		if attempt+1 < s.retryMaxAttempts {
			if err := s.retrySleep(ctx, s.jitter(retryDelay(attempt))); err != nil {
				return nil, nil, err
			}
			continue
		}
		if allowFallback && s.fallbackOn429 && s.fallbackModel != "" && activeModel != s.fallbackModel {
			activeModel, attempt = s.fallbackModel, -1
			continue
		}
		return nil, nil, &qwenHTTPError{status: resp.StatusCode, body: string(responseBody)}
	}
}

func parseDashScopeResponse(body []byte) (QwenResponse, LLMUsageSnapshot, error) {
	var native dashScopeChatResponse
	if err := json.Unmarshal(body, &native); err != nil {
		return QwenResponse{}, LLMUsageSnapshot{}, fmt.Errorf("unmarshal DashScope response: %w", err)
	}
	if native.StatusCode >= http.StatusBadRequest || native.Code != "" {
		if native.Code == "" {
			return QwenResponse{}, LLMUsageSnapshot{}, fmt.Errorf("DashScope API error (status %d): %s", native.StatusCode, native.Message)
		}
		return QwenResponse{}, LLMUsageSnapshot{}, fmt.Errorf("DashScope API error (%s): %s", native.Code, native.Message)
	}
	response := QwenResponse{}
	response.Choices = make([]struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	}, len(native.Output.Choices))
	for i, choice := range native.Output.Choices {
		response.Choices[i].Message.Content = choice.Message.Content
		response.Choices[i].Message.ToolCalls = choice.Message.ToolCalls
	}
	response.Usage.PromptTokens = native.Usage.InputTokens
	response.Usage.CompletionTokens = native.Usage.OutputTokens
	response.Usage.TotalTokens = native.Usage.TotalTokens
	usage := LLMUsageSnapshot{
		InputTokens:              int64(native.Usage.InputTokens),
		OutputTokens:             int64(native.Usage.OutputTokens),
		CachedTokens:             int64(native.Usage.PromptTokensDetails.CachedTokens),
		CacheCreationInputTokens: int64(native.Usage.PromptTokensDetails.CacheCreationInputTokens),
	}
	return response, usage, nil
}

func (s *DashScopeService) nativeChat(ctx context.Context, request dashScopeChatRequest) (QwenResponse, error) {
	if err := ensureDashScopeCacheBatchCompatible(request.Input.Messages, false); err != nil {
		return QwenResponse{}, err
	}
	request.Input.Messages = markDashScopeStablePrefix(request.Input.Messages)
	resp, release, err := s.sendNativeRequest(ctx, s.tierForModel(request.Model), request.Model, http.MethodPost, "/services/aigc/text-generation/generation", request, estimateDashScopeChatTokens(request), true)
	if err != nil {
		return QwenResponse{}, err
	}
	defer release(true)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return QwenResponse{}, fmt.Errorf("read DashScope response: %w", err)
	}
	parsed, usage, err := parseDashScopeResponse(body)
	if err != nil {
		return QwenResponse{}, err
	}
	s.recordUsage(usage)
	return parsed, nil
}

func (s *DashScopeService) Chat(ctx context.Context, model string, messages []QwenMessage) (string, error) {
	if model == "" {
		model = s.turboModel
	}
	response, err := s.nativeChat(ctx, dashScopeChatRequest{
		Model:      model,
		Input:      dashScopeInput{Messages: messages},
		Parameters: dashScopeParameters{ResultFormat: "message"},
	})
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in DashScope response")
	}
	return response.Choices[0].Message.Content, nil
}

// ChatStructured sends a native DashScope schema-constrained request. The
// provider-native path carries the descriptor under parameters.response_format
// while callers remain protocol-neutral.
func (s *DashScopeService) ChatStructured(ctx context.Context, model string, messages []QwenMessage, format *ResponseFormat) (string, error) {
	if model == "" {
		model = s.maxModel
	}
	response, err := s.nativeChat(ctx, dashScopeChatRequest{
		Model:      model,
		Input:      dashScopeInput{Messages: messages},
		Parameters: dashScopeParameters{ResultFormat: "message", ResponseFormat: format},
	})
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no choices in structured DashScope response")
	}
	return response.Choices[0].Message.Content, nil
}

// dashScopeExtractionSystemPrompt is the stable prefix for entity extraction:
// schema instructions and universe lore stay byte-identical across paragraphs,
// while the user message carries only the variable paragraph. DashScope's
// explicit context cache requires roughly 1024 tokens; deterministic padding
// keeps short seeded universes cache-eligible without putting paragraph text
// into the cache key.
func dashScopeExtractionSystemPrompt(universeContext string) string {
	prefix := fmt.Sprintf(`You are Quill's narrative entity extraction engine. Return only JSON that matches the supplied entity_extraction schema. Extract named characters, places, objects, events, factions, world_rules, and plot arcs. Use exactly one of the canonical types: %s. Preserve names and aliases exactly as written in the paragraph. Treat the following universe lore as stable context; never rewrite it or infer a different canon:

UNIVERSE LORE:
%s

STABLE SCHEMA GUIDANCE:
Every entity includes name, aliases, type, status, confidence (0..1), description, and properties. Keep category/type aligned (characters=character, places=place, objects=object, events=event, factions=faction, world_rules=world_rule, plot_developments=plot_arc).`, strings.Join(dashScopeCanonicalEntityTypes, ", "), universeContext)
	return padDashScopeCachePrefix(prefix)
}

func padDashScopeCachePrefix(prefix string) string {
	const minCachePrefixBytes = 4096 // approximately DashScope's 1024-token floor
	if len(prefix) >= minCachePrefixBytes {
		return prefix
	}
	const padding = " stable-lore-schema-token"
	remaining := minCachePrefixBytes - len(prefix)
	repetitions := remaining/len(padding) + 1
	return prefix + "\nCACHE_STABLE_PADDING:" + strings.Repeat(padding, repetitions)
}

func (s *DashScopeService) ExtractEntities(ctx context.Context, text, universeContext string) (*ExtractedEntities, error) {
	response, err := s.nativeChat(ctx, dashScopeChatRequest{
		Model: s.turboModel,
		Input: dashScopeInput{Messages: []QwenMessage{
			{Role: "system", Content: dashScopeExtractionSystemPrompt(universeContext)},
			{Role: "user", Content: text},
		}},
		Parameters: dashScopeParameters{ResultFormat: "message", ResponseFormat: dashScopeSchemaResponse("entity_extraction", dashScopeEntityJSONSchema())},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in DashScope response")
	}
	var entities ExtractedEntities
	if err := parseJSONLoose(response.Choices[0].Message.Content, &entities); err != nil {
		return nil, fmt.Errorf("unmarshal entities: %w", err)
	}
	return validateDashScopeEntities(&entities), nil
}

func (s *DashScopeService) AnalyzeRelationships(ctx context.Context, text string, entityNames []string) ([]map[string]interface{}, error) {
	entityList := strings.Join(entityNames, ", ")
	prompt := fmt.Sprintf(`Given this paragraph and the entities mentioned, identify relationships between them.

Paragraph: "%s"

Entities: %s

For every relationship, source and target MUST exactly copy one canonical name from the Entities list above. Do not abbreviate, translate, or invent names.

Return exactly this JSON object:
{"relationships": [{"source": "entity1", "target": "entity2", "type": "ALLY_OF|MEMBER_OF|LOCATED_IN|OPPOSES|PARENT_OF|MENTOR_OF|CAUSED_BY|CONTROLS|IMPRISONED_IN|CONTROLLED_BY|DEPENDS_ON|FLOWS_THROUGH|MAPS|ENEMY_OF|LOCATED_AT|CO_OCCURS_WITH", "properties": {}}]}`, text, entityList)
	response, err := s.nativeChat(ctx, dashScopeChatRequest{
		Model: s.turboModel,
		Input: dashScopeInput{Messages: []QwenMessage{
			{Role: "system", Content: "You analyze narrative relationships. Return only JSON."},
			{Role: "user", Content: prompt},
		}},
		Parameters: dashScopeParameters{ResultFormat: "message", ResponseFormat: dashScopeSchemaResponse("relationship_extraction", dashScopeRelationshipJSONSchema())},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in DashScope response")
	}
	trimmed := strings.TrimSpace(response.Choices[0].Message.Content)
	if strings.HasPrefix(trimmed, "[") {
		var relationships []map[string]interface{}
		if err := parseJSONLoose(trimmed, &relationships); err != nil {
			return nil, fmt.Errorf("unmarshal relationships array: %w", err)
		}
		return validateDashScopeRelationships(relationships), nil
	}
	var result struct {
		Relationships []map[string]interface{} `json:"relationships"`
	}
	if err := parseJSONLoose(trimmed, &result); err != nil {
		return nil, fmt.Errorf("unmarshal relationships object: %w", err)
	}
	return validateDashScopeRelationships(result.Relationships), nil
}

func (s *DashScopeService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	batch, err := s.GenerateEmbeddingBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(batch) == 0 || batch[0] == nil {
		return nil, fmt.Errorf("no embeddings in DashScope response")
	}
	return batch[0], nil
}

func (s *DashScopeService) GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	// DashScope's native text-embedding endpoint accepts at most ten texts per
	// request. Chunking here keeps direct callers safe as well as the ingestion
	// batcher, while preserving input order in the returned slice.
	if len(texts) > 10 {
		result := make([][]float32, len(texts))
		for start := 0; start < len(texts); start += 10 {
			end := start + 10
			if end > len(texts) {
				end = len(texts)
			}
			chunk, err := s.generateEmbeddingBatchChunk(ctx, texts[start:end])
			if err != nil {
				return nil, err
			}
			copy(result[start:end], chunk)
		}
		return result, nil
	}
	return s.generateEmbeddingBatchChunk(ctx, texts)
}

func (s *DashScopeService) generateEmbeddingBatchChunk(ctx context.Context, texts []string) ([][]float32, error) {
	request := dashScopeEmbeddingRequest{
		Model: s.embModel,
		Input: dashScopeEmbeddingInput{Texts: texts},
		Parameters: dashScopeEmbeddingParams{
			TextType:   "document",
			Dimension:  s.embDims,
			OutputType: "dense",
		},
	}
	resp, release, err := s.sendNativeRequest(ctx, qwenTurboTier, request.Model, http.MethodPost, "/services/embeddings/text-embedding/text-embedding", request, estimateDashScopeEmbeddingTokens(request), false)
	if err != nil {
		return nil, err
	}
	defer release(true)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read DashScope embedding response: %w", err)
	}
	var parsed dashScopeEmbeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal DashScope embedding response: %w", err)
	}
	if parsed.StatusCode >= http.StatusBadRequest || parsed.Code != "" {
		if parsed.Code == "" {
			return nil, fmt.Errorf("DashScope embedding API error (status %d): %s", parsed.StatusCode, parsed.Message)
		}
		return nil, fmt.Errorf("DashScope embedding API error (%s): %s", parsed.Code, parsed.Message)
	}
	result := make([][]float32, len(texts))
	for _, item := range parsed.Output.Embeddings {
		if item.TextIndex < 0 || item.TextIndex >= len(result) {
			return nil, fmt.Errorf("DashScope embedding returned invalid text_index %d", item.TextIndex)
		}
		if s.embDims > 0 && len(item.Embedding) != s.embDims {
			return nil, fmt.Errorf("embedding dimension mismatch: model %q returned %d, expected %d (check QWEN_EMBEDDING_DIMENSIONS / QWEN_EMBEDDING_MODEL)", s.embModel, len(item.Embedding), s.embDims)
		}
		result[item.TextIndex] = item.Embedding
	}
	for i, embedding := range result {
		if embedding == nil {
			return nil, fmt.Errorf("DashScope embedding response omitted text_index %d", i)
		}
	}
	s.recordUsage(LLMUsageSnapshot{InputTokens: int64(parsed.Usage.TotalTokens)})
	return result, nil
}

// Rerank calls DashScope's native text-rerank endpoint. Documents are kept in
// caller order and results refer back to their zero-based indexes, allowing
// MemoryService to preserve provider-independent item identity and append any
// omitted tail items deterministically.
func (s *DashScopeService) Rerank(ctx context.Context, query string, documents []string, topN int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(documents) {
		topN = len(documents)
	}
	request := dashScopeRerankRequest{
		Model: s.rerankModel,
		Input: dashScopeRerankInput{Query: query, Documents: documents},
		Parameters: dashScopeRerankParams{
			TopN:            topN,
			ReturnDocuments: false,
		},
	}
	tokens := maxInt(1, (len(query)+len(strings.Join(documents, " ")))/4)
	resp, release, err := s.sendNativeRequest(ctx, qwenTurboTier, request.Model, http.MethodPost, "/services/rerank/text-rerank/text-rerank", request, tokens, false)
	if err != nil {
		return nil, err
	}
	defer release(true)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read DashScope rerank response: %w", err)
	}
	var parsed dashScopeRerankResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal DashScope rerank response: %w", err)
	}
	if parsed.StatusCode >= http.StatusBadRequest || parsed.Code != "" {
		if parsed.Code == "" {
			return nil, fmt.Errorf("DashScope rerank API error (status %d): %s", parsed.StatusCode, parsed.Message)
		}
		return nil, fmt.Errorf("DashScope rerank API error (%s): %s", parsed.Code, parsed.Message)
	}
	results := make([]RerankResult, 0, len(parsed.Output.Results))
	for _, item := range parsed.Output.Results {
		if item.Index < 0 || item.Index >= len(documents) {
			return nil, fmt.Errorf("DashScope rerank returned invalid document index %d", item.Index)
		}
		results = append(results, RerankResult{Index: item.Index, Score: item.RelevanceScore})
	}
	if parsed.Usage.InputTokens != 0 || parsed.Usage.OutputTokens != 0 || parsed.Usage.TotalTokens != 0 {
		s.recordUsage(LLMUsageSnapshot{
			InputTokens:  int64(parsed.Usage.InputTokens),
			OutputTokens: int64(parsed.Usage.OutputTokens),
		})
	}
	return results, nil
}

func (s *DashScopeService) CheckContradictions(ctx context.Context, candidates []ContradictionCandidate) ([]models.Contradiction, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	var prompt strings.Builder
	prompt.WriteString("You are a narrative contradiction detector. Analyze the following entity claims. Return JSON array:\n")
	for i, candidate := range candidates {
		fmt.Fprintf(&prompt, "Candidate %d [%s]: Evidence A: %s | Evidence B: %s\n", i, candidate.Type, candidate.EvidenceA, candidate.EvidenceB)
	}
	prompt.WriteString("\nReturn: [{\"has_contradiction\": true/false, \"entity_index\": int, \"description\": \"...\", \"severity\": \"low|medium|high\", \"suggestion\": \"...\"}]")
	response, err := s.nativeChat(ctx, dashScopeChatRequest{
		Model: s.maxModel,
		Input: dashScopeInput{Messages: []QwenMessage{
			{Role: "system", Content: "You detect narrative contradictions. Return only JSON."},
			{Role: "user", Content: prompt.String()},
		}},
		Parameters: dashScopeParameters{ResultFormat: "message", ResponseFormat: &ResponseFormat{Type: "json_object"}},
	})
	if err != nil {
		return nil, fmt.Errorf("check contradictions: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in DashScope response")
	}
	var results []contradictionResult
	if err := parseJSONLoose(response.Choices[0].Message.Content, &results); err != nil {
		return nil, fmt.Errorf("unmarshal results: %w", err)
	}
	var contradictions []models.Contradiction
	for _, result := range results {
		if !result.HasContradiction || result.EntityIndex < 0 || result.EntityIndex >= len(candidates) {
			continue
		}
		candidate := candidates[result.EntityIndex]
		contradictions = append(contradictions, models.Contradiction{
			ID: uuid.Nil, EntityID: &candidate.EntityID, Severity: result.Severity,
			Description: result.Description, Suggestion: result.Suggestion,
			EvidenceA: candidate.EvidenceA, EvidenceAChapterID: &candidate.ChapterA,
			EvidenceB: candidate.EvidenceB, EvidenceBChapterID: &candidate.ChapterB, Status: "open",
		})
	}
	return contradictions, nil
}

// RunAgentLoop preserves the existing ReAct contract while mapping calls to
// native DashScope's input.messages + parameters.tools wire shape.
func (s *DashScopeService) RunAgentLoop(ctx context.Context, messages []QwenMessage, tools []QwenTool, executor ToolExecutor, maxDepth int) (string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if len(tools) == 0 {
		response, err := s.nativeChat(ctx, dashScopeChatRequest{Model: s.maxModel, Input: dashScopeInput{Messages: messages}, Parameters: dashScopeParameters{ResultFormat: "message"}})
		if err != nil {
			return "", fmt.Errorf("run agent loop: %w", err)
		}
		if len(response.Choices) == 0 {
			return "", nil
		}
		return response.Choices[0].Message.Content, nil
	}
	msgs := append([]QwenMessage(nil), messages...)
	compressed := false
	for depth := 0; depth < maxDepth; depth++ {
		if s.budgetMgr != nil && !compressed {
			msgs, compressed = compressDashScopeToolResults(ctx, s.budgetMgr, msgs, func(summaryContext context.Context, prompt string) (string, error) {
				return s.Chat(summaryContext, s.turboModel, []QwenMessage{
					{Role: "system", Content: "Summarize these tool-call results, preserving every fact, name, date, and relationship relevant to detecting narrative contradictions. Be concise."},
					{Role: "user", Content: prompt},
				})
			})
		}
		response, err := s.nativeChat(ctx, dashScopeChatRequest{
			Model:      s.maxModel,
			Input:      dashScopeInput{Messages: msgs},
			Parameters: dashScopeParameters{ResultFormat: "message", Tools: tools, ToolChoice: "auto"},
		})
		if err != nil {
			return "", fmt.Errorf("run agent loop: %w", err)
		}
		if len(response.Choices) == 0 {
			return "", nil
		}
		message := response.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			return message.Content, nil
		}
		msgs = append(msgs, QwenMessage{Role: "assistant", Content: message.Content, ToolCalls: message.ToolCalls})
		for _, call := range message.ToolCalls {
			if executor == nil {
				return "", fmt.Errorf("run agent loop: tool executor is nil")
			}
			result, execErr := executor.ExecuteTool(call.Function.Name, call.Function.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("error: %v", execErr)
			}
			msgs = append(msgs, QwenMessage{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Content != "" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// ChatCompletionStream exposes native SSE for callers that need it directly.
// The event payload uses the same choices[].delta contract as the existing
// stream parser, while the request itself is native DashScope.
func (s *DashScopeService) ChatCompletionStream(ctx context.Context, payload QwenRequest) (<-chan StreamChunk, error) {
	request := dashScopeChatRequest{
		Model: payload.Model,
		Input: dashScopeInput{Messages: markDashScopeStablePrefix(payload.Messages)},
		Parameters: dashScopeParameters{
			ResultFormat: "message", Tools: payload.Tools, ToolChoice: payload.ToolChoice,
			ResponseFormat: payload.ResponseFormat, Stream: true, IncrementalOutput: true,
		},
	}
	resp, release, err := s.sendNativeRequest(ctx, s.tierForModel(request.Model), request.Model, http.MethodPost, "/services/aigc/text-generation/generation", request, estimateDashScopeChatTokens(request), true)
	if err != nil {
		return nil, err
	}
	ch := make(chan StreamChunk)
	go func() {
		success := readStreamWithUsage(resp.Body, ch, func(usage streamUsage) {
			s.recordUsage(LLMUsageSnapshot{
				InputTokens:              int64(usage.InputTokens),
				OutputTokens:             int64(usage.OutputTokens),
				CachedTokens:             int64(usage.PromptTokensDetails.CachedTokens),
				CacheCreationInputTokens: int64(usage.PromptTokensDetails.CacheCreationInputTokens),
			})
		})
		release(success)
	}()
	return ch, nil
}

func (s *DashScopeService) RunAgentLoopStream(ctx context.Context, messages []QwenMessage, tools []QwenTool, executor ToolExecutor, maxDepth int, onProgress func(stage string, tc *QwenToolCall)) (string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if onProgress == nil {
		onProgress = func(string, *QwenToolCall) {}
	}
	if len(tools) == 0 {
		content, _, err := s.streamOnce(ctx, QwenRequest{Model: s.maxModel, Messages: messages})
		if err != nil {
			return "", fmt.Errorf("run agent loop stream: %w", err)
		}
		return content, nil
	}
	msgs := append([]QwenMessage(nil), messages...)
	compressed := false
	for depth := 0; depth < maxDepth; depth++ {
		if s.budgetMgr != nil && !compressed {
			msgs, compressed = compressDashScopeToolResults(ctx, s.budgetMgr, msgs, func(summaryContext context.Context, prompt string) (string, error) {
				return s.Chat(summaryContext, s.turboModel, []QwenMessage{
					{Role: "system", Content: "Summarize these tool-call results, preserving every fact, name, date, and relationship relevant to detecting narrative contradictions. Be concise."},
					{Role: "user", Content: prompt},
				})
			})
		}
		content, calls, err := s.streamOnce(ctx, QwenRequest{Model: s.maxModel, Messages: msgs, Tools: tools, ToolChoice: "auto"})
		if err != nil {
			return "", fmt.Errorf("run agent loop stream: %w", err)
		}
		if len(calls) == 0 {
			return content, nil
		}
		msgs = append(msgs, QwenMessage{Role: "assistant", Content: content, ToolCalls: calls})
		for i := range calls {
			call := calls[i]
			onProgress("tool_call", &call)
			if executor == nil {
				return "", fmt.Errorf("run agent loop stream: tool executor is nil")
			}
			result, execErr := executor.ExecuteTool(call.Function.Name, call.Function.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("error: %v", execErr)
			}
			msgs = append(msgs, QwenMessage{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Content != "" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// compressDashScopeToolResults mirrors QwenService.compressToolResults. The
// summarizer is injected so native and compatible transports share identical
// budget thresholds and transcript surgery without sharing HTTP code.
func compressDashScopeToolResults(ctx context.Context, budgetMgr *ContextBudgetManager, msgs []QwenMessage, summarize func(context.Context, string) (string, error)) ([]QwenMessage, bool) {
	if budgetMgr == nil {
		return msgs, false
	}
	used := budgetMgr.tok.CountTokensForMessages(msgs)
	usable := budgetMgr.maxContextTokens - budgetMgr.responseReserve
	if used <= usable*8/10 || len(msgs) < 3 {
		return msgs, false
	}

	boundary := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && len(msgs[i].ToolCalls) > 0 {
			boundary = i
			break
		}
	}
	if boundary <= 2 {
		return msgs, false
	}

	head := msgs[0:2]
	tail := msgs[boundary:]
	middle := msgs[2:boundary]
	var oldToolContent strings.Builder
	for _, message := range middle {
		if message.Role == "tool" {
			oldToolContent.WriteString(message.Content)
			oldToolContent.WriteString("\n")
		}
	}
	summary, err := summarize(ctx, oldToolContent.String())
	if err != nil {
		return msgs, true
	}
	compressedMsgs := make([]QwenMessage, 0, len(head)+1+len(tail))
	compressedMsgs = append(compressedMsgs, head...)
	compressedMsgs = append(compressedMsgs, QwenMessage{Role: "tool", ToolCallID: "compressed-context", Content: summary})
	compressedMsgs = append(compressedMsgs, tail...)
	return compressedMsgs, true
}

func (s *DashScopeService) streamOnce(ctx context.Context, payload QwenRequest) (string, []QwenToolCall, error) {
	ch, err := s.ChatCompletionStream(ctx, payload)
	if err != nil {
		return "", nil, err
	}
	var text strings.Builder
	var calls []QwenToolCall
	for chunk := range ch {
		switch chunk.Type {
		case "text":
			text.WriteString(chunk.Text)
		case "tool_call":
			if chunk.ToolCall != nil {
				calls = append(calls, *chunk.ToolCall)
			}
		case "error":
			return "", nil, chunk.Err
		}
	}
	return text.String(), calls, nil
}

// HealthCheck sends the smallest valid native generation request. Native
// DashScope does not document the OpenAI-compatible /models probe, so a
// one-token request validates both authentication and the configured model.
func (s *DashScopeService) HealthCheck(ctx context.Context) error {
	request := dashScopeChatRequest{
		Model:      s.maxModel,
		Input:      dashScopeInput{Messages: []QwenMessage{{Role: "user", Content: "ping"}}},
		Parameters: dashScopeParameters{ResultFormat: "message", MaxTokens: 1},
	}
	resp, release, err := s.sendNativeRequest(ctx, qwenMaxTier, request.Model, http.MethodPost, "/services/aigc/text-generation/generation", request, 1, false)
	if err != nil {
		return fmt.Errorf("call DashScope API: %w", err)
	}
	defer release(true)
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read DashScope health response: %w", readErr)
	}
	var parsed dashScopeChatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("unmarshal DashScope health response: %w", err)
	}
	if parsed.StatusCode >= http.StatusBadRequest || parsed.Code != "" {
		if parsed.Code == "" {
			return fmt.Errorf("DashScope health check failed (status %d): %s", parsed.StatusCode, parsed.Message)
		}
		return fmt.Errorf("DashScope health check failed (%s): %s", parsed.Code, parsed.Message)
	}
	return nil
}

// Compile-time assertions keep protocol drift visible at the implementation
// boundary. They also document that the native service is a drop-in client.
var _ LLMService = (*DashScopeService)(nil)
var _ LLMHealthChecker = (*DashScopeService)(nil)
var _ Reranker = (*DashScopeService)(nil)
