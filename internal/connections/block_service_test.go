package connections

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type blockServiceRepository struct {
	BlockRepository
	input   CreateIPBlockInput
	command providerops.CreateInput
	scan    providerops.ConnectionScan
}

func (r *blockServiceRepository) ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error) {
	return model.OperationReceipt{}, false, nil
}

func (r *blockServiceRepository) ConnectionScanForUser(context.Context, string, string) (providerops.ConnectionScan, error) {
	return r.scan, nil
}

func (r *blockServiceRepository) BeginConnectionIPBlock(_ context.Context, input CreateIPBlockInput,
	command providerops.CreateInput, _ time.Time) (IPBlockRecord, providerops.Operation, bool, error) {
	r.input, r.command = input, command
	return IPBlockRecord{ID: "block"}, providerops.Operation{Receipt: model.OperationReceipt{ID: "operation",
		Kind: BlockOperationKind, Status: string(providerops.StatusQueued)}}, false, nil
}

type recordingBlockSecrets struct {
	context   string
	plaintext string
}

func (s *recordingBlockSecrets) Seal(context string, plaintext []byte) (string, error) {
	s.context, s.plaintext = context, string(plaintext)
	return "vault:v1:ciphertext", nil
}

func (*recordingBlockSecrets) Open(string, string) ([]byte, error) { return nil, nil }

func TestDropServiceSealsIPAndPersistsOnlyDigestMetadata(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	signer, err := NewSigner([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := signer.Sign(HandleClaims{UserID: "owner", ScanID: "scan", NodeUUID: "6aa6d759-20de-4b11-8c0a-8e0daee3a4ee",
		IP: "2001:0db8:0:0::1", Expires: now.Add(HandleTTL)})
	if err != nil {
		t.Fatal(err)
	}
	repository := &blockServiceRepository{scan: providerops.ConnectionScan{Status: providerops.StatusSucceeded,
		ExpiresAt: now.Add(ScanTTL)}}
	secrets := &recordingBlockSecrets{}
	service := NewDropService(repository, signer, secrets)
	service.now = func() time.Time { return now }
	if _, err := service.Drop(context.Background(), "owner", handle, "request-key"); err != nil {
		t.Fatal(err)
	}
	if secrets.plaintext != "2001:db8::1" || repository.input.SealedIP != "vault:v1:ciphertext" ||
		repository.input.ExpiresAt.Sub(now) != BlockDuration {
		t.Fatalf("secret=%q input=%+v", secrets.plaintext, repository.input)
	}
	if len(repository.command.Items) != 1 || repository.command.Items[0].TargetID == secrets.plaintext ||
		strings.Contains(repository.command.RequestFingerprint, secrets.plaintext) ||
		!strings.Contains(secrets.context, repository.command.Items[0].TargetID) {
		t.Fatalf("context=%q command=%+v", secrets.context, repository.command)
	}
}
