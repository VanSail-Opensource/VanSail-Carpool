package xnode

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func (p *computePool) run(ctx context.Context) {
	defer p.master.wg.Done()
	var workers sync.WaitGroup
	slots := make(chan struct{}, 16)
	local := make(chan struct{}, 1)
	defer workers.Wait()
	var carry *computeCall
	for {
		var first *computeCall
		if carry != nil {
			first = carry
			carry = nil
		} else {
			select {
			case <-ctx.Done():
				return
			case first = <-p.incoming:
			}
		}
		if first.ctx.Err() != nil {
			continue
		}
		calls := []*computeCall{first}
		batch := compute.Batch{Version: compute.Version, Tasks: []compute.Task{first.task}}
		timer := time.NewTimer(100 * time.Millisecond)
	collect:
		for len(batch.Tasks) < compute.MaxItems {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				break collect
			case call := <-p.incoming:
				if call.ctx.Err() != nil {
					continue
				}
				candidate := compute.Batch{Version: compute.Version, Tasks: append(append([]compute.Task{}, batch.Tasks...), call.task)}
				if compute.Validate(candidate) != nil {
					carry = call
					break collect
				}
				calls = append(calls, call)
				batch = candidate
			}
		}
		timer.Stop()
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		workers.Add(1)
		go func(calls []*computeCall, b compute.Batch) {
			defer workers.Done()
			defer func() { <-slots }()
			work, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			var remaining atomic.Int32
			remaining.Store(int32(len(calls)))
			stops := make([]func() bool, 0, len(calls))
			for _, call := range calls {
				stops = append(stops, context.AfterFunc(call.ctx, func() {
					if remaining.Add(-1) == 0 {
						cancel()
					}
				}))
			}
			defer func() {
				for _, stop := range stops {
					stop()
				}
			}()
			var out compute.Results
			var finish func(bool)
			var err error
			if !p.pressure.Load() {
				select {
				case local <- struct{}{}:
				case <-work.Done():
					p.fail(calls, work.Err())
					return
				}
				if !p.pressure.Load() {
					out, err = compute.Run(work, b)
					<-local
				} else {
					<-local
					out, finish, err = p.remote(work, b)
				}
			} else {
				out, finish, err = p.remote(work, b)
			}
			if errors.Is(err, errComputeLocal) {
				select {
				case local <- struct{}{}:
					out, err = compute.Run(work, b)
					<-local
				case <-work.Done():
					err = work.Err()
				}
			}
			if err == nil {
				err = compute.ValidateResult(b, out)
			}
			if err != nil {
				if finish != nil {
					finish(false)
				}
				p.fail(calls, err)
				return
			}
			for i, call := range calls {
				call.reply <- computeReply{result: out.Items[i]}
			}
			accepted := true
			for _, call := range calls {
				select {
				case ok := <-call.accepted:
					accepted = accepted && ok
				case <-call.ctx.Done():
					accepted = false
				case <-work.Done():
					accepted = false
				}
			}
			if finish != nil {
				finish(accepted)
			}
		}(calls, batch)
	}
}
