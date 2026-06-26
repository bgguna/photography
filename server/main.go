package main

import (
	"os"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	logger "log"

	"github.com/joho/godotenv"
)

func init() {
	// log = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	err := godotenv.Load("../.env")
	if err != nil {
		logger.Fatal("Failed to load .end file. Error:", err)
	}

	environment := os.Getenv("MODE_ENV")
	if environment == "local" || environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else if environment == "production" {
		// Using standard logging for 'production'.
	} else {
		logger.Fatal("Environment", environment, "is not recognised.")
		os.Exit(1)
	}

	log.Info().Msgf("Server application started in %s mode.", environment)
}

func main() {
	log.Info().Msg("Need to add some logic here for the controller.")
}
