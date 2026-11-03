package xnode

import (
	"context"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const MaxTrafficBatchReports = 256

type TrafficCommit struct {
	NodeID, AgentID string
	Sequence        uint64
	Digest          string
	Report          wire.TrafficReport
}

type TrafficBatchRepository interface {
	RecordXNodeTrafficBatch(context.Context, []TrafficCommit) ([]error, error)
}
