// db-schema manages forward-only PostgreSQL schema changes with migration credentials.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "migrate" && os.Args[1] != "verify" && os.Args[1] != "fingerprint") {
		return fmt.Errorf("usage: db-schema migrate|verify|fingerprint")
	}
	cfg, err := config.LoadDatabase()
	if err != nil {
		return err
	}
	if cfg.Driver != "postgres" {
		return fmt.Errorf("db-schema requires DB_DRIVER=postgres")
	}
	cfg.MigrateOnStart = os.Args[1] == "migrate"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	db, err := postgres.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if os.Args[1] == "fingerprint" {
		key, err := base64.StdEncoding.DecodeString(os.Getenv("CONFIG_MASTER_KEY"))
		if err != nil {
			return err
		}
		report, err := postgres.InspectRecovery(ctx, db, key)
		if err != nil {
			return err
		}
		if report.FinancialMismatches != 0 {
			return fmt.Errorf("financial recovery invariant failed")
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	return postgres.VerifySchema(ctx, db)
}
