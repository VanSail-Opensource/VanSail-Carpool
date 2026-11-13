package activity

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Store interface {
	SaveActivityGame(context.Context, GameInput, time.Time) (Game, error)
	ListActivityGames(context.Context, bool) ([]Game, error)
	PlaceActivityBet(context.Context, string, string, int64, string, RandomSource, time.Time) (BetResult, error)
	ClaimBoostedDailyActivityRange(context.Context, string, string, string, int64, int64, int, RandomSource, time.Time) (DailyCheckIn, error)
	SaveLuckyDraw(context.Context, LuckyDrawInput, time.Time) (LuckyDraw, error)
	ListLuckyDraws(context.Context, bool) ([]LuckyDraw, error)
	PlayLuckyDraw(context.Context, string, string, string, RandomSource, time.Time) (DrawResult, error)
	PublishRaffle(context.Context, string, int64, time.Time) error
	JoinRaffle(context.Context, string, int64, int64, string, time.Time) (RaffleEntry, bool, error)
	SettleRaffle(context.Context, string, RandomSource, time.Time) (RaffleSettlement, error)
	ListActivityHistory(context.Context, string, int) (History, error)
	GroupMessageRewardStatus(context.Context, string, string, int, int64) (GroupMessageRewardStatus, error)
	RecordGroupMessage(context.Context, string, int64, int64, string, string, int, int64, int, time.Time) (GroupMessageRewardResult, error)
}

type Service struct {
	store  Store
	rng    RandomSource
	now    func() time.Time
	boosts GroupBoostSource
}

func NewService(store Store, rng RandomSource, now func() time.Time) *Service {
	if rng == nil {
		rng = CryptoRandom{}
	}
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, rng: rng, now: now}
}
func (service *Service) SaveGame(ctx context.Context, input GameInput) (Game, error) {
	if err := input.Validate(); err != nil {
		return Game{}, err
	}
	input.Name, input.Icon, input.Description = strings.TrimSpace(input.Name), strings.TrimSpace(input.Icon), strings.TrimSpace(input.Description)
	return service.store.SaveActivityGame(ctx, input, service.now().UTC())
}
func (service *Service) Games(ctx context.Context, enabledOnly bool) ([]Game, error) {
	return service.store.ListActivityGames(ctx, enabledOnly)
}
func (service *Service) PlayBet(ctx context.Context, userID, gameID string, stakeMinor int64, key string) (BetResult, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(gameID) == "" || stakeMinor <= 0 || !validIdempotencyKey(key) {
		return BetResult{}, fmt.Errorf("%w: incomplete bet request", ErrInvalidInput)
	}
	return service.store.PlaceActivityBet(ctx, userID, gameID, stakeMinor, key, service.rng, service.now().UTC())
}
func (service *Service) CheckIn(ctx context.Context, userID string, config CheckInConfig) (DailyCheckIn, error) {
	if strings.TrimSpace(userID) == "" || config.RewardMinMinor < 0 || config.RewardMaxMinor < config.RewardMinMinor {
		return DailyCheckIn{}, fmt.Errorf("%w: invalid check-in request", ErrInvalidInput)
	}
	location, err := time.LoadLocation(strings.TrimSpace(config.Timezone))
	if err != nil {
		return DailyCheckIn{}, fmt.Errorf("%w: unknown timezone", ErrInvalidInput)
	}
	boost, err := service.GroupBoost(ctx, userID)
	if err != nil {
		return DailyCheckIn{}, err
	}
	if *boost.Count == 0 {
		return DailyCheckIn{}, ErrGroupBoostRequired
	}
	now := service.now()
	return service.store.ClaimBoostedDailyActivityRange(ctx, userID, now.In(location).Format(time.DateOnly), location.String(), config.RewardMinMinor, config.RewardMaxMinor, *boost.Count, service.rng, now.UTC())
}
func (service *Service) SaveDraw(ctx context.Context, input LuckyDrawInput) (LuckyDraw, error) {
	return service.store.SaveLuckyDraw(ctx, normalizedDrawInput(input), service.now().UTC())
}
func (service *Service) Draws(ctx context.Context, enabledOnly bool) ([]LuckyDraw, error) {
	return service.store.ListLuckyDraws(ctx, enabledOnly)
}
func (service *Service) Draw(ctx context.Context, userID, drawID, key string) (DrawResult, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(drawID) == "" || !validIdempotencyKey(key) {
		return DrawResult{}, fmt.Errorf("%w: incomplete lucky-draw request", ErrInvalidInput)
	}
	return service.store.PlayLuckyDraw(ctx, userID, drawID, key, service.rng, service.now().UTC())
}
func (service *Service) PublishRaffle(ctx context.Context, id string, groupID int64) error {
	return service.store.PublishRaffle(ctx, id, groupID, service.now().UTC())
}
func (service *Service) JoinRaffle(ctx context.Context, userID string, chatID, messageID int64, text string) (RaffleEntry, bool, error) {
	return service.store.JoinRaffle(ctx, userID, chatID, messageID, text, service.now().UTC())
}
func (service *Service) SettleRaffle(ctx context.Context, id string) (RaffleSettlement, error) {
	return service.store.SettleRaffle(ctx, id, service.rng, service.now().UTC())
}
func (service *Service) History(ctx context.Context, userID string, limit int) (History, error) {
	if strings.TrimSpace(userID) == "" {
		return History{}, fmt.Errorf("%w: missing user", ErrInvalidInput)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return service.store.ListActivityHistory(ctx, userID, limit)
}
func (service *Service) GroupMessageStatus(ctx context.Context, userID string, config GroupMessageRewardConfig) (GroupMessageRewardStatus, error) {
	if strings.TrimSpace(userID) == "" {
		return GroupMessageRewardStatus{}, fmt.Errorf("%w: missing user", ErrInvalidInput)
	}
	if err := config.Validate(); err != nil {
		return GroupMessageRewardStatus{}, err
	}
	location, _ := time.LoadLocation(config.Timezone)
	status, err := service.store.GroupMessageRewardStatus(ctx, userID, service.now().In(location).Format(time.DateOnly), config.Threshold, config.RewardMinor)
	if err != nil || status.Rewarded {
		return status, err
	}
	status.RewardMinor, err = BoostRewardMinor(config.RewardMinor, config.BoostCount)
	return status, err
}
func (service *Service) RecordGroupMessage(ctx context.Context, userID string, chatID, messageID int64, config GroupMessageRewardConfig) (GroupMessageRewardResult, error) {
	if strings.TrimSpace(userID) == "" || chatID == 0 || messageID <= 0 {
		return GroupMessageRewardResult{}, fmt.Errorf("%w: invalid group message identity", ErrInvalidInput)
	}
	if err := config.Validate(); err != nil {
		return GroupMessageRewardResult{}, err
	}
	location, _ := time.LoadLocation(config.Timezone)
	now := service.now().UTC()
	return service.store.RecordGroupMessage(ctx, userID, chatID, messageID, now.In(location).Format(time.DateOnly), location.String(), config.Threshold, config.RewardMinor, config.BoostCount, now)
}
func validIdempotencyKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 128
}
