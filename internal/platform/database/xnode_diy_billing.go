package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func diyQuoteTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) (xnode.DIYBillingQuote, error) {
	q := xnode.DIYBillingQuote{Charge: model.TXBMoney(0)}
	c, err := ipConfigTx(ctx, tx)
	if err != nil {
		return q, err
	}
	q.ConfigHash = iplookup.ConfigHash(c)
	a, err := ipAllowanceTx(ctx, tx, user, now)
	if err != nil {
		return q, err
	}
	if a != nil {
		q.PurchaseID = a.PurchaseID
		q.Remaining = a.Remaining
	}
	if q.Remaining > 0 {
		q.UseQuota = true
		q.Available = true
		return q, nil
	}
	if c.LookupFeeTXB == "" {
		return q, nil
	}
	amount, err := model.ParseTXBMajor(c.LookupFeeTXB)
	if err != nil {
		return q, err
	}
	q.Charge = model.TXBMoney(amount)
	q.Available = true
	return q, nil
}

func (s *Store) XNodeDIYBillingQuote(ctx context.Context, user string, now time.Time) (xnode.DIYBillingQuote, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return xnode.DIYBillingQuote{}, err
	}
	defer tx.Rollback()
	return diyQuoteTx(ctx, tx, user, now)
}

// chargeDIYTx shares the existing IP-check allowance and write serialization.
// A stale quote never silently upgrades a quota Save into a balance charge.
func chargeDIYTx(ctx context.Context, tx *sql.Tx, user, operation string, quote *xnode.DIYBillingQuote, now time.Time) error {
	if quote == nil {
		return nil
	} // Internal pre-billing callers only.
	fresh, err := diyQuoteTx(ctx, tx, user, now)
	if err != nil {
		return err
	}
	if quote.ExpiresAt <= now.Unix() || fresh.ConfigHash != quote.ConfigHash || fresh.UseQuota != quote.UseQuota || fresh.Charge != quote.Charge || fresh.PurchaseID != quote.PurchaseID || fresh.Remaining != quote.Remaining {
		return xnode.ErrDIYQuoteChanged
	}
	if !fresh.Available {
		return xnode.ErrDIYBillingUnavailable
	}
	if fresh.UseQuota {
		result, err := tx.ExecContext(ctx, `UPDATE ip_lookup_allowances SET used=used+1 WHERE purchase_id=? AND used<total`, fresh.PurchaseID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return xnode.ErrDIYQuoteChanged
		}
	} else if amount := fresh.Charge.MinorInt64(); amount > 0 {
		balance, err := changeBalanceTx(ctx, tx, user, -amount, now)
		if err != nil {
			return err
		}
		if _, err = insertLedgerTx(ctx, tx, user, -amount, balance, "xnode_diy", operation, "DIY settings Save", now); err != nil {
			return err
		}
	}
	return nil
}

// ReplayXNodeDIYResult returns the committed revision/cost even after later saves.
func (s *Store) ReplayXNodeDIYResult(ctx context.Context, user, key, fingerprint string) (xnode.DIYSaveResult, bool, error) {
	receipt, found, routed, err := s.ReplayXNodeDIY(ctx, user, key, fingerprint)
	result := xnode.DIYSaveResult{Receipt: receipt, RoutingChanged: routed, DeploymentChanged: routed}
	if err != nil || !found {
		return result, found, err
	}
	var raw string
	if err = s.db.QueryRowContext(ctx, `SELECT result_json FROM provider_operations WHERE id=?`, receipt.ID).Scan(&raw); err != nil {
		return result, false, err
	}
	var metadata struct {
		Save *xnode.DIYSaveResult `json:"diySave"`
	}
	if raw != "" && raw != "{}" {
		if err = json.Unmarshal([]byte(raw), &metadata); err != nil {
			return result, false, err
		}
	}
	if metadata.Save != nil {
		result = *metadata.Save
	} else {
		result.Legacy = true
	}
	result.Receipt = receipt
	return result, true, nil
}
