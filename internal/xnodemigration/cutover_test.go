package xnodemigration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

type cutoverRepo struct {
	Repository
	state  xnode.MigrationState
	events *[]string
}

func (r *cutoverRepo) XNodeMigration(context.Context) (xnode.MigrationState, error) {
	return r.state, nil
}
func (r *cutoverRepo) MigrationPanelAccounts(context.Context) ([]model.User, error) { return nil, nil }
func (r *cutoverRepo) SaveXNodeMigrationNode(_ context.Context, node xnode.MigrationNode) error {
	r.state.Nodes[0] = node
	*r.events = append(*r.events, "ownership:"+node.Phase)
	return nil
}

type cutoverPanel struct {
	Panel
	verifyErr error
}

func (p cutoverPanel) Now() time.Time                                      { return time.Now().UTC() }
func (p cutoverPanel) VerifyClientEndpoints(context.Context, string) error { return nil }
func (p cutoverPanel) VerifyRelease(context.Context, string) (xnode.ReleaseEvidence, error) {
	return xnode.ReleaseEvidence{}, p.verifyErr
}
func (p cutoverPanel) Candidate(context.Context, string) (wire.Config, error) {
	return wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`)}, nil
}

type cutoverSource struct {
	Source
	syncCalls  int
	syncErr    error
	disableErr error
	events     *[]string
}

func (s *cutoverSource) SetNodeDisabled(_ context.Context, _ SourceConfig, _ string, disabled bool) error {
	if disabled {
		*s.events = append(*s.events, "source:disable")
		return s.disableErr
	}
	*s.events = append(*s.events, "source:enable")
	return nil
}
func (s *cutoverSource) SyncUsage(context.Context, SourceConfig, []model.User) error {
	s.syncCalls++
	*s.events = append(*s.events, "source:usage")
	if s.syncCalls == 1 {
		return s.syncErr
	}
	return nil
}

type cutoverController struct {
	Controller
	repo        *cutoverRepo
	applyErr    error
	rollbackErr error
	events      *[]string
}

func (c *cutoverController) Command(context.Context, string, string, string, any) (json.RawMessage, error) {
	*c.events = append(*c.events, "preflight")
	return nil, nil
}
func (c *cutoverController) Sync(context.Context, string) error {
	*c.events = append(*c.events, "sync:"+c.repo.state.Nodes[0].Phase)
	if c.repo.state.Nodes[0].Phase == "native" {
		return c.applyErr
	}
	return c.rollbackErr
}
func (c *cutoverController) WaitApplied(context.Context, string) error { return nil }
func (c *cutoverController) Flush(context.Context, string, int64, bool) (wire.TrafficBarrier, error) {
	*c.events = append(*c.events, "native:flush")
	return wire.TrafficBarrier{}, nil
}

type cutoverBackup struct{}

func (cutoverBackup) Run(context.Context) (model.BackupRun, error) {
	return model.BackupRun{Status: "complete"}, nil
}

func migrationCutoverFixture(t *testing.T) (*Service, *cutoverRepo, *cutoverSource, *cutoverController, *[]string) {
	t.Helper()
	events := []string{}
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := vault.Encrypt("xnode.migration.source", `{"url":"https://source.example","token":"private"}`)
	if err != nil {
		t.Fatal(err)
	}
	repo := &cutoverRepo{state: xnode.MigrationState{Phase: "active", SourceCipher: cipher, Nodes: []xnode.MigrationNode{{NodeID: "node", LegacyNodeID: "node", Phase: "legacy"}}}, events: &events}
	source := &cutoverSource{events: &events}
	controller := &cutoverController{repo: repo, events: &events}
	return New(repo, source, cutoverPanel{}, controller, vault, cutoverBackup{}), repo, source, controller, &events
}

func TestHandoverRestoresLegacyServingForEveryPostDisableFailure(t *testing.T) {
	for _, stage := range []string{"ambiguous-disable", "usage", "activation"} {
		t.Run(stage, func(t *testing.T) {
			service, repo, source, controller, events := migrationCutoverFixture(t)
			failure := errors.New("injected interruption")
			switch stage {
			case "ambiguous-disable":
				source.disableErr = failure
			case "usage":
				source.syncErr = failure
			case "activation":
				controller.applyErr = failure
			}
			result, err := service.Cutover(context.Background(), "node")
			if !errors.Is(err, failure) || result.Phase != "legacy" || repo.state.Nodes[0].Phase != "legacy" {
				t.Fatalf("handover=%+v,%v;events=%v", result, err, *events)
			}
			flush, usage, enable := -1, -1, -1
			for i, event := range *events {
				switch event {
				case "native:flush":
					flush = i
				case "source:usage":
					usage = i
				case "source:enable":
					enable = i
				}
			}
			if flush < 0 || usage <= flush || enable <= usage {
				t.Fatalf("source reopened before reconciled native journal: %v", *events)
			}
		})
	}
}
func TestFailedRecoveryRecordsReviewAndCanBeResumed(t *testing.T) {
	service, repo, _, controller, _ := migrationCutoverFixture(t)
	controller.applyErr = errors.New("activation failed")
	controller.rollbackErr = errors.New("agent disconnected")
	result, err := service.Cutover(context.Background(), "node")
	if err == nil || result.Phase != "review" || result.ErrorCode != "XNODE_ROLLBACK_PENDING" {
		t.Fatalf("review=%+v,%v", result, err)
	}
	controller.rollbackErr = nil
	result, err = service.Rollback(context.Background(), "node")
	if err != nil || result.Phase != "legacy" || repo.state.Nodes[0].ErrorCode != "" {
		t.Fatalf("resume=%+v,%v", result, err)
	}
}
func TestPreflightFailureDoesNotDisableLegacyNode(t *testing.T) {
	service, _, _, _, events := migrationCutoverFixture(t)
	service.panel = cutoverPanel{verifyErr: xnode.ErrUnavailable}
	if _, err := service.Cutover(context.Background(), "node"); err == nil {
		t.Fatal("unsigned incompatible build accepted")
	}
	if len(*events) != 0 {
		t.Fatalf("preflight changed serving ownership: %v", *events)
	}
}
