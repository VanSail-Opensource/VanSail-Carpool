package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func insertAdminUserAudit(ctx context.Context, tx *sql.Tx, actorID, action, userID, reason string, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &actorID, action, "user", userID, fmt.Sprintf("reason=%s", reason), now)
}
