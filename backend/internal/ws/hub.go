package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

// ParagraphSubmitter is the interface that Hub uses to submit paragraphs
// for analysis. AnalysisService satisfies this interface.
type ParagraphSubmitter interface {
	SubmitParagraph(ctx context.Context, workID, chapterID, universeID, userID uuid.UUID, text string) error
}

// RecallRequester is the interface that Hub uses to fetch contextual recall
// results. MemoryService satisfies this interface.
type RecallRequester interface {
	RecallWithQuery(ctx context.Context, universeID uuid.UUID, queryEmbedding []float32, queryText string, k int) ([]models.RecallItem, error)
}

// EmbeddingProvider is the interface that Hub uses to embed query text before
// calling RecallRequester.RecallWithQuery. QwenService satisfies this interface via
// its GenerateEmbedding method.
type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Conn wraps a WebSocket connection with per-user metadata and a write mutex.
type Conn struct {
	wsConn *websocket.Conn
	userID uuid.UUID
	mu     sync.Mutex // per-conn write lock
	done   chan struct{}
}

// Hub manages WebSocket connections keyed by userID.
//
// ponytail: per-user single connection map, no broadcasting needed yet.
// Broadcast(all users) is deferred until multi-user collaboration lands.
type Hub struct {
	conns     map[uuid.UUID]*Conn
	mu        sync.RWMutex
	authSvc   AuthValidator
	submitter ParagraphSubmitter
	recaller  RecallRequester
	embedder  EmbeddingProvider
}

// AuthValidator is the minimal auth interface used by Hub.
// services.AuthService satisfies this interface via ValidateToken.
type AuthValidator interface {
	ValidateToken(token string) (uuid.UUID, error)
}

// NewHub creates a WebSocket hub with optional auth, submitter, recaller, and embedder.
// Any parameter may be nil — the corresponding handler will be a no-op.
func NewHub(authSvc AuthValidator, submitter ParagraphSubmitter, recaller RecallRequester, embedder EmbeddingProvider) *Hub {
	return &Hub{
		conns:     make(map[uuid.UUID]*Conn),
		authSvc:   authSvc,
		submitter: submitter,
		recaller:  recaller,
		embedder:  embedder,
	}
}

// SetSubmitter wires the analysis service into the hub after construction.
// Used to break circular initialization between Hub and AnalysisService.
func (h *Hub) SetSubmitter(s ParagraphSubmitter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.submitter = s
}

// Register adds a connection to the hub for the given user.
func (h *Hub) Register(userID uuid.UUID, conn *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close existing connection for this user if any
	if old, exists := h.conns[userID]; exists {
		close(old.done)
		_ = old.wsConn.Close()
	}
	h.conns[userID] = conn
}

// Remove removes a user's connection from the hub.
func (h *Hub) Remove(userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, userID)
}

// GetConn returns the connection for a userID, or nil if not connected.
func (h *Hub) GetConn(userID uuid.UUID) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[userID]
}

// SendToUser sends a WSMessage to a specific user's connection.
// Returns an error if the user is not connected or the write fails.
func (h *Hub) SendToUser(userID uuid.UUID, msg WSMessage) error {
	conn := h.GetConn(userID)
	if conn == nil {
		return fmt.Errorf("user %s not connected", userID)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return conn.wsConn.WriteMessage(websocket.TextMessage, data)
}

// Handle processes a WebSocket connection lifecycle:
// 1. Wait for auth_init message
// 2. Validate JWT
// 3. Register connection
// 4. Start heartbeat
// 5. Read loop for incoming messages
//
// ponytail: single goroutine per conn; heartbeat every 30s.
func (h *Hub) Handle(wsConn *websocket.Conn) {
	defer wsConn.Close()

	// Phase 1: auth_init handshake
	userID, err := h.handleAuthInit(wsConn)
	if err != nil {
		// Send auth error and close
		errMsg, _ := NewMessage(TypeAuthError, map[string]string{"error": err.Error()})
		if data, merr := json.Marshal(errMsg); merr == nil {
			_ = wsConn.WriteMessage(websocket.TextMessage, data)
		}
		return
	}

	// Send auth_ok
	okMsg, _ := NewMessage(TypeAuthOK, map[string]string{"status": "ok"})
	if data, err := json.Marshal(okMsg); err == nil {
		_ = wsConn.WriteMessage(websocket.TextMessage, data)
	}

	conn := &Conn{
		wsConn: wsConn,
		userID: userID,
		done:   make(chan struct{}),
	}
	h.Register(userID, conn)
	defer h.Remove(userID)

	// Start heartbeat
	go h.heartbeat(conn)

	// Read loop — process incoming messages
	// ponytail: message dispatch via type switch in read loop.
	for {
		msgType, raw, err := wsConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				break
			}
			log.Printf("[ws] read error for user %s: %v", userID, err)
			break
		}

		if msgType != websocket.TextMessage {
			continue
		}

		var msg WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[ws] parse error for user %s: %v", userID, err)
			continue
		}

		// Dispatch by message type
		switch msg.Type {
		case TypeParagraphSubmit:
			h.handleParagraphSubmit(userID, msg)
		case TypeRecallRequest:
			h.handleRecallRequest(userID, msg)
		default:
			log.Printf("[ws] unknown message type from user %s: %s", userID, msg.Type)
		}
	}
}

