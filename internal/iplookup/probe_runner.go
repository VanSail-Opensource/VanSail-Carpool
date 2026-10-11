package iplookup

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

var probeKeys = []string{"tcp", "socks5", "socks4", "http", "tls", "shadowsocks", "trojan_alpn", "trojan_short", "trojan_close", "trojan_incomplete", "trojan_overbuffer"}

type probeOutcome struct {
	status   string
	findings []ProbeFinding
	tls      bool
}

func (s *EnhancedService) probePorts(ctx context.Context, r *Report, c Config, save func() error) error {
	if r.Ports == nil {
		seen := map[int]bool{443: true, 4430: true, 8443: true}
		for _, p := range c.ProbePorts {
			seen[p] = true
		}
		if r.LiveDetails != nil {
			for _, p := range r.LiveDetails.Shodan.Ports {
				if p > 0 && p <= 65535 {
					seen[p] = true
				}
			}
		}
		ports := []int{}
		for p := range seen {
			ports = append(ports, p)
		}
		sort.Ints(ports)
		r.Ports = []PortProbe{}
		for _, p := range ports {
			port := PortProbe{Port: p, Status: "pending", Tasks: []ProbeTask{}, Findings: []ProbeFinding{}}
			for _, key := range probeKeys {
				port.Tasks = append(port.Tasks, ProbeTask{Key: key, Status: "pending"})
			}
			r.Ports = append(r.Ports, port)
		}
		if err := save(); err != nil {
			return err
		}
	}
	var mu sync.Mutex
	var work sync.WaitGroup
	var firstErr error
	jobs := make(chan int)
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	fail := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		mu.Unlock()
	}
	for range 2 {
		work.Add(1)
		go func() {
			defer work.Done()
			for index := range jobs {
				mu.Lock()
				port := r.Ports[index]
				mu.Unlock()
				if port.Status == "success" || port.Status == "closed" {
					continue
				}
				tlsReady := false
				for _, f := range port.Findings {
					tlsReady = tlsReady || f.Protocol == "tls"
				}
				for taskIndex, key := range probeKeys {
					mu.Lock()
					task := r.Ports[index].Tasks[taskIndex]
					mu.Unlock()
					if task.Status == "success" || task.Status == "skipped" {
						continue
					}
					if workCtx.Err() != nil {
						break
					}
					if len(key) > 7 && key[:7] == "trojan_" && !tlsReady {
						mu.Lock()
						r.Ports[index].Tasks[taskIndex].Status = "skipped"
						err := save()
						mu.Unlock()
						if err != nil {
							fail(err)
						}
						continue
					}
					_, err := upstreamqueue.Do(workCtx, s.queues["probes"], func(callCtx context.Context) (struct{}, error) {
						mu.Lock()
						r.Ports[index].Status = "processing"
						t := &r.Ports[index].Tasks[taskIndex]
						t.Status = "active"
						t.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
						t.ErrorCode = ""
						t.NextMilestone = 0
						err := save()
						mu.Unlock()
						if err != nil {
							return struct{}{}, err
						}
						milestone := func(seconds int) error {
							mu.Lock()
							defer mu.Unlock()
							r.Ports[index].Tasks[taskIndex].NextMilestone = seconds
							return save()
						}
						outcome := s.probe(callCtx, r.IP, port.Port, key, c, milestone)
						if callCtx.Err() != nil {
							return struct{}{}, callCtx.Err()
						}
						mu.Lock()
						defer mu.Unlock()
						t = &r.Ports[index].Tasks[taskIndex]
						t.Status = outcome.status
						t.NextMilestone = 0
						if outcome.status == "error" {
							t.ErrorCode = "IP_PROBE_UNAVAILABLE"
						}
						r.Ports[index].Findings = append(r.Ports[index].Findings, outcome.findings...)
						tlsReady = tlsReady || outcome.tls
						if key == "tcp" && outcome.status == "closed" {
							r.Ports[index].Status = "closed"
							t.Status = "success"
							for next := taskIndex + 1; next < len(probeKeys); next++ {
								r.Ports[index].Tasks[next].Status = "skipped"
							}
						}
						return struct{}{}, save()
					})
					if err != nil {
						fail(err)
						break
					}
					mu.Lock()
					closed := r.Ports[index].Status == "closed"
					mu.Unlock()
					if closed {
						break
					}
				}
				mu.Lock()
				if workCtx.Err() == nil && r.Ports[index].Status != "closed" {
					r.Ports[index].Status = "success"
					for _, t := range r.Ports[index].Tasks {
						if t.Status == "error" {
							r.Ports[index].Status = "error"
						}
					}
					if err := save(); err != nil && firstErr == nil {
						firstErr = err
						cancel()
					}
				}
				mu.Unlock()
			}
		}()
	}
	for i := range r.Ports {
		select {
		case jobs <- i:
		case <-workCtx.Done():
			break
		}
		if workCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	work.Wait()
	return firstErr
}
