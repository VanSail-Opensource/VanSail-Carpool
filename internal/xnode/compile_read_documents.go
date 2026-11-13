package xnode

import "context"

// A candidate affects nested endpoint and routing-proof reads too. Restore the
// request-scoped cache afterward so validation never publishes hypothetical data.
func (r *compileReads) useDocuments(config *Configuration, snippet *Snippet) func() {
	var oldConfig Configuration
	var oldSnippet Snippet
	var hadConfig, hadSnippet bool
	if config != nil {
		oldConfig, hadConfig = r.configurations[config.ID]
		r.configurations[config.ID] = *config
	}
	if snippet != nil {
		oldSnippet, hadSnippet = r.snippets[snippet.ID]
		r.snippets[snippet.ID] = *snippet
	}
	return func() {
		if config != nil {
			if hadConfig {
				r.configurations[config.ID] = oldConfig
			} else {
				delete(r.configurations, config.ID)
			}
		}
		if snippet != nil {
			if hadSnippet {
				r.snippets[snippet.ID] = oldSnippet
			} else {
				delete(r.snippets, snippet.ID)
			}
		}
	}
}

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
