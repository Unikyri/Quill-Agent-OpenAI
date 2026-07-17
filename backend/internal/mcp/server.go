// Package mcp exposes Quill's existing memory tools over a small
// JSON-RPC/HTTP transport. It intentionally implements only the MCP methods
// needed by external clients; retrieval remains in services.QuillExecutor and
// services.MemoryService.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/services"
)

type tokenValidator interface {
	ValidateToken(string) (uuid.UUID, error)
}

type universeOwner interface {
	FindByID(context.Context, uuid.UUID) (*models.Universe, error)
}

type embedder interface {
	GenerateEmbedding(context.Context, string) ([]float32, error)
}

type Server struct {
	auth     tokenValidator
	owner    universeOwner
	executor *services.QuillExecutor
	memory   *services.MemoryService
	embedder embedder
}

func NewServer(auth tokenValidator, owner universeOwner, executor *services.QuillExecutor, memory *services.MemoryService, embedder embedder) *Server {
	return &Server{auth: auth, owner: owner, executor: executor, memory: memory, embedder: embedder}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (s *Server) Handle(c *fiber.Ctx) error {
	userID, err := s.authenticate(c.Get("Authorization"))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": fiber.Map{"code": "UNAUTHORIZED", "message": "invalid or missing bearer token"}})
	}
	var req rpcRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil || req.Method == "" {
		return c.Status(fiber.StatusBadRequest).JSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid JSON-RPC request"}})
	}
	if req.JSONRPC != "2.0" {
		return c.JSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "jsonrpc must be 2.0"}})
	}
	if req.Method == "notifications/initialized" {
		// MCP notifications are intentionally fire-and-forget and must not
		// produce a JSON-RPC response body.
		return c.SendStatus(fiber.StatusNoContent)
	}
	result, rpcErr := s.dispatch(c.Context(), userID, req.Method, req.Params)
	response := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
	return c.JSON(response)
}

func (s *Server) authenticate(header string) (uuid.UUID, error) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" || s.auth == nil {
		return uuid.Nil, errors.New("missing bearer token")
	}
	return s.auth.ValidateToken(strings.TrimSpace(parts[1]))
}

func (s *Server) dispatch(ctx context.Context, userID uuid.UUID, method string, raw json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]string{"name": "quill", "version": "1.0"},
		}, nil
	case "notifications/initialized", "ping":
		return map[string]interface{}{}, nil
	case "tools/list":
		return toolDefinitions(), nil
	case "tools/call":
		return s.callTool(ctx, userID, raw)
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func toolDefinitions() map[string]interface{} {
	return map[string]interface{}{"tools": []map[string]interface{}{
		{"name": "search_memory", "description": "Search semantically similar manuscript passages in a universe.", "inputSchema": objectSchema(map[string]interface{}{"universe": stringSchema(), "query": stringSchema()})},
		{"name": "query_entities", "description": "Find an entity and its graph neighbours in a universe.", "inputSchema": objectSchema(map[string]interface{}{"universe": stringSchema(), "name": stringSchema()})},
		{"name": "recall", "description": "Run Quill's hybrid memory recall for a universe.", "inputSchema": objectSchema(map[string]interface{}{"universe": stringSchema(), "query": stringSchema(), "k": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 50}})},
	}}
}

func objectSchema(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": properties, "required": []string{"universe"}, "additionalProperties": false}
}

func stringSchema() map[string]interface{} {
	return map[string]interface{}{"type": "string", "minLength": 1}
}

func (s *Server) callTool(ctx context.Context, userID uuid.UUID, raw json.RawMessage) (interface{}, *rpcError) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || params.Name == "" {
		return nil, &rpcError{Code: -32602, Message: "tools/call requires name and arguments"}
	}
	universeID, err := parseUniverse(params.Arguments)
	if err != nil {
		return nil, &rpcError{Code: -32602, Message: err.Error()}
	}
	if s.owner == nil {
		return nil, &rpcError{Code: -32000, Message: "universe ownership is not configured"}
	}
	universe, err := s.owner.FindByID(ctx, universeID)
	if err != nil || universe == nil {
		return nil, &rpcError{Code: -32004, Message: "universe not found"}
	}
	if universe.UserID != userID {
		return nil, &rpcError{Code: -32003, Message: "universe access denied"}
	}

	var text string
	switch params.Name {
	case "search_memory":
		textValue, ok := params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(textValue) == "" {
			return nil, &rpcError{Code: -32602, Message: "query is required"}
		}
		if s.executor == nil {
			return nil, &rpcError{Code: -32000, Message: "memory executor is not configured"}
		}
		args, _ := json.Marshal(map[string]string{"query": textValue})
		executor := *s.executor
		executor.UniverseID = universeID
		text, err = executor.ExecuteTool("search_vector_memory", string(args))
	case "query_entities":
		name, ok := params.Arguments["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, &rpcError{Code: -32602, Message: "name is required"}
		}
		if s.executor == nil {
			return nil, &rpcError{Code: -32000, Message: "memory executor is not configured"}
		}
		args, _ := json.Marshal(map[string]string{"entity_name": name})
		executor := *s.executor
		executor.UniverseID = universeID
		text, err = executor.ExecuteTool("query_entity_graph", string(args))
	case "recall":
		query, ok := params.Arguments["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return nil, &rpcError{Code: -32602, Message: "query is required"}
		}
		if s.memory == nil || s.embedder == nil {
			return nil, &rpcError{Code: -32000, Message: "memory recall is not configured"}
		}
		k := 10
		if value, ok := params.Arguments["k"].(float64); ok {
			k = int(value)
		}
		if k < 1 {
			k = 1
		}
		if k > 50 {
			k = 50
		}
		embedding, embedErr := s.embedder.GenerateEmbedding(ctx, query)
		if embedErr != nil {
			err = embedErr
			break
		}
		items, recallErr := s.memory.RecallWithQuery(ctx, universeID, embedding, query, k)
		if recallErr != nil {
			err = recallErr
			break
		}
		payload, marshalErr := json.Marshal(items)
		if marshalErr != nil {
			err = marshalErr
			break
		}
		text = string(payload)
	default:
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("unknown tool %q", params.Name)}
	}
	if err != nil {
		// Tool providers may return database/provider details or manuscript
		// content. Keep those details out of the MCP response boundary.
		return nil, &rpcError{Code: -32000, Message: "tool execution failed"}
	}
	return map[string]interface{}{"content": []map[string]string{{"type": "text", "text": text}}, "isError": false}, nil
}

func parseUniverse(args map[string]interface{}) (uuid.UUID, error) {
	value, ok := args["universe"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		if value, ok := args["universe_id"].(string); ok {
			return uuid.Parse(value)
		}
		return uuid.Nil, errors.New("universe is required")
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, errors.New("universe must be a UUID")
	}
	return parsed, nil
}
