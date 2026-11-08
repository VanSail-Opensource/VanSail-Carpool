package xnode

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Minimal settlement support negotiates the protocol during rollout. Full
// production additionally requires the durable shared-byte foundation.
type hourlyCreditProductionRepository interface {
	hourlyCreditRepository
	EnsureHourlyCreditDay(context.Context, string, int64, string, int64) (model.HourlyCreditDay, error)
	ReserveHourlyCredit(context.Context, model.HourlyCreditRequest) (model.HourlyCreditReservation, error)
	HourlyCreditDay(context.Context, string, int64, string) (model.HourlyCreditDay, error)
	HourlyCreditReservation(context.Context, string) (model.HourlyCreditReservation, error)
	ResolveHourlyCreditIntent(context.Context, model.HourlyCreditRequest) (model.HourlyCreditReservation, error)
	UnfinalizedHourlyCredits(context.Context, string, string, int64, int) ([]model.HourlyCreditReservation, error)
	HourlyCreditHeldBytes(context.Context, model.HourlyCreditRequest) (int64, error)
	OutstandingHourlyCredits(context.Context, string, int64, string, int) ([]model.HourlyCreditReservation, error)
}
