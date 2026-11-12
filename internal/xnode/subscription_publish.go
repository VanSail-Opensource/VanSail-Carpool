package xnode

import "context"

func (s *Service) subscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	return ReadSubscription(ctx, s, identity, func(current Identity) (SubscriptionMetadata, error) {
		return s.readSubscriptionMetadata(ctx, current)
	})
}

func (s *Service) serviceSubscriptionMetadata(ctx context.Context, identity Identity) (SubscriptionMetadata, error) {
	return ReadSubscription(ctx, s, identity, func(current Identity) (SubscriptionMetadata, error) {
		return s.readServiceSubscriptionMetadata(ctx, current)
	})
}

type subscriptionPublication struct {
	raw      []byte
	metadata SubscriptionMetadata
}

func (s *Service) BearerSubscriptionForAgent(ctx context.Context, token, explicit, agent string) ([]byte, SubscriptionMetadata, error) {
	identity, err := s.bearerIdentity(ctx, token)
	if err != nil {
		return nil, SubscriptionMetadata{}, err
	}
	// Every retry retains the original token hash/cipher in the identity anchor.
	// It cannot follow a revoked bearer into a replacement token or lifecycle.
	result, err := ReadSubscription(ctx, s, identity, func(current Identity) (subscriptionPublication, error) {
		raw, metadata, err := s.readBearerSubscriptionForAgent(ctx, current, explicit, agent)
		return subscriptionPublication{raw: raw, metadata: metadata}, err
	})
	return result.raw, result.metadata, err
}
