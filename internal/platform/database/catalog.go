package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
	"time"
)

// ComboInput is the validated catalog representation persisted by an administrator.
// SquadProductIDs contains Proxy internal-squad UUIDs; the historical field
// name remains at the service boundary for source compatibility.
type ComboInput struct {
	WhitelistIPQuota        *int
	IPLookupQuota           *int
	ID                      string
	Name                    string
	Description             string
	PriceTXBMinor           int64
	ValidityDays            int
	TrafficLimitBytes       int64
	ResetStrategy           string
	Active                  bool
	SquadProductIDs         []string
	RolloverMinRemainingBPS int
}

// SquadProductInput is the local merchandising override associated with a
// Proxy-owned internal squad.
type SquadProductInput struct {
	BillingMode        string
	PAYGPriceTXBMinor  int64
	ID                 string
	GroupID            string
	Name               string
	Description        string
	Profile            *squadprofile.Profile
	PriceTXBMinor      int64
	Visible            bool
	UpstreamPresent    bool
	StockLimit         *int
	ActivationRequired bool
	ActivationCode     string
}

// ImportedSquad is retained as a compatibility DTO. Upstream squads are no
// longer persisted by refresh operations.
type ImportedSquad struct {
	UUID string
	Name string
}

// SaveCombo creates or updates one stable live combo and schedules affected
// active users for upstream re-synchronization after an edit.
func (s *Store) retryAttempt_SaveCombo(ctx context.Context, input ComboInput) (model.Combo, error) {
	if input.WhitelistIPQuota != nil && (*input.WhitelistIPQuota < 0 || *input.WhitelistIPQuota > 1000) {
		return model.Combo{}, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	creating := input.ID == ""
	if creating {
		var err error
		input.ID, err = ids.New()
		if err != nil {
			return model.Combo{}, err
		}
	}
	squadUUIDs := uniqueSorted(input.SquadProductIDs)
	encodedSquads, err := json.Marshal(squadUUIDs)
	if err != nil {
		return model.Combo{}, fmt.Errorf("encode combo squad UUIDs: %w", err)
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Combo{}, fmt.Errorf("begin save combo: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := saveComboIPQuotaTx(ctx, tx, input, creating); err != nil {
		return model.Combo{}, err
	}
	var result sql.Result
	if creating {
		result, err = tx.ExecContext(ctx, `INSERT INTO combos(id,name,description,price_txb_minor,validity_days,traffic_limit_bytes,reset_strategy,active,rollover_min_remaining_bps,included_squad_uuids,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, input.ID, input.Name, input.Description, input.PriceTXBMinor, input.ValidityDays,
			input.TrafficLimitBytes, input.ResetStrategy, boolInt(input.Active), input.RolloverMinRemainingBPS,
			string(encodedSquads), stamp(now), stamp(now))
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE combos SET name=?,description=?,price_txb_minor=?,validity_days=?,traffic_limit_bytes=?,reset_strategy=?,active=?,rollover_min_remaining_bps=?,included_squad_uuids=?,updated_at=? WHERE id=?`,
			input.Name, input.Description, input.PriceTXBMinor, input.ValidityDays, input.TrafficLimitBytes, input.ResetStrategy,
			boolInt(input.Active), input.RolloverMinRemainingBPS, string(encodedSquads), stamp(now), input.ID)
	}
	if err != nil {
		return model.Combo{}, fmt.Errorf("save combo: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.Combo{}, fmt.Errorf("inspect saved combo: %w", rowsErr)
	} else if affected != 1 {
		return model.Combo{}, ErrNotFound
	}
	if creating && input.IPLookupQuota != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE combos SET ip_lookup_quota=? WHERE id=?`, *input.IPLookupQuota, input.ID); err != nil {
			return model.Combo{}, err
		}
	}
	if input.WhitelistIPQuota != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE combos SET whitelist_ip_quota=? WHERE id=?`, *input.WhitelistIPQuota, input.ID); err != nil {
			return model.Combo{}, err
		}
	}
	if !creating {
		rows, queryErr := tx.QueryContext(ctx, `SELECT DISTINCT user_id FROM purchases WHERE combo_id=? AND status IN ('activating','active','queued')`, input.ID)
		if queryErr != nil {
			return model.Combo{}, fmt.Errorf("list combo users for resync: %w", queryErr)
		}
		userIDs := make([]string, 0)
		for rows.Next() {
			var userID string
			if scanErr := rows.Scan(&userID); scanErr != nil {
				_ = rows.Close()
				return model.Combo{}, fmt.Errorf("scan combo user for resync: %w", scanErr)
			}
			userIDs = append(userIDs, userID)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			_ = rows.Close()
			return model.Combo{}, fmt.Errorf("iterate combo users for resync: %w", rowsErr)
		}
		_ = rows.Close()
		for _, userID := range userIDs {
			payload, marshalErr := json.Marshal(map[string]string{"userId": userID})
			if marshalErr != nil {
				return model.Combo{}, marshalErr
			}
			if enqueueErr := insertOutboxTx(ctx, tx, "xnode_sync_user", string(payload), now, now); enqueueErr != nil {
				return model.Combo{}, enqueueErr
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Combo{}, fmt.Errorf("commit combo: %w", err)
	}
	return s.ComboByID(ctx, input.ID, false)
}

// DeleteCombo hides a combo without invalidating purchases that reference it.
func (s *Store) DeleteCombo(ctx context.Context, comboID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE combos SET active=0,updated_at=? WHERE id=?`, stamp(time.Now().UTC()), comboID)
	if err != nil {
		return fmt.Errorf("hide combo: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}
