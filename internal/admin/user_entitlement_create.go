package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// AdminEntitlementCreate describes one zero-charge, administrator-owned manual
// fixed-term grant. It never debits the wallet or creates ledger debt.
type AdminEntitlementCreate struct {
	Reason            string
	ComboID           string
	ValidFrom         time.Time
	ValidUntil        time.Time
	TrafficLimitBytes int64
	ResetStrategy     string
	SquadUUIDs        []string
}

// adminEntitlementCreator is the narrow store command used by manual grants.
type adminEntitlementCreator interface {
	CreateAdminEntitlement(context.Context, database.AdminEntitlementCreateInput, time.Time) (model.Purchase, error)
	AdminEntitlementGrantByKey(context.Context, string, string, string) (database.AdminEntitlementGrantReplay, bool, error)
}

// CreateEntitlement replays accepted creation evidence before current catalog
// checks. A fresh audited, zero-charge grant inherits omitted squads from the
// active combo and validates every selected squad against the provider.
func (s *UserWorkflows) CreateEntitlement(ctx context.Context, actorID, userID, key string, grant AdminEntitlementCreate) (model.Purchase, error) {
	grant.Reason = strings.TrimSpace(grant.Reason)
	if !validCommand(actorID, key, grant.Reason) || userID == "" || strings.TrimSpace(grant.ComboID) == "" ||
		grant.ValidUntil.Sub(grant.ValidFrom) < time.Second || grant.TrafficLimitBytes <= 0 || !validResetStrategy(grant.ResetStrategy) {
		return model.Purchase{}, errors.New("invalid entitlement grant")
	}
	squads, err := normalizeUUIDs(grant.SquadUUIDs)
	if err != nil {
		return model.Purchase{}, err
	}
	grant.ComboID = strings.TrimSpace(grant.ComboID)
	grant.SquadUUIDs = squads
	grant.ValidFrom, grant.ValidUntil = grant.ValidFrom.UTC(), grant.ValidUntil.UTC()
	creator, ok := s.repository.(adminEntitlementCreator)
	if !ok {
		return model.Purchase{}, errors.New("entitlement creation is unavailable")
	}
	if replay, found, err := creator.AdminEntitlementGrantByKey(ctx, actorID, userID, strings.TrimSpace(key)); found || err != nil {
		if err != nil {
			return model.Purchase{}, err
		}
		// Historical fingerprints contain the effective included squads. An
		// omitted selection reuses those accepted facts, never today's catalog.
		if len(grant.SquadUUIDs) == 0 {
			grant.SquadUUIDs, err = normalizeUUIDs(replay.Purchase.SquadUUIDs)
			if err != nil {
				return model.Purchase{}, database.ErrConflict
			}
		}
		fingerprint, err := manualGrantFingerprint(actorID, userID, key, grant)
		if err != nil {
			return model.Purchase{}, err
		}
		if fingerprint != replay.RequestFingerprint {
			return model.Purchase{}, database.ErrConflict
		}
		return replay.Purchase, nil
	}
	combo, err := s.repository.ComboByID(ctx, grant.ComboID, true)
	if err != nil {
		return model.Purchase{}, err
	}
	if len(squads) == 0 {
		for _, included := range combo.IncludedSquads {
			squads = append(squads, included.GroupID)
		}
		if squads, err = normalizeUUIDs(squads); err != nil {
			return model.Purchase{}, err
		}
	}
	if err := s.validateImportedSquads(ctx, squads); err != nil {
		return model.Purchase{}, err
	}
	grant.SquadUUIDs = squads
	fingerprint, err := manualGrantFingerprint(actorID, userID, key, grant)
	if err != nil {
		return model.Purchase{}, err
	}
	return creator.CreateAdminEntitlement(ctx, database.AdminEntitlementCreateInput{
		ActorUserID: actorID, UserID: userID, IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Reason: grant.Reason, ComboID: grant.ComboID,
		ValidFrom: grant.ValidFrom, ValidUntil: grant.ValidUntil, TrafficLimitBytes: grant.TrafficLimitBytes,
		ResetStrategy: grant.ResetStrategy, SquadUUIDs: grant.SquadUUIDs,
	}, s.now().UTC())
}

func manualGrantFingerprint(actorID, userID, key string, grant AdminEntitlementCreate) (string, error) {
	return commandFingerprint(struct {
		ActorID, UserID, Key string
		Grant                AdminEntitlementCreate
	}{actorID, userID, strings.TrimSpace(key), grant})
}
