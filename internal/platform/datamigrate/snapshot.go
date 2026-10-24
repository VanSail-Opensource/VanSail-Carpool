package datamigrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
)

// Export creates a new consistent SQLite snapshot through the native backup API.
// O_EXCL reserves the destination and prevents replacing any existing artifact.
func Export(ctx context.Context, source, destination string) (Report, error) {
	s, err := OpenSource(ctx, source)
	if err != nil {
		return Report{}, err
	}
	defer s.Close()
	before, err := Inspect(ctx, s)
	if err != nil {
		return before, err
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		return before, err
	}
	file, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return before, err
	}
	if err = file.Close(); err != nil {
		return before, err
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return before, err
	}
	defer conn.Close()
	err = conn.Raw(func(raw any) error {
		backuper, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite native backup unavailable")
		}
		backup, err := backuper.NewBackup(abs)
		if err != nil {
			return err
		}
		for {
			if err = ctx.Err(); err != nil {
				backup.Finish()
				return err
			}
			more, stepErr := backup.Step(256)
			if stepErr != nil {
				backup.Finish()
				return stepErr
			}
			if !more {
				break
			}
		}
		return backup.Finish()
	})
	if err != nil {
		return before, err
	}
	copy, err := OpenSource(ctx, abs)
	if err != nil {
		return before, err
	}
	defer copy.Close()
	after, err := Inspect(ctx, copy)
	if err != nil {
		return after, err
	}
	if before.Hash != after.Hash {
		return after, fmt.Errorf("source changed during snapshot; quiesce writers and export again")
	}
	return after, nil
}
