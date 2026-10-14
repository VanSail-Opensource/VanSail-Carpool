package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Store) loadRenewalBatch(ctx context.Context, batchID string) (model.RenewalBatch, error) {
	var batch model.RenewalBatch
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT id,source_purchase_id,term_count,charged_txb_minor FROM renewal_batches WHERE id=?`, batchID).Scan(&batch.ID, &batch.PurchaseID, &batch.TermCount, &total); err != nil {
		return batch, err
	}
	batch.TotalPrice = model.TXBMoney(total)
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM purchases WHERE renewal_batch_id=? ORDER BY renewal_index`, batchID)
	if err != nil {
		return batch, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return batch, err
		}
		purchase, err := s.PurchaseByID(ctx, id)
		if err != nil {
			return batch, err
		}
		batch.Purchases = append(batch.Purchases, purchase)
	}
	return batch, rows.Err()
}
