package browserauth

import (
	"context"
	"errors"
	"net/url"
	"testing"
)

func TestBrowserExchangeStorageFailurePreservesRetryableLink(t *testing.T) {
	for _, failure := range []string{"owner read", "session insert"} {
		t.Run(failure, func(t *testing.T) {
			service, user := testService(t)
			ctx, token := context.Background(), browserLinkToken(t, service, user.ID)
			var repair string
			if failure == "owner read" {
				if err := service.store.DB().QueryRowContext(ctx, `SELECT updated_at FROM users WHERE id=$1`, user.ID).Scan(&repair); err != nil {
					t.Fatal(err)
				}
				if _, err := service.store.DB().ExecContext(ctx, `UPDATE users SET updated_at='invalid timestamp' WHERE id=$1`, user.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := service.store.DB().ExecContext(ctx, `CREATE FUNCTION reject_browser_session() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'session storage unavailable'; END $$`); err != nil {
					t.Fatal(err)
				}
				if _, err := service.store.DB().ExecContext(ctx, `CREATE TRIGGER reject_browser_session BEFORE INSERT ON sessions FOR EACH ROW EXECUTE FUNCTION reject_browser_session()`); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, _, _, err := service.Exchange(ctx, token); err == nil || errors.Is(err, ErrUnauthorized) {
				t.Fatal("storage failure was reported as an invalid handoff", err)
			}
			assertBrowserExchangeCounts(t, service, 1, 0)
			if failure == "owner read" {
				if _, err := service.store.DB().ExecContext(ctx, `UPDATE users SET updated_at=$1 WHERE id=$2`, repair, user.ID); err != nil {
					t.Fatal(err)
				}
			} else if _, err := service.store.DB().ExecContext(ctx, `DROP TRIGGER reject_browser_session ON sessions`); err != nil {
				t.Fatal(err)
			}
			owner, bearer, _, destination, err := service.Exchange(ctx, token)
			if err != nil || owner.ID != user.ID || len(bearer) != 43 || destination != "/admin/settings" {
				t.Fatal("same handoff could not recover after storage repair", err)
			}
			assertBrowserExchangeCounts(t, service, 0, 1)
			if _, _, _, _, err = service.Exchange(ctx, token); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("successful handoff was reusable", err)
			}
		})
	}
}

func TestBrowserExchangeAuthorizesBeforeConsumingHandoff(t *testing.T) {
	for _, denied := range []string{"allowlist", "role", "destination", "inactive", "blocked"} {
		t.Run(denied, func(t *testing.T) {
			service, user := testService(t)
			ctx, token := context.Background(), browserLinkToken(t, service, user.ID)
			var query string
			switch denied {
			case "allowlist":
				delete(service.admins, user.TelegramID)
			case "role":
				query = `UPDATE users SET role='user'`
			case "destination":
				query = `UPDATE browser_access_links SET return_to='https://outside.example/'`
			case "inactive":
				query = `UPDATE users SET account_status='purging'`
			case "blocked":
				query = `UPDATE users SET bot_blocked=1`
			}
			if query != "" {
				if _, err := service.store.DB().ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, _, _, err := service.Exchange(ctx, token); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("ineligible handoff was not rejected", err)
			}
			assertBrowserExchangeCounts(t, service, 1, 0)
		})
	}
}

func browserLinkToken(t *testing.T, service *Service, userID string) string {
	t.Helper()
	user, err := service.store.UserByID(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	link, _, err := service.Link(context.Background(), user, "/admin/settings")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	return fragment.Get("token")
}

func assertBrowserExchangeCounts(t *testing.T, service *Service, links, sessions int) {
	t.Helper()
	var gotLinks, gotSessions int
	if err := service.store.DB().QueryRowContext(context.Background(), `SELECT (SELECT COUNT(*) FROM browser_access_links),(SELECT COUNT(*) FROM sessions WHERE scope='browser')`).Scan(&gotLinks, &gotSessions); err != nil {
		t.Fatal(err)
	}
	if gotLinks != links || gotSessions != sessions {
		t.Fatal("handoff/session counts changed outside the successful transaction", gotLinks, gotSessions)
	}
}
