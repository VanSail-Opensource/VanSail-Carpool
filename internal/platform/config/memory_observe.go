package config

import (
	"log/slog"
	"runtime/metrics"
)

// LogMemory records runtime ownership without exposing heap profiles or secrets.
func LogMemory(logger *slog.Logger, event string, fields ...any) {
	samples := []metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/released:bytes"}, {Name: "/memory/classes/heap/objects:bytes"}, {Name: "/sched/goroutines:goroutines"}}
	metrics.Read(samples)
	values := []any{"go_memory_bytes", samples[0].Value.Uint64() - samples[1].Value.Uint64(), "heap_bytes", samples[2].Value.Uint64(), "goroutines", samples[3].Value.Uint64()}
	logger.Info(event, append(values, fields...)...)
}
