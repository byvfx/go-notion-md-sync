package main

import (
	"log"
	"os"

	"github.com/byvfx/go-notion-md-sync/pkg/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}