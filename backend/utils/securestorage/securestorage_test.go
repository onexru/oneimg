package securestorage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"oneimg/backend/config"
)

func TestEncryptedFileRoundTrip(t *testing.T) {
	useTestKey(t, "round-trip-key")
	path := filepath.Join(t.TempDir(), "image.webp")
	want := []byte("fake image bytes that must not be stored in plaintext")

	if err := WriteFile(path, want, true); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !IsEncrypted(stored) {
		t.Fatal("stored file is missing the encrypted format marker")
	}
	if bytes.Contains(stored, want) {
		t.Fatal("stored file contains plaintext payload")
	}

	got, encrypted, err := ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	if !encrypted || !bytes.Equal(got, want) {
		t.Fatalf("unexpected round trip: encrypted=%v got=%q", encrypted, got)
	}
	size, encrypted, err := PlaintextSize(path)
	if err != nil {
		t.Fatalf("get plaintext size: %v", err)
	}
	if !encrypted || size != int64(len(want)) {
		t.Fatalf("unexpected plaintext size: encrypted=%v size=%d", encrypted, size)
	}
}

func TestPlaintextFileCompatibility(t *testing.T) {
	useTestKey(t, "plaintext-key")
	path := filepath.Join(t.TempDir(), "legacy.png")
	want := []byte("legacy plaintext image")
	if err := WriteFile(path, want, false); err != nil {
		t.Fatalf("write plaintext file: %v", err)
	}

	got, encrypted, err := ReadFile(path)
	if err != nil {
		t.Fatalf("read plaintext file: %v", err)
	}
	if encrypted || !bytes.Equal(got, want) {
		t.Fatalf("unexpected plaintext read: encrypted=%v got=%q", encrypted, got)
	}
	size, encrypted, err := PlaintextSize(path)
	if err != nil || encrypted || size != int64(len(want)) {
		t.Fatalf("unexpected plaintext size: encrypted=%v size=%d err=%v", encrypted, size, err)
	}
}

func TestEncryptedFileRejectsTamperingAndWrongKey(t *testing.T) {
	useTestKey(t, "original-key")
	payload, err := Encrypt([]byte("authenticated image"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	tampered := append([]byte(nil), payload...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := Decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}

	config.App.ConfigSecret = "different-key"
	if _, err := Decrypt(payload); err == nil {
		t.Fatal("ciphertext was decrypted with the wrong key")
	}
}

func useTestKey(t *testing.T, secret string) {
	t.Helper()
	previous := config.App
	config.App = &config.Config{ConfigSecret: secret}
	t.Cleanup(func() {
		config.App = previous
	})
}
