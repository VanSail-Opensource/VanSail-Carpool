package xnode

import (
	"context"
	"sort"
	"strconv"
)

// UserNodeMetric is a memory-only, owner-scoped activity projection.
type UserNodeMetric struct {
	NodeID                 string  `json:"nodeId"`
	Name                   string  `json:"name"`
	CountryCode            string  `json:"countryCode"`
	Available              bool    `json:"available"`
	CurrentUse             bool    `json:"currentUse"`
	Connections            string  `json:"connections"`
	UploadBytesPerSecond   float64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	UploadBytes            string  `json:"uploadBytes"`
	DownloadBytes          string  `json:"downloadBytes"`
	WeightedBytes          string  `json:"weightedBytes"`
}

type ProfileMetrics struct {
	UserMetrics
	Nodes                []UserNodeMetric `json:"nodes"`
	Usage                []ProfileUsage   `json:"usage"`
	UsedTrafficBytes     string           `json:"usedTrafficBytes"`
	LifetimeTrafficBytes string           `json:"lifetimeTrafficBytes"`
}

// UserNodeMetrics never substitutes source-provider data for unavailable agents.
func (m *Master) UserNodeMetrics(ctx context.Context, userID string) ([]UserNodeMetric, error) {
	identity, err := m.service.repo.XNodeIdentityByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	scope, _, err := m.service.metricNodeScope(ctx, identity)
	if err != nil {
		return nil, err
	}
	nodes, err := m.service.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return nil, err
	}
	principals, err := m.service.MemberPrincipals(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	now := m.service.now()
	result := []UserNodeMetric{}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, node := range nodes {
		owned, included := scope[node.ID]
		if !included {
			continue
		}
		snapshot := m.live[node.ID]
		item := UserNodeMetric{NodeID: node.ID, Name: node.Name, CountryCode: node.CountryCode, Connections: "0", UploadBytes: "0", DownloadBytes: "0", WeightedBytes: "0"}
		item.Available = owned && node.Enabled && m.peers[node.ID] != nil && activityAvailable(snapshot, now, global.Telemetry.ReportSeconds)
		if item.Available {
			count := int64(0)
			for _, principal := range principals {
				id := int(principal)
				count += int64(snapshot.report.Users.Connections[id])
				rate := snapshot.report.Users.Rates[id]
				item.UploadBytesPerSecond += rate.UploadBytesPerSecond
				item.DownloadBytesPerSecond += rate.DownloadBytesPerSecond
			}
			item.Connections = strconv.FormatInt(count, 10)
			item.CurrentUse = count > 0 || item.UploadBytesPerSecond > 0 || item.DownloadBytesPerSecond > 0
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// ProfileMetrics combines live native observations with existing exact accounting.
func (m *Master) ProfileMetrics(ctx context.Context, userID string) (ProfileMetrics, error) {
	live, err := m.UserMetrics(ctx, userID)
	if err != nil {
		return ProfileMetrics{}, err
	}
	nodes, err := m.UserNodeMetrics(ctx, userID)
	if err != nil {
		return ProfileMetrics{}, err
	}
	result := ProfileMetrics{UserMetrics: live, Nodes: nodes, Usage: []ProfileUsage{}}
	return m.service.profileAccounting(ctx, userID, result)
}
