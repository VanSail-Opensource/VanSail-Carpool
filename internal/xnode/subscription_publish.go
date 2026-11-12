package xnode

import "context"

func (s *Service) subscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	fence, err := s.BeginSubscriptionRead(ctx, identity)
	if err != nil {
		return SubscriptionMetadata{}, err
	}
	result, readErr := s.readSubscriptionMetadata(ctx, identity)
	if err = s.CheckSubscriptionRead(ctx, fence); err != nil {
		return SubscriptionMetadata{}, err
	}
	return result, readErr
}

func (s *Service) serviceSubscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	fence, err := s.BeginSubscriptionRead(ctx, identity)
	if err != nil {
		return SubscriptionMetadata{}, err
	}
	result, readErr := s.readServiceSubscriptionMetadata(ctx, identity)
	if err = s.CheckSubscriptionRead(ctx, fence); err != nil {
		return SubscriptionMetadata{}, err
	}
	return result, readErr
}

func (s *Service) BearerSubscriptionForAgent(ctx context.Context, token, explicit, agent string) ([]byte, SubscriptionMetadata, error) {
	identity, err := s.bearerIdentity(ctx, token)
	if err != nil {
		return nil, SubscriptionMetadata{}, err
	}
	fence, err := s.BeginSubscriptionRead(ctx, identity)
	if err != nil {
		return nil, SubscriptionMetadata{}, err
	}
	raw, metadata, readErr := s.readBearerSubscriptionForAgent(ctx, identity, explicit, agent)
	if err = s.CheckSubscriptionRead(ctx, fence); err != nil {
		return nil, SubscriptionMetadata{}, err
	}
	return raw, metadata, readErr
}
