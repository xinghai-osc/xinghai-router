package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
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

const channelCredentialFormat = "xh-credential:"
const channelCredentialPrefix = channelCredentialFormat + "v1:"

func channelCredentialStorage() (string, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("CHANNEL_CREDENTIAL_STORAGE")))
	if mode == "" {
		mode = "plaintext"
	}
	if mode != "plaintext" && mode != "encrypted" {
		return "", fmt.Errorf("CHANNEL_CREDENTIAL_STORAGE must be plaintext or encrypted")
	}
	return mode, nil
}

func channelCredentialValue(key, stored string) (string, error) {
	if strings.HasPrefix(stored, channelCredentialFormat) {
		if !strings.HasPrefix(stored, channelCredentialPrefix) {
			return "", fmt.Errorf("unsupported channel credential format")
		}
		plain, err := crypt(key, strings.TrimPrefix(stored, channelCredentialPrefix), true)
		if err != nil || plain == "" {
			return "", fmt.Errorf("could not decrypt channel credential")
		}
		return plain, nil
	}
	if plain, err := crypt(key, stored, true); err == nil {
		return plain, nil
	}
	return stored, nil
}

func encryptIfNeeded(key, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if len(key) < 24 {
		return "", fmt.Errorf("ENCRYPTION_KEY must contain at least 24 characters")
	}
	plain, err := channelCredentialValue(key, stored)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(stored, channelCredentialPrefix) {
		return stored, nil
	}
	encrypted, err := crypt(key, plain, false)
	if err != nil {
		return "", err
	}
	return channelCredentialPrefix + encrypted, nil
}

func importChannelCredential(key, value, mode string) (string, error) {
	switch mode {
	case "plaintext":
		if strings.HasPrefix(value, channelCredentialFormat) {
			if _, err := channelCredentialValue(key, value); err != nil {
				return "", err
			}
		}
		return value, nil
	case "encrypted":
		return encryptIfNeeded(key, value)
	default:
		return "", fmt.Errorf("invalid channel credential storage mode")
	}
}
