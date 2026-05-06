package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/webmalex/ch_watch/internal/model"
)

func NormalizePath(path string) string {
	return filepath.Clean(path)
}

func IsSQLFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".sql")
}

func IsWithinRoot(root, path string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func SnapshotFile(root, path string, now time.Time) (model.FileFingerprint, bool, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return model.FileFingerprint{}, false, fmt.Errorf("normalize root: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return model.FileFingerprint{}, false, fmt.Errorf("normalize path: %w", err)
	}
	if !IsWithinRoot(absRoot, absPath) {
		return model.FileFingerprint{}, false, nil
	}
	if !IsSQLFile(absPath) {
		return model.FileFingerprint{}, false, nil
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.FileFingerprint{}, false, nil
		}
		return model.FileFingerprint{}, false, err
	}
	if !info.Mode().IsRegular() {
		return model.FileFingerprint{}, false, nil
	}
	return model.FileFingerprint{
		Path:     absPath,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Recorded: now,
	}, true, nil
}
