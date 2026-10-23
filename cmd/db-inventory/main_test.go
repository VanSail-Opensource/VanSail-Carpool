package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/dbinventory"
)

// Hosted CI verifies deterministic metadata across independently migrated fixtures.
func TestFinalSchemaInventory(t *testing.T) {
	read := func(name string) dbinventory.Inventory {
		t.Helper()
		output := filepath.Join(t.TempDir(), name+" #%")
		if err := run(output, ""); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(output, "sqlite-inventory.json"))
		if err != nil {
			t.Fatal(err)
		}
		var inv dbinventory.Inventory
		if err = json.Unmarshal(raw, &inv); err != nil {
			t.Fatal(err)
		}
		return inv
	}
	a, b := read("first"), read("second")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("final fixture schema differs across runs")
	}
	var users bool
	for _, table := range a.Tables {
		if table.Name != "users" {
			continue
		}
		users = true
		for _, column := range table.Columns {
			if column.Name == "username" {
				t.Fatal("captured pre-startup user schema")
			}
		}
	}
	if !users || len(a.Migrations) == 0 || len(a.Objects) == 0 {
		t.Fatal("incomplete schema inventory")
	}
}
