package xnode

import (
	"context"
	"encoding/json"
)

func (s *Service) SaveOwnedOutboundsAs(ctx context.Context, actor, owner, node string, revision int64, raw json.RawMessage) (Customization, error) {
	if actor == "" {
		return Customization{}, ErrInvalid
	}
	return s.saveOwnedOutbounds(ctx, owner, node, revision, raw, actor)
}

func (s *Service) SaveOwnedRoutingAs(ctx context.Context, actor, owner, node string, revision int64, raw json.RawMessage) (Customization, error) {
	if actor == "" {
		return Customization{}, ErrInvalid
	}
	return s.saveOwnedRouting(ctx, owner, node, revision, raw, actor)
}

func (s *Service) persistCustomization(ctx context.Context, actor string, item Customization) (Customization, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeCustomization(context.Context, string, Customization) (Customization, error)
		})
		if !ok {
			return item, ErrUnavailable
		}
		return writer.SaveAdminXNodeCustomization(ctx, actor, item)
	}
	if err := s.repo.SaveXNodeCustomization(ctx, item); err != nil {
		return item, err
	}
	item.Revision++
	return item, nil
}
