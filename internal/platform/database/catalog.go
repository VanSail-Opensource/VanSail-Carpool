package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

// ComboInput is the validated catalog representation persisted by an administrator.
// SquadProductIDs contains Proxy internal-squad UUIDs; the historical field
// name remains at the service boundary for source compatibility.
type ComboInput struct {
	Features          squadprofile.Features
	IPLookupQuota     *int
	ID                string
	Name              string
	Description       string
	PriceTXBMinor     int64
	ValidityDays      int
	TrafficLimitBytes int64
	ResetStrategy     string
	Active            bool
	SquadProductIDs   []string
}

// SquadProductInput is the local merchandising override associated with a
// Proxy-owned internal squad.
type SquadProductInput struct {
	PAYGState          string
	BillingMode        string
	PAYGPriceTXBMinor  int64
	ID                 string
	GroupID            string
	Name               string
	Description        string
	Features           squadprofile.Features
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
	var featuresJSON any
	if input.Features != nil {
		features, e := squadprofile.NormalizeFeatures(input.Features)
		if e != nil {
			return model.Combo{}, e
		}
		encoded, e := json.Marshal(features)
		if e != nil {
			return model.Combo{}, e
		}
		featuresJSON = string(encoded)
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
		result, err = tx.ExecContext(ctx, `INSERT INTO combos(id,name,description,price_txb_minor,validity_days,traffic_limit_bytes,reset_strategy,active,included_squad_uuids,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, input.ID, input.Name, input.Description, input.PriceTXBMinor, input.ValidityDays,
			input.TrafficLimitBytes, input.ResetStrategy, boolInt(input.Active),
			string(encodedSquads), stamp(now), stamp(now))
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE combos SET name=$1,description=$2,price_txb_minor=$3,validity_days=$4,traffic_limit_bytes=$5,reset_strategy=$6,active=$7,included_squad_uuids=$8,updated_at=$9 WHERE id=$10`,
			input.Name, input.Description, input.PriceTXBMinor, input.ValidityDays, input.TrafficLimitBytes, input.ResetStrategy,
			boolInt(input.Active), string(encodedSquads), stamp(now), input.ID)
	}
	if err != nil {
		return model.Combo{}, fmt.Errorf("save combo: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return model.Combo{}, fmt.Errorf("inspect saved combo: %w", rowsErr)
	} else if affected != 1 {
		return model.Combo{}, ErrNotFound
	}
	if featuresJSON != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE combos SET features_json=$1 WHERE id=$2`, featuresJSON, input.ID); err != nil {
			return model.Combo{}, err
		}
	}
	if creating && input.IPLookupQuota != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE combos SET ip_lookup_quota=$1 WHERE id=$2`, *input.IPLookupQuota, input.ID); err != nil {
			return model.Combo{}, err
		}
	}
	if !creating {
		rows, queryErr := tx.QueryContext(ctx, `SELECT DISTINCT user_id FROM purchases WHERE combo_id=$1 AND status IN ('activating','active','queued')`, input.ID)
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
	result, err := s.db.ExecContext(ctx, `UPDATE combos SET active=0,updated_at=$1 WHERE id=$2`, stamp(time.Now().UTC()), comboID)
	if err != nil {
		return fmt.Errorf("hide combo: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}
