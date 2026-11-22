package xnode

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type hostOriginReadRepository struct {
	Repository
	endpoints                 []Endpoint
	nodes                     []Node
	stored                    map[string]Node
	nodeErrors, bindingErrors map[string]error
	nodeReads, bindingReads   map[string]int
	endpointReads, nodesReads int
	endpointErr, nodesErr     error
}

func (r *hostOriginReadRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	r.endpointReads++
	return slices.Clone(r.endpoints), r.endpointErr
}

func (r *hostOriginReadRepository) XNodeNodes(context.Context) ([]Node, error) {
	r.nodesReads++
	return slices.Clone(r.nodes), r.nodesErr
}

func (r *hostOriginReadRepository) XNodeNode(_ context.Context, id string) (Node, error) {
	r.nodeReads[id]++
	if err := r.nodeErrors[id]; err != nil {
		return Node{}, err
	}
	node, ok := r.stored[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	return node, nil
}

func (r *hostOriginReadRepository) XNodeDDNSBindings(_ context.Context, id string) (map[string]DDNSHostBinding, error) {
	r.bindingReads[id]++
	return nil, r.bindingErrors[id]
}

func hostOriginReadFixture(t *testing.T) (*Master, *hostOriginReadRepository) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := &hostOriginReadRepository{stored: map[string]Node{}, nodeErrors: map[string]error{}, bindingErrors: map[string]error{}, nodeReads: map[string]int{}, bindingReads: map[string]int{}}
	service := NewService(repo, nil, vault, nil, nil)
	service.now = func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }
	return NewMaster(service, nil), repo
}

func saveHostOriginReadSettings(t *testing.T, master *Master, repo *hostOriginReadRepository, node string, settings NodeSettings) {
	t.Helper()
	cipher, err := master.service.sealNodeSettings(node, settings)
	if err != nil {
		t.Fatal(err)
	}
	repo.stored[node] = Node{ID: node, SettingsCipher: cipher}
}

func retainedHostOrigin(master *Master, addresses ...string) hostOrigin {
	now := master.service.now()
	return hostOrigin{addresses: addresses, refreshAt: now.Add(time.Minute), validUntil: now.Add(5 * time.Minute)}
}
