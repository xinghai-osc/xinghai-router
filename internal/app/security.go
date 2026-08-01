package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
func equalSecret(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

// randomID formats a v4 UUID by hand; several are minted per proxied request, and
// fmt.Sprintf with five byte-slice verbs was the most allocation-heavy step of the path.
func randomID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:]), nil
}
func randomSecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
func passwordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyPasswordHash is a fixed bcrypt hash used when no user row is found so login
// failures take similar time whether or not the email exists.
var dummyPasswordHash = func() string {
	hash, err := bcrypt.GenerateFromPassword([]byte("xinghai-login-timing-pad"), bcrypt.DefaultCost)
	if err != nil {
		return "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	}
	return string(hash)
}()
// channelKeyValue resolves a stored channel API key to its usable form. Channel
// keys are stored plaintext; rows written before that change hold ciphertext and
// are decrypted transparently when ENCRYPTION_KEY still matches.
func channelKeyValue(encryptionKey, stored string) (string, error) {
	if stored == "" {
		return "", errInvalid
	}
	if plain, err := crypt(encryptionKey, stored, true); err == nil {
		return plain, nil
	}
	return stored, nil
}

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
