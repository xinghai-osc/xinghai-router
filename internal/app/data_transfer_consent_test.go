package app

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestDataTransferTermsVersionMatchesDocument(t *testing.T) {
	sum := sha256.Sum256([]byte(dataTransferTermsDocument))
	if got := hex.EncodeToString(sum[:]); got != dataTransferTermsVersion {
		t.Fatalf("terms version = %s, want document SHA-256 %s", dataTransferTermsVersion, got)
	}
}
