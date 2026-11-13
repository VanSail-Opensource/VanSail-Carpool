package postgres

import (
	"context"
	"fmt"
)

// SequenceState preserves the next-value behavior required by a recovered writer.
// Unlike the historical Sequences projection, uncalled values remain distinct.
type SequenceState struct {
	LastValue int64 `json:"lastValue"`
	IsCalled  bool  `json:"isCalled"`
	Start     int64 `json:"start"`
	Increment int64 `json:"increment"`
	Minimum   int64 `json:"minimum"`
	Maximum   int64 `json:"maximum"`
	Cache     int64 `json:"cache"`
	Cycle     bool  `json:"cycle"`
}

func readSequenceStates(ctx context.Context, q queryer, tables []TableContract) (map[string]SequenceState, error) {
	inspected := make(map[string]bool, len(tables))
	for _, table := range tables {
		inspected[table.Name] = true
	}
	rows, err := q.QueryContext(ctx, `SELECT s.relname,COALESCE(t.relname,''),
		p.seqstart,p.seqincrement,p.seqmin,p.seqmax,p.seqcache,p.seqcycle
		FROM pg_class s JOIN pg_namespace n ON n.oid=s.relnamespace
		JOIN pg_sequence p ON p.seqrelid=s.oid
		LEFT JOIN pg_depend d ON d.classid='pg_class'::regclass AND d.objid=s.oid
			AND d.refclassid='pg_class'::regclass AND d.deptype IN ('a','i')
		LEFT JOIN pg_class t ON t.oid=d.refobjid
		WHERE n.nspname='public' AND s.relkind='S' ORDER BY s.relname`)
	if err != nil {
		return nil, fmt.Errorf("inspect recovery sequence catalog: %w", err)
	}
	defer rows.Close()
	states := make(map[string]SequenceState)
	var names []string
	for rows.Next() {
		var name, owner string
		var state SequenceState
		if err := rows.Scan(&name, &owner, &state.Start, &state.Increment, &state.Minimum, &state.Maximum, &state.Cache, &state.Cycle); err != nil {
			return nil, err
		}
		// Goose history belongs to the migration runner. Upgrade comparisons can
		// deliberately inspect an earlier projection, so include only its tables.
		if !inspected[owner] && name != "xnode_service_proxy_id_seq" {
			continue
		}
		names = append(names, name)
		states[name] = state
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The snapshot uses one pgx connection; close catalog iteration before reading
	// each sequence relation rather than issuing a nested query on busy rows.
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, name := range names {
		state := states[name]
		if err := q.QueryRowContext(ctx, `SELECT last_value,is_called FROM "public".`+quote(name)).Scan(&state.LastValue, &state.IsCalled); err != nil {
			return nil, fmt.Errorf("inspect recovery sequence state: %w", err)
		}
		states[name] = state
	}
	return states, nil
}
