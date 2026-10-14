package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/google/uuid"
)

func validCommand(actorID, key, reason string) bool {
	reason = strings.TrimSpace(reason)
	return strings.TrimSpace(actorID) != "" && strings.TrimSpace(key) != "" && len(strings.TrimSpace(key)) <= 128 && len(reason) >= 3 && len(reason) <= 500
}

func normalizeUUIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		parsed, err := uuid.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, errors.New("invalid squad UUID")
		}
		canonical := parsed.String()
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 160 {
			return nil, errors.New("invalid identifier")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func commandFingerprint(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func (s *UserWorkflows) validateComboAndSquads(ctx context.Context, comboID string, squads []string) error {
	if _, err := s.repository.ComboByID(ctx, comboID, false); err != nil {
		return err
	}
	if len(squads) == 0 {
		return nil
	}
	live, err := s.importer.ListInternalSquads(ctx)
	if err != nil {
		return err
	}
	available := make(map[string]struct{}, len(live))
	for _, squad := range live {
		available[strings.ToLower(squad.UUID)] = struct{}{}
	}
	for _, squad := range squads {
		if _, exists := available[strings.ToLower(squad)]; !exists {
			return database.ErrNotFound
		}
	}
	return nil
}
