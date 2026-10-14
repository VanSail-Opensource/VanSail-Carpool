// Package main starts the VanSail Carpool service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/app"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("tx carpool stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "serve":
		return serve()
	case "healthcheck":
		return healthcheck(os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func serve() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	application, err := app.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("build application: %w", err)
	}
	defer func() {
		if closeErr := application.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close application: %w", closeErr)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return application.Run(ctx)
}

func healthcheck(args []string) error {
	set := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := set.String("url", "http://127.0.0.1:8080/healthz", "health endpoint")
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("parse healthcheck flags: %w", err)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, *url, nil)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("perform healthcheck: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned %s", response.Status)
	}
	return nil
}
