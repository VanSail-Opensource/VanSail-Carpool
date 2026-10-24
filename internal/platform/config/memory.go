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
	temporary, err := filepath.Abs(filepath.Join(cfg.DataDir, "tmp"))
	if err != nil {
		return fmt.Errorf("resolve process temporary directory: %w", err)
	}
	if err := os.MkdirAll(temporary, 0o700); err != nil {
		return fmt.Errorf("create process temporary directory: %w", err)
	}
	for _, name := range []string{"TMPDIR", "TMP", "TEMP", "SQLITE_TMPDIR"} {
		if name == "TMPDIR" || os.Getenv(name) == "" {
			if err := os.Setenv(name, temporary); err != nil {
				return fmt.Errorf("set %s temporary directory: %w", name, err)
			}
		}
	}
	if _, explicit := os.LookupEnv("GOMEMLIMIT"); !explicit {
		if limit := containerMemoryLimit(os.ReadFile); limit > 0 {
			fraction := int64(5)
			if os.Getenv("PG_EMBEDDED") == "true" {
				fraction = 3 // PostgreSQL shares this cgroup: 384 MiB of a 1 GiB limit.
			}
			debug.SetMemoryLimit(limit / 8 * fraction)
		}
	}
	logger.Info("process memory budget", "go_limit_bytes", debug.SetMemoryLimit(-1), "proxy_cache_bytes", cfg.ProxyCacheMaxBytes, "temporary_directory", temporary, "sqlite_temporary_directory", os.Getenv("SQLITE_TMPDIR"))
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
