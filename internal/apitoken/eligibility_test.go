package apitoken

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type eligibilityRepository struct {
	Repository
	user          model.User
	token         Token
	touches       int
	managementOps int
}

func (r *eligibilityRepository) CreateAdminAPIToken(_ context.Context, token Token) error {
	r.token = token
	r.managementOps++
	return nil
}
func (r *eligibilityRepository) AdminAPITokenByHash(context.Context, []byte) (Token, error) {
	return r.token, nil
}
func (r *eligibilityRepository) UserByID(context.Context, string) (model.User, error) {
	return r.user, nil
}
func (r *eligibilityRepository) TouchAdminAPIToken(context.Context, string, time.Time) error {
	r.touches++
	return nil
}
func (r *eligibilityRepository) AdminAPITokens(context.Context, string) ([]Token, error) {
	r.managementOps++
	return []Token{r.token}, nil
}
func (r *eligibilityRepository) RevokeAdminAPIToken(context.Context, string, string, time.Time) error {
	r.managementOps++
	return nil
}

func TestAPITokensRequireActiveUnblockedAdministrator(t *testing.T) {
	t.Parallel()
	for _, state := range []struct {
		name, role, status string
		blocked            bool
	}{
		{"demoted", "user", "active", false},
		{"purging", "admin", "purging", false},
		{"purged", "admin", "purged", false},
		{"blocked", "admin", "active", true},
		{"unknown status", "admin", "", false},
	} {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			repo := &eligibilityRepository{user: model.User{ID: "admin", Role: "admin", AccountStatus: "active", TelegramID: 42}}
			service := New(repo, []int64{42})
			input := CreateInput{Name: "monitoring", Scopes: []string{"admin:read"}}
			issued, err := service.Create(ctx, repo.user, input)
			if err != nil {
				t.Fatal(err)
			}
			repo.user.Role, repo.user.AccountStatus, repo.user.BotBlocked = state.role, state.status, state.blocked
			if _, err := service.Authenticate(ctx, issued.Token); !errors.Is(err, ErrUnauthorized) || repo.touches != 0 {
				t.Fatal("ineligible current owner authenticated or updated token use", err, repo.touches)
			}
			if _, err := service.Create(ctx, repo.user, input); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("ineligible owner created token", err)
			}
			if _, err := service.List(ctx, repo.user); !errors.Is(err, ErrUnauthorized) {
				t.Fatal("ineligible owner listed token metadata", err)
			}
			if err := service.Revoke(ctx, repo.user, issued.Item.ID); !errors.Is(err, ErrUnauthorized) || repo.managementOps != 1 {
				t.Fatal("ineligible owner mutated token store", err, repo.managementOps)
			}
			repo.user.Role, repo.user.AccountStatus, repo.user.BotBlocked = "admin", "active", false
			principal, err := service.Authenticate(ctx, issued.Token)
			if err != nil || principal.User.ID != repo.user.ID || repo.touches != 1 {
				t.Fatal("restored eligible owner could not authenticate retained token", err)
			}
		})
	}
}
