package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/bgguna/photography/internal/auth"
	"github.com/bgguna/photography/internal/db"
)

func main() {
	// Load .env file
	_ = godotenv.Load("../../.env")

	email := flag.String("email", "", "Admin email address")
	password := flag.String("password", "", "Admin password")
	dbPath := flag.String("db", "../../db/gallery.sqlite", "Path to database")
	flag.Parse()

	// Validate inputs
	if *email == "" || *password == "" {
		fmt.Println("Usage: createadmin -email <email> -password <password>")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create schema if needed
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);
	`
	if _, err := database.Exec(schema); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Create auth service
	as := auth.NewAuthService(database)

	// Create admin user
	user, err := as.CreateUser(*email, *password, "admin")
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	fmt.Printf("Admin user created successfully!\n")
	fmt.Printf("Email: %s\n", user.Email)
	fmt.Printf("Role: %s\n", user.Role)
	fmt.Printf("ID: %d\n", user.ID)
}
