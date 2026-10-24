// db-migrate performs explicit, verified SQLite-to-PostgreSQL conversions.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/datamigrate"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: db-migrate inspect|export|import|verify|cutover-check --source snapshot.db [--output report.json] [--snapshot new.db]")
	}
	operation := os.Args[1]
	flags := flag.NewFlagSet("db-migrate", flag.ContinueOnError)
	sourcePath := flags.String("source", "", "verified source SQLite file (never modified)")
	output := flags.String("output", "", "new protected JSON verification report")
	snapshot := flags.String("snapshot", "", "new consistent SQLite snapshot destination")
	expectedHash := flags.String("expected-hash", "", "required exact hash for a recovery-check")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	var report datamigrate.Report
	var err error
	if operation == "export" {
		if *sourcePath == "" || *snapshot == "" {
			return fmt.Errorf("export requires --source and --snapshot")
		}
		report, err = datamigrate.Export(ctx, *sourcePath, *snapshot)
	} else {
		var source *datamigrate.Source
		if operation != "cutover-check" && operation != "recovery-check" {
			if *sourcePath == "" {
				return fmt.Errorf("--source is required")
			}
			source, err = datamigrate.OpenSource(ctx, *sourcePath)
			if err != nil {
				return err
			}
			defer source.Close()
		}
		if operation == "inspect" {
			report, err = datamigrate.Inspect(ctx, source)
		} else {
			cfg, e := config.LoadDatabase()
			if e != nil {
				return e
			}
			if cfg.Driver != "postgres" {
				return fmt.Errorf("target requires DB_DRIVER=postgres")
			}
			cfg.MigrateOnStart = false
			target, e := postgres.Open(ctx, cfg)
			if e != nil {
				return e
			}
			defer target.Close()
			switch operation {
			case "import":
				report, err = datamigrate.Import(ctx, source, cfg.URL)
			case "verify":
				key, e := base64.StdEncoding.DecodeString(os.Getenv("CONFIG_MASTER_KEY"))
				if e != nil {
					return fmt.Errorf("invalid CONFIG_MASTER_KEY")
				}
				report, err = datamigrate.Verify(ctx, source, target, key)
			case "cutover-check":
				report, err = datamigrate.CutoverCheck(ctx, target)
			case "recovery-check":
				key, e := base64.StdEncoding.DecodeString(os.Getenv("CONFIG_MASTER_KEY"))
				if e != nil || *expectedHash == "" {
					return fmt.Errorf("recovery-check requires a valid CONFIG_MASTER_KEY and --expected-hash")
				}
				report, err = datamigrate.InspectPostgres(ctx, target, key)
				if err == nil && (report.Hash != *expectedHash || report.FinancialMismatches != 0) {
					err = fmt.Errorf("recovered data differs from verification evidence")
				}
			default:
				return fmt.Errorf("unsupported migration operation")
			}
		}
	}
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if *output == "" {
		_, err = os.Stdout.Write(raw)
		return err
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
