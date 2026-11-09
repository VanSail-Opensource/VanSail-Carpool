package admin

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// adminHourlyRepository is the optional store surface used only by the
// administrator hourly projection. Narrow assertion keeps existing test doubles
// compatible while the store remains the single accounting owner.
type adminHourlyRepository interface {
	UserByID(context.Context, string) (model.User, error)
	Balance(context.Context, string) (model.Money, error)
	HourlyExtraPreference(context.Context, string, int64) (model.HourlyExtraPreference, error)
	HourlyCreditDay(context.Context, string, int64, string) (model.HourlyCreditDay, error)
	OutstandingHourlyCredits(context.Context, string, int64, string, int) ([]model.HourlyCreditReservation, error)
	ListAdminHourlyEnrollments(context.Context, string, int64) ([]database.AdminHourlyEnrollment, error)
}

// AdminHourlySquad is one administrator-visible enrollment without native
// credentials, proxy/period references, generations, or fingerprints. The
// enrollment ID is the safe target for the administrator disable endpoint.
type AdminHourlySquad struct {
	EnrollmentID  string      `json:"enrollmentId"`
	GroupID       string      `json:"groupId"`
	Name          string      `json:"name"`
	HourlyPrice   model.Money `json:"hourlyPrice"`
	OfferEnabled  bool        `json:"offerEnabled"`
	Status        string      `json:"status"`
	PaidThrough   *time.Time  `json:"paidThrough"`
	CooldownUntil *time.Time  `json:"cooldownUntil"`
	ActivatedAt   *time.Time  `json:"activatedAt"`
	DisabledAt    *time.Time  `json:"disabledAt"`
	CreatedAt     time.Time   `json:"createdAt"`
}

// AdminHourlyView is the owner-scoped administrator projection: policy, wallet,
// extra preference, current UTC-day usage, and every enrollment status.
type AdminHourlyView struct {
	Policy          model.HourlyPolicySnapshot  `json:"policy"`
	Balance         model.Money                 `json:"balance"`
	ExtraPreference model.HourlyExtraPreference `json:"extraPreference"`
	Usage           *model.HourlyAccountUsage   `json:"usage"`
	Squads          []AdminHourlySquad          `json:"squads"`
}

// AdminHourly builds the projection from existing accounting reads. It never
// creates allowance rows and never exposes credential material.
func (s *Service) AdminHourly(ctx context.Context, userID string) (AdminHourlyView, error) {
	repository, ok := s.repository.(adminHourlyRepository)
	if !ok || s.settings == nil {
		return AdminHourlyView{}, ErrHourlyPolicyUnavailable
	}
	user, err := repository.UserByID(ctx, userID)
	if err != nil {
		return AdminHourlyView{}, err
	}
	view := AdminHourlyView{Squads: []AdminHourlySquad{}}
	if view.Policy, err = s.settings.HourlyPolicy(ctx); err != nil {
		return AdminHourlyView{}, err
	}
	if err = model.ValidateHourlyPolicy(view.Policy.Policy); err != nil {
		return AdminHourlyView{}, ErrHourlyPolicyUnavailable
	}
	if view.Balance, err = repository.Balance(ctx, user.ID); err != nil {
		return AdminHourlyView{}, err
	}
	if view.ExtraPreference, err = repository.HourlyExtraPreference(ctx, user.ID, user.AccountGeneration); err != nil {
		return AdminHourlyView{}, err
	}
	now := s.now().UTC()
	if view.Usage, err = adminHourlyUsage(ctx, repository, user, view.Policy.Policy, now); err != nil {
		return AdminHourlyView{}, err
	}
	enrollments, err := repository.ListAdminHourlyEnrollments(ctx, user.ID, user.AccountGeneration)
	if err != nil {
		return AdminHourlyView{}, err
	}
	for _, item := range enrollments {
		view.Squads = append(view.Squads, AdminHourlySquad{EnrollmentID: item.EnrollmentID, GroupID: item.GroupID,
			Name: item.Name, HourlyPrice: model.TXBMoney(item.HourlyPriceTXBMinor), OfferEnabled: item.OfferEnabled,
			Status: item.Status, PaidThrough: item.PaidThrough, CooldownUntil: item.CooldownUntil,
			ActivatedAt: item.ActivatedAt, DisabledAt: item.DisabledAt, CreatedAt: item.CreatedAt})
	}
	return view, nil
}

// adminHourlyUsage reuses the member accounting projection without granting or
// reserving anything.
func adminHourlyUsage(ctx context.Context, repository adminHourlyRepository, owner model.User,
	policy model.HourlyPolicy, now time.Time) (*model.HourlyAccountUsage, error) {
	day := now.UTC().Format(time.DateOnly)
	usage := &model.HourlyAccountUsage{DayUTC: day, FreeAllowanceBytes: policy.DailyAllowanceBytes,
		ReducedRateBytesPerSecond: policy.ReducedRateBytesPerSecond,
		ExtraExpiresAt:            now.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)}
	value, err := repository.HourlyCreditDay(ctx, owner.ID, owner.AccountGeneration, day)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		usage.FreeAllowanceBytes, usage.FreeUsedBytes, usage.FreeReservedBytes = value.FreeAllowanceBytes, value.FreeAdmittedBytes, value.FreeReservedBytes
		usage.ExtraFundedBytes, usage.ExtraUsedBytes, usage.ExtraReservedBytes = value.ExtraFundedBytes, value.ExtraAdmittedBytes, value.ExtraReservedBytes
	}
	outstanding, err := repository.OutstandingHourlyCredits(ctx, owner.ID, owner.AccountGeneration, "", 1)
	if err != nil {
		return nil, err
	}
	usage.AccountingPending = len(outstanding) != 0
	return usage, nil
}
