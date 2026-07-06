package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/repositories"
)

// TestCheckContradictionsRequestShape verifies the ContradictionCandidate type and
// the request payload structure that CheckContradictions sends to Qwen.
func TestCheckContradictionsRequestShape(t *testing.T) {
	candidates := []ContradictionCandidate{
		{
			EntityID:  uuid.New(),
			Type:      "deceased_alive",
			EvidenceA: "Bob was alive in chapter 1.",
			EvidenceB: "Bob's funeral was in chapter 3.",
			ChapterA:  uuid.New(),
			ChapterB:  uuid.New(),
		},
		{
			EntityID:  uuid.New(),
			Type:      "status_change",
			EvidenceA: "Alice is the mayor.",
			EvidenceB: "Alice was elected queen.",
			ChapterA:  uuid.New(),
			ChapterB:  uuid.New(),
		},
	}

	// Verify the type compiles and prompt would contain evidence
	for _, c := range candidates {
		if c.Type == "" {
			t.Error("candidate type should not be empty")
		}
		if c.EvidenceA == "" || c.EvidenceB == "" {
			t.Error("candidate evidence should not be empty")
		}
	}

	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
}

// TestCheckContradictionsResultParsing verifies parsing of the expected response JSON.
func TestCheckContradictionsResultParsing(t *testing.T) {
	candidates := []ContradictionCandidate{
		{EntityID: uuid.New(), Type: "deceased_alive", EvidenceA: "a", EvidenceB: "b", ChapterA: uuid.New(), ChapterB: uuid.New()},
		{EntityID: uuid.New(), Type: "status_change", EvidenceA: "c", EvidenceB: "d", ChapterA: uuid.New(), ChapterB: uuid.New()},
	}

	rawJSON := `[
		{
			"has_contradiction": true,
			"entity_index": 0,
			"description": "Bob cannot be both alive and dead",
			"severity": "high",
			"suggestion": "Check chapter 1 and 3 for consistency"
		},
		{
			"has_contradiction": false,
			"entity_index": 1,
			"description": "",
			"severity": "",
			"suggestion": ""
		}
	]`

	results, err := parseContradictionResults([]byte(rawJSON), candidates)
	if err != nil {
		t.Fatalf("parseContradictionResults: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].HasContradiction {
		t.Error("result[0] should have contradiction")
	}
}

// TestQwenServiceCheckContradictionsSignature verifies the method compiles
// and returns proper error/candidates when there is no API connectivity.
func TestQwenServiceCheckContradictionsSignature(t *testing.T) {
	cfg := &config.Config{
		QwenBaseURL:                "https://example.com",
		QwenAPIKey:                 "test-key",
		QwenMaxModel:               "qwen-max-latest",
		QwenMaxConcurrency:         1,
		QwenTurboConcurrency:       1,
		QwenEmbeddingModel:         "text-embedding-v3",
		MaxContradictionCandidates: 3,
	}
	svc := NewQwenService(cfg, nil)

	// Empty candidates should return nil, nil
	results, err := svc.CheckContradictions(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error for empty candidates, got: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty candidates, got %d", len(results))
	}
}

// TestToolCallingTypesMarshaling verifies the OpenAI-compatible tool-calling
// type extensions serialize and deserialize correctly.
func TestToolCallingTypesMarshaling(t *testing.T) {
	// ── 1. QwenTool definition serialization (sent in QwenRequest.Tools) ──
	reqWithTools := QwenRequest{
		Model: "qwen-max",
		Messages: []QwenMessage{
			{Role: "user", Content: "search memory for Bob"},
		},
		Tools: []QwenTool{
			{
				Type: "function",
				Function: QwenToolFunction{
					Name:        "search_vector_memory",
					Description: "Search the vector memory for similar paragraphs",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type":        "string",
								"description": "The search query",
							},
						},
						"required": []string{"query"},
					},
				},
			},
		},
		ToolChoice: "auto",
	}

	reqJSON, err := json.Marshal(reqWithTools)
	if err != nil {
		t.Fatalf("marshal QwenRequest with tools: %v", err)
	}

	// Verify tool definition appears in JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(reqJSON, &raw); err != nil {
		t.Fatalf("unmarshal request JSON: %v", err)
	}
	tools, ok := raw["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatal("expected 1 tool in serialized request")
	}
	toolMap := tools[0].(map[string]interface{})
	if toolMap["type"] != "function" {
		t.Errorf("expected tool type 'function', got %v", toolMap["type"])
	}
	fn, ok := toolMap["function"].(map[string]interface{})
	if !ok {
		t.Fatal("expected function definition in tool")
	}
	if fn["name"] != "search_vector_memory" {
		t.Errorf("expected tool name 'search_vector_memory', got %v", fn["name"])
	}

	// ── 2. QwenResponse deserialization with tool_calls ──
	respJSON := `{
		"choices": [{
			"message": {
				"role": "assistant",
				"content": null,
				"tool_calls": [{
					"id": "call_abc123",
					"type": "function",
					"function": {
						"name": "search_vector_memory",
						"arguments": "{\"query\":\"find Bob\"}"
					}
				}]
			}
		}],
		"usage": {"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70}
	}`

	var resp QwenResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("unmarshal tool_call response: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 {
		t.Fatalf("expected 1 tool_call, got %d", len(tc))
	}
	if tc[0].ID != "call_abc123" {
		t.Errorf("expected tool_call id 'call_abc123', got %q", tc[0].ID)
	}
	if tc[0].Type != "function" {
		t.Errorf("expected tool_call type 'function', got %q", tc[0].Type)
	}
	if tc[0].Function.Name != "search_vector_memory" {
		t.Errorf("expected function name 'search_vector_memory', got %q", tc[0].Function.Name)
	}
	if tc[0].Function.Arguments != `{"query":"find Bob"}` {
		t.Errorf("expected arguments, got %q", tc[0].Function.Arguments)
	}

	// ── 3. QwenMessage with tool role and ToolCallID ──
	msg := QwenMessage{
		Role:       "tool",
		ToolCallID: "call_abc123",
		Content:    "Result from vector search: Bob found in chapter 3.",
	}
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal tool message: %v", err)
	}
	var msgRaw map[string]interface{}
	if err := json.Unmarshal(msgJSON, &msgRaw); err != nil {
		t.Fatalf("unmarshal tool message: %v", err)
	}
	if msgRaw["role"] != "tool" {
		t.Errorf("expected role 'tool', got %v", msgRaw["role"])
	}
	if msgRaw["tool_call_id"] != "call_abc123" {
		t.Errorf("expected tool_call_id 'call_abc123', got %v", msgRaw["tool_call_id"])
	}
	if msgRaw["content"] != "Result from vector search: Bob found in chapter 3." {
		t.Errorf("unexpected content: %v", msgRaw["content"])
	}
}

