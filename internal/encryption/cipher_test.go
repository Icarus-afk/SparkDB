package encryption

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCipher_WithKey(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	hexKey := hex.EncodeToString(key)

	c, err := GetCipher(hexKey, "")
	if err != nil {
		t.Fatalf("GetCipher() with key error: %v", err)
	}
	if c == nil {
		t.Fatal("GetCipher returned nil")
	}
}

func TestGetCipher_WithKeyFile(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	hexKey := hex.EncodeToString(key)

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "enc.key")
	if err := os.WriteFile(keyFile, []byte(hexKey), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := GetCipher("", keyFile)
	if err != nil {
		t.Fatalf("GetCipher() with key file error: %v", err)
	}
	if c == nil {
		t.Fatal("GetCipher returned nil")
	}
}

func TestGetCipher_WithEnvVar(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	hexKey := hex.EncodeToString(key)

	oldVal := os.Getenv("SPARKDB_ENCRYPTION_KEY")
	os.Setenv("SPARKDB_ENCRYPTION_KEY", hexKey)
	defer os.Setenv("SPARKDB_ENCRYPTION_KEY", oldVal)

	c, err := GetCipher("", "")
	if err != nil {
		t.Fatalf("GetCipher() with env var error: %v", err)
	}
	if c == nil {
		t.Fatal("GetCipher returned nil")
	}
}

func TestGetCipher_NoKey(t *testing.T) {
	oldVal := os.Getenv("SPARKDB_ENCRYPTION_KEY")
	os.Unsetenv("SPARKDB_ENCRYPTION_KEY")
	defer os.Setenv("SPARKDB_ENCRYPTION_KEY", oldVal)

	_, err := GetCipher("", "")
	if err == nil {
		t.Fatal("expected error when no key provided")
	}
}

func TestGetCipher_KeyOverKeyFile(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	hexKey := hex.EncodeToString(key)

	wrongKey := make([]byte, 32)
	rand.Read(wrongKey)

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "enc.key")
	if err := os.WriteFile(keyFile, []byte(hex.EncodeToString(wrongKey)), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := GetCipher(hexKey, keyFile)
	if err != nil {
		t.Fatalf("GetCipher() error: %v", err)
	}

	plaintext := []byte("test data")
	ciphertext, _ := c.Encrypt(plaintext)
	decrypted, _ := c.Decrypt(ciphertext)
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("key parameter should take precedence over key file")
	}
}

func TestGetCipher_InvalidHex(t *testing.T) {
	_, err := GetCipher("not-valid-hex", "")
	if err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestGetCipher_KeyFileNotFound(t *testing.T) {
	_, err := GetCipher("", "/nonexistent/key.file")
	if err == nil {
		t.Fatal("expected error for nonexistent key file")
	}
}

func TestGetCipher_KeyFileEmpty(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "empty.key")
	if err := os.WriteFile(keyFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := GetCipher("", keyFile)
	if err == nil {
		t.Fatal("expected error for empty key file")
	}
}

func TestEncryptFile_NonExistent(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := NewCipher(key)

	err := c.EncryptFile("/nonexistent/path/file.db")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDecryptFile_NonExistent(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := NewCipher(key)

	err := c.DecryptFile("/nonexistent/path/file.db")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestEncryptCopy_NonExistentSource(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := NewCipher(key)

	err := c.EncryptCopy("/nonexistent/src.db", "/tmp/dst.db")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestDecryptCopy_NonExistentSource(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := NewCipher(key)

	err := c.DecryptCopy("/nonexistent/src.db", "/tmp/dst.db")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestEncryptReader_Empty(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, _ := NewCipher(key)

	plaintext := []byte{}
	ciphertext, err := c.EncryptReader(bytes.NewReader(plaintext))
	if err != nil {
		t.Fatalf("EncryptReader(empty) error: %v", err)
	}
	decrypted, _ := c.Decrypt(ciphertext)
	if len(decrypted) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(decrypted))
	}
}

func TestNewCipherFromHex_Invalid(t *testing.T) {
	_, err := NewCipherFromHex("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Decrypt([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}
}

func TestNewCipher_KeySizes(t *testing.T) {
	for _, size := range []int{16, 24, 32} {
		key := make([]byte, size)
		rand.Read(key)
		c, err := NewCipher(key)
		if err != nil {
			t.Fatalf("NewCipher(%d bytes) error: %v", size, err)
		}
		if c == nil {
			t.Fatal("NewCipher returned nil")
		}
	}
}

func TestNewCipher_InvalidKeySize(t *testing.T) {
	key := make([]byte, 10)
	_, err := NewCipher(key)
	if err == nil {
		t.Fatal("expected error for 10-byte key")
	}
}

func TestNewCipherFromHex(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	hexStr := hex.EncodeToString(key)
	c, err := NewCipherFromHex(hexStr)
	if err != nil {
		t.Fatalf("NewCipherFromHex() error: %v", err)
	}
	if c == nil {
		t.Fatal("NewCipherFromHex returned nil")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("hello sparkdb encryption test")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_Empty(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := c.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt(empty) error: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt(empty) error: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty result, got %d bytes", len(decrypted))
	}
}

func TestDecrypt_Tampered(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("test data")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext[len(ciphertext)-1] ^= 0xff
	_, err = c.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting tampered ciphertext")
	}
}

func TestEncryptFile_DecryptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	original := []byte("SQLite format 3\x00some database content")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := c.EncryptFile(path); err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	encrypted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, original) {
		t.Fatal("encrypted file equals original")
	}

	if err := c.DecryptFile(path); err != nil {
		t.Fatalf("DecryptFile() error: %v", err)
	}

	decrypted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, original) {
		t.Fatal("decrypted content does not match original")
	}
}

func TestEncryptCopy_DecryptCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.db")
	dst := filepath.Join(dir, "encrypted.db")
	restored := filepath.Join(dir, "restored.db")

	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	original := []byte("database content for copy test")
	if err := os.WriteFile(src, original, 0644); err != nil {
		t.Fatal(err)
	}

	if err := c.EncryptCopy(src, dst); err != nil {
		t.Fatalf("EncryptCopy() error: %v", err)
	}

	if err := c.DecryptCopy(dst, restored); err != nil {
		t.Fatalf("DecryptCopy() error: %v", err)
	}

	result, err := os.ReadFile(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, original) {
		t.Fatal("decrypt-copy mismatch")
	}
}

func TestEncryptReader(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	c, err := NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	input := bytes.NewReader([]byte("reader test data"))
	encrypted, err := c.EncryptReader(input)
	if err != nil {
		t.Fatalf("EncryptReader() error: %v", err)
	}

	decrypted, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if string(decrypted) != "reader test data" {
		t.Fatalf("got %q, want %q", decrypted, "reader test data")
	}
}
