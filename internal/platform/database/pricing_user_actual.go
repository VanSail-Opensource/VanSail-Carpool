package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func (s *Store) pricingUserActual(ctx context.Context, user string, from, until time.Time) (*float64, string, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT features_json FROM pricing_features WHERE entity_type='user' AND entity_id=? AND watermark<? ORDER BY watermark DESC LIMIT 1`, user, pricingStamp(from)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "MISSING", nil
	}
	if err != nil {
		return nil, "", err
	}
	var feature struct {
		Nodes map[string]json.RawMessage `json:"nodeDistributions"`
	}
	if err = json.Unmarshal([]byte(raw), &feature); err != nil {
		return nil, "", err
	}
	if len(feature.Nodes) == 0 {
		return nil, "MISSING", nil
	}
	total := 0.0
	for node := range feature.Nodes {
		value, status, e := s.pricingActual(ctx, "user_node", user+":"+node, "traffic_bytes", from, until)
		if e != nil {
			return nil, "", e
		}
		if status == "CENSORED" {
			return nil, status, nil
		}
		if status != "COMPLETE" || value == nil {
			return nil, "INCOMPLETE", nil
		}
		total += *value
	}
	return &total, "COMPLETE", nil
}
