package emby

import (
	"context"
	"errors"
	"testing"
)

func TestCreateRejectionRefundRequiresConfirmedCandidateAbsence(t *testing.T) {
	for _, test := range []struct {
		name        string
		committed   bool
		lookupFails bool
		wantRefund  int
	}{
		{name: "rejected and absent", wantRefund: 1},
		{name: "candidate exists after rejection", committed: true},
		{name: "candidate absence unavailable", lookupFails: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: ProvisioningRecord{
				Account:            Account{ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued},
				PasswordCiphertext: "sealed", PasswordContext: passwordContext("user"),
			}}
			remote := &rejectedCreateRemote{serviceRemote: newServiceRemote(), lookupFails: test.lookupFails}
			remote.createErr, remote.createCommits = terminalServiceError, test.committed
			service := NewService(repository, remote, fixedPrice(100), &serviceSecrets{})
			err := service.Provision(context.Background(), "account")
			if repository.refundCalls != test.wantRefund {
				t.Fatalf("refunds=%d error=%v", repository.refundCalls, err)
			}
			if test.wantRefund == 1 {
				if err != nil || repository.record.Status != StatusFailed {
					t.Fatalf("confirmed rejection did not refund: %v", err)
				}
			} else if !errors.Is(err, ErrPendingReview) || repository.record.Status != StatusPendingReview ||
				repository.record.PasswordCiphertext != "sealed" || len(remote.password) != 0 {
				t.Fatalf("unproven rejection lost recovery state: status=%s error=%v", repository.record.Status, err)
			}
		})
	}
}

func TestPreviouslyCreatedSetupRequiresAbsenceProofBeforeRefund(t *testing.T) {
	for _, test := range []struct {
		name       string
		remoteID   string
		secretErr  error
		wantRefund int
	}{
		{name: "lost secret after create attempt", secretErr: errors.New("retired encryption key")},
		{name: "known remote account and lost secret", remoteID: "remote", secretErr: errors.New("retired encryption key")},
		{name: "stored identity and candidate authoritatively absent", remoteID: "missing", wantRefund: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &serviceRepository{record: ProvisioningRecord{
				Account:            Account{ID: "account", UserID: "user", BaseUsername: "ada", CandidateUsername: "ada",
					RemoteUserID: test.remoteID, Status: StatusQueued},
				PasswordCiphertext: "sealed", PasswordContext: passwordContext("user"), CreateAttempted: true,
			}}
			service := NewService(repository, newServiceRemote(), fixedPrice(100), &serviceSecrets{openErr: test.secretErr})
			err := service.Provision(context.Background(), "account")
			if repository.refundCalls != test.wantRefund {
				t.Fatalf("refunds=%d error=%v", repository.refundCalls, err)
			}
			if test.wantRefund == 1 {
				if err != nil || repository.record.Status != StatusFailed {
					t.Fatalf("confirmed absent account did not refund: %v", err)
				}
			} else if !errors.Is(err, ErrPendingReview) || repository.record.PasswordCiphertext != "sealed" {
				t.Fatalf("unresolved account lost its paid recovery: %v", err)
			}
		})
	}
}

type rejectedCreateRemote struct {
	*serviceRemote
	lookupFails bool
}

func (r *rejectedCreateRemote) FindUserByName(ctx context.Context, name string) (RemoteUser, bool, error) {
	if r.lookupFails && r.createCalls > 0 {
		return RemoteUser{}, false, transientServiceError
	}
	return r.serviceRemote.FindUserByName(ctx, name)
}
