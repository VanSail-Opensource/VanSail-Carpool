package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
)

// ConfigureMemory leaves room for SQLite, sockets and operating-system pages.
// An explicitly supplied GOMEMLIMIT remains authoritative, including "off".
func ConfigureMemory(cfg Config, logger *slog.Logger) error {
	temporary := filepath.Join(cfg.DataDir, "tmp")
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		return fmt.Errorf("create SQLite temporary directory: %w", err)
	}
	if os.Getenv("SQLITE_TMPDIR") == "" {
		if err := os.Setenv("SQLITE_TMPDIR", temporary); err != nil {
			return fmt.Errorf("set SQLite temporary directory: %w", err)
		}
	}
	if _, explicit := os.LookupEnv("GOMEMLIMIT"); !explicit {
		if limit := containerMemoryLimit(os.ReadFile); limit > 0 {
			debug.SetMemoryLimit(limit / 8 * 5)
		}
	}
	logger.Info("process memory budget", "go_limit_bytes", debug.SetMemoryLimit(-1), "proxy_cache_bytes", cfg.ProxyCacheMaxBytes)
	return nil
}

func containerMemoryLimit(read func(string) ([]byte, error)) int64 {
	paths := []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"}
	if data, err := read("/proc/self/cgroup"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.SplitN(line, ":", 3)
			if len(parts) != 3 {
				continue
			}
			root, name := "/sys/fs/cgroup", "memory.max"
			if parts[1] != "" {
				if !strings.Contains(","+parts[1]+",", ",memory,") {
					continue
				}
				root, name = "/sys/fs/cgroup/memory", "memory.limit_in_bytes"
			}
			for relative := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(parts[2])), "/"); relative != "." && relative != ""; relative = filepath.ToSlash(filepath.Dir(relative)) {
				if strings.HasPrefix(relative, "..") {
					break
				}
				paths = append(paths, filepath.Join(root, relative, name))
			}
		}
	}
	var limit int64
	for _, path := range paths {
		data, err := read(path)
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		// v1 represents unlimited with a near-MaxInt64 sentinel.
		if err == nil && n > 0 && n < 1<<60 && (limit == 0 || n < limit) {
			limit = n
		}
	}
	return limit
}
