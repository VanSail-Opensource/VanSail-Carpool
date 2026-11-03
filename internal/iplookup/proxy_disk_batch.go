package iplookup

import (
	"context"
	"database/sql"
)

// Each call writes at most one bounded 256-row batch. Candidate selection and
// insertion share the transaction; duplicate input never consumes the allowance.
// Scalar existence probes keep each lookup on the composite index. A hash anti
// join can otherwise scan the growing temporary inventory for every small batch.
func insertInventoryPrefixes(ctx context.Context, tx *sql.Tx, generation int64, values []diskPrefix, limit, current int) (int, bool, error) {
	if len(values) == 0 {
		return 0, false, nil
	}
	families, bits := make([]int64, len(values)), make([]int64, len(values))
	networks := make([][]byte, len(values))
	for i, v := range values {
		families[i], bits[i], networks[i] = int64(v.family), int64(v.bits), v.network
	}
	allowance := len(values)
	if limit > 0 {
		allowance = max(0, min(allowance, limit-current))
	}
	var added, candidates int
	err := tx.QueryRowContext(ctx, `WITH input AS (
	 SELECT DISTINCT family,bits,network FROM unnest($2::bigint[],$3::bigint[],$4::bytea[]) AS p(family,bits,network)
	), candidates AS MATERIALIZED (
	 SELECT * FROM input i WHERE (SELECT true FROM prefixes p
	 WHERE p.gen=$1 AND p.family=i.family AND p.bits=i.bits AND p.network=i.network LIMIT 1) IS NULL
	), added AS (
	 INSERT INTO prefixes(gen,family,bits,network) SELECT $1,family,bits,network FROM candidates
	 ORDER BY family,bits,network LIMIT $5 ON CONFLICT DO NOTHING RETURNING 1
	) SELECT (SELECT count(*) FROM added),(SELECT count(*) FROM candidates)`,
		generation, families, bits, networks, allowance).Scan(&added, &candidates)
	return added, candidates > allowance, err
}

func insertInventoryDomains(ctx context.Context, tx *sql.Tx, generation int64, hosts []string, allowance int) (int, bool, error) {
	if len(hosts) == 0 {
		return 0, false, nil
	}
	allowance = max(0, min(allowance, len(hosts)))
	var added, candidates int
	err := tx.QueryRowContext(ctx, `WITH candidates AS MATERIALIZED (
	 SELECT DISTINCT host FROM unnest($2::text[]) AS input(host)
	 WHERE (SELECT true FROM domains d WHERE d.gen=$1 AND d.host=input.host LIMIT 1) IS NULL
	), added AS (
	 INSERT INTO domains(gen,host) SELECT $1,host FROM candidates ORDER BY host
	 LIMIT $3 ON CONFLICT DO NOTHING RETURNING 1
	) SELECT (SELECT count(*) FROM added),(SELECT count(*) FROM candidates)`, generation, hosts, allowance).Scan(&added, &candidates)
	return added, candidates > allowance, err
}
