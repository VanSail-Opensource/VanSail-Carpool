package database

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Use an indexed broad candidate boundary, then parse exact instants. Variable
// RFC3339Nano fractions do not sort chronologically against whole seconds.
func dueEntitlementTransitionsTx(ctx context.Context, tx *sql.Tx, now time.Time) (expired, unopened, queued []string, err error) {
	upper := stamp(now.UTC().Truncate(time.Second).Add(time.Second))
	rows, err := tx.QueryContext(ctx, `SELECT id,status,valid_from,valid_until FROM purchases
		WHERE status IN ('active','activating','queued','failed')
		AND (valid_until<$1 OR status='queued' AND valid_from<$1)`, upper)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	type candidate struct {
		id   string
		from time.Time
	}
	ready := make([]candidate, 0)
	for rows.Next() {
		var id, status, rawFrom, rawUntil string
		if err := rows.Scan(&id, &status, &rawFrom, &rawUntil); err != nil {
			return nil, nil, nil, err
		}
		until, err := parseStamp(rawUntil)
		if err != nil {
			return nil, nil, nil, err
		}
		if !until.After(now) {
			if status == "active" || status == "activating" {
				expired = append(expired, id)
			} else {
				unopened = append(unopened, id)
			}
			continue
		}
		if status == "queued" {
			from, err := parseStamp(rawFrom)
			if err != nil {
				return nil, nil, nil, err
			}
			if !from.After(now) {
				ready = append(ready, candidate{id: id, from: from})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].from.Equal(ready[j].from) {
			return ready[i].id < ready[j].id
		}
		return ready[i].from.Before(ready[j].from)
	})
	for _, item := range ready {
		queued = append(queued, item.id)
	}
	return expired, unopened, queued, nil
}
