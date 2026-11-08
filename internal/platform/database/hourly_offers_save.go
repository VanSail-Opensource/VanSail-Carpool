package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SaveHourlyOffer commits an explicit revision-checked offer and its audit.
// Revision zero creates a previously unconfigured offer; existing prices and
// grandfathered billing modes are never used as defaults or rewritten.
func (s *Store) SaveHourlyOffer(ctx context.Context, actor string, next model.HourlyOffer, expectedRevision int64) (model.HourlyOffer, error) {
	if actor == "" || next.GroupID == "" || len(next.GroupID) > 128 || strings.TrimSpace(next.GroupID) != next.GroupID || expectedRevision < 0 || next.HourlyPriceTXBMinor < 0 || next.Enabled && next.HourlyPriceTXBMinor == 0 {
		return model.HourlyOffer{}, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.HourlyOffer, error) {
		var group string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM xnode_groups WHERE id=$1 FOR SHARE`, next.GroupID).Scan(&group); errors.Is(err, sql.ErrNoRows) {
			return model.HourlyOffer{}, ErrNotFound
		} else if err != nil {
			return model.HourlyOffer{}, err
		}
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "hourly.offer:"+next.GroupID); err != nil {
			return model.HourlyOffer{}, err
		}
		current, err := scanHourlyOffer(tx.QueryRowContext(ctx, `SELECT `+hourlyOfferColumns+` FROM hourly_offers WHERE group_id=$1 FOR UPDATE`, next.GroupID))
		if err != nil && !errors.Is(err, ErrNotFound) {
			return model.HourlyOffer{}, err
		}
		if current.Revision != expectedRevision || current.Revision == math.MaxInt64 {
			return model.HourlyOffer{}, ErrConflict
		}
		now := time.Now().UTC()
		stored, err := scanHourlyOffer(tx.QueryRowContext(ctx, `INSERT INTO hourly_offers(group_id,enabled,hourly_price_minor,revision,created_at,updated_at) VALUES($1,$2,$3,1,$4,$4) ON CONFLICT(group_id) DO UPDATE SET enabled=excluded.enabled,hourly_price_minor=excluded.hourly_price_minor,revision=hourly_offers.revision+1,updated_at=excluded.updated_at RETURNING `+hourlyOfferColumns, next.GroupID, boolInt(next.Enabled), next.HourlyPriceTXBMinor, now))
		if err != nil {
			return model.HourlyOffer{}, err
		}
		id, err := ids.New()
		if err != nil {
			return model.HourlyOffer{}, err
		}
		detail, err := json.Marshal(map[string]any{"enabled": stored.Enabled, "hourlyPriceTXBMinor": stored.HourlyPriceTXBMinor, "revision": stored.Revision})
		if err != nil {
			return model.HourlyOffer{}, err
		}
		if err = insertAuditTx(ctx, tx, id, &actor, "xnode.hourly_offer.update", "xnode_group", next.GroupID, string(detail), now); err != nil {
			return model.HourlyOffer{}, err
		}
		return stored, nil
	})
}
