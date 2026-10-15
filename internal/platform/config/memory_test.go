package config

import (
	"io"
	"io/fs"
	"log/slog"
	"runtime/debug"
	"testing"
)

func TestContainerMemoryLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  int64
	}{
		{"v2", map[string]string{"/sys/fs/cgroup/memory.max": "268435456\n"}, 256 << 20},
		{"v1", map[string]string{"/sys/fs/cgroup/memory/memory.limit_in_bytes": "268435456"}, 256 << 20},
		{"ancestor", map[string]string{"/proc/self/cgroup": "0::/tenant/service", "/sys/fs/cgroup/tenant/service/memory.max": "max", "/sys/fs/cgroup/tenant/memory.max": "268435456"}, 256 << 20},
		{"unlimited", map[string]string{"/sys/fs/cgroup/memory.max": "max", "/sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712"}, 0},
		{"missing", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func(path string) ([]byte, error) {
				if value, ok := tc.files[path]; ok {
					return []byte(value), nil
				}
				return nil, fs.ErrNotExist
			}
			if got := containerMemoryLimit(read); got != tc.want {
				t.Fatalf("limit=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestExplicitMemoryLimitRemainsAuthoritative(t *testing.T) {
	old := debug.SetMemoryLimit(42 << 20)
	t.Cleanup(func() { debug.SetMemoryLimit(old) })
	t.Setenv("GOMEMLIMIT", "64MiB")
	t.Setenv("SQLITE_TMPDIR", "")
	if err := ConfigureMemory(Config{DataDir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	if got := debug.SetMemoryLimit(-1); got != 42<<20 {
		t.Fatalf("explicit runtime limit changed: %d", got)
	}
}
