package main

import (
	"os"
	"log"

	"github.com/joho/godotenv"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Failed to load .env file. Error:", err)
	}

	environment := os.Getenv("MODE_ENV")
	log.Println("Server application started in", environment, "mode.")
}
