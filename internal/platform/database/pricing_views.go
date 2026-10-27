package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ReadPricingDataset exposes a bounded administrative view of the export registry.
func (s *Store) ReadPricingDataset(ctx context.Context, name string, scope PricingExportScope, limit int) ([]map[string]any, error) {
	key, _ := json.Marshal([]any{name, scope, limit})
	return pricingCached(ctx, s, "dataset:"+string(key), 20*time.Second, func() ([]map[string]any, error) { return s.readPricingDataset(ctx, name, scope, limit) })
}
func (s *Store) readPricingDataset(ctx context.Context, name string, scope PricingExportScope, limit int) ([]map[string]any, error) {
	page, err := s.pricingDatasetPage(ctx, name, scope, limit, "")
	return page.Items, err
}

// PricingProducts includes fixed products without persisting redundant settings.
type pricingProductPage struct {
	Items []map[string]any
	Next  string
}

func (s *Store) PricingProducts(ctx context.Context, cursor string) ([]map[string]any, string, error) {
	page, err := pricingCached(ctx, s, "products:"+cursor, 20*time.Second, func() (pricingProductPage, error) {
		items, next, e := s.pricingProducts(ctx, cursor)
		return pricingProductPage{items, next}, e
	})
	return page.Items, page.Next, err
}
func (s *Store) pricingProducts(ctx context.Context, cursor string) ([]map[string]any, string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind,id,name,price FROM (SELECT 'combo' AS kind,id,name,price_txb_minor AS price FROM combos UNION ALL SELECT CASE WHEN billing_mode='payg' THEN 'payg' ELSE 'addon' END AS kind,group_id AS id,group_id AS name,CASE WHEN billing_mode='payg' THEN payg_price_minor ELSE price_txb_minor END AS price FROM squad_product_overrides) p WHERE kind||':'||id>? ORDER BY kind,id LIMIT 101`, cursor)
	if err != nil {
		return nil, "", err
	}
	type product struct {
		kind, id, name string
		price          int64
	}
	products := []product{}
	for rows.Next() {
		var p product
		if err = rows.Scan(&p.kind, &p.id, &p.name, &p.price); err != nil {
			rows.Close()
			return nil, "", err
		}
		products = append(products, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(products) > 100 {
		products = products[:100]
		p := products[99]
		next = p.kind + ":" + p.id
	}
	ctx, err = s.pricingReadContext(ctx, "", time.Now().UTC())
	if err != nil {
		return nil, "", err
	}

	out := []map[string]any{}
	for _, p := range products {
		policy, e := loadPricingPolicy(ctx, s.db, p.kind, p.id, p.price)
		if e != nil {
			return nil, "", e
		}
		out = append(out, map[string]any{"id": p.kind + ":" + p.id, "productType": p.kind, "productId": p.id, "name": p.name, "fixedPriceMinor": strconv.FormatInt(p.price, 10), "policy": policy})
	}
	if err = s.pricingProductReadiness(ctx, out, time.Now().UTC()); err != nil {
		return nil, "", err
	}
	return out, next, nil
}

// PricingStorage measures actual table/index footprint rather than assumed row sizes.
func (s *Store) PricingStorage(ctx context.Context) (map[string]any, error) {
	return pricingCached(ctx, s, "storage", 5*time.Minute, func() (map[string]any, error) { return s.pricingStorage(ctx) })
}
func (s *Store) pricingStorage(ctx context.Context) (map[string]any, error) {
	var total int64
	var databaseBytes int64
	if IsPostgres(s.db) {
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(pg_total_relation_size(c.oid)),0) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='r' AND c.relname LIKE 'pricing_%'`).Scan(&total); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&databaseBytes); err != nil {
			return nil, err
		}
	} else {
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(pgsize),0) FROM dbstat WHERE name IN (SELECT name FROM sqlite_master WHERE name LIKE 'pricing_%' OR tbl_name LIKE 'pricing_%')`).Scan(&total); err != nil {
			return nil, err
		}
		var pages, size int64
		if err := s.db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pages); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&size); err != nil {
			return nil, err
		}
		databaseBytes = pages * size
	}
	return map[string]any{"pricingBytes": fmt.Sprint(total), "databaseBytes": fmt.Sprint(databaseBytes), "pricingBudgetBytes": fmt.Sprint(700 << 20), "databaseTargetBytes": fmt.Sprint(3_000_000_000), "pressure": total >= 700<<20 || databaseBytes >= 3_000_000_000, "measuredAt": time.Now().UTC()}, nil
}
