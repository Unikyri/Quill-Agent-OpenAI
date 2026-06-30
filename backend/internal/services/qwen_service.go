package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/quill/backend/internal/config"
)

type QwenService struct {
	client   *http.Client
	baseURL  string
	apiKey   string
	maxSem   chan struct{}
	turboSem chan struct{}
}

func NewQwenService(cfg *config.Config) *QwenService {
	return &QwenService{
		client: &http.Client{Timeout: 30 * time.Second},
		baseURL: cfg.QwenBaseURL,
		apiKey: cfg.QwenAPIKey,
		maxSem: make(chan struct{}, cfg.QwenMaxConcurrency),
		turboSem: make(chan struct{}, cfg.QwenTurboConcurrency),
	}
}

type QwenRequest struct {
	Model    string        `json:"model"`
	Messages []QwenMessage `json:"messages"`
	Format   interface{}   `json:"format,omitempty"`
}

type QwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type QwenResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
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
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	Type        string            `json:"type"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
}

type ExtractedEntities struct {
	Characters     []ExtractedEntity `json:"characters"`
	Places         []ExtractedEntity `json:"places"`
	Events         []ExtractedEntity `json:"events"`
	Factions       []ExtractedEntity `json:"factions"`
	WorldRules     []ExtractedEntity `json:"world_rules"`
	PlotDevelopments []ExtractedEntity `json:"plot_developments"`
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
		Model: "qwen-turbo-latest",
		Messages: []QwenMessage{
			{Role: "system", Content: "You extract narrative entities. Return only JSON."},
			{Role: "user", Content: prompt},
		},
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, "qwen-turbo", payload)
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
		Model: "text-embedding-v3",
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
		Model: "text-embedding-v3",
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
		Model: "qwen-turbo-latest",
		Messages: []QwenMessage{
			{Role: "system", Content: "You analyze narrative relationships. Return only JSON."},
			{Role: "user", Content: prompt},
		},
	}

	respBody, err := s.callWithSemaphore(ctx, s.turboSem, "qwen-turbo", payload)
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
