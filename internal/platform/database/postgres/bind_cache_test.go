package postgres

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestBindingCachePreservesTypesAndProtectedGrammar(t *testing.T) {
	cache := &bindingCache{}
	for _, query := range []string{"SELECT ?", "SELECT $1", "SELECT '?' WHERE flag IS ? /* ? */", "INSERT OR IGNORE INTO t VALUES(?) RETURNING id"} {
		for repeat := 0; repeat < 3; repeat++ {
			for _, value := range []any{true, false, int64(3), "private-value", nil, []byte("bytes")} {
				wantArgs := []driver.NamedValue{{Ordinal: 1, Value: value}}
				gotArgs := append([]driver.NamedValue(nil), wantArgs...)
				want, wantErr := Bind(query, wantArgs)
				got, gotErr := cache.bind(query, gotArgs)
				if got != want || (gotErr != nil) != (wantErr != nil) || !reflect.DeepEqual(gotArgs, wantArgs) {
					t.Fatalf("%q value=%T: %q %v, want %q %v", query, value, got, gotErr, want, wantErr)
				}
			}
		}
	}
	for key, entry := range cache.items {
		if strings.Contains(key.query+key.kinds+entry.query, "private-value") {
			t.Fatal("argument value retained in grammar cache")
		}
	}
	for _, query := range []string{"SELECT ?", "SELECT '", "SELECT ?|array['x']"} {
		if _, err := cache.bind(query, nil); err == nil {
			t.Fatal("cached grammar bypassed validation", query)
		}
	}
}

func TestBindingCacheBoundsAndConcurrentReuse(t *testing.T) {
	cache := &bindingCache{}
	for i := 0; i < bindCacheEntries*2; i++ {
		query := fmt.Sprintf("SELECT ? /* %d %s */", i, strings.Repeat("x", 8000))
		if _, err := cache.bind(query, []driver.NamedValue{{Value: int64(i)}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(cache.items) > bindCacheEntries || cache.bytes > bindCacheBytes || cache.order.Len() != len(cache.items) {
		t.Fatal("unbounded SQL grammar cache", len(cache.items), cache.bytes)
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for i := range 100 {
				args := []driver.NamedValue{{Value: i%2 == 0}}
				if got, err := cache.bind("SELECT ?", args); err != nil || got != "SELECT $1::bigint" || args[0].Value != int64(1-i%2) {
					t.Error("concurrent boolean binding changed", got, args, err)
				}
			}
		})
	}
	workers.Wait()
}
