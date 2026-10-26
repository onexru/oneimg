package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A canonical (single-nested) object must resolve to <root>/<rel>.
func TestResolveCanonicalLayout(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "2026", "10", "a.webp"), "canonical")
	d := NewLocal(root)
	got := mustResolve(t, d, "uploads/2026/10/a.webp")
	want := mustAbs(t, filepath.Join(root, "2026", "10", "a.webp"))
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

// Objects written by v3.5.6/v3.5.7 live at <root>/uploads/<rel> and must stay
// readable (#41 fixed the writer, this keeps the historical reader working).
func TestResolveLegacyNestedLayout(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "uploads", "2026", "10", "b.webp"), "legacy")
	d := NewLocal(root)
	got := mustResolve(t, d, "uploads/2026/10/b.webp")
	want := mustAbs(t, filepath.Join(root, "uploads", "2026", "10", "b.webp"))
	if got != want {
		t.Fatalf("legacy object not found\n got %s\nwant %s", got, want)
	}
	rc, err := d.Get(context.Background(), Ref{Key: "uploads/2026/10/b.webp"})
	if err != nil {
		t.Fatalf("Get legacy: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, len("legacy"))
	if _, err := rc.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "legacy" {
		t.Fatalf("content %q", buf)
	}
}

// Upload must write to the canonical path, never the legacy nested one.
func TestUploadWritesCanonicalLayout(t *testing.T) {
	root := t.TempDir()
	d := NewLocal(root)
	key := "uploads/2026/10/c.webp"
	if _, err := d.Upload(context.Background(), Ref{Key: key}, strings.NewReader("body"), UploadOptions{Size: 4}); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "2026", "10", "c.webp")); err != nil {
		t.Fatalf("not at canonical path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "uploads", "2026", "10", "c.webp")); err == nil {
		t.Fatalf("written to legacy nested path")
	}
}

// After fixing a load, a canonical object wins over a stale legacy duplicate.
func TestResolvePrefersCanonicalOverLegacy(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "2026", "10", "d.webp"), "new")
	write(t, filepath.Join(root, "uploads", "2026", "10", "d.webp"), "old")
	d := NewLocal(root)
	got := mustResolve(t, d, "uploads/2026/10/d.webp")
	want := mustAbs(t, filepath.Join(root, "2026", "10", "d.webp"))
	if got != want {
		t.Fatalf("canonical should win\n got %s\nwant %s", got, want)
	}
}

// Traversal and symlinks stay rejected on both probes.
func TestResolveRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	d := NewLocal(root)
	for _, key := range []string{"uploads/../etc/passwd", "etc/passwd", "uploads//abs", "uploads/a/../../b"} {
		if _, err := d.Get(context.Background(), Ref{Key: key}); err == nil {
			t.Fatalf("key %q should be rejected", key)
		}
	}
	outside := t.TempDir()
	write(t, filepath.Join(outside, "secret.webp"), "secret")
	if err := os.MkdirAll(filepath.Join(root, "uploads"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "uploads", "2026")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := d.Get(context.Background(), Ref{Key: "uploads/2026/secret.webp"}); err == nil {
		t.Fatal("symlinked legacy path must be rejected")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustResolve(t *testing.T, d Driver, key string) string {
	t.Helper()
	ld, ok := d.(*localDriver)
	if !ok {
		t.Fatalf("not a localDriver")
	}
	got, err := ld.resolve(Ref{Key: key})
	if err != nil {
		t.Fatalf("resolve %q: %v", key, err)
	}
	abs, err := filepath.Abs(got)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
