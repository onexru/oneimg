package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitialAdminPassword must be called only after the database confirms it has
// zero users. Explicit operator values remain unchanged; otherwise persist a
// cryptographically random bootstrap password, never serialize/log its value.
// Existing account passwords are never reset by configuration loading.
func InitialAdminPassword(cfg *Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("missing initial admin configuration")
	}
	if cfg.DefaultPass != "" {
		return cfg.DefaultPass, nil
	}
	return initialAdminPasswordFile("./data/.initial_admin_password")
}
func initialAdminPasswordFile(path string) (string, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("bootstrap password must be a regular file")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read bootstrap password: %w", err)
		}
		secret := strings.TrimSuffix(string(data), "\n")
		if len(secret) < 32 || len(secret) > 72 || strings.ContainsAny(secret, "\r\n\t ") {
			return "", fmt.Errorf("invalid stored bootstrap password; refusing to replace")
		}
		if err := os.Chmod(path, 0600); err != nil {
			return "", fmt.Errorf("protect bootstrap password: %w", err)
		}
		return secret, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect bootstrap password: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("create bootstrap password directory: %w", err)
	}
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate bootstrap password: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(data)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create bootstrap password without overwriting: %w", err)
	}
	_, writeErr := file.WriteString(secret + "\n")
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return "", fmt.Errorf("persist bootstrap password: %w", writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close bootstrap password: %w", closeErr)
	}
	return secret, nil
}
