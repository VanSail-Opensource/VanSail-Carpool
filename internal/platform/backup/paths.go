package backup

import (
	"fmt"
	"path/filepath"
	"strings"
)

// managedPath accepts only direct children created by this service. Checking
// only the final file with Lstat does not contain a path through a symlinked
// parent directory. Historical .db exports remain readable and deletable.
func (s *Service) managedPath(path string) (string, error) {
	directory, err := filepath.Abs(s.directory)
	if err != nil {
		return "", fmt.Errorf("resolve backup directory: %w", err)
	}
	target, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve backup path: %w", err)
	}
	if filepath.Dir(target) != directory {
		return "", fmt.Errorf("stored backup path is outside the managed backup directory")
	}
	name := filepath.Base(target)
	if !strings.HasPrefix(name, "tx-carpool-") || (!strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".dump")) {
		return "", fmt.Errorf("stored backup filename is not recognized")
	}
	return target, nil
}
