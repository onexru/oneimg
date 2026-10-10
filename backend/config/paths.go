package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// UploadRoot returns the configured filesystem root, not the public URL prefix.
func UploadRoot() string {
	if App != nil && strings.TrimSpace(App.UploadRoot) != "" {
		return App.UploadRoot
	}
	return "./uploads"
}

// LocalUploadPath maps stable /uploads URLs to files. Reject traversal before
// cleaning it away, and reject symlinks in existing path components. The upload
// directory must only be writable by the application (no hostile local writers).
func LocalUploadPath(publicPath string) (string, error) {
	p := strings.SplitN(strings.TrimSpace(publicPath), "?", 2)[0]
	p = strings.TrimPrefix(p, "/")
	if !strings.HasPrefix(p, "uploads/") || strings.Contains(p, "\\") || strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("not a local upload path")
	}
	p = strings.TrimPrefix(p, "uploads/")
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", fmt.Errorf("unsafe upload path")
		}
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if p == "." || filepath.IsAbs(p) {
		return "", fmt.Errorf("unsafe upload path")
	}
	root, err := filepath.Abs(UploadRoot())
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, p)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("upload path escapes root")
	}
	current := root
	// A root symlink is also rejected; parents of the root are deployment-owned.
	components := append([]string{""}, strings.Split(rel, string(filepath.Separator))...)
	for _, part := range components {
		if part != "" {
			current = filepath.Join(current, part)
		}
		info, statErr := os.Lstat(current)
		if statErr != nil && !os.IsNotExist(statErr) {
			return "", statErr
		}
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("upload path contains symlink")
		}
	}
	return full, nil
}

func positiveEnvInt(key string, fallback int) int {
	n, err := strconv.Atoi(getEnv(key, ""))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
func nonnegativeEnvInt(key string, fallback int) int {
	n, err := strconv.Atoi(getEnv(key, ""))
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
func positiveEnvDuration(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(getEnv(key, ""))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
