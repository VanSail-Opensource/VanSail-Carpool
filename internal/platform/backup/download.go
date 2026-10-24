package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrBackupNotFound indicates that a completed, downloadable backup does not
// exist for the supplied identifier.
var ErrBackupNotFound = errors.New("backup not found")

// Download is an authenticated caller's handle to a verified stored snapshot.
// Callers must close File after streaming it.
type Download struct {
	File     *os.File
	Name     string
	Size     int64
	BackupID string
}

// OpenDownload opens a completed backup by its database identifier. Stored
// paths are treated as untrusted because the schema-aware editor can alter
// records: the resolved file must remain a regular, non-symlink member of the
// configured backup directory.
func (s *Service) OpenDownload(ctx context.Context, backupID string) (Download, error) {
	path, expectedSize, err := s.completedBackupPath(ctx, backupID)
	if err != nil {
		return Download{}, err
	}
	file, info, err := s.openStoredFile(path)
	if err != nil {
		return Download{}, err
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		_ = file.Close()
		return Download{}, fmt.Errorf("backup size no longer matches verified record")
	}
	return Download{File: file, Name: filepath.Base(path), Size: info.Size(), BackupID: backupID}, nil
}

func (s *Service) completedBackupPath(ctx context.Context, backupID string) (string, int64, error) {
	if strings.TrimSpace(backupID) == "" {
		return "", 0, ErrBackupNotFound
	}
	var path string
	var size int64
	err := s.db.QueryRowContext(ctx, `SELECT path,size_bytes FROM backup_runs WHERE id=? AND status='complete'`, backupID).Scan(&path, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrBackupNotFound
	}
	if err != nil {
		return "", 0, fmt.Errorf("lookup completed backup: %w", err)
	}
	return path, size, nil
}

func (s *Service) openStoredFile(path string) (*os.File, os.FileInfo, error) {
	cleanDirectory, err := filepath.Abs(s.directory)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve backup directory: %w", err)
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve backup path: %w", err)
	}
	relative, err := filepath.Rel(cleanDirectory, cleanPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return nil, nil, fmt.Errorf("stored backup path is outside the backup directory")
	}
	name := filepath.Base(cleanPath)
	if !strings.HasPrefix(name, "tx-carpool-") || (!strings.HasSuffix(name, ".db")&&!strings.HasSuffix(name,".dump")) {
		return nil, nil, fmt.Errorf("stored backup filename is not recognized")
	}
	linkInfo, err := os.Lstat(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrBackupNotFound
		}
		return nil, nil, fmt.Errorf("inspect stored backup: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("stored backup is not a regular file")
	}
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open stored backup: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("stat stored backup: %w", err)
	}
	if !os.SameFile(linkInfo, info) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("stored backup changed while opening")
	}
	return file, info, nil
}
