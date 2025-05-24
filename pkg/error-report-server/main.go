// Copyright (C) 2025 pi-apps-go contributors
// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
// Check COPYING for more information about the license.

// Module: error-report-server/main.go
// Description: Main entry point for the error report server.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/botspot/pi-apps/pkg/error-report-server/server"
)

func main() {
	// Parse command line flags
	addr := flag.String("addr", ":8080", "Address to listen on")
	flag.Parse()

	// Create and start the server
	server := server.NewServer("")

	// Start the token cleanup goroutine
	go server.CleanupExpiredTokens()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Starting error report server on %s", *addr)
		if err := server.Start(*addr); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down server...")
}
