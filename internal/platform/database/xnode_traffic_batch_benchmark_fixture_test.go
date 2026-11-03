package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func trafficBatchBenchmarkFixture(b *testing.B, nodes, users int) (*Store, []xnode.TrafficCommit, string) {
	b.Helper()
	ctx := b.Context()
	db, err := testdatabase.Open(ctx, b)
	if err != nil {
		b.Fatal(err)
	}
	s := NewStore(db)
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "config", Name: "Benchmark", DocumentCipher: "encrypted"}); err != nil {
		b.Fatal(err)
	}
	at := time.Now().UTC()
	var samples []wire.TrafficSample
	for i := range users {
		user, _, err := s.UpsertTelegramUser(ctx, model.TelegramProfile{ID: int64(991500 + i), FirstName: "Benchmark"}, false)
		if err != nil {
			b.Fatal(err)
		}
		identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, CredentialsCipher: "encrypted"})
		if err != nil {
			b.Fatal(err)
		}
		period := fmt.Sprintf("batch-period-%d", i)
		if err := s.SaveXNodePeriod(ctx, xnode.Period{ID: period, ProxyUserID: identity.ProxyUserID, StartedAt: at, LimitBytes: 1 << 50, Strategy: "NO_RESET", Status: "open"}); err != nil {
			b.Fatal(err)
		}
		samples = append(samples, wire.TrafficSample{UserID: int(identity.ProxyUserID), AccountingPeriod: period, From: at, Through: at, UploadBytes: 1, DownloadBytes: 2})
	}
	var items []xnode.TrafficCommit
	for i := range nodes {
		node, agent := fmt.Sprintf("batch-node-%d", i), fmt.Sprintf("batch-agent-%d", i)
		if err := s.SaveXNodeNode(ctx, xnode.Node{ID: node, Name: node, ConfigID: "config", Enabled: true, MultiplierFP: 1250000, SettingsCipher: "encrypted"}); err != nil {
			b.Fatal(err)
		}
		if err := s.BindXNodeAgent(ctx, node, agent); err != nil {
			b.Fatal(err)
		}
		items = append(items, xnode.TrafficCommit{NodeID: node, AgentID: agent, Report: wire.TrafficReport{Samples: samples}})
	}
	return s, items, "batch-period-0"
}
