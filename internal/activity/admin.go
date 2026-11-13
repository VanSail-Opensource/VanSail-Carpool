package activity

import (
	"context"
	"errors"
	"strings"
	"time"
)

type adminStore interface {
	SaveAdminActivityGame(context.Context, string, GameInput, time.Time) (Game, error)
	SaveAdminLuckyDraw(context.Context, string, LuckyDrawInput, time.Time) (LuckyDraw, error)
	PublishAdminRaffle(context.Context, string, string, int64, time.Time) error
}

// SaveAdminGame uses the transaction that also authorizes and audits the actor.
func (service *Service) SaveAdminGame(ctx context.Context, actorID string, input GameInput) (Game, error) {
	store, ok := service.store.(adminStore)
	if !ok {
		return Game{}, errors.New("activity administrator repository is unavailable")
	}
	if err := input.Validate(); err != nil {
		return Game{}, err
	}
	input.Name, input.Icon, input.Description = strings.TrimSpace(input.Name), strings.TrimSpace(input.Icon), strings.TrimSpace(input.Description)
	return store.SaveAdminActivityGame(ctx, actorID, input, service.now().UTC())
}

// SaveAdminDraw retains the same revision and reward rules as member settlement.
func (service *Service) SaveAdminDraw(ctx context.Context, actorID string, input LuckyDrawInput) (LuckyDraw, error) {
	store, ok := service.store.(adminStore)
	if !ok {
		return LuckyDraw{}, errors.New("activity administrator repository is unavailable")
	}
	return store.SaveAdminLuckyDraw(ctx, actorID, normalizedDrawInput(input), service.now().UTC())
}

func (service *Service) PublishAdminRaffle(ctx context.Context, actorID, id string, groupID int64) error {
	store, ok := service.store.(adminStore)
	if !ok {
		return errors.New("activity administrator repository is unavailable")
	}
	return store.PublishAdminRaffle(ctx, actorID, id, groupID, service.now().UTC())
}

func normalizedDrawInput(input LuckyDrawInput) LuckyDrawInput {
	input.Name, input.Description = strings.TrimSpace(input.Name), strings.TrimSpace(input.Description)
	input.Prizes = append([]PrizeInput(nil), input.Prizes...)
	for index := range input.Prizes {
		input.Prizes[index].Name = strings.TrimSpace(input.Prizes[index].Name)
		input.Prizes[index].Reward.CouponID = strings.TrimSpace(input.Prizes[index].Reward.CouponID)
	}
	return input
}
