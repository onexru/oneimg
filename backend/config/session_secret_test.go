package config

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirTemp moves the process into a fresh directory (the secret helpers resolve
// ./data relative to the working directory) and restores it afterwards.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

// The session secret must survive a process restart: issue #38 reported every
// user being logged out because the key was regenerated on each start.
func TestPersistentSessionSecretIsStable(t *testing.T) {
	chdirTemp(t)
	first := loadOrCreatePersistentSessionSecret()
	if len(first) < 32 {
		t.Fatalf("secret too short: %d chars", len(first))
	}
	second := loadOrCreatePersistentSessionSecret()
	if first != second {
		t.Fatalf("secret changed across calls\nfirst  %s\nsecond %s", first, second)
	}
	stored, err := os.ReadFile(filepath.Join("data", ".session_secret"))
	if err != nil {
		t.Fatalf("secret file missing: %v", err)
	}
	if got := string(stored); got != first+"\n" {
		t.Fatalf("unexpected file content %q", got)
	}
}

// The file must not be world readable.
func TestPersistentSessionSecretPermissions(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	chdirTemp(t)
	secret := loadOrCreatePersistentSessionSecret()
	_ = secret
	info, err := os.Stat(filepath.Join("data", ".session_secret"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("permissions %o, want 0600", perm)
	}
}

// An existing key is reused rather than replaced, so upgrades keep sessions.
func TestPersistentSessionSecretReusesExisting(t *testing.T) {
	chdirTemp(t)
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	const existing = "0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(filepath.Join("data", ".session_secret"), []byte(existing+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := loadOrCreatePersistentSessionSecret(); got != existing {
		t.Fatalf("got %q want %q", got, existing)
	}
}
