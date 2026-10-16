// Package statistics owns cached product metrics and read-only provider statistics.
package statistics

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

// Digest is the remote range growth summary.
type Digest struct {
	CreatedUsers int64
	ExpiredUsers int64
}

// TrafficSeries is one raw per-node traffic series.
type TrafficSeries struct {
	UUID        string
	Name        string
	CountryCode string
	DailyBytes  []int64
}

// Traffic contains the provider's range labels and raw series.
type Traffic struct {
	Categories []string
	Series     []TrafficSeries
}

// Node joins stable node metadata with live system rates.
type Node struct {
	UUID             string
	Name             string
	CountryCode      string
	Online           bool
	UsersOnline      int64
	RXBytesPerSecond int64
	TXBytesPerSecond int64
	XrayVersion      string
	Multiplier       float64
}

type Provider interface {
	Digest(context.Context, time.Time, time.Time) (Digest, error)
	Traffic(context.Context, time.Time, time.Time) (Traffic, error)
	UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (rollover.UsageSnapshot, error)
	Nodes(context.Context) ([]Node, error)
}

// SquadNameProvider exposes the live Remnawave identity labels used only to
// decorate statistics. The labels remain upstream-owned and are not stored.
type SquadNameProvider interface {
	SquadNames(context.Context) (map[string]string, error)
}
