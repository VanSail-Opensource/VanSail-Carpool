package xnode

import "context"

func (r *compileReads) XNodeNodes(ctx context.Context) ([]Node, error) {
	if r.nodesLoaded {
		return r.nodes, nil
	}
	items, err := r.Repository.XNodeNodes(ctx)
	if err == nil {
		r.nodes, r.nodesLoaded = items, true
		for _, item := range items {
			r.nodeByID[item.ID] = item
		}
	}
	return items, err
}

func (r *compileReads) XNodeNode(ctx context.Context, id string) (Node, error) {
	if item, ok := r.nodeByID[id]; ok {
		return item, nil
	}
	item, err := r.Repository.XNodeNode(ctx, id)
	if err == nil {
		r.nodeByID[id] = item
	}
	return item, err
}

func (r *compileReads) XNodeEndpoints(ctx context.Context) ([]Endpoint, error) {
	if r.endpointsLoaded {
		return r.endpoints, nil
	}
	items, err := r.Repository.XNodeEndpoints(ctx)
	if err == nil {
		r.endpoints, r.endpointsLoaded = items, true
	}
	return items, err
}

func (r *compileReads) XNodeUserRestricted(ctx context.Context, id string) (bool, error) {
	if value, ok := r.restricted[id]; ok {
		return value, nil
	}
	value, err := r.Repository.XNodeUserRestricted(ctx, id)
	if err == nil {
		r.restricted[id] = value
	}
	return value, err
}

func (r *compileReads) UserDisabledSquads(ctx context.Context, id string) ([]string, error) {
	if items, ok := r.disabled[id]; ok {
		return items, nil
	}
	items, err := r.Repository.UserDisabledSquads(ctx, id)
	if err == nil {
		r.disabled[id] = items
	}
	return items, err
}
