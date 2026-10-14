package iplookup

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type subnetProber func(context.Context, string, int, string, Config, func(int) error) probeOutcome

func (s *EnhancedService) probeSubnet(ctx context.Context, r *Report, c Config, save func() error) error {
	return s.runSubnetProbes(ctx, r, c, save, s.probe)
}

// runSubnetProbes always scans eligible IPv4 rows, independently of reputation gating.
func (s *EnhancedService) runSubnetProbes(ctx context.Context, r *Report, c Config, save func() error, probe subnetProber) error {
	if r.Subnet == nil || r.Subnet.Reason == "ipv6" {
		return nil
	}
	prepareSubnetProbes(r)
	r.Subnet.Status = "processing"
	if err := save(); err != nil {
		return err
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	var work sync.WaitGroup
	var firstErr error
	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
	}
	jobs := make(chan int)
	for range 2 {
		work.Add(1)
		go func() {
			defer work.Done()
			for index := range jobs {
				for portIndex := range r.Subnet.Addresses[index].Ports {
					for taskIndex, key := range subnetProbeKeys {
						mu.Lock()
						row := &r.Subnet.Addresses[index]
						port := &row.Ports[portIndex]
						if row.Status == "flagged" {
							skipSubnetPortTasks(port)
							if !subnetProbeFinished(port.Status) {
								port.Status = "skipped"
							}
						}
						ip, number := row.IP, port.Port
						finished := subnetProbeFinished(port.Status) || subnetProbeFinished(port.Tasks[taskIndex].Status)
						mu.Unlock()
						if finished || workCtx.Err() != nil {
							continue
						}
						_, err := upstreamqueue.Do(workCtx, s.queues["probes"], func(callCtx context.Context) (struct{}, error) {
							mu.Lock()
							port := &r.Subnet.Addresses[index].Ports[portIndex]
							port.Status = "processing"
							task := &port.Tasks[taskIndex]
							task.Status, task.StartedAt = "active", time.Now().UTC().Format(time.RFC3339Nano)
							task.ErrorCode = ""
							err := save()
							mu.Unlock()
							if err != nil {
								return struct{}{}, err
							}
							out := probe(callCtx, ip, number, key, c, func(int) error { return nil })
							if out.err != nil {
								return struct{}{}, out.err
							}
							if err := callCtx.Err(); err != nil {
								return struct{}{}, err
							}
							mu.Lock()
							defer mu.Unlock()
							row := &r.Subnet.Addresses[index]
							port = &row.Ports[portIndex]
							task = &port.Tasks[taskIndex]
							task.Status, task.Result = out.status, out.result
							if out.status == "error" {
								task.ErrorCode = "IP_PROBE_UNAVAILABLE"
							}
							recordProbeOutcome(port, key, out)
							if key == "tcp" && out.status != "success" {
								port.Status = out.status
								if !subnetProbeFinished(port.Status) {
									port.Status = "error"
								}
								skipSubnetPortTasks(port)
							}
							applySubnetProbeFinding(row)
							if row.IP == r.IP {
								for _, finding := range out.findings {
									if finding.Protocol == "cloudflare" && finding.Confidence == "confirmed" {
										upsertPortRefusal(r, "cloudflare_cdn", "cloudflare", number)
										break
									}
								}
							}
							return struct{}{}, save()
						})
						if err != nil {
							fail(err)
							break
						}
					}
					mu.Lock()
					port := &r.Subnet.Addresses[index].Ports[portIndex]
					if workCtx.Err() == nil && !subnetProbeFinished(port.Status) {
						port.Status = "success"
						for _, task := range port.Tasks {
							if task.Status == "error" {
								port.Status = "error"
							}
						}
						if err := save(); err != nil {
							mu.Unlock()
							fail(err)
							break
						}
					}
					mu.Unlock()
				}
			}
		}()
	}
	for index := range r.Subnet.Addresses {
		select {
		case jobs <- index:
		case <-workCtx.Done():
		}
		if workCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	work.Wait()
	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	finishSubnetProbes(r.Subnet)
	return save()
}
