package xnode

import (
	"context"
	"testing"
	"time"
)

type profileCostRepository struct{ profileHistoryRepository }

func (r profileCostRepository) XNodePAYGNodeCosts(_ context.Context, owner string, from, to time.Time) ([]PAYGNodeCost, bool, error) {
	if owner != "member" || !from.Before(to) {
		return nil, false, ErrInvalid
	}
	return []PAYGNodeCost{{NodeID: "removed", SpentMinor: "9007199254740993"}, {NodeID: "credit-only", Name: "Prepaid node", SpentMinor: "300"}}, false, nil
}

func TestPAYGHistoryKeepsExactCostsSeparateFromTraffic(t *testing.T) {
	s := NewService(profileCostRepository{}, nil, nil, nil, nil)
	history, err := s.OwnedPAYGHistory(context.Background(), "member")
	if err != nil || !history.CostAvailable || len(history.Nodes) != 2 {
		t.Fatal(history, err)
	}
	credit, traffic := history.Nodes[0], history.Nodes[1]
	if credit.NodeID != "credit-only" || credit.UploadBytes != "0" || credit.DownloadBytes != "0" || credit.SpentMinor != "300" {
		t.Fatal("pre-purchased credit fabricated traffic", credit)
	}
	if traffic.UploadBytes != "18014398509481986" || traffic.DownloadBytes != "10" || traffic.SpentMinor != "9007199254740993" || len(history.Hours) != 2 {
		t.Fatal("exact traffic or charge evidence changed", history)
	}
	legacy := NewService(profileHistoryRepository{}, nil, nil, nil, nil)
	withoutCosts, err := legacy.OwnedPAYGHistory(context.Background(), "member")
	if err != nil || withoutCosts.CostAvailable || withoutCosts.Nodes[0].SpentMinor != "" {
		t.Fatal("missing charge evidence was fabricated", withoutCosts, err)
	}
}
