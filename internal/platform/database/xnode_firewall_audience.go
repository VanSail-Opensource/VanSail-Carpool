package database

import (
	"context"
	"database/sql"

	"github.com/RoaringBitmap/roaring/v2"
)

// A journal transaction never changes membership. Reuse its serializable
// snapshot, but create a fresh cache inside every transaction retry attempt.
type firewallAudienceSnapshot struct {
	raw    []byte
	active *roaring.Bitmap
}

func (a *firewallAudienceSnapshot) load(ctx context.Context, tx *sql.Tx) error {
	if a.active != nil {
		return nil
	}
	raw, err := firewallAudience(ctx, tx)
	if err != nil {
		return err
	}
	active, err := firewallBitmap(raw)
	if err != nil {
		return err
	}
	a.raw, a.active = raw, active
	return nil
}
