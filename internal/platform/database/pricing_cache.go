package database

import (
	"context"
	"encoding/json"
	"time"
)

func pricingCached[T any](ctx context.Context, s *Store, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	var value T
	raw, err := s.pricingCache.Load(ctx, key, ttl, func() ([]byte, error) {
		v, e := load()
		if e != nil {
			return nil, e
		}
		return json.Marshal(v)
	})
	if err == nil {
		err = json.Unmarshal(raw, &value)
	}
	return value, err
}
