package emby

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestProvisioningFailureStatesRedactProviderDiagnostics(t *testing.T) {
	for _, state := range []string{"retry", "terminal", "pending review"} {
		t.Run(state, func(t *testing.T) {
			repository := &serviceRepository{record: ProvisioningRecord{Account: Account{ID: "account", UserID: "user", BaseUsername: "ada", Status: StatusQueued}, PasswordCiphertext: "sealed", PasswordContext: passwordContext("user")}}
			remote := newServiceRemote()
			cause := transientServiceError
			if state == "terminal" {
				cause = terminalServiceError
			}
			remote.findErr = fmt.Errorf("%w: GET https://private.example/secret?api_key=private-credential response=private-body", cause)
			if state == "pending review" {
				repository.record.CreateAttempted = true
				repository.record.CandidateUsername = "ada"
			}
			err := NewService(repository, remote, fixedPrice(100), &serviceSecrets{}).Provision(context.Background(), "account")
			if state == "retry" && (!errors.Is(err, transientServiceError) || repository.retryCalls != 1) {
				t.Fatal("retry classification lost", err, repository.retryCalls)
			}
			if state == "terminal" && (err != nil || repository.refundCalls != 1) {
				t.Fatal("terminal failure lost its refund", err, repository.refundCalls)
			}
			if state == "pending review" && (!errors.Is(err, ErrPendingReview) || repository.refundCalls != 0 || repository.record.Status != StatusPendingReview) {
				t.Fatal("ambiguous creation lost review", err, repository.record.Status)
			}
			messages := repository.refundReason
			if repository.retryError != nil {
				messages += " " + repository.retryError.Error()
			}
			if err != nil {
				messages += " " + err.Error()
			}
			for _, private := range []string{"private.example", "private-credential", "private-body", "api_key"} {
				if strings.Contains(messages, private) {
					t.Fatalf("provider diagnostic escaped stage %s: %s", state, messages)
				}
			}
		})
	}
}
