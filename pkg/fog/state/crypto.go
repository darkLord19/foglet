package state

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	masterKeySize = 32
	nonceSize     = 12
)

var (
	errInvalidCiphertext = errors.New("invalid ciphertext")
)

// loadOrCreateMasterKey loads a 32-byte key from disk or creates one with 0600 permissions.
func loadOrCreateMasterKey(path string) ([]byte, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}

	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != masterKeySize {
			return nil, fmt.Errorf("invalid key size: got %d bytes", len(key))
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	key = make([]byte, masterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return loadOrCreateMasterKey(path)
		}
		return nil, fmt.Errorf("create key file: %w", err)
	}

	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write key file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close key file: %w", err)
	}

	return key, nil
}

func encrypt(secretName string, plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte(secretName))
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decrypt(secretName string, ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) <= nonceSize {
		return nil, errInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, []byte(secretName))
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
