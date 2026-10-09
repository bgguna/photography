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
	_ = godotenv.Load("../.env")

	email := flag.String("email", "", "Admin email address")
	password := flag.String("password", "", "Admin password")
	dbPath := flag.String("db", "../db/gallery.sqlite", "Path to database")
	schemaPath := flag.String("schema", "../db/schema.sql", "Path to schema.sql")
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

	// Apply the full schema (idempotent: every statement is IF NOT EXISTS)
	schema, err := os.ReadFile(*schemaPath)
	if err != nil {
		log.Fatalf("Failed to read schema %s: %v", *schemaPath, err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		log.Fatalf("Failed to apply schema: %v", err)
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
