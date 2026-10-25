package catalog

import "context"

// RemoteTopology is shared nonfinancial merchandising metadata. Price, stock,
// owner permissions and entitlement boundaries are read separately and freshly.
type RemoteTopology struct {
	Squads     []RemoteSquad
	Nodes      []RemoteNode
	Accessible map[string][]string
}

type remoteTopologyLister interface {
	ListCatalogTopology(context.Context) (RemoteTopology, error)
}

type topologySnapshot struct{ RemoteTopology }

func (t topologySnapshot) ListCatalogSquads(context.Context) ([]RemoteSquad, error) {
	return t.Squads, nil
}
func (t topologySnapshot) ListCatalogNodes(context.Context) ([]RemoteNode, error) {
	return t.Nodes, nil
}
func (t topologySnapshot) AccessibleCatalogNodeUUIDs(_ context.Context, id string) ([]string, error) {
	return t.Accessible[id], nil
}
