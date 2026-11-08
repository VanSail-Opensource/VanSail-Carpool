package xnode

import "context"

func (r *compileReads) XNodeHourlyPrincipal(ctx context.Context, id string) (HourlyPrincipal, error) {
	if source, ok := r.Repository.(hourlyPrincipalRepository); ok {
		return source.XNodeHourlyPrincipal(ctx, id)
	}
	return HourlyPrincipal{}, ErrUnavailable
}

// The projection is cached only within one epoch-fenced compile scope.
func (r *compileReads) XNodeHourlyPrincipals(ctx context.Context, owner, node string) ([]HourlyPrincipal, error) {
	key := owner + "\x00" + node
	if items, exists := r.hourly[key]; exists {
		return items, nil
	}
	result := []HourlyPrincipal{}
	if source, ok := r.Repository.(hourlyPrincipalRepository); ok {
		var err error
		result, err = source.XNodeHourlyPrincipals(ctx, owner, node)
		if err != nil {
			return nil, err
		}
	}
	if r.hourly == nil {
		r.hourly = map[string][]HourlyPrincipal{}
	}
	r.hourly[key] = result
	return result, nil
}

func (r *compileReads) EnsureXNodeHourlyPrincipal(ctx context.Context, id, cipher string) (HourlyPrincipal, error) {
	if source, ok := r.Repository.(hourlyPrincipalRepository); ok {
		return source.EnsureXNodeHourlyPrincipal(ctx, id, cipher)
	}
	return HourlyPrincipal{}, ErrUnavailable
}
