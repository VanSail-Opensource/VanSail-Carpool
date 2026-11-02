package testdatabase

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func prepareTemplate(ctx context.Context) (string, error) {
	if value := os.Getenv("POSTGRES_CONTRACT_URL"); value != "" {
		return value, nil
	}
	value := os.Getenv("POSTGRES_FIXTURE_URL")
	if value == "" {
		value = os.Getenv("POSTGRES_TEST_ADMIN_URL")
	}
	if value == "" {
		value = os.Getenv("POSTGRES_TEST_URL")
	}
	if value == "" {
		return "", fmt.Errorf("hosted PostgreSQL is required: set POSTGRES_TEST_ADMIN_URL")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	parsed.Path = "/postgres"
	admin, err := sql.Open("pgx", parsed.String())
	if err != nil {
		return "", err
	}
	defer admin.Close()
	name := databaseName()
	if _, err = admin.ExecContext(ctx, `CREATE DATABASE `+quote(name)); err != nil {
		return "", err
	}
	parsed.Path = "/" + name
	db, err := postgres.Open(ctx, configuration(parsed.String(), true))
	if err != nil {
		return "", err
	}
	if err = db.Close(); err != nil {
		return "", err
	}
	return parsed.String(), nil
}
