package database

import (
	"cmp"
	"context"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Store) RecordXNodeTrafficBatch(ctx context.Context, items []xnode.TrafficCommit) ([]error, error) {
	if len(items) > xnode.MaxTrafficBatchReports {
		return nil, xnode.ErrInvalid
	}
	results := make([]error, len(items))
	var selected []int
	for i, item := range items {
		sequence, err := wire.TrafficSequence(item.AgentID, item.Report)
		if err != nil || sequence != item.Sequence || item.NodeID == "" || item.AgentID == "" || item.Report.StreamID == "" && item.Report.DroppedSamples != 0 {
			results[i] = xnode.ErrInvalid
		} else {
			selected = append(selected, i)
		}
	}
	slices.SortStableFunc(selected, func(a, b int) int {
		x, y := items[a], items[b]
		return cmp.Or(cmp.Compare(x.NodeID, y.NodeID), cmp.Compare(x.AgentID, y.AgentID), cmp.Compare(x.Report.StreamID, y.Report.StreamID), cmp.Compare(x.Sequence, y.Sequence))
	})
	valid := make([]xnode.TrafficCommit, 0, len(selected))
	for _, i := range selected {
		valid = append(valid, items[i])
	}
	if len(valid) == 0 {
		return results, nil
	}
	for attempt := 0; ; attempt++ {
		outcomes, err := s.commitTrafficBatch(ctx, valid)
		if attempt < 3 && s.retryTransaction(ctx, err, attempt) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for j, i := range selected {
			results[i] = outcomes[j]
		}
		return results, nil
	}
}

func (s *Store) commitTrafficBatch(ctx context.Context, items []xnode.TrafficCommit) ([]error, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	batch, err := prepareTrafficBatch(ctx, tx, items)
	if err != nil {
		return nil, err
	}
	results := make([]error, len(items))
	for i, item := range items {
		if _, err = tx.ExecContext(ctx, `SAVEPOINT traffic_batch_item`); err != nil {
			return nil, err
		}
		err = s.recordXNodeTrafficTx(ctx, tx, item.NodeID, item.AgentID, item.Sequence, item.Digest, item.Report, batch)
		if err != nil {
			if transactionConflict(err) || ctx.Err() != nil {
				return nil, err
			}
			results[i] = err
			if _, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT traffic_batch_item`); err != nil {
				return nil, err
			}
		}
		if _, err = tx.ExecContext(ctx, `RELEASE SAVEPOINT traffic_batch_item`); err != nil {
			return nil, err
		}
	}
	if err = batch.native.flush(ctx, tx); err != nil {
		return nil, err
	}
	if err = batch.flushCursors(ctx, tx); err != nil {
		return nil, err
	}
	return results, tx.Commit()
}
