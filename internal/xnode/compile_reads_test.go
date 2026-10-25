package xnode

import (
	"context"
	"testing"
)

type compileReadsRepository struct {
	Repository
	nodeReads       int
	nodesReads      int
	endpointReads   int
	disabledReads   int
	restrictedReads int
}

func (r *compileReadsRepository) XNodeNode(context.Context, string) (Node, error) {
	r.nodeReads++
	return Node{ID: "node"}, nil
}

func (r *compileReadsRepository) XNodeNodes(context.Context) ([]Node, error) {
	r.nodesReads++
	return []Node{{ID: "node"}}, nil
}

func (r *compileReadsRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	r.endpointReads++
	return []Endpoint{}, nil
}

func (r *compileReadsRepository) UserDisabledSquads(context.Context, string) ([]string, error) {
	r.disabledReads++
	return []string{}, nil
}

func (r *compileReadsRepository) XNodeUserRestricted(context.Context, string) (bool, error) {
	r.restrictedReads++
	return false, nil
}

func TestCompileReadsMemoizesSharedInventory(t *testing.T) {
	repo := &compileReadsRepository{}
	service := NewService(repo, nil, nil, nil, nil)
	reads := service.withCompileReads().repo.(*compileReads)
	ctx := context.Background()

	for range 2 {
		if _, err := reads.XNodeNode(ctx, "node"); err != nil {
			t.Fatal(err)
		}
		if _, err := reads.XNodeNodes(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := reads.XNodeEndpoints(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := reads.UserDisabledSquads(ctx, "member"); err != nil {
			t.Fatal(err)
		}
		if _, err := reads.XNodeUserRestricted(ctx, "member"); err != nil {
			t.Fatal(err)
		}
	}

	if repo.nodeReads != 1 || repo.nodesReads != 1 || repo.endpointReads != 1 || repo.disabledReads != 1 || repo.restrictedReads != 1 {
		t.Fatalf("shared reads repeated: node=%d nodes=%d endpoints=%d disabled=%d restricted=%d", repo.nodeReads, repo.nodesReads, repo.endpointReads, repo.disabledReads, repo.restrictedReads)
	}
}
