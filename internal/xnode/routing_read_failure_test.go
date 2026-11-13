package xnode

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestRoutingReadFailurePreservesMigrationAndDeployment(t *testing.T) {
	for _, cause := range []error{errors.New("temporary database outage"), context.DeadlineExceeded, context.Canceled, ErrUnavailable, ErrConflict} {
		t.Run(cause.Error(), func(t *testing.T) {
			failure := fmt.Errorf("read routing settings: %w", cause)
			t.Run("legacy", func(t *testing.T) {
				service, repo, _ := routingReadFailureFixture(t, failure)
				repo.legacy = []Customization{{ProxyUserID: 7, NodeID: "legacy-node"}}
				err := service.migrateLegacyRouting(context.Background())
				if !errors.Is(err, cause) || len(repo.queued) != 0 {
					t.Fatal("failed read permanently converted legacy routing", err, repo.queued)
				}
			})
			t.Run("deployment", func(t *testing.T) {
				service, repo, _ := routingReadFailureFixture(t, failure)
				op := providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Status: "queued"}}
				err := service.runRoutingOperation(context.Background(), op)
				if !errors.Is(err, cause) || repo.advances != 0 || repo.plans[0].Phase != "applying" || len(repo.completed) != 0 {
					t.Fatal("failed read consumed the admitted generation", err, repo.advances, repo.completed)
				}
			})
		})
	}
}

func TestProvenInvalidRoutingStillDisablesTheGeneration(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		service, repo, _ := routingReadFailureFixture(t, fmt.Errorf("unsupported template: %w", ErrInvalid))
		repo.legacy = []Customization{{ProxyUserID: 7, NodeID: "legacy-node"}}
		err := service.migrateLegacyRouting(context.Background())
		if err != nil || len(repo.queued) != 1 || repo.queued[0].Phase != "disabled" || repo.queued[0].Reason != "routing_legacy_incompatible" {
			t.Fatal("proven incompatible legacy graph was not disabled", err, repo.queued)
		}
	})
	t.Run("deployment", func(t *testing.T) {
		service, repo, _ := routingReadFailureFixture(t, fmt.Errorf("invalid graph: %w", ErrInvalid))
		op := providerops.Operation{Receipt: model.OperationReceipt{ID: "operation", Status: "queued"}}
		err := service.runRoutingOperation(context.Background(), op)
		if err != nil || repo.advances != 1 || repo.plans[0].Phase != "disabled" || len(repo.completed) != 1 {
			t.Fatal("proven invalid generation lost its terminal outcome", err, repo.advances, repo.completed)
		}
		if repo.completed[0].Status != providerops.StatusFailed || repo.completed[0].ErrorCode != "XNODE_ROUTING_DEPENDENCIES_CHANGED" {
			t.Fatal("invalid generation received the wrong completion", repo.completed[0])
		}
	})
}
