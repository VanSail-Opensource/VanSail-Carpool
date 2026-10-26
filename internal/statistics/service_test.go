package statistics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

func TestRefreshRemoteUsesCreatedUsersAndRolloverTermUsage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	firstID, secondID := "remote-1", "remote-2"
	repository := &statisticsRepositoryStub{members: []model.StatisticsUsageMember{
		{Purchase: model.Purchase{ID: "purchase-1", UserID: "user-1", PriceTXBMinor: 1_000, AutoRenewEnabled: true, ValidFrom: now.AddDate(0, 0, -10), ValidUntil: now.AddDate(0, 0, 20)}, RemoteUserID: firstID},
		{Purchase: model.Purchase{ID: "purchase-2", UserID: "user-2", PriceTXBMinor: 1_000, AutoRenewEnabled: true, ValidFrom: now.AddDate(0, 0, -40), ValidUntil: now.AddDate(0, 0, 20)}, RemoteUserID: secondID},
	}}
	firstUsed, secondUsed := int64(250), int64(750)
	provider := &statisticsProviderStub{
		digest: Digest{CreatedUsers: 9, ExpiredUsers: 4},
		snapshots: map[string]usageaccounting.UsageSnapshot{
			firstID:  {LimitBytes: 1_000, Strategy: "NO_RESET", WeightedUsedBytes: firstUsed, NodeSeriesAvailable: true},
			secondID: {LimitBytes: 1_000, Strategy: "NO_RESET", WeightedUsedBytes: secondUsed, NodeSeriesAvailable: true},
		},
	}
	remote, err := NewService(repository, provider).refreshRemote(context.Background(), now)
	if err != nil {
		t.Fatalf("refreshRemote(): %v", err)
	}
	if remote.WeeklyUserIncrease != 9 || remote.MonthlyAverageUsageBPS != 5_000 || remote.MonthlyAverageUsage != 50 {
		t.Fatalf("remote statistics = %+v", remote)
	}
	if !provider.starts[firstID].Equal(now.AddDate(0, 0, -10)) || !provider.starts[secondID].Equal(now.AddDate(0, 0, -40)) {
		t.Fatalf("usage starts = %+v", provider.starts)
	}
}

func TestRefreshRemoteSkipsUnavailableMemberUsage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	remoteID := "remote-1"
	sentinel := errors.New("provider unavailable")
	repository := &statisticsRepositoryStub{members: []model.StatisticsUsageMember{{
		Purchase: model.Purchase{ID: "purchase-1", UserID: "user-1", ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour)}, RemoteUserID: remoteID,
	}}}
	provider := &statisticsProviderStub{usageErrors: map[string]error{remoteID: sentinel}}
	remote, err := NewService(repository, provider).refreshRemote(context.Background(), now)
	if err != nil || remote.MonthlyAverageUsageBPS != 0 {
		t.Fatalf("refreshRemote() = (%+v, %v)", remote, err)
	}
}

func TestRefreshRemoteSkipsMembersWithoutUsableProviderIdentity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	remoteID := "remote-1"
	used := int64(250)
	repository := &statisticsRepositoryStub{members: []model.StatisticsUsageMember{
		{Purchase: model.Purchase{ID: "missing-upstream", UserID: "user-2", ValidFrom: now.AddDate(0, 0, -10), ValidUntil: now.AddDate(0, 0, 10)}, RemoteUserID: "missing"},
		{Purchase: model.Purchase{ID: "usable", UserID: "user-3", ValidFrom: now.AddDate(0, 0, -10), ValidUntil: now.AddDate(0, 0, 10)}, RemoteUserID: remoteID},
	}}
	provider := &statisticsProviderStub{
		snapshots:   map[string]usageaccounting.UsageSnapshot{remoteID: {LimitBytes: 1_000, Strategy: "NO_RESET", WeightedUsedBytes: used, NodeSeriesAvailable: true}},
		usageErrors: map[string]error{"missing": errors.New("account unavailable")},
	}
	remote, err := NewService(repository, provider).refreshRemote(context.Background(), now)
	if err != nil || remote.MonthlyAverageUsageBPS != 2_500 {
		t.Fatalf("refreshRemote() = (%+v, %v)", remote, err)
	}
}

type statisticsRepositoryStub struct {
	members []model.StatisticsUsageMember
}

func (r *statisticsRepositoryStub) ProductDatabaseStatistics(context.Context, time.Time) (model.DatabaseStatistics, error) {
	return model.DatabaseStatistics{}, nil
}
func (r *statisticsRepositoryStub) ActiveMemberUsageForStatistics(context.Context, time.Time) ([]model.StatisticsUsageMember, error) {
	return append([]model.StatisticsUsageMember(nil), r.members...), nil
}
func (r *statisticsRepositoryStub) LoadStatisticsPartition(context.Context, string) ([]byte, time.Time, error) {
	return nil, time.Time{}, ErrPartitionNotFound
}
func (r *statisticsRepositoryStub) SaveStatisticsPartition(context.Context, string, []byte, time.Time) error {
	return nil
}

type statisticsProviderStub struct {
	digest      Digest
	snapshots   map[string]usageaccounting.UsageSnapshot
	usageErrors map[string]error
	starts      map[string]time.Time
	nodes       []Node
}

func (p *statisticsProviderStub) Digest(context.Context, time.Time, time.Time) (Digest, error) {
	return p.digest, nil
}
func (p *statisticsProviderStub) Traffic(context.Context, time.Time, time.Time) (Traffic, error) {
	return Traffic{}, nil
}
func (p *statisticsProviderStub) UsageSnapshot(_ context.Context, id string, start, _ time.Time) (usageaccounting.UsageSnapshot, error) {
	if p.starts == nil {
		p.starts = make(map[string]time.Time)
	}
	p.starts[id] = start
	if err := p.usageErrors[id]; err != nil {
		return usageaccounting.UsageSnapshot{}, err
	}
	return p.snapshots[id], nil
}
func (p *statisticsProviderStub) Nodes(context.Context) ([]Node, error) {
	return append([]Node(nil), p.nodes...), nil
}
