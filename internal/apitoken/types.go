// Package apitoken owns scoped administrator credentials for panel automation.
package apitoken

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	ErrUnauthorized = errors.New("API token authentication required")
	ErrInvalid      = errors.New("invalid API token settings")
	ErrLimit        = errors.New("active API token limit reached")
	ErrNotFound     = errors.New("API token not found")
)

type Token struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       []byte     `json:"-"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

type CreateInput struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays *int     `json:"expiresInDays,omitempty"`
}

type Issued struct {
	Item  Token  `json:"item"`
	Token string `json:"token"`
}

type Principal struct {
	Item Token
	User model.User
}

type Repository interface {
	CreateAdminAPIToken(context.Context, Token) error
	AdminAPITokens(context.Context, string) ([]Token, error)
	AdminAPITokenByHash(context.Context, []byte) (Token, error)
	TouchAdminAPIToken(context.Context, string, time.Time) error
	RevokeAdminAPIToken(context.Context, string, string, time.Time) error
	UserByID(context.Context, string) (model.User, error)
}
