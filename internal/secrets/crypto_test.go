package secrets

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key))
	}
}

func TestEncryptionDecryption(t *testing.T) {
	key, _ := GenerateKey()
	plaintext := []byte("secret message")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted message does not match plaintext")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	plaintext := []byte("secret message")

	ciphertext, _ := Encrypt(plaintext, key1)

	_, err := Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Expected error when decrypting with wrong key, got nil")
	}
}

func TestKeySaveLoad(t *testing.T) {
	key, _ := GenerateKey()
	tmpFile, err := os.CreateTemp("", "recac-key-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if err := SaveKey(tmpFile.Name(), key); err != nil {
		t.Fatalf("SaveKey failed: %v", err)
	}

	loadedKey, err := LoadKey(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadKey failed: %v", err)
	}

	if hex.EncodeToString(key) != hex.EncodeToString(loadedKey) {
		t.Error("Loaded key does not match saved key")
	}
}
