package browserauth

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/secret"
)

func testService(t *testing.T) (*Service, model.User) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "browser.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	store := database.NewStore(db)
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, err := url.Parse("https://panel.example")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 80304, FirstName: "Admin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	return New(store, vault, nil, public, []int64{user.TelegramID}), user
}

func TestBrowserMemberCannotCreateLocalAccountOrAdminSession(t *testing.T) {
	service, _ := testService(t)
	if _, _, _, err := service.CreateSession(context.Background(), model.TelegramProfile{ID: 999}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("member creates session=%v", err)
	}
	if _, err := service.store.UserByTelegramID(context.Background(), 999); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("OAuth mirrored nonadmin=%v", err)
	}
}

func TestBrowserOAuthSecretIsEncryptedAndPreserved(t *testing.T) {
	service, user := testService(t)
	ctx := context.Background()
	first, err := service.SaveConfig(ctx, user.ID, Config{Enabled: true, ClientID: "12345", ClientSecret: "topsecret"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.store.GetSetting(ctx, configKey)
	if err != nil || !saved.Encrypted || strings.Contains(saved.Value, first.ClientSecret) {
		t.Fatalf("encrypted setting=%+v,err=%v", saved, err)
	}
	second, err := service.SaveConfig(ctx, user.ID, Config{Enabled: true, ClientID: "12345"})
	if err != nil || second.ClientSecret != first.ClientSecret {
		t.Fatalf("secret preservation=%+v,%v", second, err)
	}
	if _, err := service.SaveConfig(ctx, user.ID, Config{Enabled: true, ClientID: "not-a-bot"}); err == nil {
		t.Fatal("invalid bot ID accepted")
	}
}

func TestBrowserDestinationRejectsExternalAndTraversal(t *testing.T) {
	for _, test := range []struct {
		raw   string
		valid bool
	}{{"/admin", true}, {"/admin/users/id?q=1", true}, {"https://evil.example/admin", false}, {"//evil.example/admin", false}, {"/admin/../member", false}, {"/admin/%2e%2e/member", false}, {"/admin\\evil", false}, {"/member", false}, {"/admin#token=leak", false}} {
		t.Run(test.raw, func(t *testing.T) {
			if got := ValidDestination(test.raw); got != test.valid {
				t.Fatalf("valid=%v,want=%v", got, test.valid)
			}
		})
	}
}

func TestBrowserSessionRechecksDeploymentAllowlist(t *testing.T) {
	service, user := testService(t)
	_, token, _, err := service.CreateSession(context.Background(), model.TelegramProfile{ID: user.TelegramID})
	if err != nil {
		t.Fatal(err)
	}
	delete(service.admins, user.TelegramID)
	if _, _, err = service.Session(context.Background(), token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("removed administrator retained browser session: %v", err)
	}
}
