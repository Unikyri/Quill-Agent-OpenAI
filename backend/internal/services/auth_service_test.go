package services

import (
	"testing"

	"github.com/google/uuid"

	"github.com/quill/backend/internal/config"
	"github.com/quill/backend/internal/models"
)

func TestAuthServiceValidateToken(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpirationHours: 1}
	svc := NewAuthService(nil, cfg)

	user := &models.User{ID: uuid.New(), Email: "test@example.com"}
	token, err := svc.generateToken(user)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gotID, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if gotID != user.ID {
		t.Errorf("ValidateToken user_id = %v, want %v", gotID, user.ID)
	}
}

func TestAuthServiceValidateTokenInvalid(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpirationHours: 1}
	svc := NewAuthService(nil, cfg)

	_, err := svc.ValidateToken("not.a.token")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}

func TestAuthServiceValidateTokenWrongSecret(t *testing.T) {
	cfg := &config.Config{JWTSecret: "test-secret", JWTExpirationHours: 1}
	svc := NewAuthService(nil, cfg)

	user := &models.User{ID: uuid.New(), Email: "test@example.com"}
	token, err := svc.generateToken(user)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	otherSvc := NewAuthService(nil, &config.Config{JWTSecret: "other-secret"})
	_, err = otherSvc.ValidateToken(token)
	if err == nil {
		t.Error("expected error for token signed with different secret, got nil")
	}
}