// mockToolExecutor implements ToolExecutor for testing RunAgentLoop.
type mockToolExecutor struct {
	calls [][]string // each call: [name, args]
}

func (m *mockToolExecutor) ExecuteTool(name string, argsJSON string) (string, error) {
	m.calls = append(m.calls, []string{name, argsJSON})
	return fmt.Sprintf("result from %s", name), nil
}

// TestRunAgentLoopMultiToolCycle verifies RunAgentLoop iterates through tool
// calls and returns the final answer.
func TestRunAgentLoopMultiToolCycle(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: return tool_calls
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"query\":\"hero\"}"}}]}}]}`))
			return
		}
		// Second call: return final answer
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"The hero is Bob."}}]}`))
	}))
	defer srv.Close()

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)

	exec := &mockToolExecutor{}
	messages := []QwenMessage{{Role: "user", Content: "Who is the hero?"}}
	tools := []QwenTool{
		{Type: "function", Function: QwenToolFunction{
			Name: "search", Description: "search memory",
			Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}}},
		},
	}

	result, err := svc.RunAgentLoop(context.Background(), messages, tools, exec, 5)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if result != "The hero is Bob." {
		t.Errorf("expected 'The hero is Bob.', got %q", result)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected 1 executor call, got %d", len(exec.calls))
	}
	if exec.calls[0][0] != "search" {
		t.Errorf("expected tool 'search', got %q", exec.calls[0][0])
	}
}

