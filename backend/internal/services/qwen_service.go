package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
)

type QwenService struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	maxSem     chan struct{}
	turboSem   chan struct{}
	budgetMgr  *ContextBudgetManager
	maxModel   string
	turboModel string
	embModel   string
}

// budgetMgr may be nil — RunAgentLoop then skips tool-result compression
// entirely (current unbounded behavior), which also lets tests construct
// QwenService without a tokenizer.
func NewQwenService(cfg *config.Config, budgetMgr *ContextBudgetManager) *QwenService {
	return &QwenService{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.QwenBaseURL,
		apiKey:     cfg.QwenAPIKey,
		maxSem:     make(chan struct{}, cfg.QwenMaxConcurrency),
		turboSem:   make(chan struct{}, cfg.QwenTurboConcurrency),
		budgetMgr:  budgetMgr,
		maxModel:   cfg.QwenMaxModel,
		turboModel: cfg.QwenTurboModel,
		embModel:   cfg.QwenEmbeddingModel,
	}
}

// HealthCheck performs a lightweight reachability check against the Qwen API.
// Any response that is not a server error is treated as reachable; network
// failures or 5xx responses are reported as unreachable by the caller.
func (s *QwenService) HealthCheck(ctx context.Context) error {
	url := strings.TrimRight(s.baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create health request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("call qwen api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("qwen api returned status %d", resp.StatusCode)
	}
	return nil
}

type QwenRequest struct {
	Model          string          `json:"model"`
	Messages       []QwenMessage   `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Tools          []QwenTool      `json:"tools,omitempty"`
	ToolChoice     interface{}     `json:"tool_choice,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
}

// ResponseFormat requests structured JSON output from the model. DashScope
// requires the prompt to also mention "json" somewhere when this is set.
type ResponseFormat struct {
	Type string `json:"type"`
}

type QwenMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []QwenToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type QwenTool struct {
	Type     string           `json:"type"`
	Function QwenToolFunction `json:"function"`
}

// QwenToolFunction defines a function callable by the model.
type QwenToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// QwenToolCall represents a tool call requested by the model in a response.
type QwenToolCall struct {
	ID       string                   `json:"id"`
	Type     string                   `json:"type"`
	Function QwenToolCallFunction     `json:"function"`
}

// QwenToolCallFunction holds the function name and JSON-encoded arguments
// for a tool call.
type QwenToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolExecutor dispatches tool calls by name. Implementations handle the
// argument parsing and execution for each registered tool.
type ToolExecutor interface {
	ExecuteTool(name string, argsJSON string) (string, error)
}

type QwenResponse struct {
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []QwenToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *QwenService) callWithSemaphore(ctx context.Context, sem chan struct{}, model string, payload interface{}) ([]byte, error) {
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call qwen api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qwen api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (s *QwenService) callEmbedding(ctx context.Context, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call embedding api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type ExtractedEntity struct {
	Name        string                 `json:"name"`
	Aliases     []string               `json:"aliases"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
}

type ExtractedEntities struct {
	Characters       []ExtractedEntity `json:"characters"`
	Places           []ExtractedEntity `json:"places"`
	Events           []ExtractedEntity `json:"events"`
	Factions         []ExtractedEntity `json:"factions"`
	WorldRules       []ExtractedEntity `json:"world_rules"`
	PlotDevelopments []ExtractedEntity `json:"plot_developments"`
}

// Chat sends a raw chat completion request and returns the response content.
// Wraps callWithSemaphore via turboSem for any model and message slice.
func (s *QwenService) Chat(ctx context.Context, model string, messages []QwenMessage) (string, error) {
	payload := QwenRequest{
		Model:    model,
		Messages: messages,
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, model, payload)
	if err != nil {
		return "", err
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return "", fmt.Errorf("unmarshal chat response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in chat response")
	}

	return qwenResp.Choices[0].Message.Content, nil
}

func (s *QwenService) ExtractEntities(ctx context.Context, text string, universeContext string) (*ExtractedEntities, error) {
	prompt := fmt.Sprintf(`You are a narrative analysis AI. Extract ALL named entities from this paragraph.

Context: %s

Paragraph: "%s"

Respond with ONLY valid JSON in this format:
{
  "characters": [{"name": "...", "aliases": [], "type": "character", "status": "active", "description": "...", "properties": {}}],
  "places": [{"name": "...", "type": "place", "description": "...", "properties": {}}],
  "events": [{"name": "...", "type": "event", "description": "...", "properties": {}}],
  "factions": [{"name": "...", "type": "faction", "description": "...", "properties": {}}],
  "world_rules": [{"name": "...", "type": "world_rule", "description": "...", "properties": {}}],
  "plot_developments": [{"name": "...", "type": "plot_arc", "description": "...", "properties": {}}]
}`, universeContext, text)

	payload := QwenRequest{
		Model: s.turboModel,
		Messages: []QwenMessage{
			{Role: "system", Content: "You extract narrative entities. Return only JSON."},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, s.turboModel, payload)
	if err != nil {
		return nil, err
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := qwenResp.Choices[0].Message.Content
	var entities ExtractedEntities
	if err := json.Unmarshal([]byte(content), &entities); err != nil {
		return nil, fmt.Errorf("unmarshal entities: %w", err)
	}

	return &entities, nil
}

func (s *QwenService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	payload := EmbeddingRequest{
		Model: s.embModel,
		Input: []string{text},
	}

	respBody, err := s.callEmbedding(ctx, payload)
	if err != nil {
		return nil, err
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return embResp.Data[0].Embedding, nil
}

func (s *QwenService) GenerateEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := EmbeddingRequest{
		Model: s.embModel,
		Input: texts,
	}

	respBody, err := s.callEmbedding(ctx, payload)
	if err != nil {
		return nil, err
	}

	var embResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal embeddings: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		result[d.Index] = d.Embedding
	}

	return result, nil
}

func (s *QwenService) AnalyzeRelationships(ctx context.Context, text string, entityNames []string) ([]map[string]interface{}, error) {
	entityList := ""
	for i, name := range entityNames {
		if i > 0 {
			entityList += ", "
		}
		entityList += name
	}

	prompt := fmt.Sprintf(`Given this paragraph and the entities mentioned, identify relationships between them.

Paragraph: "%s"

Entities: %s

Return JSON array of relationships:
[{"source": "entity1", "target": "entity2", "type": "ALLY_OF|ENEMY_OF|LOCATED_AT|MEMBER_OF", "properties": {}}]`, text, entityList)

	payload := QwenRequest{
		Model: s.turboModel,
		Messages: []QwenMessage{
			{Role: "system", Content: "You analyze narrative relationships. Return only JSON."},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, s.turboModel, payload)
	if err != nil {
		return nil, err
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := qwenResp.Choices[0].Message.Content
	var relationships []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &relationships); err != nil {
		return nil, fmt.Errorf("unmarshal relationships: %w", err)
	}

	return relationships, nil
}

// ContradictionCandidate represents a potential contradiction to check.
type ContradictionCandidate struct {
	EntityID  uuid.UUID `json:"entity_id"`
	Type      string    `json:"type"` // "deceased_alive", "status_change", "semantic"
	EvidenceA string    `json:"evidence_a"`
	EvidenceB string    `json:"evidence_b"`
	ChapterA  uuid.UUID `json:"chapter_a"`
	ChapterB  uuid.UUID `json:"chapter_b"`
}

// CheckContradictions sends a batch of candidates to Qwen-Max for semantic
// contradiction detection. Limited to MaxContradictionCandidates per call.
//
// ponytail: batch call to Qwen-Max; single round-trip for up to 3 candidates.
func (s *QwenService) CheckContradictions(ctx context.Context, candidates []ContradictionCandidate) ([]models.Contradiction, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// Build prompt
	prompt := fmt.Sprintf("You are a narrative contradiction detector. Analyze the following entity claims. Return JSON array:\n")
	for i, c := range candidates {
		prompt += fmt.Sprintf("Candidate %d [%s]: Evidence A: %s | Evidence B: %s\n", i, c.Type, c.EvidenceA, c.EvidenceB)
	}
	prompt += "\nReturn: [{\"has_contradiction\": true/false, \"entity_index\": int, \"description\": \"...\", \"severity\": \"low|medium|high\", \"suggestion\": \"...\"}]"

	payload := QwenRequest{
		Model: s.maxModel,
		Messages: []QwenMessage{
			{Role: "system", Content: "You detect narrative contradictions. Return only JSON."},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	respBody, err := s.callWithSemaphore(ctx, s.maxSem, s.maxModel, payload)
	if err != nil {
		return nil, fmt.Errorf("check contradictions: %w", err)
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(qwenResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := qwenResp.Choices[0].Message.Content
	var results []contradictionResult
	if err := parseJSONLoose(content, &results); err != nil {
		return nil, fmt.Errorf("unmarshal results: %w", err)
	}

	var contradictions []models.Contradiction
	for _, r := range results {
		if !r.HasContradiction {
			continue
		}
		if r.EntityIndex < 0 || r.EntityIndex >= len(candidates) {
			continue
		}
		c := candidates[r.EntityIndex]
		contra := models.Contradiction{
			ID:                 uuid.Nil,
			UniverseID:         uuid.Nil, // caller must set
			EntityID:           &c.EntityID,
			Severity:           r.Severity,
			Description:        r.Description,
			Suggestion:         r.Suggestion,
			EvidenceA:          c.EvidenceA,
			EvidenceAChapterID: &c.ChapterA,
			EvidenceB:          c.EvidenceB,
			EvidenceBChapterID: &c.ChapterB,
			Status:             "open",
		}
		contradictions = append(contradictions, contra)
	}

	return contradictions, nil
}

// parseJSONLoose unmarshals s into v, falling back to stripping markdown code
// fences (```json ... ``` or ``` ... ```) and retrying once. Defensive
// fallback for models that wrap JSON in fences despite response_format being
// set — the ONE place this fence-stripping logic lives.
func parseJSONLoose(s string, v any) error {
	if err := json.Unmarshal([]byte(s), v); err == nil {
		return nil
	}
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	return json.Unmarshal([]byte(cleaned), v)
}

// contradictionResult mirrors Qwen's JSON response per candidate.
type contradictionResult struct {
	HasContradiction bool   `json:"has_contradiction"`
	EntityIndex      int    `json:"entity_index"`
	Description      string `json:"description"`
	Severity         string `json:"severity"`
	Suggestion       string `json:"suggestion"`
}

// parseContradictionResults decodes the Qwen response JSON into contradiction results.
// Exported for testing.
func parseContradictionResults(raw []byte, candidates []ContradictionCandidate) ([]contradictionResult, error) {
	var results []contradictionResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// RunAgentLoop executes a ReAct loop with function calling. It sends messages
// and tools to the Qwen API, executes tool calls via the executor, and loops
// until a final answer is returned or maxDepth is exhausted.
//
// If tools is nil or empty, RunAgentLoop falls back to a single chat completion
// without the tool-calling loop.
//
// ponytail: single method on QwenService reusing callWithSemaphore + maxSem.
func (s *QwenService) RunAgentLoop(ctx context.Context, messages []QwenMessage, tools []QwenTool, executor ToolExecutor, maxDepth int) (string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Empty tools fallback — single chat completion
	if len(tools) == 0 {
		payload := QwenRequest{
			Model:    s.maxModel,
			Messages: messages,
		}
		respBody, err := s.callWithSemaphore(ctx, s.maxSem, s.maxModel, payload)
		if err != nil {
			return "", fmt.Errorf("run agent loop: %w", err)
		}
		var qwenResp QwenResponse
		if err := json.Unmarshal(respBody, &qwenResp); err != nil {
			return "", fmt.Errorf("unmarshal response: %w", err)
		}
		if len(qwenResp.Choices) == 0 {
			return "", nil
		}
		return qwenResp.Choices[0].Message.Content, nil
	}

	// ponytail: copy messages to avoid mutating caller's slice
	msgs := make([]QwenMessage, len(messages))
	copy(msgs, messages)

	compressed := false

	for depth := 0; depth < maxDepth; depth++ {
		if s.budgetMgr != nil && !compressed {
			msgs, compressed = s.compressToolResults(ctx, msgs)
		}

		payload := QwenRequest{
			Model:      s.maxModel,
			Messages:   msgs,
			Tools:      tools,
			ToolChoice: "auto",
		}

		respBody, err := s.callWithSemaphore(ctx, s.maxSem, s.maxModel, payload)
		if err != nil {
			return "", fmt.Errorf("run agent loop: %w", err)
		}

		var qwenResp QwenResponse
		if err := json.Unmarshal(respBody, &qwenResp); err != nil {
			return "", fmt.Errorf("unmarshal response: %w", err)
		}
		if len(qwenResp.Choices) == 0 {
			return "", nil
		}

		choice := qwenResp.Choices[0]
		msg := choice.Message

		// No tool calls → final answer
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		// Append assistant message with tool calls
		msgs = append(msgs, QwenMessage{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
		})

		// Execute each tool and append results
		for _, tc := range msg.ToolCalls {
			result, execErr := executor.ExecuteTool(tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("error: %v", execErr)
			}
			msgs = append(msgs, QwenMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// Depth exhausted — return the last assistant message content if any
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Content != "" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// RunAgentLoopStream is a streaming variant of RunAgentLoop: it drives each
// iteration via ChatCompletionStream instead of a single-shot call, so
// intermediate progress can be surfaced through onProgress as tool calls
// complete. RunAgentLoop itself is unchanged and remains the default,
// non-streaming path; the two share identical loop semantics and return
// contract (final answer string, same tool-dispatch behavior).
//
// onProgress is called synchronously once per completed tool call, before
// that tool is executed. It may be nil.
func (s *QwenService) RunAgentLoopStream(ctx context.Context, messages []QwenMessage, tools []QwenTool, executor ToolExecutor, maxDepth int, onProgress func(stage string, tc *QwenToolCall)) (string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if onProgress == nil {
		onProgress = func(string, *QwenToolCall) {}
	}

	// Empty tools fallback — single streamed completion, mirrors RunAgentLoop.
	if len(tools) == 0 {
		payload := QwenRequest{
			Model:    s.maxModel,
			Messages: messages,
		}
		content, _, err := s.streamOnce(ctx, payload)
		if err != nil {
			return "", fmt.Errorf("run agent loop stream: %w", err)
		}
		return content, nil
	}

	// ponytail: copy messages to avoid mutating caller's slice (same as RunAgentLoop)
	msgs := make([]QwenMessage, len(messages))
	copy(msgs, messages)

	compressed := false

	for depth := 0; depth < maxDepth; depth++ {
		if s.budgetMgr != nil && !compressed {
			msgs, compressed = s.compressToolResults(ctx, msgs)
		}

		payload := QwenRequest{
			Model:      s.maxModel,
			Messages:   msgs,
			Tools:      tools,
			ToolChoice: "auto",
		}

		content, toolCalls, err := s.streamOnce(ctx, payload)
		if err != nil {
			return "", fmt.Errorf("run agent loop stream: %w", err)
		}

		// No tool calls → final answer
		if len(toolCalls) == 0 {
			return content, nil
		}

		// Append assistant message with tool calls
		msgs = append(msgs, QwenMessage{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		})

		// Execute each completed tool call and append results
		for _, tc := range toolCalls {
			onProgress("tool_call", &tc)
			result, execErr := executor.ExecuteTool(tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				result = fmt.Sprintf("error: %v", execErr)
			}
			msgs = append(msgs, QwenMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// Depth exhausted — return the last assistant message content if any
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && msgs[i].Content != "" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// streamOnce gates on maxSem (matching RunAgentLoop's concurrency limits —
// ChatCompletionStream itself does not gate), drains one ChatCompletionStream
// call to completion, and returns the accumulated assistant text plus any
// completed tool calls.
func (s *QwenService) streamOnce(ctx context.Context, payload QwenRequest) (string, []QwenToolCall, error) {
	select {
	case s.maxSem <- struct{}{}:
		defer func() { <-s.maxSem }()
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}

	ch, err := s.ChatCompletionStream(ctx, payload)
	if err != nil {
		return "", nil, err
	}

	var text strings.Builder
	var toolCalls []QwenToolCall
	for chunk := range ch {
		switch chunk.Type {
		case "text":
			text.WriteString(chunk.Text)
		case "tool_call":
			toolCalls = append(toolCalls, *chunk.ToolCall)
		case "error":
			return "", nil, chunk.Err
		}
	}
	return text.String(), toolCalls, nil
}

// compressToolResults summarizes accumulated tool-call results into a single
// message when the transcript is over 80% of the usable context window,
// keeping the original system+user head and the most recent tool-call
// iteration verbatim (so tool_call_id pairing stays valid). Runs at most
// once per RunAgentLoop call — the returned bool is true once an attempt
// (successful or not) has been made, so the caller stops retrying.
//
// ponytail: best-effort — a failed qwen-turbo call just skips compression,
// it never fails the loop.
func (s *QwenService) compressToolResults(ctx context.Context, msgs []QwenMessage) ([]QwenMessage, bool) {
	used := s.budgetMgr.tok.CountTokensForMessages(msgs)
	usable := s.budgetMgr.maxContextTokens - s.budgetMgr.responseReserve
	if used <= usable*8/10 {
		return msgs, false
	}

	if len(msgs) < 3 {
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
		// No compressible middle yet — keep trying on later iterations.
		return msgs, false
	}

	head := msgs[0:2]
	tail := msgs[boundary:]
	middle := msgs[2:boundary]

	var oldToolContent strings.Builder
	for _, m := range middle {
		if m.Role == "tool" {
			oldToolContent.WriteString(m.Content)
			oldToolContent.WriteString("\n")
		}
	}

	payload := QwenRequest{
		Model: s.turboModel,
		Messages: []QwenMessage{
			{Role: "system", Content: "Summarize these tool-call results, preserving every fact, name, date, and relationship relevant to detecting narrative contradictions. Be concise."},
			{Role: "user", Content: oldToolContent.String()},
		},
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, s.turboModel, payload)
	if err != nil {
		return msgs, true // best-effort: skip compression, don't retry again this call
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(respBody, &qwenResp); err != nil || len(qwenResp.Choices) == 0 {
		return msgs, true
	}
	summary := qwenResp.Choices[0].Message.Content

	compressedMsgs := make([]QwenMessage, 0, len(head)+1+len(tail))
	compressedMsgs = append(compressedMsgs, head...)
	compressedMsgs = append(compressedMsgs, QwenMessage{
		Role:    "assistant",
		Content: "Prior investigation summary:\n" + summary,
	})
	compressedMsgs = append(compressedMsgs, tail...)

	return compressedMsgs, true
}
