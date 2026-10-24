package backup

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ConfigurePostgres selects logical exports and disables online file replacement.
// Call only during composition, before any backup request or worker starts.
func (s *Service) ConfigurePostgres(connectionURL string) { s.postgresURL = connectionURL }

func (s *Service) exportPostgres(ctx context.Context, temporary, final string) error {
	parsed, err := pgx.ParseConfig(s.postgresURL)
	if err != nil {
		return fmt.Errorf("invalid PostgreSQL backup configuration")
	}
	u, err := url.Parse(s.postgresURL)
	if err != nil {
		return fmt.Errorf("invalid PostgreSQL backup URL")
	}
	environment := make([]string, 0, len(os.Environ())+10)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "PG") {
			environment = append(environment, item)
		}
	}
	environment = append(environment, "PGHOST="+parsed.Host, "PGPORT="+strconv.Itoa(int(parsed.Port)), "PGDATABASE="+parsed.Database, "PGUSER="+parsed.User, "PGPASSWORD="+parsed.Password, "PGCONNECT_TIMEOUT=10")
	for _, option := range []string{"sslmode", "sslrootcert", "sslcert", "sslkey"} {
		if value := u.Query().Get(option); value != "" {
			environment = append(environment, "PG"+strings.ToUpper(option)+"="+value)
		}
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	dump := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-acl", "--no-password", "--exclude-extension=pg_stat_statements")
	dump.Env = environment
	dump.Stdout = file
	dump.Stderr = io.Discard
	err = dump.Run()
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		os.Remove(temporary)
		return fmt.Errorf("PostgreSQL logical export failed")
	}
	verify := exec.CommandContext(ctx, "pg_restore", "--list", temporary)
	verify.Stdout = io.Discard
	verify.Stderr = io.Discard
	if err = verify.Run(); err != nil {
		os.Remove(temporary)
		return fmt.Errorf("PostgreSQL archive verification failed")
	}
	return os.Rename(temporary, final)
}
