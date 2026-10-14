package statistics

import (
	"context"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) cachedSquadNames() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]string)
	for _, group := range s.snapshot.Database.ComboBySquad {
		addSquadName(result, group.ID, group.Label)
	}
	for _, group := range s.snapshot.Database.SquadByCombo {
		for _, segment := range group.Segments {
			addSquadName(result, segment.ID, segment.Label)
		}
	}
	return result
}

func (s *Service) resolveSquadNames(ctx context.Context) map[string]string {
	names := s.cachedSquadNames()
	provider, ok := s.provider.(SquadNameProvider)
	if !ok {
		return names
	}
	current, err := provider.SquadNames(ctx)
	if err != nil {
		return names
	}
	for uuid, name := range current {
		addSquadName(names, uuid, name)
	}
	return names
}

func addSquadName(names map[string]string, uuid, name string) {
	uuid, name = strings.TrimSpace(uuid), strings.TrimSpace(name)
	if uuid != "" && name != "" && name != uuid {
		names[uuid] = name
	}
}

func applySquadNames(statistics *model.DatabaseStatistics, names map[string]string) {
	if statistics == nil || len(names) == 0 {
		return
	}
	for groupIndex := range statistics.SquadByCombo {
		for segmentIndex := range statistics.SquadByCombo[groupIndex].Segments {
			segment := &statistics.SquadByCombo[groupIndex].Segments[segmentIndex]
			if name := names[segment.ID]; name != "" {
				segment.Label = name
			}
		}
	}
	for groupIndex := range statistics.ComboBySquad {
		group := &statistics.ComboBySquad[groupIndex]
		if name := names[group.ID]; name != "" {
			group.Label = name
		}
	}
}
