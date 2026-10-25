package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type snapshotCatalogRemote struct {
	*catalogNodeRemote
	reads int
}

func (r *snapshotCatalogRemote) ListCatalogTopology(context.Context) (RemoteTopology, error) {
	r.reads++
	return RemoteTopology{Squads: r.squads, Nodes: r.nodes, Accessible: r.accessible}, nil
}

func TestCatalogBatchesTopologyWithoutSharingOwnerPriceOrStock(t *testing.T) {
	remote := &snapshotCatalogRemote{catalogNodeRemote: newCatalogNodeRemote()}
	remote.nodes = []RemoteNode{{UUID: "node-1", Name: "Node"}}
	remote.accessible["addon-squad"] = []string{"node-1"}
	repository := nodeCatalogRepository()
	repository.active = &model.Purchase{SquadUUIDs: []string{"addon-squad"}}
	repository.combos[0].Price = model.TXBMoney(100)
	service := NewService(repository, remote, time.Minute)
	first, err := service.CatalogForUser(context.Background(), model.User{ID: "owner"})
	if err != nil || !first.Addons[0].StockHeldByCurrentUser || len(first.Addons[0].AccessibleNodes) != 1 {
		t.Fatal(first, err)
	}
	repository.active = nil
	repository.combos[0].Price = model.TXBMoney(200)
	zero := 0
	repository.addons[0].StockRemaining = &zero
	second, err := service.CatalogForUser(context.Background(), model.User{ID: "other"})
	if err != nil || second.Addons[0].StockHeldByCurrentUser || second.Combos[0].Price.Minor != "200" || second.Addons[0].StockRemaining == nil || *second.Addons[0].StockRemaining != 0 {
		t.Fatal("cached financial/owner projection", second, err)
	}
	if remote.reads != 2 || len(remote.requestedSquads) != 0 {
		t.Fatal("per-squad group queries were retained", remote.reads, remote.requestedSquads)
	}
}
