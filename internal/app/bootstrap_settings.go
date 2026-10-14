package app

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func ensureBootstrapSettings(ctx context.Context, store *database.Store, vault *secret.Vault) error {
	if _, err := store.GetSetting(ctx, "telegram.webhook_secret"); errors.Is(err, database.ErrNotFound) {
		value, tokenErr := ids.Token(32)
		if tokenErr != nil {
			return tokenErr
		}
		encrypted, encryptErr := vault.Encrypt("telegram.webhook_secret", value)
		if encryptErr != nil {
			return encryptErr
		}
		if err := store.PutSetting(ctx, "telegram.webhook_secret", encrypted, true, nil); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	defaults := map[string]string{
		"billing.ezpay.enabled": "false", "billing.ezpay.methods": "alipay,wxpay,qqpay,bank,jdpay",
		"billing.bepusdt.enabled": "false", "billing.bepusdt.methods": "usdt.trc20,usdt.erc20,usdt.polygon,usdt.bep20,usdt.aptos,usdt.solana,usdt.xlayer,usdt.arbitrum,usdt.plasma,usdt.ton",
		"billing.bepusdt.ack": "ok", "billing.stars.enabled": "true",
		"activity.timezone": "Asia/Shanghai", "activity.daily_reward_min_txb": "0", "activity.daily_reward_max_txb": "0",
		"activity.group_message_threshold": "0", "activity.group_message_reward_txb": "0",
		catalog.RolloverBeforeAutoRenewalSetting: "true",
	}
	for key, value := range defaults {
		if _, err := store.GetSetting(ctx, key); errors.Is(err, database.ErrNotFound) {
			if err := store.PutSetting(ctx, key, value, false, nil); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
