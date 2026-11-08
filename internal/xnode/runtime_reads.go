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

func fundedPAYGAccounts(ctx context.Context, repo paygRepository, node string) ([]PAYGFunding, error) {
	if reader, ok := repo.(interface {
		XNodePAYGFunding(context.Context, string) ([]PAYGFunding, error)
	}); ok {
		return reader.XNodePAYGFunding(ctx, node)
	}
	var accounts []PAYGAccount
	var err error
	if reader, ok := repo.(interface {
		XNodePAYGFundedAccounts(context.Context, string) ([]PAYGAccount, error)
	}); ok {
		accounts, err = reader.XNodePAYGFundedAccounts(ctx, node)
	} else {
		accounts, err = repo.XNodePAYGAccounts(ctx, "", node)
	}
	if err != nil {
		return nil, err
	}
	out := make([]PAYGFunding, 0, len(accounts))
	for _, account := range accounts {
		out = append(out, account.Funding())
	}
	return out, nil
}

func connectedPAYGAccounts(ctx context.Context, repo paygRepository, node string, ids []int64) ([]PAYGAccount, error) {
	if reader, ok := repo.(interface {
		XNodePAYGConnectedAccounts(context.Context, string, []int64) ([]PAYGAccount, error)
	}); ok {
		return reader.XNodePAYGConnectedAccounts(ctx, node, ids)
	}
	return repo.XNodePAYGAccounts(ctx, "", node)
}
