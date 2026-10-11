package main

import (
	"fmt"
	"os"

	"github.com/qamati/IOT_V2/pkg/config"
	"github.com/qamati/IOT_V2/pkg/db"
)

type AppConfig struct {
	Database db.Config `koanf:"database"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Println("iotpctl version 0.1.0")
	case "status":
		fmt.Println("iotp cluster status: ready")
	case "migrate":
		handleMigrate()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func handleMigrate() {
	action := "up"
	if len(os.Args) >= 3 {
		action = os.Args[2]
	}

	cfg, err := config.Load(config.Options[AppConfig]{
		Default: AppConfig{
			Database: db.Config{
				Host:     "127.0.0.1",
				Port:     5432,
				Database: "iotp",
				User:     "iotp",
				Password: "iotp_secret",
				SSLMode:  "disable",
			},
		},
		EnvPrefix: "IOTP_",
	})
	if err != nil {
		fmt.Printf("failed to load database config: %v\n", err)
		os.Exit(1)
	}

	migrationsDir := "db/migrations"
	if len(os.Args) >= 4 {
		migrationsDir = os.Args[3]
	}

	connStr := cfg.Database.ConnString()

	switch action {
	case "up":
		fmt.Printf("running migrations up from %s...\n", migrationsDir)
		if err := db.MigrateUp(connStr, migrationsDir); err != nil {
			fmt.Printf("migration up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrations applied successfully")
	case "down":
		fmt.Printf("running migration down from %s...\n", migrationsDir)
		if err := db.MigrateDown(connStr, migrationsDir); err != nil {
			fmt.Printf("migration down failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migration rolled back successfully")
	default:
		fmt.Printf("unknown migrate action: %s (expected 'up' or 'down')\n", action)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("iotpctl — Command-line management tool for iotp")
	fmt.Println("\nUsage:")
	fmt.Println("  iotpctl <command> [arguments]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  status                 Check cluster or service health")
	fmt.Println("  version                Print the tool version")
	fmt.Println("  migrate [up|down]      Run or roll back database schema migrations")
	fmt.Println("  seed                   Seed demo tenants, devices, and dashboards")
}
