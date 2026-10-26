package xnode

import "context"

func (s *Service) cadenceIdentities(ctx context.Context) ([]Identity, error) {
	if repo, ok := s.repo.(interface {
		XNodeCadenceIdentities(context.Context) ([]Identity, error)
	}); ok {
		return repo.XNodeCadenceIdentities(ctx)
	}
	return s.repo.XNodeIdentities(ctx)
}

func fundedPAYGAccounts(ctx context.Context, repo paygRepository, node string) ([]PAYGAccount, error) {
	if reader, ok := repo.(interface {
		XNodePAYGFundedAccounts(context.Context, string) ([]PAYGAccount, error)
	}); ok {
		return reader.XNodePAYGFundedAccounts(ctx, node)
	}
	return repo.XNodePAYGAccounts(ctx, "", node)
}

func connectedPAYGAccounts(ctx context.Context, repo paygRepository, node string, ids []int64) ([]PAYGAccount, error) {
	if reader, ok := repo.(interface {
		XNodePAYGConnectedAccounts(context.Context, string, []int64) ([]PAYGAccount, error)
	}); ok {
		return reader.XNodePAYGConnectedAccounts(ctx, node, ids)
	}
	return repo.XNodePAYGAccounts(ctx, "", node)
}
