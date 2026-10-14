package affiliates

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

type Repository interface {
	AcceptAffiliateReferral(context.Context, int64, int64, time.Time) (string, bool, error)
	AffiliateOverview(context.Context, string, string) (Overview, error)
	AffiliateReferrals(context.Context, string, int) (ReferralPage, error)
	AffiliateConfig(context.Context) (Config, error)
	SaveAffiliateConfig(context.Context, string, ConfigInput, time.Time) (Config, error)
}

type BotSource interface {
	GetMe(context.Context) (telegram.User, error)
}

type Service struct {
	repository Repository
	identity   *IdentityCache
}

func NewService(repository Repository, source BotSource) *Service {
	return &Service{repository: repository, identity: NewIdentityCache(source, 24*time.Hour)}
}

func (s *Service) AcceptReferral(ctx context.Context, inviteeID, inviterID int64) (string, bool, error) {
	return s.repository.AcceptAffiliateReferral(ctx, inviteeID, inviterID, time.Now().UTC())
}

func (s *Service) Overview(ctx context.Context, userID string) (Overview, error) {
	return s.repository.AffiliateOverview(ctx, userID, s.identity.Snapshot().Username)
}

func (s *Service) Referrals(ctx context.Context, userID string, page int) (ReferralPage, error) {
	return s.repository.AffiliateReferrals(ctx, userID, page)
}

func (s *Service) Admin(ctx context.Context) (AdminView, error) {
	config, err := s.repository.AffiliateConfig(ctx)
	if err != nil {
		return AdminView{}, err
	}
	return AdminView{Version: config.Version, Bot: s.identity.Snapshot(), Tiers: config.Tiers}, nil
}

func (s *Service) Save(ctx context.Context, actorID string, input ConfigInput) (AdminView, error) {
	config, err := s.repository.SaveAffiliateConfig(ctx, actorID, input, time.Now().UTC())
	if err != nil {
		return AdminView{}, err
	}
	return AdminView{Version: config.Version, Bot: s.identity.Snapshot(), Tiers: config.Tiers}, nil
}

func (s *Service) RefreshBotIdentity(ctx context.Context) error {
	return s.identity.Refresh(ctx, time.Now().UTC())
}

// BotUsername returns the last verified Telegram bot username, if available.
func (s *Service) BotUsername() string {
	return s.identity.Snapshot().Username
}
