package xnode

import (
	"context"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// ServiceAccess is an imported standalone account's authoritative access term.
// It is encrypted with its credentials; periods retain exact usage separately.
// Probe identities have no term and retain their existing bounded bindings.
type ServiceAccess struct {
	LimitBytes int64     `json:"limitBytes,string"`
	Strategy   string    `json:"strategy"`
	StartedAt  time.Time `json:"startedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	GroupIDs   []string  `json:"groupIds"`
	Disabled   bool      `json:"disabled"`
}

func (a ServiceAccess) validate() error {
	if a.LimitBytes < 0 || a.StartedAt.IsZero() || !a.ExpiresAt.After(a.StartedAt) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range a.GroupIDs {
		if id == "" || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	switch a.Strategy {
	case "NO_RESET", "DAY", "WEEK", "MONTH", "MONTH_ROLLING":
		return nil
	default:
		return ErrInvalid
	}
}

// Equal compares access terms without depending on group order or time zones.
func (a ServiceAccess) Equal(b ServiceAccess) bool {
	left, right := slices.Clone(a.GroupIDs), slices.Clone(b.GroupIDs)
	slices.Sort(left)
	slices.Sort(right)
	return a.LimitBytes == b.LimitBytes && a.Strategy == b.Strategy && a.StartedAt.Equal(b.StartedAt) && a.ExpiresAt.Equal(b.ExpiresAt) && a.Disabled == b.Disabled && slices.Equal(left, right)
}

// ServiceAccess returns the saved standalone term, or nil for a probe identity.
func (s *Service) ServiceAccess(ctx context.Context, identity Identity) (*ServiceAccess, error) {
	if identity.UserID != "" || identity.ServiceName == "" {
		return nil, ErrInvalid
	}
	credentials, err := loadCredentials(s.vault, identity)
	if err != nil || credentials.ServiceAccess == nil {
		return nil, err
	}
	return credentials.ServiceAccess, credentials.ServiceAccess.validate()
}

func (s *Service) startServicePeriod(ctx context.Context, identity Identity, access ServiceAccess, at time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	return s.repo.SaveXNodePeriod(ctx, Period{ID: id, ProxyUserID: identity.ProxyUserID, StartedAt: at, LimitBytes: access.LimitBytes, Strategy: access.Strategy, LastResetAt: &at, Status: "open"})
}

// Called with the service mutex held, using the same source-reset and late-tail
// reconciliation as paid identities, without creating a purchase or principal.
func (s *Service) syncServiceUsage(ctx context.Context, current Identity, next LegacyUsage) error {
	access, err := s.ServiceAccess(ctx, current)
	if err != nil {
		return err
	}
	previous, previousErr := s.repo.XNodeLegacyUsage(ctx, current.ProxyUserID)
	if previousErr != nil && !isMissing(previousErr) {
		return previousErr
	}
	if previousErr == nil && !next.SyncedAt.After(previous.SyncedAt) {
		return nil
	}
	if access != nil {
		rotate := current.CurrentPeriodID == ""
		if !rotate && previousErr == nil && previous.PeriodID == current.CurrentPeriodID && changedLegacyReset(previous.LastResetAt, next.LastResetAt) {
			period, err := s.repo.XNodePeriod(ctx, current.CurrentPeriodID)
			if err != nil {
				return err
			}
			if err = s.closePeriod(ctx, period, true); err != nil {
				return err
			}
			rotate = true
		}
		if rotate {
			at := access.StartedAt
			if next.LastResetAt != nil && next.LastResetAt.After(at) {
				at = *next.LastResetAt
			}
			if err = s.startServicePeriod(ctx, current, *access, at); err != nil {
				return err
			}
			current, err = s.repo.XNodeIdentity(ctx, current.ProxyUserID)
			if err != nil {
				return err
			}
		}
	}
	next.ProxyUserID, next.PeriodID = current.ProxyUserID, current.CurrentPeriodID
	return s.repo.SyncXNodeLegacyUsage(ctx, next)
}