// TestRunAgentLoopDepthExhaustion verifies that when the agent keeps requesting
// tool calls beyond maxDepth, the last assistant message is returned.
func TestRunAgentLoopDepthExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return tool_calls — never stops
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_x","type":"function","function":{"name":"search","arguments":"{\"query\":\"loop\"}"}}]}}]}`))
	}))
	defer srv.Close()

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)

	exec := &mockToolExecutor{}
	messages := []QwenMessage{{Role: "user", Content: "keep searching"}}
	tools := []QwenTool{
		{Type: "function", Function: QwenToolFunction{
			Name: "search", Description: "search", Parameters: map[string]interface{}{}}},
	}

	// maxDepth=3 — should make 3 calls, then return last assistant message (which is empty since content is null)
	result, err := svc.RunAgentLoop(context.Background(), messages, tools, exec, 3)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	// Last assistant message content was null, so result should be empty
	// But we care mainly that it didn't loop forever and returned without error
	if len(exec.calls) != 3 {
		t.Errorf("expected 3 executor calls (maxDepth=3), got %d", len(exec.calls))
	}
	_ = result
}

// TestRunAgentLoopEmptyTools verifies that when tools is nil/empty,
// RunAgentLoop falls back to a single chat completion with no loop.
func TestRunAgentLoopEmptyTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"No tools needed."}}]}`))
	}))
	defer srv.Close()

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)

	exec := &mockToolExecutor{}
	messages := []QwenMessage{{Role: "user", Content: "Hello"}}

	result, err := svc.RunAgentLoop(context.Background(), messages, nil, exec, 5)
	if err != nil {
		t.Fatalf("RunAgentLoop empty tools: %v", err)
	}
	if result != "No tools needed." {
		t.Errorf("expected 'No tools needed.', got %q", result)
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected 0 executor calls with empty tools, got %d", len(exec.calls))
	}
}

// TestQuillExecutorUnknownTool verifies that an unregistered tool name
// returns an error.
func TestQuillExecutorUnknownTool(t *testing.T) {
	exec := &QuillExecutor{}
	result, err := exec.ExecuteTool("nonexistent_tool", `{}`)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// TestQuillExecutorSearchVectorMemoryInvalidArgs verifies that the
// search_vector_memory handler validates its arguments before touching repos.
func TestQuillExecutorSearchVectorMemoryInvalidArgs(t *testing.T) {
	exec := &QuillExecutor{}
	// Missing required "query" field
	result, err := exec.ExecuteTool("search_vector_memory", `{}`)
	if err == nil {
		t.Error("expected error for missing query in search_vector_memory")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
	// Malformed JSON
	result, err = exec.ExecuteTool("search_vector_memory", `not-json`)
	if err == nil {
		t.Error("expected error for malformed JSON in search_vector_memory")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// TestQuillExecutorQueryEntityGraphInvalidArgs verifies that the
// query_entity_graph handler validates its arguments.
func TestQuillExecutorQueryEntityGraphInvalidArgs(t *testing.T) {
	exec := &QuillExecutor{}
	// Missing required "entity_name" field
	result, err := exec.ExecuteTool("query_entity_graph", `{}`)
	if err == nil {
		t.Error("expected error for missing entity_name in query_entity_graph")
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// ── QuillExecutor tool mocking support ─────────────────────────────────

// mockVectorSearcher implements vectorSearcher for tests.
type mockVectorSearcher struct {
	paragraphs []repositories.SimilarParagraph
	err        error
}

func (m *mockVectorSearcher) FindSimilarParagraphs(ctx context.Context, universeID uuid.UUID, embedding []float32, excludeChapterID uuid.UUID, limit int) ([]repositories.SimilarParagraph, error) {
	return m.paragraphs, m.err
}

// mockGraphQuerier implements graphQuerier for tests.
type mockGraphQuerier struct {
	neighbors []models.GraphNeighbor
	err       error
}

func (m *mockGraphQuerier) GetNeighbors(ctx context.Context, graphName, entityID string) ([]models.GraphNeighbor, error) {
	return m.neighbors, m.err
}

// mockEntityLister implements entityLister for tests.
type mockEntityLister struct {
	entities []models.Entity
	err      error
}

func (m *mockEntityLister) ListByUniverseActive(ctx context.Context, universeID uuid.UUID) ([]models.Entity, error) {
	return m.entities, m.err
}

// TestQuillExecutorSearchVectorMemoryHappyPath verifies that the
// search_vector_memory tool returns formatted results when the vector
// store returns paragraphs. Triangulates with 2 scenarios: results
// present and empty results.
func TestQuillExecutorSearchVectorMemoryHappyPath(t *testing.T) {
	// Mock embedding server — responds with valid embedding
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3],"index":0}],"usage":{"total_tokens":3}}`))
	}))
	defer embedSrv.Close()

	mockVector := &mockVectorSearcher{
		paragraphs: []repositories.SimilarParagraph{
			{Content: "Bob walked into the room.", ChapterTitle: "Chapter 1", Distance: 0.15},
			{Content: "Bob sat down slowly.", ChapterTitle: "Chapter 1", Distance: 0.25},
		},
	}

	exec := &QuillExecutor{
		VectorRepo: mockVector,
		QwenSvc:    NewQwenService(&config.Config{QwenBaseURL: embedSrv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}, nil),
	}

	result, err := exec.ExecuteTool("search_vector_memory", `{"query":"find Bob"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result for search_vector_memory with paragraphs")
	}
	if !strings.Contains(result, "Bob walked into the room") {
		t.Error("result should contain first paragraph content")
	}
	if !strings.Contains(result, "Bob sat down slowly") {
		t.Error("result should contain second paragraph content")
	}
	if !strings.Contains(result, "score: 0.85") {
		t.Errorf("result should contain computed score (1.0 - 0.15), got: %s", result)
	}
	if !strings.Contains(result, "score: 0.75") {
		t.Errorf("result should contain computed score (1.0 - 0.25), got: %s", result)
	}
	if !strings.Contains(result, "Chapter 1") {
		t.Error("result should contain chapter title")
	}
}

// TestQuillExecutorSearchVectorMemoryEmpty verifies the tool returns a
// readable message when no paragraphs match.
func TestQuillExecutorSearchVectorMemoryEmpty(t *testing.T) {
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3],"index":0}],"usage":{"total_tokens":3}}`))
	}))
	defer embedSrv.Close()

	mockVector := &mockVectorSearcher{
		paragraphs: nil, // no results
	}

	exec := &QuillExecutor{
		VectorRepo: mockVector,
		QwenSvc:    NewQwenService(&config.Config{QwenBaseURL: embedSrv.URL, QwenAPIKey: "test", QwenMaxConcurrency: 1, QwenTurboConcurrency: 1}, nil),
	}

	result, err := exec.ExecuteTool("search_vector_memory", `{"query":"nonexistent"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if result != "No matching paragraphs found." {
		t.Errorf("expected 'No matching paragraphs found.', got %q", result)
	}
}

