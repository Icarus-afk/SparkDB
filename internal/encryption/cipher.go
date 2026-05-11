package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

type Cipher struct {
	key  []byte
	aead cipher.AEAD
}

func NewCipher(key []byte) (*Cipher, error) {
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, fmt.Errorf("invalid key size %d: must be 16, 24, or 32 bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return &Cipher{key: key, aead: aead}, nil
}

func NewCipherFromHex(hexKey string) (*Cipher, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}
	return NewCipher(key)
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return key, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

func (c *Cipher) EncryptFile(path string) error {
	plaintext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := os.WriteFile(path, ciphertext, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (c *Cipher) DecryptFile(path string) error {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if err := os.WriteFile(path, plaintext, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func (c *Cipher) EncryptCopy(src, dst string) error {
	plaintext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := os.WriteFile(dst, ciphertext, 0644); err != nil {
		return fmt.Errorf("write destination: %w", err)
	}

	return nil
}

func (c *Cipher) DecryptCopy(src, dst string) error {
	ciphertext, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	plaintext, err := c.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	if err := os.WriteFile(dst, plaintext, 0644); err != nil {
		return fmt.Errorf("write destination: %w", err)
	}

	return nil
}

func (c *Cipher) EncryptReader(r io.Reader) ([]byte, error) {
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return c.Encrypt(plaintext)
}

func GetCipher(key, keyFile string) (*Cipher, error) {
	if key == "" && keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		key = string(data)
	}
	if key == "" {
		key = os.Getenv("SPARKDB_ENCRYPTION_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("no encryption key found")
	}
	return NewCipherFromHex(key)
}
