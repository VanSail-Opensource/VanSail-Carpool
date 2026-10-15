package iplookup

import "context"

// runInventory checkpoints requested-IP and /24 membership before any other checks.
func (w *Worker) runInventory(ctx context.Context, r *Report, c Config, persist func(Report) error) error {
	if r.Checkpoint.Enhanced == nil {
		r.Checkpoint.Enhanced = &EnhancedCheckpoint{}
	}
	e := r.Checkpoint.Enhanced
	if e.InventoryDone {
		return nil
	}
	e.Phase = "proxy"
	if err := persist(*r); err != nil {
		return err
	}
	pool, err := w.Enhanced.pool(ctx, c)
	if err != nil {
		return err
	}
	defer pool.release()
	r.Proxy, err = pool.assessmentContext(ctx, r.IP)
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	applyProxyRefusal(r)
	e.Phase = "subnet"
	if err := persist(*r); err != nil {
		return err
	}
	if err := initializeSubnet(ctx, r, pool); err != nil {
		return err
	}
	if len(r.Refusals) > 0 {
		finishInventorySubnet(r, "ip_not_passed")
	}
	e.InventoryDone, e.Phase = true, "checking"
	return persist(*r)
}
