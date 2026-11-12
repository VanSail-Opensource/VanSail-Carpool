package admin

import (
	"context"
	"time"
)

// The settings fixture exposes the same required atomic contract as production.
// It has no independent append-audit phase after publishing the setting.
func (r *adminSettingsRepository) PutAdminSetting(ctx context.Context, key, value string, encrypted bool, actor string, now time.Time) error {
	if err := r.PutSetting(ctx, key, value, encrypted, &actor); err != nil {
		return err
	}
	r.auditCalls++
	r.lastAuditAt = now
	return nil
}
