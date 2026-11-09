package connections

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// SecretBox seals short-lived provider targets with context-bound encryption.
type SecretBox interface {
	Seal(context string, plaintext []byte) (string, error)
	Open(context string, ciphertext string) ([]byte, error)
}

// BlockRepository persists encrypted active blocks and owner-scoped receipts.
type BlockRepository interface {
	ProviderOperationForActorKey(context.Context, string, string, string, string) (model.OperationReceipt, bool, error)
	ConnectionScanForUser(context.Context, string, string) (providerops.ConnectionScan, error)
	BeginConnectionIPBlock(context.Context, CreateIPBlockInput, providerops.CreateInput, time.Time) (IPBlockRecord, providerops.Operation, bool, error)
	ListConnectionIPBlocksForUser(context.Context, string) ([]IPBlockRecord, error)
	ConnectionIPBlockForUser(context.Context, string, string) (IPBlockRecord, error)
	BeginConnectionIPUnblock(context.Context, string, string, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
}

// DropService upgrades the legacy drop endpoint to a durable block workflow.
type DropService struct {
	repository BlockRepository
	signer     *Signer
	secrets    SecretBox
	now        func() time.Time
}

// NewDropService creates the durable connection block command service.
func NewDropService(repository BlockRepository, signer *Signer, secrets SecretBox) *DropService {
	return &DropService{repository: repository, signer: signer, secrets: secrets, now: time.Now}
}

// List returns decrypted active blocks to their owner.
func (s *DropService) List(ctx context.Context, userID string) ([]IPBlock, error) {
	records, err := s.repository.ListConnectionIPBlocksForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	blocks := make([]IPBlock, 0, len(records))
	for _, record := range records {
		plaintext, openErr := openIPBlockTarget(ctx, s.repository, s.secrets, record)
		if openErr != nil {
			return nil, openErr
		}
		blocks = append(blocks, IPBlock{ID: record.ID, IP: string(plaintext), NodeUUID: record.NodeUUID,
			Status: record.Status, CreatedAt: record.CreatedAt, ExpiresAt: record.ExpiresAt})
		clear(plaintext)
	}
	return blocks, nil
}

// Unblock queues an owner-scoped removal; actorID may identify an administrator.
func (s *DropService) Unblock(ctx context.Context, actorID, ownerID, blockID, idempotencyKey string) (model.OperationReceipt, error) {
	fingerprint := hash("connection-unblock:v1:" + strings.TrimSpace(blockID))
	if receipt, found, err := s.repository.ProviderOperationForActorKey(ctx, actorID, UnblockOperationKind,
		idempotencyKey, fingerprint); found || err != nil {
		return receipt, err
	}
	record, err := s.repository.ConnectionIPBlockForUser(ctx, blockID, ownerID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	operation, _, err := s.repository.BeginConnectionIPUnblock(ctx, record.ID, ownerID, providerops.CreateInput{
		ActorUserID: actorID, OwnerUserID: ownerID, Kind: UnblockOperationKind,
		IdempotencyKey: strings.TrimSpace(idempotencyKey), RequestFingerprint: fingerprint,
		Items: []providerops.ItemInput{{Key: "ip", TargetType: "connection_ip_hmac", TargetID: record.IPDigest}},
	}, s.now().UTC())
	return operation.Receipt, err
}

func blockFingerprint(handle string) string { return hash("connection-block:v1:" + handle) }

func hash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
