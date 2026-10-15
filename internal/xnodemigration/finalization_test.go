package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type finalRepository struct {
	Repository
	state      xnode.MigrationState
	identities []xnode.Identity
	writes     int
}

func (r *finalRepository) XNodeMigration(context.Context) (xnode.MigrationState, error) {
	return r.state, nil
}
func (r *finalRepository) SaveXNodeMigration(_ context.Context, state xnode.MigrationState) error {
	r.state = state
	r.writes++
	return nil
}
func (r *finalRepository) XNodeIdentities(context.Context) ([]xnode.Identity, error) {
	return r.identities, nil
}
func (r *finalRepository) MigrationPanelAccounts(context.Context) ([]model.User, error) {
	return nil, nil
}

type finalSource struct {
	Source
	snapshot Snapshot
	accounts []Account
	fenced   bool
	fenceErr error
}

func (s *finalSource) Capture(context.Context, SourceConfig, []model.User, []MonitoringMapping) (Snapshot, error) {
	return s.snapshot, nil
}
func (s *finalSource) SyncUsage(context.Context, SourceConfig, []model.User) error { return nil }
func (s *finalSource) WithProvisioningFence(ctx context.Context, fn func(context.Context) error) error {
	if s.fenceErr != nil {
		return s.fenceErr
	}
	s.fenced = true
	defer func() { s.fenced = false }()
	return fn(ctx)
}
func (s *finalSource) FinalAccounts(context.Context, SourceConfig) ([]Node, []Account, error) {
	if !s.fenced {
		return nil, nil, errors.New("final read escaped provisioning fence")
	}
	return s.snapshot.Nodes, s.accounts, nil
}

func finalizationFixture(t *testing.T) (*Service, *finalRepository, *finalSource) {
	t.Helper()
	service, base, _, _, _ := migrationCutoverFixture(t)
	base.state.Nodes[0].Phase = "native"
	repo := &finalRepository{state: base.state}
	source := &finalSource{snapshot: Snapshot{Nodes: []Node{{ID: "node"}}}}
	service.repo, service.source = repo, source
	// The controller only checks acknowledgements for this finalization fixture.
	initial, err := service.vault.Encrypt("xnode.migration.snapshot", `{"monitoring":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	repo.state.SnapshotCipher = initial
	return service, repo, source
}

func TestFinalizationRequiresStagedExportAndIsReplaySafe(t *testing.T) {
	service, repo, source := finalizationFixture(t)
	ctx := context.Background()
	if _, err := service.Finalize(ctx, "unstaged"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("unstaged finalization: %v", err)
	}
	state, err := service.StageFinal(ctx, "source-backup-sha256")
	if err != nil || len(state.FinalDigest) != 64 || state.Phase != "active" || state.FinalSnapshotCipher == "" {
		t.Fatalf("stage: %+v, %v", state, err)
	}
	encoded, _ := json.Marshal(state)
	var public map[string]any
	_ = json.Unmarshal(encoded, &public)
	if _, exposed := public["FinalSnapshotCipher"]; exposed || public["finalDigest"] != state.FinalDigest {
		t.Fatalf("invalid public state: %s", encoded)
	}
	result, err := service.Finalize(ctx, state.FinalDigest)
	if err != nil || result.Phase != "complete" || result.FinalizedAt == nil || source.fenced {
		t.Fatalf("final: %+v, %v", result, err)
	}
	writes := repo.writes
	if _, err = service.Finalize(ctx, state.FinalDigest); err != nil || repo.writes != writes {
		t.Fatalf("replayed completion writes=%d: %v", repo.writes, err)
	}
}

func TestFinalizationRejectsEnabledSourceAndUnfinishedOwnership(t *testing.T) {
	service, repo, source := finalizationFixture(t)
	source.snapshot.Nodes[0].Enabled = true
	if _, err := service.StageFinal(context.Background(), "backup"); !errors.Is(err, xnode.ErrConflict) || repo.writes != 0 {
		t.Fatalf("enabled source: %v", err)
	}
	source.snapshot.Nodes[0].Enabled = false
	repo.state.Nodes[0].Phase = "review"
	if _, err := service.StageFinal(context.Background(), "backup"); !errors.Is(err, xnode.ErrConflict) || repo.writes != 0 {
		t.Fatalf("unfinished handover: %v", err)
	}
}

func TestInterruptedFinalizationRetainsActiveBridge(t *testing.T) {
	service, repo, source := finalizationFixture(t)
	state, err := service.StageFinal(context.Background(), "backup")
	if err != nil {
		t.Fatal(err)
	}
	source.fenceErr = context.DeadlineExceeded
	if _, err = service.Finalize(context.Background(), state.FinalDigest); !errors.Is(err, context.DeadlineExceeded) || repo.state.Phase != "active" {
		t.Fatalf("interrupted finalization: %+v, %v", repo.state, err)
	}
	source.fenceErr = nil
	if _, err = service.Finalize(context.Background(), state.FinalDigest); err != nil {
		t.Fatalf("resume: %v", err)
	}
}

func TestSourceReenabledAfterExportCannotFinalize(t *testing.T) {
	service, repo, source := finalizationFixture(t)
	state, err := service.StageFinal(context.Background(), "backup")
	if err != nil {
		t.Fatal(err)
	}
	source.snapshot.Nodes[0].Enabled = true
	if _, err = service.Finalize(context.Background(), state.FinalDigest); !errors.Is(err, xnode.ErrConflict) || repo.state.Phase != "active" {
		t.Fatalf("reopened source completed migration: %+v,%v", repo.state, err)
	}
}

func TestFinalAccountVerificationRejectsTrafficChangesAndNewPurchases(t *testing.T) {
	service, repo, _ := finalizationFixture(t)
	repo.identities = []xnode.Identity{{UserID: "member", LegacyUserID: "42"}}
	before := Account{LegacyID: 42, PanelUserID: "member", UUID: "uuid", Trojan: "secret", Chacha: "chacha", UsedBytes: 10, LifetimeBytes: 20}
	snapshot := Snapshot{Accounts: []Account{before}}
	if err := service.verifyFinalAccounts(context.Background(), snapshot, []Account{before}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"current", "lifetime", "uuid", "trojan", "chacha"} {
		after := before
		switch field {
		case "current":
			after.UsedBytes++
		case "lifetime":
			after.LifetimeBytes++
		case "uuid":
			after.UUID = "changed"
		case "trojan":
			after.Trojan = "changed"
		case "chacha":
			after.Chacha = "changed"
		}
		if err := service.verifyFinalAccounts(context.Background(), snapshot, []Account{after}); !errors.Is(err, xnode.ErrConflict) {
			t.Fatalf("changed %s: %v", field, err)
		}
	}
	repo.identities = append(repo.identities, xnode.Identity{UserID: "new-purchase", LegacyUserID: "43"})
	if err := service.verifyFinalAccounts(context.Background(), snapshot, []Account{before}); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("new purchase missing from export: %v", err)
	}
}
