package app

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type memberUsageRepository struct {
	xnode.Repository
	identity    xnode.Identity
	identityErr error
	usage       []xnode.DailyUsage
	imported    []xnode.ImportedHistory
	owner       string
	readIDs     []int64
}

func (r *memberUsageRepository) XNodeIdentityByUser(_ context.Context, owner string) (xnode.Identity, error) {
	r.owner = owner
	return r.identity, r.identityErr
}
func (r *memberUsageRepository) XNodeIdentity(_ context.Context, id int64) (xnode.Identity, error) {
	r.readIDs = append(r.readIDs, id)
	return r.identity, nil
}
func (r *memberUsageRepository) XNodeUsage(_ context.Context, id int64, _, _ time.Time) ([]xnode.DailyUsage, error) {
	r.readIDs = append(r.readIDs, id)
	return r.usage, nil
}
func (r *memberUsageRepository) XNodeImportedHistory(_ context.Context, id int64, _, _ time.Time) ([]xnode.ImportedHistory, error) {
	r.readIDs = append(r.readIDs, id)
	return r.imported, nil
}
func (*memberUsageRepository) XNodeNodes(context.Context) ([]xnode.Node, error) {
	return []xnode.Node{{ID: "node", Name: "Member node"}, {ID: "empty"}}, nil
}

func TestMemberNodeUsageUsesOwnerAndExactDecimalTotals(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	legacy := "999"
	for _, pointer := range []*string{nil, &legacy} {
		repo := &memberUsageRepository{
			identity: xnode.Identity{UserID: "member", ProxyUserID: 42},
			usage: []xnode.DailyUsage{
				{NodeID: "node", Date: "2026-11-01", UploadBytes: math.MaxInt64, DownloadBytes: 1},
				{NodeID: "node", Date: "2026-11-02", UploadBytes: math.MaxInt64, DownloadBytes: math.MaxInt64},
			},
			imported: []xnode.ImportedHistory{{NodeID: "node", Date: "2026-11-01", RawBytes: math.MaxInt64}},
		}
		provider := proxyProvider{native: xnode.NewService(repo, nil, nil, nil, nil)}
		view, err := provider.NodeUsageForMember(context.Background(), model.User{ID: "member", ProxyUserID: pointer}, start, start.Add(48*time.Hour))
		if err != nil || repo.owner != "member" || !reflect.DeepEqual(repo.readIDs, []int64{42, 42, 42}) {
			t.Fatalf("usage owner changed: view=%+v owner=%s reads=%v err=%v", view, repo.owner, repo.readIDs, err)
		}
		if len(view.Nodes) != 2 || !reflect.DeepEqual(view.Nodes[0].DailyBytes, []string{"18446744073709551615", "18446744073709551614", "0"}) || view.Nodes[0].TotalBytes != "36893488147419103229" {
			t.Fatalf("usage totals lost precision: %+v", view)
		}
		if view.Nodes[1].TotalBytes != "0" || !reflect.DeepEqual(view.Nodes[1].DailyBytes, []string{"0", "0", "0"}) {
			t.Fatalf("missing usage did not remain zero: %+v", view.Nodes[1])
		}
	}
}

func TestMemberNodeUsageRejectsMissingAndMismatchedOwners(t *testing.T) {
	for _, scenario := range []struct {
		name, user, owner string
		lookup, want      error
	}{
		{name: "missing panel owner", owner: "member", want: xnode.ErrInvalid},
		{name: "missing native identity", user: "member", lookup: xnode.ErrNotFound, want: xnode.ErrNotFound},
		{name: "storage interruption", user: "member", lookup: context.DeadlineExceeded, want: context.DeadlineExceeded},
		{name: "mismatched native identity", user: "member", owner: "another-member", want: xnode.ErrConflict},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repo := &memberUsageRepository{identity: xnode.Identity{UserID: scenario.owner, ProxyUserID: 42}, identityErr: scenario.lookup}
			provider := proxyProvider{native: xnode.NewService(repo, nil, nil, nil, nil)}
			_, err := provider.NodeUsageForMember(context.Background(), model.User{ID: scenario.user}, time.Now(), time.Now())
			if !errors.Is(err, scenario.want) || len(repo.readIDs) != 0 {
				t.Fatalf("invalid owner reached usage: reads=%v err=%v", repo.readIDs, err)
			}
		})
	}
}

func TestMemberNodeUsageRejectsNegativeCounters(t *testing.T) {
	for _, sample := range []xnode.DailyUsage{{UploadBytes: -1}, {DownloadBytes: -1}} {
		repo := &memberUsageRepository{identity: xnode.Identity{UserID: "member", ProxyUserID: 42}, usage: []xnode.DailyUsage{sample}}
		provider := proxyProvider{native: xnode.NewService(repo, nil, nil, nil, nil)}
		if _, err := provider.NodeUsageForMember(context.Background(), model.User{ID: "member"}, time.Now(), time.Now()); !errors.Is(err, xnode.ErrInvalid) {
			t.Fatalf("invalid counter accepted: sample=%+v err=%v", sample, err)
		}
	}
}
