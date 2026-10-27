package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func registerPricingCompute(worker *outbox.Worker, store *database.Store, master *xnode.Master) error {
	return worker.Register(database.PricingComputeJob, outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		id, err := jobpayload.TargetID(job, "inputId")
		if err != nil {
			return err
		}
		task, err := store.PricingComputeInput(ctx, id)
		if err != nil {
			return err
		}
		err = master.Compute(ctx, task, func(result compute.Result) error {
			return store.AcceptPricingCompute(ctx, task, result, time.Now().UTC())
		})
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, xnode.ErrUnavailable) {
			return outbox.ErrBusy
		}
		return err
	}))
}

func (a *Application) runComputeLane(ctx context.Context) {
	// Analytics tolerate subsecond admission; avoid ten empty database lease
	// scans per second while still admitting bounded batches independently.
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	pressureCtx, stopPressure := context.WithCancel(ctx)
	pressureDone := make(chan struct{})
	go func() { defer close(pressureDone); a.runComputePressure(pressureCtx) }()
	defer func() { stopPressure(); <-pressureDone }()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := a.outbox.DrainCompute(ctx, 16); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Warn("compute admission deferred", "error", err)
			}
		}
	}
}

func (a *Application) runComputePressure(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	sampler := computePressureSampler{}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.xnodeMaster.SetComputePressure(sampler.observe(now))
		}
	}
}
