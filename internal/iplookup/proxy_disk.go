package iplookup

import (
	"context"
	"database/sql"
	"fmt"
	runtimeconfig "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"log/slog"
	"os"
	"sync"
	"time"
)

// inventoryDisk owns disposable PostgreSQL temporary tables on one pinned session.
// No upstream inventory enters durable application relations or the WAL.
type inventoryDisk struct {
	pruneMu     sync.Mutex
	mu          sync.Mutex
	db          *sql.Conn
	dir         string
	quota       int64
	next        int64
	generations map[int64]*diskGeneration
}

type diskGeneration struct {
	store *inventoryDisk
	id    int64
	refs  int
}

// ConfigureInventoryCache bounds transient inventory storage before workers start.
func (s *EnhancedService) ConfigureInventoryCache(ctx context.Context, pool *sql.DB, dir string, quota int64) error {
	s.diskRequired = true
	if quota == 0 {
		quota = 1 << 30
	}
	store, err := openInventoryDisk(ctx, pool, dir, quota)
	s.disk, s.diskError = store, err
	return err
}

// CloseInventoryCache drops only this session's temporary tables after readers stop.
func (s *EnhancedService) CloseInventoryCache() error {
	if s == nil || s.disk == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.disk.db.ExecContext(ctx, `DROP TABLE IF EXISTS pg_temp.prefixes,pg_temp.domains`)
	closeErr := s.disk.db.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func openInventoryDisk(ctx context.Context, pool *sql.DB, dir string, quota int64) (*inventoryDisk, error) {
	if quota < 16<<20 {
		return nil, fmt.Errorf("proxy cache allowance too small")
	}
	if pool == nil {
		return nil, fmt.Errorf("PostgreSQL proxy cache connection is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("proxy cache directory is not a regular directory")
	}
	conn, err := pool.Conn(ctx)
	if err != nil {
		return nil, err
	}
	statements := []string{
		`DROP TABLE IF EXISTS pg_temp.prefixes,pg_temp.domains`,
		`CREATE TEMPORARY TABLE prefixes(gen BIGINT NOT NULL,family BIGINT NOT NULL,bits BIGINT NOT NULL,network BYTEA NOT NULL,PRIMARY KEY(gen,family,bits,network)) ON COMMIT PRESERVE ROWS`,
		`CREATE INDEX canonical ON prefixes(family,bits,network,gen)`,
		`CREATE TEMPORARY TABLE domains(gen BIGINT NOT NULL,host TEXT COLLATE "C" NOT NULL,PRIMARY KEY(gen,host)) ON COMMIT PRESERVE ROWS`,
	}
	for _, statement := range statements {
		if _, err = conn.ExecContext(ctx, statement); err != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			_, _ = conn.ExecContext(cleanup, `DROP TABLE IF EXISTS pg_temp.prefixes,pg_temp.domains`)
			cancel()
			conn.Close()
			return nil, err
		}
	}
	return &inventoryDisk{db: conn, dir: dir, quota: quota, generations: map[int64]*diskGeneration{}}, nil
}

func (d *inventoryDisk) newGeneration(ctx context.Context) (*prefixIndex, error) {
	if err := d.prune(ctx); err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	generation := &diskGeneration{store: d, id: d.next, refs: 1}
	d.generations[generation.id] = generation
	return &prefixIndex{disk: generation}, nil
}

func (p *prefixIndex) retain() *prefixIndex {
	if p != nil && p.disk != nil {
		d := p.disk.store
		d.mu.Lock()
		p.disk.refs++
		d.mu.Unlock()
	}
	return p
}

func (p *prefixIndex) release() {
	if p != nil && p.disk != nil {
		d := p.disk.store
		d.mu.Lock()
		p.disk.refs--
		d.mu.Unlock()
	}
}

func (d *inventoryDisk) prune(ctx context.Context) error {
	d.pruneMu.Lock()
	defer d.pruneMu.Unlock()
	d.mu.Lock()
	retired := []int64{}
	for id, generation := range d.generations {
		if generation.refs == 0 {
			retired = append(retired, id)
		}
	}
	d.mu.Unlock()
	for _, id := range retired {
		for _, query := range []string{
			"DELETE FROM prefixes WHERE gen=$1 AND (family,bits,network) IN (SELECT family,bits,network FROM prefixes WHERE gen=$2 LIMIT 256)",
			"DELETE FROM domains WHERE gen=$1 AND host IN (SELECT host FROM domains WHERE gen=$2 LIMIT 256)",
		} {
			for {
				if err := ctx.Err(); err != nil {
					return err
				}
				d.mu.Lock()
				result, err := d.db.ExecContext(ctx, query, id, id)
				var count int64
				if err == nil {
					count, err = result.RowsAffected()
				}
				d.mu.Unlock()
				if err != nil {
					return err
				}
				if count == 0 {
					break
				}
			}
		}
		d.mu.Lock()
		delete(d.generations, id)
		d.mu.Unlock()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reclaim(ctx)
}

func (d *inventoryDisk) usage() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var size int64
	if err := d.db.QueryRowContext(ctx, `SELECT pg_total_relation_size('pg_temp.prefixes')+pg_total_relation_size('pg_temp.domains')`).Scan(&size); err != nil {
		return d.quota + 1
	}
	return size
}

func (s *EnhancedService) inventoryMemorySummary() {
	if s.disk != nil {
		runtimeconfig.LogMemory(slog.Default(), "proxy inventory cache", "disk_bytes", s.disk.usage(), "allowance_bytes", s.disk.quota)
	}
}
