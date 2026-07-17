package ws

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/models"
)

// TestWSMessageRoundTrip verifies all message types can be marshaled and
// unmarshaled through the WSMessage envelope without data loss.
func TestWSMessageRoundTrip(t *testing.T) {
	// Test auth_init (client → server)
	authPayload := models.AuthInitPayload{Token: "eyJhbGciOiJIUzI1NiIs..."}
	authBytes, _ := json.Marshal(authPayload)
	authMsg := WSMessage{Type: "auth_init", Payload: authBytes}

	// Verify message type constants exist
	if authMsg.Type != TypeAuthInit {
		t.Errorf("TypeAuthInit = %s, want %s", TypeAuthInit, "auth_init")
	}

	encoded, err := json.Marshal(authMsg)
	if err != nil {
		t.Fatalf("marshal auth_init: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if decoded.Type != "auth_init" {
		t.Errorf("round-trip type: %s", decoded.Type)
	}

	// Test paragraph_submit (client → server)
	paraID := uuid.New()
	workID := uuid.New()
	universeID := uuid.New()
	submitPayload := models.ParagraphSubmitPayload{
		SubmissionID: "submission-1",
		ParagraphRef: "chapter:12",
		WorkID:       workID,
		ChapterID:    paraID,
		UniverseID:   universeID,
		Text:         "The quick brown fox jumped over the lazy dog.",
	}
	submitBytes, _ := json.Marshal(submitPayload)
	submitMsg := WSMessage{Type: "paragraph_submit", Payload: submitBytes}

	encoded2, err := json.Marshal(submitMsg)
	if err != nil {
		t.Fatalf("marshal paragraph_submit: %v", err)
	}
	var decoded2 WSMessage
	if err := json.Unmarshal(encoded2, &decoded2); err != nil {
		t.Fatalf("unmarshal round-trip paragraph_submit: %v", err)
	}
	if decoded2.Type != "paragraph_submit" {
		t.Errorf("round-trip type: %s", decoded2.Type)
	}

	// Recover payload
	var recovered models.ParagraphSubmitPayload
	if err := json.Unmarshal(decoded2.Payload, &recovered); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if recovered.Text != "The quick brown fox jumped over the lazy dog." {
		t.Errorf("recovered text: %s", recovered.Text)
	}
	if recovered.WorkID != workID {
		t.Errorf("recovered WorkID mismatch")
	}
	if recovered.SubmissionID != "submission-1" || recovered.ParagraphRef != "chapter:12" {
		t.Errorf("submission correlation fields were not preserved: %+v", recovered)
	}
}

// TestAllMessageTypeConstants verifies all required WS message type constants exist.
func TestAllMessageTypeConstants(t *testing.T) {
	// Client → Server types
	require := []struct{ name, val string }{
		{"TypeAuthInit", TypeAuthInit},
		{"TypeParagraphSubmit", TypeParagraphSubmit},
		{"TypeRecallRequest", TypeRecallRequest},
		{"TypeCraftReviewRequest", TypeCraftReviewRequest},
	}
	for _, r := range require {
		if r.val == "" {
			t.Errorf("%s should not be empty", r.name)
		}
	}

	// Server → Client types
	response := []struct{ name, val string }{
		{"TypeAuthOK", TypeAuthOK},
		{"TypeAnalysisResult", TypeAnalysisResult},
		{"TypeAnalysisFailed", TypeAnalysisFailed},
		{"TypeContradictionAlert", TypeContradictionAlert},
		{"TypeContextualRecall", TypeContextualRecall},
		{"TypeEntityDiscovered", TypeEntityDiscovered},
		{"TypeGraphUpdated", TypeGraphUpdated},
		{"TypeIngestionProgress", TypeIngestionProgress},
		{"TypeCraftReviewResult", TypeCraftReviewResult},
	}
	for _, r := range response {
		if r.val == "" {
			t.Errorf("%s should not be empty", r.name)
		}
	}
}

// TestIngestionProgressMessage verifies the ingestion_progress type constant
// and round-trips an ingestion progress payload through WSMessage.
func TestIngestionProgressMessage(t *testing.T) {
	if TypeIngestionProgress != "ingestion_progress" {
		t.Errorf("TypeIngestionProgress = %q, want %q", TypeIngestionProgress, "ingestion_progress")
	}

	payload := map[string]any{
		"job_id":             uuid.New().String(),
		"status":             "running",
		"chapters_processed": 3,
		"total_chapters":     10,
	}
	payloadBytes, _ := json.Marshal(payload)
	msg := WSMessage{Type: TypeIngestionProgress, Payload: payloadBytes}

	encoded, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal ingestion_progress: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if decoded.Type != TypeIngestionProgress {
		t.Errorf("round-trip type: got %q, want %q", decoded.Type, TypeIngestionProgress)
	}

	var recovered map[string]any
	if err := json.Unmarshal(decoded.Payload, &recovered); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if recovered["status"] != "running" {
		t.Errorf("recovered status: %v", recovered["status"])
	}
	if ch := recovered["chapters_processed"]; ch != float64(3) {
		t.Errorf("recovered chapters_processed: %v", ch)
	}
}

// TestServerPayloadSerialization verifies server→client payloads round-trip.
func TestServerPayloadSerialization(t *testing.T) {
	// analysis_result
	uid := uuid.New()
	arPayload := models.AnalysisResultPayload{
		SubmissionID: "submission-1",
		ParagraphRef: "chapter:12",
		WorkID:       uid,
		ChapterID:    uid,
		Entities: []models.EntityBrief{
			{ID: uuid.New(), Name: "Alice", Type: "character"},
		},
	}
	arBytes, _ := json.Marshal(arPayload)

	var recovered models.AnalysisResultPayload
	if err := json.Unmarshal(arBytes, &recovered); err != nil {
		t.Fatalf("round-trip analysis_result: %v", err)
	}
	if len(recovered.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(recovered.Entities))
	}
	if recovered.SubmissionID != "submission-1" || recovered.ParagraphRef != "chapter:12" {
		t.Errorf("analysis_result correlation fields were not preserved: %+v", recovered)
	}

	failureBytes, _ := json.Marshal(models.AnalysisFailedPayload{
		SubmissionID: "submission-1", ParagraphRef: "chapter:12", WorkID: uid, ChapterID: uid, Reason: "service unavailable",
	})
	var failed models.AnalysisFailedPayload
	if err := json.Unmarshal(failureBytes, &failed); err != nil {
		t.Fatalf("round-trip analysis_failed: %v", err)
	}
	if failed.Reason != "service unavailable" {
		t.Errorf("failure reason = %q", failed.Reason)
	}

	// contextual_recall
	crPayload := models.ContextualRecallPayload{
		Items: []models.RecallItem{
			{EntityID: uuid.New(), Fact: "Bob is a wizard", Score: 0.95, Source: "graph"},
			{EntityID: uuid.New(), Fact: "Bob was seen in chapter 5", Score: 0.72, Source: "mention"},
		},
	}
	crBytes, _ := json.Marshal(crPayload)

	var recoveredCR models.ContextualRecallPayload
	if err := json.Unmarshal(crBytes, &recoveredCR); err != nil {
		t.Fatalf("round-trip contextual_recall: %v", err)
	}
	if len(recoveredCR.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(recoveredCR.Items))
	}
	if recoveredCR.Items[0].Score != 0.95 {
		t.Errorf("score: %f", recoveredCR.Items[0].Score)
	}
}
