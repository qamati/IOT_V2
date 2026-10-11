package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "version":
		fmt.Println("iotpctl version 0.1.0 (Stage 0 Foundation)")
	case "status":
		fmt.Println("iotp cluster status: ready")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("iotpctl — Command-line management tool for iotp")
	fmt.Println("\nUsage:")
	fmt.Println("  iotpctl <command> [arguments]")
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  status      Check cluster or service health")
	fmt.Println("  version     Print the tool version")
	fmt.Println("  migrate     Run database schema migrations")
	fmt.Println("  seed        Seed demo tenants, devices, and dashboards")
}
