package database

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) loadRenewalBatch(ctx context.Context, batchID string) (model.RenewalBatch, error) {
	var batch model.RenewalBatch
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT id,source_purchase_id,term_count,charged_txb_minor FROM renewal_batches WHERE id=$1`, batchID).Scan(&batch.ID, &batch.PurchaseID, &batch.TermCount, &total); err != nil {
		return batch, err
	}
	batch.TotalPrice = model.TXBMoney(total)
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM purchases WHERE renewal_batch_id=$1 ORDER BY renewal_index`, batchID)
	if err != nil {
		return batch, err
	}
	defer func() { _ = rows.Close() }()
	var purchaseIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return batch, err
		}
		purchaseIDs = append(purchaseIDs, id)
	}
	// Release the result's connection before hydrating each purchase. A nested
	// query here otherwise stalls even a single-request pool with one connection.
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return batch, err
	}
	for _, id := range purchaseIDs {
		purchase, err := s.PurchaseByID(ctx, id)
		if err != nil {
			return batch, err
		}
		batch.Purchases = append(batch.Purchases, purchase)
	}
	return batch, nil
}
