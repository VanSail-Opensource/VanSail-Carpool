package xnode

import "context"

// Encrypted configuration/snippet references repeat across route branches and
// node preflights. Retain successful reads only within this compile scope.
func (r *compileReads) XNodeConfiguration(ctx context.Context, id string) (Configuration, error) {
	if item, ok := r.configurations[id]; ok {
		return item, nil
	}
	item, err := r.Repository.XNodeConfiguration(ctx, id)
	if err == nil {
		r.configurations[id] = item
	}
	return item, err
}

func (r *compileReads) XNodeSnippet(ctx context.Context, id string) (Snippet, error) {
	if item, ok := r.snippets[id]; ok {
		return item, nil
	}
	item, err := r.Repository.XNodeSnippet(ctx, id)
	if err == nil {
		r.snippets[id] = item
	}
	return item, err
}

func (r *compileReads) XNodeDDNSBindings(ctx context.Context, id string) (map[string]DDNSHostBinding, error) {
	if items, ok := r.ddnsBindings[id]; ok {
		return items, nil
	}
	items := map[string]DDNSHostBinding{}
	if source, ok := r.Repository.(ddnsBindingReader); ok {
		var err error
		items, err = source.XNodeDDNSBindings(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	r.ddnsBindings[id] = items
	return items, nil
}
