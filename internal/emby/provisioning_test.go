package emby

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestProvisionFailureClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		remoteErr  error
		wantRefund int
		wantRetry  int
		wantError  bool
	}{
		{name: "terminal provider rejection refunds", remoteErr: terminalServiceError, wantRefund: 1},
		{name: "transient outage requeues", remoteErr: transientServiceError, wantRetry: 1, wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: ProvisioningRecord{Account: Account{ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued},
				PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user"}}
			remote := newServiceRemote()
			remote.findErr = test.remoteErr
			service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
			err := service.Provision(context.Background(), "account")
			if (err != nil) != test.wantError || repository.refundCalls != test.wantRefund || repository.retryCalls != test.wantRetry {
				t.Fatalf("Provision() error=%v refund=%d retry=%d", err, repository.refundCalls, repository.retryCalls)
			}
		})
	}
}

func TestPasswordErrorsCannotPersistOrReturnEchoedPlaintext(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		cause      error
		wantReview bool
	}{
		{name: "terminal", cause: fmt.Errorf("%w: rejected temporary", terminalServiceError), wantReview: true},
		{name: "transient", cause: fmt.Errorf("%w: rejected temporary", transientServiceError)},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: ProvisioningRecord{Account: Account{
				ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued,
			}, PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user"}}
			remote := newServiceRemote()
			remote.passwordErr = test.cause
			service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
			err := service.Provision(context.Background(), "account")
			if test.wantReview {
				if !errors.Is(err, ErrPendingReview) || repository.refundCalls != 0 || repository.retryCalls != 0 {
					t.Fatalf("Provision() error=%v refund=%d retry=%d", err, repository.refundCalls, repository.retryCalls)
				}
			} else if !errors.Is(err, transientServiceError) || repository.retryCalls != 1 || repository.refundCalls != 0 {
				t.Fatalf("Provision() error=%v refund=%d retry=%d", err, repository.refundCalls, repository.retryCalls)
			}
			persisted := repository.refundReason
			if repository.retryError != nil {
				persisted += " " + repository.retryError.Error()
			}
			if strings.Contains(persisted, "temporary") || (err != nil && strings.Contains(err.Error(), "temporary")) {
				t.Fatalf("password leaked through provisioning error: returned=%q persisted=%q", err, persisted)
			}
		})
	}
}

func TestProvisionRecoveryAndTerminalStateBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		record      ProvisioningRecord
		seed        *RemoteUser
		secretError error
		wantRefund  int
		wantActive  bool
	}{
		{
			name: "stored remote id resumes policy work",
			record: ProvisioningRecord{Account: Account{ID: "one", UserID: "user-one", BaseUsername: "ada", RemoteUserID: "remote-one", CandidateUsername: "ada", Status: StatusQueued},
				PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user-one"},
			seed: &RemoteUser{ID: "remote-one", Name: "ada", Policy: basePolicy()}, wantActive: true,
		},
		{
			name: "restored server reconciles changed remote id by candidate",
			record: ProvisioningRecord{Account: Account{ID: "two", UserID: "user-two", BaseUsername: "river", RemoteUserID: "old-id", CandidateUsername: "river", Status: StatusQueued},
				PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user-two"},
			seed: &RemoteUser{ID: "new-id", Name: "river", Policy: basePolicy()}, wantActive: true,
		},
		{
			name: "missing secret refunds", record: ProvisioningRecord{Account: Account{ID: "three", UserID: "user-three", Status: StatusQueued}}, wantRefund: 1,
		},
		{
			name:        "undecryptable secret refunds",
			record:      ProvisioningRecord{Account: Account{ID: "four", UserID: "user-four", Status: StatusQueued}, PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user-four"},
			secretError: errors.New("authentication failed"), wantRefund: 1,
		},
		{name: "active replay is complete", record: ProvisioningRecord{Account: Account{ID: "five", UserID: "user-five", Status: StatusActive}}, wantActive: true},
		{name: "failed replay is complete", record: ProvisioningRecord{Account: Account{ID: "six", UserID: "user-six", Status: StatusFailed}}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: test.record}
			remote := newServiceRemote()
			if test.seed != nil {
				remote.addUser(*test.seed)
			}
			secrets := &serviceSecrets{openErr: test.secretError}
			service := NewService(repository, remote, fixedPrice(100), secrets)
			if err := service.Provision(context.Background(), test.record.ID); err != nil {
				t.Fatalf("Provision() error = %v", err)
			}
			if repository.refundCalls != test.wantRefund {
				t.Fatalf("refund calls = %d, want %d", repository.refundCalls, test.wantRefund)
			}
			if test.wantActive && test.record.Status == StatusQueued && repository.record.Status != StatusActive {
				t.Fatalf("status = %s, want active", repository.record.Status)
			}
		})
	}
}

func TestProvisionReconcilesCommittedCreateTimeout(t *testing.T) {
	t.Parallel()
	repository := &serviceRepository{record: ProvisioningRecord{Account: Account{ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued},
		PasswordCiphertext: "sealed", PasswordContext: "emby.provisioning.password:user"}}
	remote := newServiceRemote()
	remote.createErr, remote.createCommits = transientServiceError, true
	service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
	if err := service.Provision(context.Background(), "account"); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if repository.record.Status != StatusActive || remote.createCalls != 1 {
		t.Fatalf("record=%+v createCalls=%d", repository.record, remote.createCalls)
	}
}

func TestSetupAndLinkedValidationBranches(t *testing.T) {
	t.Parallel()
	activeRepository := &serviceRepository{record: ProvisioningRecord{Account: Account{ID: "existing", UserID: "user", Status: StatusActive}}}
	activeRemote := newServiceRemote()
	service := NewService(activeRepository, activeRemote, fixedPrice(100), &serviceSecrets{})
	if account, created, err := service.Setup(context.Background(), "user", "password", Preferences{}); err != nil || created || account.ID != "existing" {
		t.Fatalf("Setup(existing) = (%+v, %v, %v)", account, created, err)
	}

	validationRepository := &serviceRepository{baseUsername: "ada"}
	validationService := NewService(validationRepository, newServiceRemote(), fixedPrice(100), &serviceSecrets{})
	tests := []struct {
		name        string
		password    string
		preferences Preferences
	}{
		{name: "empty password"},
		{name: "unknown library", password: "password", preferences: Preferences{DisabledLibraryIDs: []string{"missing"}}},
		{name: "unknown rating", password: "password", preferences: Preferences{MaxParentalRating: int32Pointer(99)}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := validationService.Setup(context.Background(), "user", test.password, test.preferences); err == nil {
				t.Fatal("Setup() error = nil")
			}
		})
	}

	inactiveRepository := &serviceRepository{record: ProvisioningRecord{Account: Account{ID: "inactive", UserID: "user", Status: StatusQueued}}}
	inactiveService := NewService(inactiveRepository, newServiceRemote(), fixedPrice(100), &serviceSecrets{})
	if _, err := inactiveService.UpdatePreferences(context.Background(), "user", Preferences{}); !errors.Is(err, ErrRemoteAccountMissing) {
		t.Fatalf("UpdatePreferences(inactive) = %v", err)
	}
	if err := inactiveService.ChangePassword(context.Background(), "user", "", "new"); !errors.Is(err, ErrRemoteAccountMissing) {
		t.Fatalf("ChangePassword(inactive) = %v", err)
	}
	if err := inactiveService.ChangePassword(context.Background(), "user", "", ""); !errors.Is(err, ErrInvalidSetup) {
		t.Fatalf("ChangePassword(invalid) = %v", err)
	}
}
