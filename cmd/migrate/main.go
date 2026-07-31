package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/xinghai-osc/xinghai-router/internal/migrate"
)

func main() {
	var sourceDSN, sourceDriver, targetDSN, encryptionKey string

	flag.StringVar(&sourceDSN, "from", "", "Source xinghai-api database DSN")
	flag.StringVar(&sourceDriver, "driver", "mysql", "Source database driver (mysql or postgres)")
	flag.StringVar(&targetDSN, "to", "", "Target xinghai-router PostgreSQL DSN")
	flag.StringVar(&encryptionKey, "encryption-key", "", "Encryption key for channel/provider secrets (reads ENCRYPTION_KEY env if empty)")
	flag.Parse()

	if sourceDSN == "" {
		sourceDSN = os.Getenv("MIGRATE_FROM_DATABASE_URL")
	}
	if targetDSN == "" {
		targetDSN = os.Getenv("DATABASE_URL")
	}
	if encryptionKey == "" {
		encryptionKey = os.Getenv("ENCRYPTION_KEY")
	}
	if sourceDSN == "" {
		log.Fatal("source DSN required: set --from flag or MIGRATE_FROM_DATABASE_URL env")
	}
	if targetDSN == "" {
		log.Fatal("target DSN required: set --to flag or DATABASE_URL env")
	}
	if encryptionKey == "" {
		log.Fatal("encryption key required: set --encryption-key flag or ENCRYPTION_KEY env")
	}

	fmt.Printf("Migrating from %s (%s) to PostgreSQL\n", sourceDSN, sourceDriver)
	if err := migrate.Run(context.Background(), sourceDSN, sourceDriver, targetDSN, encryptionKey, nil); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Migration completed successfully")
}
