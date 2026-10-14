package app

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
)

func migrationCredentials(store *database.Store, settings *admin.SettingsService, vault *secret.Vault) func(context.Context) (string, string, error) {
	return func(ctx context.Context) (string, string, error) {
		state, err := store.XNodeMigration(ctx)
		if err != nil {
			return "", "", err
		}
		if state.Phase == "complete" {
			return "", "", xnode.ErrConflict
		}
		if state.SourceCipher != "" {
			raw, err := vault.Decrypt("xnode.migration.source", state.SourceCipher)
			if err != nil {
				return "", "", err
			}
			var source xnodemigration.SourceConfig
			if json.Unmarshal([]byte(raw), &source) != nil {
				return "", "", xnode.ErrInvalid
			}
			return source.URL, source.Token, nil
		}
		base, err := settings.Optional(ctx, "remnawave.base_url")
		if err != nil {
			return "", "", err
		}
		token, err := settings.Optional(ctx, "remnawave.api_token")
		return base, token, err
	}
}
