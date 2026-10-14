package catalog

import (
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func selectedSquadUUIDs(catalog model.Catalog, comboID string, addonIDs []string) []string {
	selected := make(map[string]struct{})
	for _, combo := range catalog.Combos {
		if combo.ID != comboID {
			continue
		}
		for _, squad := range combo.IncludedSquads {
			selected[squad.RemnaSquadUUID] = struct{}{}
		}
		break
	}
	for _, addonID := range addonIDs {
		trimmed := strings.TrimSpace(addonID)
		for _, addon := range catalog.Addons {
			if addon.ID == trimmed || addon.RemnaSquadUUID == trimmed {
				selected[addon.RemnaSquadUUID] = struct{}{}
				break
			}
		}
	}
	result := make([]string, 0, len(selected))
	for squadUUID := range selected {
		result = append(result, squadUUID)
	}
	sort.Strings(result)
	return result
}
