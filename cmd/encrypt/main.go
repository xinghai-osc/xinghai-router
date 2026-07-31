package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xinghai-osc/xinghai-router/internal/migrate"
)

func main() {
	var targetDSN, encryptionKey string

	flag.StringVar(&targetDSN, "to", "", "Target xinghai-router PostgreSQL DSN")
	flag.StringVar(&encryptionKey, "encryption-key", "", "Encryption key for channel/provider secrets (reads ENCRYPTION_KEY env if empty)")
	flag.Parse()

	if targetDSN == "" {
		targetDSN = os.Getenv("DATABASE_URL")
	}
	if encryptionKey == "" {
		encryptionKey = os.Getenv("ENCRYPTION_KEY")
	}
	if targetDSN == "" {
		log.Fatal("target DSN required: set --to flag or DATABASE_URL env")
	}
	if encryptionKey == "" {
		log.Fatal("encryption key required: set --encryption-key flag or ENCRYPTION_KEY env")
	}

	fmt.Println("Encrypting existing channel keys")
	if err := migrate.EncryptExistingChannelKeys(context.Background(), targetDSN, encryptionKey, nil); err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}
	fmt.Println("Encryption completed successfully")
}
