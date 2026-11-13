package accounts

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestSessionLookupPreservesTransientFailuresAndSameCookieRecovery(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{errors.New("database temporarily unavailable"), context.DeadlineExceeded, context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{sessionUser: model.User{ID: "member"}, sessionLookupErr: failure}
			service := newAccountsServiceForTest(repository, nil, nil, nil, nil, 1)
			if user, err := service.UserBySession(context.Background(), "retained-cookie"); user.ID != "" || !errors.Is(err, failure) || errors.Is(err, ErrInvalidAuthentication) {
				t.Fatalf("transient lookup = %+v, %v", user, err)
			}
			repository.sessionLookupErr = nil
			user, err := service.UserBySession(context.Background(), "retained-cookie")
			wantHash := sha256.Sum256([]byte("retained-cookie"))
			if err != nil || user.ID != "member" || string(repository.lookupSessionHash) != string(wantHash[:]) || repository.sessionUserID != "" {
				t.Fatalf("same-cookie recovery = %+v, %v; session write = %q", user, err, repository.sessionUserID)
			}
		})
	}
}

func TestSessionLookupRejectsOnlyMissingExpiredOrAbsentCredentials(t *testing.T) {
	t.Parallel()
	for _, missing := range []error{database.ErrNotFound, fmt.Errorf("expired session: %w", database.ErrNotFound)} {
		repository := &accountsRepository{sessionLookupErr: missing}
		service := newAccountsServiceForTest(repository, nil, nil, nil, nil, 1)
		if _, err := service.UserBySession(context.Background(), ""); !errors.Is(err, ErrInvalidAuthentication) || repository.lookupSessionHash != nil {
			t.Fatalf("empty cookie queried storage or was accepted: %v", err)
		}
		if _, err := service.UserBySession(context.Background(), "missing-cookie"); !errors.Is(err, ErrInvalidAuthentication) {
			t.Fatalf("missing/expired cookie was not rejected: %v", err)
		}
	}
}
