package browserauth

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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
	public, err := url.Parse("https://panel.example")
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 80304, FirstName: "Admin"}, true)
	if err != nil {
		t.Fatal(err)
	}
	return New(store, public, []int64{user.TelegramID}), user
}

func TestBrowserMemberCannotCreateLocalAccountOrAdminSession(t *testing.T) {
	service, _ := testService(t)
	if _, _, err := service.Link(context.Background(), model.User{TelegramID: 999, Role: "admin"}, "/admin"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("member creates session=%v", err)
	}
	if _, err := service.store.UserByTelegramID(context.Background(), 999); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("handoff created nonadmin=%v", err)
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
	_, token, _, err := testBrowserSession(t, service, user)
	if err != nil {
		t.Fatal(err)
	}
	delete(service.admins, user.TelegramID)
	if _, _, err = service.Session(context.Background(), token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("removed administrator retained browser session: %v", err)
	}
}

func testBrowserSession(t *testing.T, service *Service, user model.User) (model.User, string, time.Time, error) {
	t.Helper()
	link, _, err := service.Link(context.Background(), user, "/admin/settings")
	if err != nil {
		return model.User{}, "", time.Time{}, err
	}
	target, err := url.Parse(link)
	if err != nil {
		return model.User{}, "", time.Time{}, err
	}
	fragment, err := url.ParseQuery(target.Fragment)
	if err != nil {
		return model.User{}, "", time.Time{}, err
	}
	identity, token, expires, _, err := service.Exchange(context.Background(), fragment.Get("token"))
	return identity, token, expires, err
}