// handleAuthInit validates the auth_init message and returns the authenticated userID.
func (h *Hub) handleAuthInit(wsConn *websocket.Conn) (uuid.UUID, error) {
	// Read first message — must be auth_init
	wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	msgType, raw, err := wsConn.ReadMessage()
	if err != nil {
		return uuid.Nil, fmt.Errorf("read auth_init: %w", err)
	}
	if msgType != websocket.TextMessage {
		return uuid.Nil, fmt.Errorf("expected text message for auth_init")
	}

	var msg WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return uuid.Nil, fmt.Errorf("parse auth_init: %w", err)
	}
	if msg.Type != TypeAuthInit {
		return uuid.Nil, fmt.Errorf("expected auth_init, got %s", msg.Type)
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return uuid.Nil, fmt.Errorf("parse auth_init payload: %w", err)
	}

	if h.authSvc == nil {
		// No auth service configured — reject all tokens
		return uuid.Nil, fmt.Errorf("auth service not available")
	}

	userID, err := h.authSvc.ValidateToken(payload.Token)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}

	// Clear read deadline for normal operation
	wsConn.SetReadDeadline(time.Time{})
	return userID, nil
}

// heartbeat sends pings every 30 seconds and cleans up on failure.
func (h *Hub) heartbeat(conn *Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			conn.mu.Lock()
			err := conn.wsConn.WriteMessage(websocket.PingMessage, nil)
			conn.mu.Unlock()
			if err != nil {
				return
			}
		case <-conn.done:
			return
		}
	}
}

// handleParagraphSubmit processes a paragraph_submit message.
// Delegates to the ParagraphSubmitter interface (backed by AnalysisService).
func (h *Hub) handleParagraphSubmit(userID uuid.UUID, msg WSMessage) {
	var payload struct {
		WorkID     uuid.UUID `json:"work_id"`
		ChapterID  uuid.UUID `json:"chapter_id"`
		UniverseID uuid.UUID `json:"universe_id"`
		Text       string    `json:"text"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[ws] parse paragraph_submit: %v", err)
		return
	}

	if h.submitter == nil {
		log.Printf("[ws] paragraph_submit: no submitter configured")
		return
	}

	if err := h.submitter.SubmitParagraph(context.Background(), payload.WorkID, payload.ChapterID, payload.UniverseID, userID, payload.Text); err != nil {
		log.Printf("[ws] submit paragraph: %v", err)
	}
}

// handleRecallRequest processes a recall_request message.
// Delegates to the RecallRequester interface (backed by MemoryService).
func (h *Hub) handleRecallRequest(userID uuid.UUID, msg WSMessage) {
	var payload struct {
		UniverseID uuid.UUID `json:"universe_id"`
		Query      string    `json:"query"`
		K          int       `json:"k"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		log.Printf("[ws] parse recall_request: %v", err)
		return
	}

	if payload.K <= 0 {
		payload.K = 5
	}
	if payload.K > 20 {
		payload.K = 20
	}

	if h.recaller == nil {
		log.Printf("[ws] recall_request: no recaller configured")
		return
	}

	// Embed the query string before passing to Recall
	var embedding []float32
	if h.embedder != nil && payload.Query != "" {
		var err error
		embedding, err = h.embedder.GenerateEmbedding(context.Background(), payload.Query)
		if err != nil {
			log.Printf("[ws] embed query: %v", err)
			errMsg, _ := NewMessage(TypeError, map[string]string{"error": "failed to embed query"})
			_ = h.SendToUser(userID, errMsg)
			return
		}
	}

	items, err := h.recaller.RecallWithQuery(context.Background(), payload.UniverseID, embedding, payload.Query, payload.K)
	if err != nil {
		log.Printf("[ws] recall: %v", err)
		errMsg, _ := NewMessage(TypeError, map[string]string{"error": err.Error()})
		_ = h.SendToUser(userID, errMsg)
		return
	}

	resultMsg, _ := NewMessage(TypeContextualRecall, map[string]interface{}{
		"universe_id": payload.UniverseID,
		"items":       items,
	})
	_ = h.SendToUser(userID, resultMsg)
}
