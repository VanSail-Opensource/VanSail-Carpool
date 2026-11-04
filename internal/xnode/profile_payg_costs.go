package xnode

import (
	"context"
	"math/big"
	"time"
)

type PAYGNodeCost struct{ NodeID, Name, CountryCode, SpentMinor string }
type paygNodeCostRepository interface {
	XNodePAYGNodeCosts(context.Context, string, time.Time, time.Time) ([]PAYGNodeCost, bool, error)
}

// Costs come from immutable PAYG purchases in the same window, never from a
// current price multiplied by historical traffic. Cost-only nodes remain valid.
func (s *Service) appendPAYGCosts(ctx context.Context, user string, history *PAYGNodeHistory) error {
	repo, ok := s.repo.(paygNodeCostRepository)
	if !ok {
		return nil
	}
	costs, truncated, err := repo.XNodePAYGNodeCosts(ctx, user, history.From, history.To)
	if err != nil {
		return err
	}
	history.CostAvailable = true
	history.Truncated = history.Truncated || truncated
	indexes := map[string]int{}
	for i := range history.Nodes {
		indexes[history.Nodes[i].NodeID] = i
		history.Nodes[i].SpentMinor = "0"
	}
	for _, cost := range costs {
		minor, ok := new(big.Int).SetString(cost.SpentMinor, 10)
		if !ok || minor.Sign() < 0 {
			return ErrInvalid
		}
		index, exists := indexes[cost.NodeID]
		if !exists {
			index = len(history.Nodes)
			indexes[cost.NodeID] = index
			history.Nodes = append(history.Nodes, PAYGHistoryNode{NodeID: cost.NodeID, Name: cost.Name, CountryCode: cost.CountryCode, UploadBytes: "0", DownloadBytes: "0"})
		}
		history.Nodes[index].SpentMinor = minor.String()
	}
	return nil
}
