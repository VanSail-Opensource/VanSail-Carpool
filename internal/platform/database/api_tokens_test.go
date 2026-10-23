package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
)

func TestAPITokenHashOnlyStorageExpiryAndOwnerRevocation(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 81701)
	other := createTestUser(t, store, 81702)
	for _, id := range []string{user.ID, other.ID} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, id); err != nil {
			t.Fatal(err)
		}
	}
	user.Role, other.Role = "admin", "admin"
	service := apitoken.New(store, []int64{user.TelegramID, other.TelegramID})
	issued, err := service.Create(ctx, user, apitoken.CreateInput{Name: "scoped", Scopes: []string{"xnode:write"}})
	if err != nil {
		t.Fatal(err)
	}
	var hash []byte
	if err = store.DB().QueryRowContext(ctx, `SELECT token_hash FROM admin_api_tokens WHERE id=?`, issued.Item.ID).Scan(&hash); err != nil || len(hash) != 32 {
		t.Fatalf("hash=%x,%v", hash, err)
	}
	if _, err = service.Authenticate(ctx, issued.Token); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(ctx, user)
	if err != nil || len(items) != 1 || items[0].LastUsedAt == nil {
		t.Fatalf("metadata=%+v,%v", items, err)
	}
	encoded, _ := json.Marshal(items)
	if strings.Contains(string(encoded), issued.Token) || strings.Contains(string(encoded), `"Hash"`) {
		t.Fatal("token secret/hash exposed")
	}
	if err = service.Revoke(ctx, other, issued.Item.ID); !errors.Is(err, apitoken.ErrNotFound) {
		t.Fatalf("foreign revocation=%v", err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE admin_api_tokens SET expires_at='2000-01-01T00:00:00Z' WHERE id=?`, issued.Item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, issued.Token); !errors.Is(err, apitoken.ErrUnauthorized) {
		t.Fatalf("expired accepted: %v", err)
	}
	var audit string
	if err = store.DB().QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='api_token.created' AND target_id=?`, issued.Item.ID).Scan(&audit); err != nil || strings.Contains(audit, issued.Token) {
		t.Fatalf("audit leaked secret: %v", err)
	}
}

func TestAPITokenActiveLimitReleasesAfterIdempotentRevocation(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 81703)
	user.Role = "admin"
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=?`, user.ID); err != nil {
		t.Fatal(err)
	}
	service := apitoken.New(store, []int64{user.TelegramID})
	var first apitoken.Issued
	for i := 0; i < 50; i++ {
		issued, err := service.Create(ctx, user, apitoken.CreateInput{Name: "automation", Scopes: []string{"admin:read"}})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = issued
		}
	}
	if _, err := service.Create(ctx, user, apitoken.CreateInput{Name: "over limit", Scopes: []string{"admin:read"}}); !errors.Is(err, apitoken.ErrLimit) {
		t.Fatalf("active limit bypassed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := service.Revoke(ctx, user, first.Item.ID); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_api_tokens WHERE id=?`, first.Item.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("revoked credential remains: %d %v", count, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE target_type='api_token' AND target_id=? AND action='api_token.revoked'`, first.Item.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("idempotent audit: %d %v", count, err)
	}
	if _, err := service.Authenticate(ctx, first.Token); !errors.Is(err, apitoken.ErrUnauthorized) {
		t.Fatalf("deleted credential authenticated: %v", err)
	}
	if _, err := service.Create(ctx, user, apitoken.CreateInput{Name: "replacement", Scopes: []string{"admin:read"}}); err != nil {
		t.Fatal(err)
	}
}
