package main

import (
	"strings"
	"testing"
)

func TestChannelCredentialRotationPreservesFormat(t *testing.T) {
	const oldKey = "fake-old-encryption-key-for-test"
	const newKey = "fake-new-encryption-key-for-test"
	const prefix = "xh-credential:v1:"
	legacy, err := crypt(oldKey, "fake-provider-key", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{legacy, prefix + legacy} {
		for _, target := range []target{{Table: "channels", Column: "api_key"}, {Table: "channel_api_keys", Column: "key_encrypted"}} {
			plain, format, skip, err := decryptRotationValue(oldKey, source, target)
			if err != nil || skip || plain != "fake-provider-key" {
				t.Fatal("rotation could not decode channel credential")
			}
			if strings.HasPrefix(source, prefix) != (format == prefix) {
				t.Fatal("rotation changed the channel credential format")
			}
			encrypted, err := crypt(newKey, plain, false)
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, skip, err := decryptRotationValue(newKey, format+encrypted, target)
			if err != nil || skip || decoded != plain {
				t.Fatal("rotated channel credential is unusable")
			}
		}
	}
}

func TestChannelCredentialRotationRejectsCorruptTaggedValues(t *testing.T) {
	const key = "fake-old-encryption-key-for-test"
	const prefix = "xh-credential:v1:"
	valid, err := crypt("different-fake-encryption-key", "fake-provider-key", false)
	if err != nil {
		t.Fatal(err)
	}
	target := target{Table: "channels", Column: "api_key"}
	for _, stored := range []string{prefix, prefix + "broken", prefix + valid, "xh-credential:v2:unknown"} {
		plain, _, skip, err := decryptRotationValue(key, stored, target)
		if err == nil || skip || plain != "" {
			t.Fatal("tagged corrupt credential must stop rotation, not be skipped")
		}
	}
	if _, _, skip, err := decryptRotationValue(key, "fake-plaintext-key", target); err != nil || !skip {
		t.Fatal("legacy plaintext compatibility changed")
	}
}
