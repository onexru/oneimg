// Package securestorage provides authenticated encryption for image objects.
// The header is self-describing so plaintext and encrypted objects can coexist
// across local and remote storage while the feature switch changes over time.
package securestorage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"oneimg/backend/config"
)

var (
	fileMagic       = []byte{'O', 'N', 'E', 'I', 'M', 'G', 'E', 'N', 'C', 1}
	ErrInvalidFile  = errors.New("加密文件格式无效")
	ErrKeyNotReady  = errors.New("CONFIG_SECRET 或 SESSION_SECRET 未配置，无法使用加密存储")
	ErrNotEncrypted = errors.New("文件未加密")
)

// Encode returns data unchanged when encryption is disabled, otherwise it
// returns a versioned AES-256-GCM payload.
func Encode(data []byte, enabled bool) ([]byte, error) {
	if !enabled {
		return data, nil
	}
	return Encrypt(data)
}

// Decode transparently decrypts an encrypted payload and leaves legacy
// plaintext payloads unchanged. The boolean result reports whether data was
// encrypted in storage.
func Decode(data []byte) ([]byte, bool, error) {
	if !IsEncrypted(data) {
		return data, false, nil
	}
	plaintext, err := Decrypt(data)
	if err != nil {
		return nil, true, err
	}
	return plaintext, true, nil
}

// ReadAll loads and transparently decodes one stored object.
func ReadAll(reader io.Reader) ([]byte, bool, error) {
	stored, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	return Decode(stored)
}

// Encrypt encrypts data with a random nonce and authenticates the versioned
// file header as additional data.
func Encrypt(data []byte) ([]byte, error) {
	gcm, err := newGCM()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成加密随机数失败: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, data, fileMagic)
	result := make([]byte, 0, len(fileMagic)+len(nonce)+len(ciphertext))
	result = append(result, fileMagic...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// Decrypt verifies and decrypts a payload produced by Encrypt.
func Decrypt(data []byte) ([]byte, error) {
	if !IsEncrypted(data) {
		return nil, ErrNotEncrypted
	}

	gcm, err := newGCM()
	if err != nil {
		return nil, err
	}
	headerSize := len(fileMagic)
	minimumSize := headerSize + gcm.NonceSize() + gcm.Overhead()
	if len(data) < minimumSize {
		return nil, ErrInvalidFile
	}

	nonceEnd := headerSize + gcm.NonceSize()
	plaintext, err := gcm.Open(nil, data[headerSize:nonceEnd], data[nonceEnd:], fileMagic)
	if err != nil {
		return nil, fmt.Errorf("解密文件失败，密钥不匹配或文件已损坏: %w", err)
	}
	return plaintext, nil
}

// IsEncrypted reports whether data starts with the current encrypted-file
// format marker.
func IsEncrypted(data []byte) bool {
	return len(data) >= len(fileMagic) && bytes.Equal(data[:len(fileMagic)], fileMagic)
}

// WriteFile stores data as plaintext or encrypted bytes according to enabled.
// Encrypted files are created with owner-only permissions.
func WriteFile(path string, data []byte, enabled bool) error {
	stored, err := Encode(data, enabled)
	if err != nil {
		return err
	}

	permission := os.FileMode(0644)
	if enabled {
		permission = 0600
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, permission)
	if err != nil {
		return err
	}
	if enabled {
		if err := file.Chmod(permission); err != nil {
			_ = file.Close()
			return err
		}
	}
	if _, err := file.Write(stored); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// ReadFile reads a local file and transparently decrypts it when necessary.
// The boolean result indicates whether the on-disk representation was
// encrypted.
func ReadFile(path string) ([]byte, bool, error) {
	stored, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return Decode(stored)
}

// IsEncryptedFile checks only the file header and does not load the full file.
func IsEncryptedFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	header := make([]byte, len(fileMagic))
	if _, err := io.ReadFull(file, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(header, fileMagic), nil
}

// PlaintextSize returns the logical size of a file without decrypting its
// content. This keeps storage accounting stable when encryption is enabled.
func PlaintextSize(path string) (int64, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false, err
	}
	encrypted, err := IsEncryptedFile(path)
	if err != nil || !encrypted {
		return info.Size(), encrypted, err
	}

	gcm, err := newGCM()
	if err != nil {
		return 0, true, err
	}
	overhead := int64(len(fileMagic) + gcm.NonceSize() + gcm.Overhead())
	if info.Size() < overhead {
		return 0, true, ErrInvalidFile
	}
	return info.Size() - overhead, true, nil
}

func newGCM() (cipher.AEAD, error) {
	key, err := encryptionKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptionKey() ([]byte, error) {
	if config.App == nil {
		return nil, ErrKeyNotReady
	}
	secret := strings.TrimSpace(config.App.ConfigSecret)
	if secret == "" {
		secret = strings.TrimSpace(config.App.SessionSecret)
	}
	if secret == "" {
		return nil, ErrKeyNotReady
	}

	// Domain separation prevents file encryption from reusing the exact key
	// material used by encrypted configuration fields.
	sum := sha256.Sum256([]byte("oneimg:file-storage:v1:" + secret))
	return sum[:], nil
}