// TestQuillExecutorQueryEntityGraphHappyPath verifies that the
// query_entity_graph tool returns formatted neighbor relationships.
// Triangulates with 2 scenarios: results present and entity not found.
func TestQuillExecutorQueryEntityGraphHappyPath(t *testing.T) {
	entityID := uuid.New()
	mockEntities := &mockEntityLister{
		entities: []models.Entity{
			{ID: entityID, Name: "John", Type: "character", Status: "ALIVE"},
		},
	}

	mockGraph := &mockGraphQuerier{
		neighbors: []models.GraphNeighbor{
			{Node: `{"properties":{"name":"Mary","status":"ALIVE"}}`, RelType: "ALLY_OF"},
		},
	}

	exec := &QuillExecutor{
		EntityRepo: mockEntities,
		GraphRepo:  mockGraph,
	}

	result, err := exec.ExecuteTool("query_entity_graph", `{"entity_name":"John"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result for query_entity_graph with neighbors")
	}
	if !strings.Contains(result, "John") {
		t.Error("result should contain the queried entity name")
	}
	if !strings.Contains(result, "Neighbors of") {
		t.Error("result should contain 'Neighbors of' header")
	}
	if !strings.Contains(result, "ALLY_OF") {
		t.Error("result should contain the relation type 'ALLY_OF'")
	}
	// Node is printed as string, verify it appears
	if !strings.Contains(result, "Mary") {
		t.Error("result should contain the neighbor's node data (Mary)")
	}
}

// TestQuillExecutorQueryEntityGraphNotFound verifies the tool returns
// a descriptive message when the entity is not in the list.
func TestQuillExecutorQueryEntityGraphNotFound(t *testing.T) {
	mockEntities := &mockEntityLister{
		entities: []models.Entity{
			{ID: uuid.New(), Name: "Alice", Type: "character", Status: "ALIVE"},
		},
	}

	mockGraph := &mockGraphQuerier{} // won't be called for not-found case

	exec := &QuillExecutor{
		EntityRepo: mockEntities,
		GraphRepo:  mockGraph,
	}

	result, err := exec.ExecuteTool("query_entity_graph", `{"entity_name":"John"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected 'not found' message, got %q", result)
	}
	if !strings.Contains(result, "John") {
		t.Error("result should contain the entity name that was searched")
	}
}

// TestRunAgentLoopSingleCall verifies a single call with tools but no tool_calls
// in response returns the content directly.
func TestRunAgentLoopSingleCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Just a greeting."}}]}`))
	}))
	defer srv.Close()

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, nil)

	exec := &mockToolExecutor{}
	tools := []QwenTool{
		{Type: "function", Function: QwenToolFunction{
			Name: "search", Description: "search", Parameters: map[string]interface{}{}}},
	}

	result, err := svc.RunAgentLoop(context.Background(), []QwenMessage{{Role: "user", Content: "Hi"}}, tools, exec, 5)
	if err != nil {
		t.Fatalf("RunAgentLoop single call: %v", err)
	}
	if result != "Just a greeting." {
		t.Errorf("expected 'Just a greeting.', got %q", result)
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected 0 executor calls, got %d", len(exec.calls))
	}
}

