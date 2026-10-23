package accounts

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
	"time"
)

func TestAuthenticateAndSessionLookup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	profile := model.TelegramProfile{ID: 42, FirstName: "Ada"}
	repository := &accountsRepository{user: model.User{ID: "user-1", TelegramID: profile.ID}}
	validator := &accountsValidator{profile: profile}
	service := newAccountsServiceForTest(repository, validator, &accountsTelegram{}, &accountsProxy{}, &accountsSettings{}, profile.ID)

	user, token, expiresAt, err := service.Authenticate(context.Background(), "signed-init-data")
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}
	if user.ID != "user-1" || token == "" || !expiresAt.Equal(now.Add(7*24*time.Hour)) || !repository.upsertAdmin {
		t.Fatalf("Authenticate() = user %+v, token length %d, expiry %s, admin %t", user, len(token), expiresAt, repository.upsertAdmin)
	}
	wantHash := sha256.Sum256([]byte(token))
	if string(repository.sessionHash) != string(wantHash[:]) || repository.sessionUserID != user.ID || !repository.sessionExpires.Equal(expiresAt) {
		t.Fatal("session was not stored as the expected token hash")
	}

	repository.sessionUser = user
	got, err := service.UserBySession(context.Background(), token)
	if err != nil || got.ID != user.ID || string(repository.lookupSessionHash) != string(wantHash[:]) {
		t.Fatalf("UserBySession() = (%+v, %v)", got, err)
	}
}

func TestAuthenticateMarksConfiguredAdminIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile model.TelegramProfile
		want    bool
	}{
		{name: "first configured admin", profile: model.TelegramProfile{ID: 42}, want: true},
		{name: "second configured admin", profile: model.TelegramProfile{ID: 43}, want: true},
		{name: "ordinary user", profile: model.TelegramProfile{ID: 44}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{user: model.User{ID: "user-1", TelegramID: test.profile.ID}}
			service := NewService(repository, repository, &accountsValidator{profile: test.profile}, &accountsTelegram{}, &accountsProxy{}, &accountsSettings{}, []int64{42, 43}, 7*24*time.Hour)
			if _, _, _, err := service.Authenticate(context.Background(), "signed-init-data"); err != nil {
				t.Fatalf("Authenticate(): %v", err)
			}
			if repository.upsertAdmin != test.want {
				t.Fatalf("upsert admin = %t, want %t", repository.upsertAdmin, test.want)
			}
		})
	}
}

func TestAuthenticationFailures(t *testing.T) {
	t.Parallel()

	testError := errors.New("failure")
	tests := []struct {
		name      string
		configure func(*accountsRepository, *accountsValidator)
		want      error
	}{
		{name: "invalid init data", configure: func(_ *accountsRepository, validator *accountsValidator) { validator.err = testError }, want: ErrInvalidAuthentication},
		{name: "upsert", configure: func(repository *accountsRepository, _ *accountsValidator) { repository.upsertErr = testError }, want: testError},
		{name: "session persistence", configure: func(repository *accountsRepository, _ *accountsValidator) { repository.replaceSessionErr = testError }, want: testError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{user: model.User{ID: "user"}}
			validator := &accountsValidator{profile: model.TelegramProfile{ID: 1}}
			test.configure(repository, validator)
			_, _, _, err := newAccountsServiceForTest(repository, validator, &accountsTelegram{}, &accountsProxy{}, &accountsSettings{}, 99).Authenticate(context.Background(), "raw")
			if !errors.Is(err, test.want) {
				t.Fatalf("Authenticate() error = %v, want %v", err, test.want)
			}
		})
	}

	service := newAccountsServiceForTest(&accountsRepository{sessionLookupErr: testError}, &accountsValidator{}, &accountsTelegram{}, &accountsProxy{}, &accountsSettings{}, 1)
	if _, err := service.UserBySession(context.Background(), ""); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatalf("empty UserBySession() error = %v", err)
	}
	if _, err := service.UserBySession(context.Background(), "token"); !errors.Is(err, ErrInvalidAuthentication) {
		t.Fatalf("failed UserBySession() error = %v", err)
	}
}

func TestAuthenticateRechecksLinkedProxyUser(t *testing.T) {
	t.Parallel()

	remoteID := "remote-1"
	profile := model.TelegramProfile{ID: 42, FirstName: "Ada"}
	upstreamFailure := errors.New("upstream 503")
	tests := []struct {
		name         string
		linked       accountsFindResponse
		wantRecovery bool
		wantUpstream bool
		wantSession  bool
	}{
		{name: "linked user still exists", linked: accountsFindResponse{exists: true}, wantSession: true},
		{name: "confirmed missing starts recovery", linked: accountsFindResponse{}, wantRecovery: true, wantSession: true},
		{name: "temporary failure falls back to local authentication", linked: accountsFindResponse{err: upstreamFailure}, wantUpstream: true, wantSession: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			user := model.User{ID: "user-1", TelegramID: profile.ID, OnboardingState: "complete", ProxyUserID: &remoteID}
			repository := &accountsRepository{user: user, recoveryUser: model.User{
				ID: user.ID, TelegramID: user.TelegramID, OnboardingState: "complete",
			}}
			remote := &accountsProxy{linkedResponse: test.linked}
			_, token, _, err := newAccountsServiceForTest(repository, &accountsValidator{profile: profile}, &accountsTelegram{}, remote, &accountsSettings{}, 99).Authenticate(context.Background(), "signed")
			if test.wantUpstream && (err != nil || token == "" || repository.sessionUserID == "") {
				t.Fatalf("Authenticate() did not fall back to local session: token %q, err %v, stored session %q", token, err, repository.sessionUserID)
			}
			if err != nil || (token != "") != test.wantSession {
				t.Fatalf("Authenticate() = token %q, err %v", token, err)
			}
			if repository.recoveryStarted != test.wantRecovery {
				t.Fatalf("recovery started = %t, want %t", repository.recoveryStarted, test.wantRecovery)
			}
			if test.wantRecovery && repository.recoveryReason != remoteID {
				t.Fatalf("recovery reason = %q", repository.recoveryReason)
			}
		})
	}
}
