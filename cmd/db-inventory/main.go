// db-inventory captures the persistence contract without opening a production writer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/dbinventory"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

func main() {
	output := flag.String("output", "docs/plans/postgresql", "inventory destination")
	source := flag.String("source", "", "optional SQLite database, opened read-only")
	flag.Parse()
	if err := run(*output, *source); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output, source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if source != "" {
		return dbinventory.Capture(ctx, source, output)
	}
	directory, err := os.MkdirTemp("", "vansail-schema-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory) // Only the newly created, private fixture directory.
	path := filepath.Join(directory, "fixture.db")
	db, err := database.Open(ctx, path)
	if err != nil {
		return err
	}
	vault, err := secret.NewVault(make([]byte, 32))
	if err == nil {
		err = database.NewStore(db).MigrateTelegramIdentities(ctx, vault)
	}
	closeErr := db.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return dbinventory.Capture(ctx, path, output)
}
