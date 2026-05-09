package secrets

import (
	"crypto/rand"
	"errors"
	"testing"
)

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock rand error")
}

func TestGenerateKeyError(t *testing.T) {
	originalReader := rand.Reader
	defer func() { rand.Reader = originalReader }()

	rand.Reader = &errorReader{}

	_, err := GenerateKey()
	if err == nil {
		t.Error("Expected error from GenerateKey")
	}
}

func TestEncryptDecryptCipherErrors(t *testing.T) {
	invalidKey := []byte("invalid-key-len") // 15 bytes
	_, err := Encrypt([]byte("data"), invalidKey)
	if err == nil {
		t.Error("Expected error in Encrypt with invalid key")
	}

	_, err = Decrypt([]byte("data"), invalidKey)
	if err == nil {
		t.Error("Expected error in Decrypt with invalid key")
	}
}

func TestEncryptRandError(t *testing.T) {
	originalReader := rand.Reader
	defer func() { rand.Reader = originalReader }()

	key := make([]byte, 32) // valid key
	rand.Reader = &errorReader{}

	_, err := Encrypt([]byte("data"), key)
	if err == nil {
		t.Error("Expected error in Encrypt with bad rand")
	}
}

func TestDecryptFailOpen(t *testing.T) {
	key, _ := GenerateKey()
	ciphertext, _ := Encrypt([]byte("secret"), key)

	// Corrupt ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xff

	_, err := Decrypt(ciphertext, key)
	if err == nil {
		t.Error("Expected decryption to fail")
	}
}
