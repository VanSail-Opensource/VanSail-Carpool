package xnode

import (
	"context"
	"strings"
	"time"
)

const hourlyPrincipalPrefix = "member-hourly-"

// HourlyPrincipal projects durable commerce scope; its presence grants nothing.
// One enrollment keeps one principal and period across prepaid hour boundaries.
type HourlyPrincipal struct {
	EnrollmentID, OwnerID, GroupID, PeriodID string
	AccountGeneration, ProxyUserID           int64
	Status                                   string
	ActivatedAt, PaidThrough                 time.Time
}

type hourlyPrincipalRepository interface {
	XNodeHourlyPrincipal(context.Context, string) (HourlyPrincipal, error)
	XNodeHourlyPrincipals(context.Context, string, string) ([]HourlyPrincipal, error)
	EnsureXNodeHourlyPrincipal(context.Context, string, string) (HourlyPrincipal, error)
}

func (s *Service) hourlyPrincipal(ctx context.Context, id string) (HourlyPrincipal, error) {
	if source, ok := s.repo.(hourlyPrincipalRepository); ok {
		return source.XNodeHourlyPrincipal(ctx, id)
	}
	return HourlyPrincipal{}, ErrUnavailable
}

func isHourlyIdentity(identity Identity) bool {
	return strings.HasPrefix(identity.ServiceName, hourlyPrincipalPrefix)
}

func (s *Service) hourlyPrincipals(ctx context.Context, owner, node string) ([]HourlyPrincipal, error) {
	if source, ok := s.repo.(hourlyPrincipalRepository); ok {
		return source.XNodeHourlyPrincipals(ctx, owner, node)
	}
	return []HourlyPrincipal{}, nil
}

// PrepareHourlyPrincipal creates only dormant credentials. Commerce separately
// rechecks its offer and wallet before committing an activation and paid boundary.
func (s *Service) PrepareHourlyPrincipal(ctx context.Context, enrollment string) (HourlyPrincipal, error) {
	if enrollment == "" || s.vault == nil {
		return HourlyPrincipal{}, ErrInvalid
	}
	repo, ok := s.repo.(hourlyPrincipalRepository)
	if !ok {
		return HourlyPrincipal{}, ErrUnavailable
	}
	p, err := repo.XNodeHourlyPrincipal(ctx, enrollment)
	if err != nil {
		return HourlyPrincipal{}, err
	}
	owner, err := s.repo.UserByID(ctx, p.OwnerID)
	if err != nil {
		return HourlyPrincipal{}, err
	}
	if owner.AccountGeneration != p.AccountGeneration {
		return HourlyPrincipal{}, ErrConflict
	}
	// The parent owns the public subscription token; neither identity grants
	// access until commerce commits paid scope and compilation admits it.
	if _, err = s.EnsureIdentity(ctx, owner); err != nil {
		return HourlyPrincipal{}, err
	}
	credentials, err := NewCredentials()
	if err != nil {
		return HourlyPrincipal{}, err
	}
	cipher, err := sealCredentials(s.vault, Identity{ServiceName: hourlyPrincipalPrefix + enrollment}, credentials)
	if err != nil {
		return HourlyPrincipal{}, err
	}
	return repo.EnsureXNodeHourlyPrincipal(ctx, enrollment, cipher)
}

func hourlyPrincipalAvailable(p HourlyPrincipal, now time.Time) bool {
	return p.Status == "active" && p.ProxyUserID > 0 && p.PeriodID != "" && p.PaidThrough.After(now)
}
