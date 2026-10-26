package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func metricNumber(raw json.RawMessage) (float64, bool) {
	v, err := strconv.ParseFloat(strings.Trim(string(raw), `"`), 64)
	return v, err == nil && v >= 0
}

func machineUtilization(raw json.RawMessage) (float64, float64, bool) {
	var m struct {
		CPU    []json.RawMessage          `json:"cpu_usage"`
		Memory map[string]json.RawMessage `json:"memory"`
	}
	if json.Unmarshal(raw, &m) != nil || len(m.CPU) == 0 {
		return 0, 0, false
	}
	cpu := 0.0
	for _, r := range m.CPU {
		v, ok := metricNumber(r)
		if !ok || v > 100 {
			return 0, 0, false
		}
		cpu += v
	}
	memory, ok := metricNumber(m.Memory["usedPercent"])
	return cpu / float64(len(m.CPU)), memory, ok && memory <= 100
}

func (a *Application) observePricingHealth(ctx context.Context, now time.Time) error {
	metrics, err := a.xnodeMaster.Metrics(ctx, true)
	if err != nil {
		return err
	}
	probes := map[string]bool{}
	if a.connectivity != nil {
		snapshot, e := a.connectivity.Snapshot(ctx)
		if e == nil && !snapshot.Stale {
			endpoints, e := a.xnodeService.Endpoints(ctx)
			if e != nil {
				return e
			}
			byEndpoint := map[string]string{}
			for _, p := range endpoints {
				byEndpoint[p.ID] = p.NodeID
			}
			for _, h := range snapshot.Hosts {
				if h.Latest != nil && h.Latest.FinishedAt != nil && now.Sub(*h.Latest.FinishedAt) < 5*time.Minute && h.Latest.Status == "failed" {
					probes[byEndpoint[h.HostUUID]] = true
				}
			}
		}
	}
	for _, node := range metrics.Nodes {
		cpu, memory, complete := machineUtilization(node.Machine)
		reported := time.Time{}
		if node.ReportedAt != nil {
			reported = *node.ReportedAt
		}
		maintenance, e := a.store.PricingMaintenance(ctx, node.NodeID, now)
		if e != nil {
			return e
		}
		h := a.store.PricingHealth().Observe(pricing.Observation{NodeID: node.NodeID, At: now, ReportedAt: reported, Connected: node.Connected, Ready: node.Ready, Synchronized: node.Synchronized, Stale: node.Stale, ProbeUnavailable: probes[node.NodeID], CPU: cpu, Memory: memory, Complete: complete, Maintenance: maintenance})
		if err = a.store.RecordPricingHealth(ctx, h); err != nil {
			return err
		}
	}
	return nil
}