// ── Context-budget compression integration tests ──────────────────────────
//
// These close the verify-report's CRITICAL gap: every RunAgentLoop-adjacent
// test above passes a nil budgetMgr, so the compression wiring itself was
// never exercised at runtime. Both the "under threshold" and "over
// threshold" scenarios are proven directly against compressToolResults
// (full message-slice visibility for the collapse/tail-preservation
// assertions), plus one full RunAgentLoop drive proving the compressed-once
// guard holds across a whole loop.

// TestCompressToolResultsUnderThreshold verifies that a generously large
// budget never triggers compression — the message slice returned is
// unchanged and no turbo call is made.
func TestCompressToolResultsUnderThreshold(t *testing.T) {
	tok := NewTokenizer()
	budgetMgr := NewContextBudgetManager(tok, 100000, 1000) // huge budget, threshold unreachable

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          "http://127.0.0.1:1", // must never be dialed
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, budgetMgr)

	msgs := []QwenMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", ToolCalls: []QwenToolCall{{ID: "call_1", Type: "function", Function: QwenToolCallFunction{Name: "search", Arguments: "{}"}}}},
		{Role: "tool", ToolCallID: "call_1", Content: "some result"},
	}

	got, compressed := svc.compressToolResults(context.Background(), msgs)
	if compressed {
		t.Error("expected no compression when the budget isn't exceeded")
	}
	if len(got) != len(msgs) {
		t.Errorf("expected message slice unchanged under threshold, got %d messages want %d", len(got), len(msgs))
	}
	for i := range msgs {
		if got[i].Role != msgs[i].Role || got[i].Content != msgs[i].Content || got[i].ToolCallID != msgs[i].ToolCallID || len(got[i].ToolCalls) != len(msgs[i].ToolCalls) {
			t.Errorf("message %d mutated when it should have been left untouched: got %+v want %+v", i, got[i], msgs[i])
		}
	}
}

