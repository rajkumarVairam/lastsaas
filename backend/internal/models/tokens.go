package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TokenType string

const (
	TokenTypeEmailVerification TokenType = "email_verification"
	TokenTypePasswordReset     TokenType = "password_reset"
	TokenTypeMagicLink         TokenType = "magic_link"
	TokenTypeMFA               TokenType = "mfa_pending"
)

type VerificationToken struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId"`
	Token     string             `json:"-" bson:"token"`
	Type      TokenType          `json:"type" bson:"type"`
	ExpiresAt time.Time          `json:"expiresAt" bson:"expiresAt"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	UsedAt    *time.Time         `json:"usedAt,omitempty" bson:"usedAt,omitempty"`
}

type RefreshToken struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID       primitive.ObjectID `json:"userId" bson:"userId"`
	TokenHash    string             `json:"-" bson:"tokenHash"`
	IPAddress    string             `json:"ipAddress,omitempty" bson:"ipAddress,omitempty"`
	UserAgent    string             `json:"userAgent,omitempty" bson:"userAgent,omitempty"`
	DeviceInfo   string             `json:"deviceInfo,omitempty" bson:"deviceInfo,omitempty"`
	ExpiresAt    time.Time          `json:"expiresAt" bson:"expiresAt"`
	CreatedAt    time.Time          `json:"createdAt" bson:"createdAt"`
	LastActiveAt time.Time          `json:"lastActiveAt" bson:"lastActiveAt"`
	IsRevoked    bool               `json:"isRevoked" bson:"isRevoked"`
}

type RevokedToken struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TokenHash string             `bson:"tokenHash"`
	ExpiresAt time.Time          `bson:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"`
}

type OAuthState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	State     string             `bson:"state"`
	ExpiresAt time.Time          `bson:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"`
}
