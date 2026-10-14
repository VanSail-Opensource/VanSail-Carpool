package accounts

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestReserveUsername(t *testing.T) {
	t.Parallel()
	onboardingUser := model.User{ID: "user-1", OnboardingState: "intro"}
	testError := errors.New("failure")
	tests := []struct {
		name, username string
		repository     *accountsRepository
		remote         *accountsRemnawave
		want           error
	}{
		{name: "available", username: " alice ", repository: &accountsRepository{user: onboardingUser}, remote: &accountsRemnawave{}},
		{name: "invalid", username: "ab", repository: &accountsRepository{}, remote: &accountsRemnawave{}},
		{name: "provider lookup failed", username: "alice", repository: &accountsRepository{}, remote: &accountsRemnawave{findResponses: []accountsFindResponse{{err: testError}}}, want: testError},
		{name: "upstream duplicate", username: "alice", repository: &accountsRepository{}, remote: &accountsRemnawave{findResponses: []accountsFindResponse{{exists: true}}}, want: ErrUsernameUnavailable},
		{name: "local duplicate", username: "alice", repository: &accountsRepository{reserveErr: database.ErrConflict}, remote: &accountsRemnawave{}, want: ErrUsernameUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := newAccountsServiceForTest(test.repository, &accountsValidator{}, &accountsTelegram{}, test.remote, &accountsSettings{}, 1).
				ReserveUsername(context.Background(), onboardingUser, test.username)
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("ReserveUsername() error = %v, want %v", err, test.want)
			}
			if test.name == "available" && (err != nil || test.repository.reservedUsername != "alice") {
				t.Fatalf("ReserveUsername() = %v, reserved %q", err, test.repository.reservedUsername)
			}
		})
	}
}

func TestAcceptAgreementCompletesLocallyWithoutProvider(t *testing.T) {
	t.Parallel()
	username := "alice"
	remoteID := "remote-1"
	providerFailure := errors.New("provider is unavailable")
	for _, test := range []struct {
		name string
		user model.User
	}{
		{name: "first signup", user: model.User{ID: "user-1", Username: &username, OnboardingState: "agreement"}},
		{name: "new agreement for linked user", user: model.User{ID: "user-1", Username: &username, RemnaUserID: &remoteID, OnboardingState: "agreement"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{user: model.User{ID: test.user.ID, OnboardingState: "complete", RemnaUserID: test.user.RemnaUserID},
				agreementRevision: 1, requiredAgreementIDs: []string{"terms"}}
			remote := &accountsRemnawave{findResponses: []accountsFindResponse{{err: providerFailure}}, createErr: providerFailure}
			completed, err := newAccountsServiceForTest(repository, &accountsValidator{}, &accountsTelegram{}, remote, &accountsSettings{}, 1).
				AcceptAgreementRevision(context.Background(), test.user, 1, []string{"terms"})
			if err != nil || completed.OnboardingState != "complete" || repository.completedUserID != test.user.ID {
				t.Fatalf("AcceptAgreementRevision() = (%+v, %v), completed ID %q", completed, err, repository.completedUserID)
			}
			if remote.findIndex != 0 || len(remote.created) != 0 {
				t.Fatalf("agreement acceptance called Remnawave: lookups %d creates %d", remote.findIndex, len(remote.created))
			}
		})
	}
}

func TestAcceptAgreementRejectsInvalidLocalState(t *testing.T) {
	t.Parallel()
	username := "alice"
	testError := errors.New("database failure")
	valid := model.User{ID: "user-1", Username: &username, OnboardingState: "agreement"}
	for _, test := range []struct {
		name       string
		user       model.User
		revision   int
		wanted     error
		completion error
	}{
		{name: "missing username", user: model.User{ID: "user-1", OnboardingState: "agreement"}, revision: 1, wanted: ErrAgreementStateConflict},
		{name: "wrong step", user: model.User{ID: "user-1", Username: &username, OnboardingState: "username"}, revision: 1, wanted: ErrAgreementStateConflict},
		{name: "stale revision", user: valid, revision: 2, wanted: ErrAgreementRevisionConflict},
		{name: "storage failure", user: valid, revision: 1, completion: testError, wanted: testError},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{agreementRevision: 1, requiredAgreementIDs: []string{"terms"}, completeErr: test.completion}
			_, err := newAccountsServiceForTest(repository, &accountsValidator{}, &accountsTelegram{}, &accountsRemnawave{}, &accountsSettings{}, 1).
				AcceptAgreementRevision(context.Background(), test.user, test.revision, []string{"terms"})
			if !errors.Is(err, test.wanted) {
				t.Fatalf("AcceptAgreementRevision() = %v, want %v", err, test.wanted)
			}
		})
	}
}
