package xnode

import "context"

// Administrator writes carry the actor to the same transaction that changes
// desired state. Raw methods remain available to trusted migration/worker code.
func (s *Service) SaveConfigurationAs(ctx context.Context, actor string, item Configuration) (Configuration, error) {
	if actor == "" {
		return item, ErrInvalid
	}
	return s.saveConfiguration(ctx, item, actor)
}

func (s *Service) SaveSnippetAs(ctx context.Context, actor string, item Snippet) (Snippet, error) {
	if actor == "" {
		return item, ErrInvalid
	}
	return s.saveSnippet(ctx, item, actor)
}

func (s *Service) DeleteConfigurationAs(ctx context.Context, actor, id string) error {
	if actor == "" {
		return ErrInvalid
	}
	writer, ok := s.repo.(interface {
		DeleteAdminXNodeConfiguration(context.Context, string, string) error
	})
	if !ok {
		return ErrUnavailable
	}
	return writer.DeleteAdminXNodeConfiguration(ctx, actor, id)
}

func (s *Service) DeleteSnippetAs(ctx context.Context, actor, id string) error {
	if actor == "" {
		return ErrInvalid
	}
	writer, ok := s.repo.(interface {
		DeleteAdminXNodeSnippet(context.Context, string, string) error
	})
	if !ok {
		return ErrUnavailable
	}
	return writer.DeleteAdminXNodeSnippet(ctx, actor, id)
}

func (s *Service) persistConfiguration(ctx context.Context, actor string, item Configuration) (Configuration, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeConfiguration(context.Context, string, Configuration) (Configuration, error)
		})
		if !ok {
			return item, ErrUnavailable
		}
		return writer.SaveAdminXNodeConfiguration(ctx, actor, item)
	}
	if err := s.repo.SaveXNodeConfiguration(ctx, item); err != nil {
		return item, err
	}
	return s.repo.XNodeConfiguration(ctx, item.ID)
}

func (s *Service) persistSnippet(ctx context.Context, actor string, item Snippet) (Snippet, error) {
	if actor != "" {
		writer, ok := s.repo.(interface {
			SaveAdminXNodeSnippet(context.Context, string, Snippet) (Snippet, error)
		})
		if !ok {
			return item, ErrUnavailable
		}
		return writer.SaveAdminXNodeSnippet(ctx, actor, item)
	}
	if err := s.repo.SaveXNodeSnippet(ctx, item); err != nil {
		return item, err
	}
	return s.repo.XNodeSnippet(ctx, item.ID)
}
