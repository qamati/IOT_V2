package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qamati/IOT_V2/pkg/db"
)

func main() {
	cfg := db.Config{
		Host:     "127.0.0.1",
		Port:     5432,
		Database: "iotp",
		User:     "iotp",
		Password: "iotp_secret",
		SSLMode:  "disable",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping failed: %v", err)
	}
	fmt.Println("Successfully connected to TimescaleDB / PostgreSQL on 127.0.0.1:5432!")

	if err := db.MigrateUp(cfg.ConnString(), "db/migrations"); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	fmt.Println("Migrations applied successfully!")

	// Query created tables
	rows, err := pool.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
	if err != nil {
		log.Fatalf("query tables failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Tables in public schema:")
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		fmt.Printf(" - %s\n", name)
	}
}
