package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bamdadam/backend/src/server"
	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	// Get database connection string from environment or use default
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/technical_assessment?sslmode=disable"
	}

	// Initialize PostgreSQL connection
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := ":8080"
	log.Printf("Server starting on %s", addr)
	if err := server.Run(serverCtx, db, addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
