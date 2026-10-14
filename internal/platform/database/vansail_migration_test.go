package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVanSailMigrationPreservesCustomCopyAndBrowserAccess(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 80400)
	var seeded string
	if err := store.db.QueryRowContext(ctx, `SELECT published_json FROM onboarding_content WHERE kind='welcome'`).Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seeded, "VanSail Carpool") {
		t.Fatal("fresh installation retained old branding")
	}
	legacy := strings.ReplaceAll(seeded, "VanSail Carpool", "TX Carpool")
	custom := strings.Replace(legacy, "It only takes a few seconds to complete setup.", "Our onboarding is tailored.", 1)
	if custom == legacy {
		t.Fatal("custom fixture did not change the seed")
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE onboarding_content SET draft_json=?,published_json=?,draft_revision=7,published_revision=6 WHERE kind='welcome'`, custom, legacy); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSetting(ctx, "auth.browser.telegram", "retired-encrypted-credential", true, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	session, link := sha256.Sum256([]byte("existing-device")), sha256.Sum256([]byte("existing-handoff"))
	if err := store.CreateBrowserSession(ctx, session[:], user.ID, now, now.Add(30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateBrowserAccessLink(ctx, link[:], user.ID, "/admin/settings", now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	script, err := migrations.ReadFile("migrations/074_vansail_brand_and_link_auth.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, string(script)); err != nil {
		t.Fatal(err)
	}
	var draft, published string
	var draftRevision, publishedRevision int
	if err = store.db.QueryRowContext(ctx, `SELECT draft_json,published_json,draft_revision,published_revision FROM onboarding_content WHERE kind='welcome'`).Scan(&draft, &published, &draftRevision, &publishedRevision); err != nil {
		t.Fatal(err)
	}
	if draft != custom || published != seeded || draftRevision != 7 || publishedRevision != 6 {
		t.Fatal("migration changed custom copy or document revisions")
	}
	if _, err = store.GetSetting(ctx, "auth.browser.telegram"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("retired credential remains: %v", err)
	}
	if _, _, err = store.BrowserSession(ctx, session[:], now); err != nil {
		t.Fatalf("existing browser device lost: %v", err)
	}
	next := sha256.Sum256([]byte("new-browser-device"))
	if _, _, err = store.ExchangeBrowserAccessLink(ctx, link[:], next[:], now, now.Add(30*24*time.Hour)); err != nil {
		t.Fatalf("existing handoff lost: %v", err)
	}
}
