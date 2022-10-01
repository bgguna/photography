package main

import (
  "os"
  "log"
)

func init() {
  log.SetOutput(os.Stdout)
}

func main() {
  log.Println("Server application started...")
}
