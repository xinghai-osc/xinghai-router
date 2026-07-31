package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// crypt encrypts or decrypts value using AES-GCM with a key derived from the
// supplied secret. It mirrors the crypt function used by the router so that
// migrated provider credentials and API keys can be decrypted at runtime.
func crypt(key, value string, decrypt bool) (string, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if decrypt {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return "", err
		}
		if len(raw) < gcm.NonceSize() {
			return "", fmt.Errorf("invalid encrypted value")
		}
		plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
		return string(plain), err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// isEncrypted reports whether value looks like an AES-GCM ciphertext produced by
// crypt: it is non-empty, only contains base64url characters, and has a length
// that is a multiple of 4 (RawURLEncoding without padding).
func isEncrypted(value string) bool {
	if value == "" {
		return false
	}
	if len(value)%4 != 0 {
		return false
	}
	for _, r := range value {
		switch r {
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
			'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
			'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
			'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
			'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '_':
			continue
		default:
			return false
		}
	}
	return true
}

// encryptIfNeeded encrypts value with key if it is not already encrypted.
func encryptIfNeeded(key, value string) (string, error) {
	if value == "" || isEncrypted(value) {
		return value, nil
	}
	return crypt(key, value, false)
}
