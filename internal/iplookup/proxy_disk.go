package iplookup

import (
	"context"
	"database/sql"
	"fmt"
	runtimeconfig "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"log/slog"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
)

// inventoryDisk contains disposable canonical evidence, never subscription data.
// One connection bounds SQLite memory and prevents long readers growing journals.
type inventoryDisk struct {
	pruneMu     sync.Mutex
	mu          sync.Mutex
	db          *sql.DB
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

// ConfigureInventoryCache must run before the service starts. Failure disables
// collection rather than silently returning to an unbounded in-memory inventory.
func (s *EnhancedService) ConfigureInventoryCache(ctx context.Context, dir string, quota int64) error {
	s.diskRequired = true
	if quota == 0 {
		quota = 1 << 30
	}
	store, err := openInventoryDisk(ctx, dir, quota)
	s.disk, s.diskError = store, err
	return err
}

// CloseInventoryCache runs after source workers and member readers have joined.
func (s *EnhancedService) CloseInventoryCache() error {
	if s == nil || s.disk == nil {
		return nil
	}
	if err := s.disk.db.Close(); err != nil {
		return err
	}
	return removeInventoryFiles(s.disk.dir)
}

func openInventoryDisk(ctx context.Context, dir string, quota int64) (*inventoryDisk, error) {
	if quota < 16<<20 {
		return nil, fmt.Errorf("proxy cache allowance too small")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("proxy cache directory is not a regular directory")
	}
	if err := removeInventoryFiles(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "inventory.db")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=cache_size(-2048)&_pragma=mmap_size(0)&_pragma=temp_store(FILE)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &inventoryDisk{db: db, dir: dir, quota: quota, generations: map[int64]*diskGeneration{}}
	// Reserve half the allowance for rollback journals, plus a fixed header margin.
	// Bounded writes and indexed reads do not construct source-sized temp tables.
	pages := (quota - (1 << 20)) / 2 / 4096
	schema := fmt.Sprintf("PRAGMA page_size=4096; PRAGMA auto_vacuum=INCREMENTAL; PRAGMA journal_mode=DELETE; PRAGMA max_page_count=%d; ", pages)
	schema += "CREATE TABLE prefixes(gen INTEGER NOT NULL,family INTEGER NOT NULL,bits INTEGER NOT NULL,network BLOB NOT NULL,PRIMARY KEY(gen,family,bits,network)) WITHOUT ROWID; CREATE INDEX canonical ON prefixes(family,bits,network,gen); CREATE TABLE domains(gen INTEGER NOT NULL,host TEXT NOT NULL,PRIMARY KEY(gen,host)) WITHOUT ROWID;"
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
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
			"DELETE FROM prefixes WHERE gen=? AND (family,bits,network) IN (SELECT family,bits,network FROM prefixes WHERE gen=? LIMIT 256)",
			"DELETE FROM domains WHERE gen=? AND host IN (SELECT host FROM domains WHERE gen=? LIMIT 256)",
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
	var total int64
	for _, name := range []string{"inventory.db", "inventory.db-journal", "inventory.db-wal", "inventory.db-shm"} {
		if info, err := os.Stat(filepath.Join(d.dir, name)); err == nil {
			total += info.Size()
		}
	}
	return total
}

func (s *EnhancedService) inventoryMemorySummary() {
	if s.disk != nil {
		runtimeconfig.LogMemory(slog.Default(), "proxy inventory cache", "disk_bytes", s.disk.usage(), "allowance_bytes", s.disk.quota)
	}
}
