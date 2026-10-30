package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CompileMembers is disposable input for one compilation, never a commerce proof.
type CompileMembers struct {
	Users      map[string]model.User
	Purchases  map[string][]model.Purchase
	Periods    map[string]Period
	Restricted map[string]bool
	Disabled   map[string][]string
}

func (r *compileReads) preloadMembers(ctx context.Context, identities []Identity) error {
	source, ok := r.Repository.(interface {
		XNodeCompileMembers(context.Context, []string) (CompileMembers, error)
	})
	if !ok {
		return nil
	}
	ids := []string{}
	for _, identity := range identities {
		if identity.UserID != "" {
			ids = append(ids, identity.UserID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	inputs, err := source.XNodeCompileMembers(ctx, ids)
	if err != nil {
		return err
	}
	for id, value := range inputs.Users {
		r.users[id] = value
	}
	for id, value := range inputs.Purchases {
		r.purchases[id] = value
	}
	for id, value := range inputs.Periods {
		r.periods[id] = value
	}
	for id, value := range inputs.Restricted {
		r.restricted[id] = value
	}
	for id, value := range inputs.Disabled {
		r.disabled[id] = value
	}
	return nil
}
