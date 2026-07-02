package ws

import (
	"encoding/json"

	"github.com/quill/backend/internal/models"
)

// Message type constants for the WebSocket protocol.
const (
	// Client → Server
	TypeAuthInit        = "auth_init"
	TypeParagraphSubmit = "paragraph_submit"
	TypeRecallRequest   = "recall_request"

	// Server → Client
	TypeAuthOK            = "auth_ok"
	TypeAuthError         = "auth_error"
	TypeAnalysisResult    = "analysis_result"
	TypeContradictionAlert = "contradiction_alert"
	TypeContextualRecall  = "contextual_recall"
	TypeEntityDiscovered   = "entity_discovered"
	TypeGraphUpdated       = "graph_updated"
	TypeIngestionProgress  = "ingestion_progress"
	TypeError              = "error"
)

// WSMessage is the envelope for all WebSocket communication.
// Re-exported from models for the ws package convenience.
type WSMessage = models.WSMessage

// ParseMessage extracts the type from raw JSON bytes.
func ParseMessage(raw []byte) (WSMessage, error) {
	var msg WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return WSMessage{}, err
	}
	return msg, nil
}

// MarshalMessage serializes a WSMessage to JSON bytes.
func MarshalMessage(msg WSMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// NewMessage creates a WSMessage with the given type and payload.
func NewMessage(msgType string, payload interface{}) (WSMessage, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return WSMessage{}, err
	}
	return WSMessage{
		Type:    msgType,
		Payload: payloadBytes,
	}, nil
}
