package app

import (
	"context"
	"encoding/json"
	"errors"
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

func (a *Application) observePricingHealth(ctx context.Context, now time.Time, recorder *pricingHealthRecorder) error {
	nodes, err := a.xnodeMaster.MachineMetrics(ctx)
	if err != nil {
		return err
	}
	recorder.retain(nodes)
	// Anchor the observation after taking the telemetry snapshot; the worker's
	// initial timestamp can precede a report received during the inventory reads.
	observedAt := time.Now().UTC()
	registry := a.store.PricingHealth()
	var result error
	pending := make([]pricing.Health, 0, len(nodes))
	for _, node := range nodes {
		if ctx.Err() != nil {
			return errors.Join(result, ctx.Err())
		}
		if !node.Enabled {
			registry.Remove(node.NodeID)
			pending = append(pending, pricing.Health{NodeID: node.NodeID, State: "HEALTHY", At: observedAt})
			continue
		}
		cpu, memory, complete := machineUtilization(node.Machine)
		reported := time.Time{}
		if node.ReportedAt != nil {
			reported = *node.ReportedAt
		}
		maintenance, e := a.store.PricingMaintenance(ctx, node.NodeID, now)
		if e != nil {
			result = errors.Join(result, e)
			continue
		}
		h := registry.Observe(pricing.Observation{NodeID: node.NodeID, At: observedAt, ReportedAt: reported, ReceivedAt: node.ReceivedAt, ValidUntil: node.ValidUntil, Connected: node.Connected, Ready: node.Ready, Synchronized: node.Synchronized, Stale: node.Stale, CPU: cpu, Memory: memory, Complete: complete, Maintenance: maintenance})
		pending = append(pending, h)
	}
	return errors.Join(result, recorder.recordBatch(ctx, a.store, pending))
}
