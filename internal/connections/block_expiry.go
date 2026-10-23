package connections

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// ExpiryRepository owns sensitive-row cleanup for scheduled unblocks.
type ExpiryRepository interface {
	ConnectionIPBlockByID(context.Context, string) (IPBlockRecord, error)
	FinalizeConnectionIPBlockExpiry(context.Context, string, bool, time.Time) error
}

// ExpiryWorker is the scheduled backstop to Proxy's plugin timeout.
type ExpiryWorker struct {
	repository ExpiryRepository
	remote     BlockRemote
	secrets    SecretBox
	now        func() time.Time
}

func NewExpiryWorker(repository ExpiryRepository, remote BlockRemote, secrets SecretBox) *ExpiryWorker {
	return &ExpiryWorker{repository: repository, remote: remote, secrets: secrets, now: time.Now}
}

func (w *ExpiryWorker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	blockID, err := jobpayload.TargetID(job, "blockId")
	if err != nil {
		return err
	}
	block, err := w.repository.ConnectionIPBlockByID(ctx, blockID)
	if err != nil {
		if errors.Is(err, ErrIPBlockNotFound) {
			return nil
		}
		return err
	}
	if w.now().UTC().Before(block.ExpiresAt) {
		return errors.New("connection IP block expiry ran before its deadline")
	}
	plaintext, err := w.secrets.Open(ipBlockSecretContext(block.UserID, block.NodeUUID, block.IPDigest), block.SealedIP)
	if err == nil {
		scoped, controlErr := scopedControl(ctx, w.remote, block.UserID, string(plaintext), block.NodeUUID, false, 0, true)
		err = controlErr
		if !scoped {
			err = w.remote.UnblockIP(ctx, string(plaintext), block.NodeUUID)
		}
	}
	if err != nil {
		if job.Attempts >= 10 {
			if deleteErr := w.repository.FinalizeConnectionIPBlockExpiry(ctx, block.ID, false, w.now().UTC()); deleteErr != nil {
				return fmt.Errorf("scrub exhausted IP block: %w", deleteErr)
			}
		}
		return err
	}
	return w.repository.FinalizeConnectionIPBlockExpiry(ctx, block.ID, true, w.now().UTC())
}
