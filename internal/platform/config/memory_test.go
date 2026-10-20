package config

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, os.Getenv(name))
	}
	if err := ConfigureMemory(Config{DataDir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	if got := debug.SetMemoryLimit(-1); got != 42<<20 {
		t.Fatalf("explicit runtime limit changed: %d", got)
	}
}

func TestProcessScratchUnderDataDirectory(t *testing.T) {
	data := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP", "SQLITE_TMPDIR"} {
		t.Setenv(name, "")
	}
	t.Setenv("GOMEMLIMIT", "off")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := ConfigureMemory(Config{DataDir: data}, logger); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(data, "tmp")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP", "SQLITE_TMPDIR"} {
		if got := os.Getenv(name); got != want {
			t.Fatalf("%s=%q, want %q", name, got, want)
		}
	}
	f, err := os.CreateTemp("", "country-asset-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if filepath.Dir(f.Name()) != want {
		t.Fatal("process temporary file escaped DATA_DIR/tmp")
	}
	for _, name := range []string{"TMP", "TEMP", "SQLITE_TMPDIR"} {
		t.Setenv(name, data)
	}
	t.Setenv("TMPDIR", "/read-only-root")
	if err := ConfigureMemory(Config{DataDir: data}, logger); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("TMPDIR") != want {
		t.Fatal("process scratch did not replace TMPDIR")
	}
	for _, name := range []string{"TMP", "TEMP", "SQLITE_TMPDIR"} {
		if os.Getenv(name) != data {
			t.Fatal("explicit override lost", name)
		}
	}
}
