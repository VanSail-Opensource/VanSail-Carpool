package xnode

import (
	"context"
	"math/big"
	"sort"
	"time"
)

const MaxPAYGHistoryNodes = 128

type PAYGNodeUsageHour struct {
	NodeID, Name, CountryCode, UploadBytes, DownloadBytes string
	Hour                                                  time.Time
}

type PAYGHistoryHour struct {
	NodeID        string    `json:"nodeId"`
	Hour          time.Time `json:"hour"`
	UploadBytes   string    `json:"uploadBytes"`
	DownloadBytes string    `json:"downloadBytes"`
}

type PAYGHistoryNode struct {
	SpentMinor    string `json:"spentMinor,omitempty"`
	NodeID        string `json:"nodeId"`
	Name          string `json:"name"`
	CountryCode   string `json:"countryCode"`
	UploadBytes   string `json:"uploadBytes"`
	DownloadBytes string `json:"downloadBytes"`
}

type PAYGNodeHistory struct {
	CostAvailable bool              `json:"costAvailable"`
	From          time.Time         `json:"from"`
	To            time.Time         `json:"to"`
	Nodes         []PAYGHistoryNode `json:"nodes"`
	Hours         []PAYGHistoryHour `json:"hours"`
	Truncated     bool              `json:"truncated"`
}

type paygNodeHistoryRepository interface {
	XNodePAYGNodeHistory(context.Context, string, time.Time, time.Time) ([]PAYGNodeUsageHour, bool, error)
}

// OwnedPAYGHistory reuses exact owner-scoped recorded history; no attribution is reconstructed.
func (s *Service) OwnedPAYGHistory(ctx context.Context, user string) (PAYGNodeHistory, error) {
	return s.profilePAYGHistory(ctx, user)
}

func (s *Service) profilePAYGHistory(ctx context.Context, user string) (PAYGNodeHistory, error) {
	now := s.now().UTC()
	result := PAYGNodeHistory{From: now.Add(-PAYGWindow).Truncate(time.Minute), To: now, Nodes: []PAYGHistoryNode{}, Hours: []PAYGHistoryHour{}}
	repo, ok := s.repo.(paygNodeHistoryRepository)
	if !ok {
		return result, nil
	}
	rows, truncated, err := repo.XNodePAYGNodeHistory(ctx, user, result.From, result.To)
	if err != nil {
		return result, err
	}
	result.Truncated = truncated
	type total struct {
		view             PAYGHistoryNode
		upload, download big.Int
	}
	nodes := map[string]*total{}
	for _, row := range rows {
		upload, upOK := new(big.Int).SetString(row.UploadBytes, 10)
		download, downOK := new(big.Int).SetString(row.DownloadBytes, 10)
		if !upOK || !downOK || upload.Sign() < 0 || download.Sign() < 0 {
			return result, ErrInvalid
		}
		if nodes[row.NodeID] == nil {
			nodes[row.NodeID] = &total{view: PAYGHistoryNode{NodeID: row.NodeID, Name: row.Name, CountryCode: row.CountryCode}}
		}
		node := nodes[row.NodeID]
		node.upload.Add(&node.upload, upload)
		node.download.Add(&node.download, download)
		result.Hours = append(result.Hours, PAYGHistoryHour{row.NodeID, row.Hour, row.UploadBytes, row.DownloadBytes})
	}
	for _, node := range nodes {
		node.view.UploadBytes, node.view.DownloadBytes = node.upload.String(), node.download.String()
		result.Nodes = append(result.Nodes, node.view)
	}
	if err := s.appendPAYGCosts(ctx, user, &result); err != nil {
		return result, err
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].NodeID < result.Nodes[j].NodeID })
	if len(result.Nodes) > MaxPAYGHistoryNodes {
		result.Truncated = true
		result.Nodes = result.Nodes[:MaxPAYGHistoryNodes]
		selected := map[string]bool{}
		for _, node := range result.Nodes {
			selected[node.NodeID] = true
		}
		hours := result.Hours[:0]
		for _, hour := range result.Hours {
			if selected[hour.NodeID] {
				hours = append(hours, hour)
			}
		}
		result.Hours = hours
	}
	return result, nil
}