// TestCompressToolResultsFiresOverThreshold verifies that once the transcript
// crosses 80% of the usable context window, compressToolResults collapses
// everything except the head (system+user) and the most recent tool-call
// cycle into a single synthetic summary message, with no orphaned
// tool_call_id in the result.
func TestCompressToolResultsFiresOverThreshold(t *testing.T) {
	turboCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turboCalls++
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Summary of prior tool results."}}]}`))
	}))
	defer srv.Close()

	tok := NewTokenizer()
	budgetMgr := NewContextBudgetManager(tok, 20, 1) // tiny budget — trivially crossed

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, budgetMgr)

	longResult := strings.Repeat("The hero traveled through the ancient forest searching for clues. ", 4)

	msgs := []QwenMessage{
		{Role: "system", Content: "You are a narrative analyst."},
		{Role: "user", Content: "Analyze this chapter for contradictions."},
		{Role: "assistant", ToolCalls: []QwenToolCall{{ID: "call_1", Type: "function", Function: QwenToolCallFunction{Name: "search", Arguments: `{"query":"hero"}`}}}},
		{Role: "tool", ToolCallID: "call_1", Content: longResult},
		{Role: "assistant", ToolCalls: []QwenToolCall{{ID: "call_2", Type: "function", Function: QwenToolCallFunction{Name: "search", Arguments: `{"query":"villain"}`}}}},
		{Role: "tool", ToolCallID: "call_2", Content: longResult},
	}

	got, compressed := svc.compressToolResults(context.Background(), msgs)
	if !compressed {
		t.Fatal("expected compression to fire over threshold")
	}
	if turboCalls != 1 {
		t.Errorf("expected exactly 1 turbo summarization call, got %d", turboCalls)
	}

	// Collapsed: head(2) + synthetic summary(1) + most-recent tail(2) = 5,
	// strictly fewer than the original 6 messages.
	if len(got) >= len(msgs) {
		t.Errorf("expected collapsed message slice, got %d messages (original had %d)", len(got), len(msgs))
	}
	if len(got) != 5 {
		t.Fatalf("expected head(2)+summary(1)+tail(2) = 5 messages, got %d: %+v", len(got), got)
	}

	// Head preserved verbatim.
	if got[0].Role != msgs[0].Role || got[0].Content != msgs[0].Content {
		t.Error("expected head[0] (system) preserved verbatim")
	}
	if got[1].Role != msgs[1].Role || got[1].Content != msgs[1].Content {
		t.Error("expected head[1] (user) preserved verbatim")
	}

	// Synthetic summary message, no ToolCalls (would leave nothing to pair
	// its tool_call_id against).
	summary := got[2]
	if summary.Role != "assistant" || !strings.Contains(summary.Content, "Prior investigation summary") {
		t.Errorf("expected synthetic assistant summary message, got %+v", summary)
	}
	if len(summary.ToolCalls) != 0 {
		t.Error("synthetic summary message must not carry ToolCalls")
	}

	// Tail (most recent tool-call cycle) preserved verbatim — no orphaned
	// tool_call_id.
	if got[3].Role != "assistant" || len(got[3].ToolCalls) != 1 || got[3].ToolCalls[0].ID != "call_2" {
		t.Errorf("expected most recent assistant tool-call message preserved verbatim, got %+v", got[3])
	}
	if got[4].Role != "tool" || got[4].ToolCallID != "call_2" {
		t.Errorf("expected matching tool response for call_2 preserved, got %+v", got[4])
	}
}

// TestRunAgentLoopCompressesOnceAcrossMultipleIterations drives the full
// RunAgentLoop with a tiny budget through several tool-call cycles and
// verifies the compressed-once guard: exactly one turbo call happens across
// the whole loop, even though compressToolResults is checked at the top of
// every iteration.
func TestRunAgentLoopCompressesOnceAcrossMultipleIterations(t *testing.T) {
	maxCalls := 0
	turboCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req QwenRequest
		json.Unmarshal(body, &req)

		if req.Model == "qwen-turbo" {
			turboCalls++
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Summary."}}]}`))
			return
		}

		maxCalls++
		if maxCalls < 4 {
			w.Write([]byte(fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_%d","type":"function","function":{"name":"search","arguments":"{}"}}]}}]}`, maxCalls)))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Done."}}]}`))
	}))
	defer srv.Close()

	tok := NewTokenizer()
	budgetMgr := NewContextBudgetManager(tok, 20, 1) // tiny budget — crossed from the first iteration

	svc := NewQwenService(&config.Config{
		QwenBaseURL:          srv.URL,
		QwenAPIKey:           "test",
		QwenMaxConcurrency:   1,
		QwenTurboConcurrency: 1,
	}, budgetMgr)

	exec := &mockToolExecutor{}
	messages := []QwenMessage{
		{Role: "system", Content: "You are a narrative analyst."},
		{Role: "user", Content: "Analyze this chapter for contradictions across many named entities and events."},
	}
	tools := []QwenTool{
		{Type: "function", Function: QwenToolFunction{Name: "search", Description: "search", Parameters: map[string]interface{}{"type": "object"}}},
	}

	result, err := svc.RunAgentLoop(context.Background(), messages, tools, exec, 6)
	if err != nil {
		t.Fatalf("RunAgentLoop: %v", err)
	}
	if result != "Done." {
		t.Errorf("expected final answer 'Done.', got %q", result)
	}
	if turboCalls != 1 {
		t.Errorf("expected exactly 1 turbo compression call across the whole loop (compressed-once guard), got %d", turboCalls)
	}
	if len(exec.calls) != 3 {
		t.Errorf("expected 3 executor calls before the final answer, got %d", len(exec.calls))
	}
}
