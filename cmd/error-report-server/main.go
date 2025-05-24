// Copyright (C) 2025 pi-apps-go contributors
// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Module: cmd/error-report-server/main.go
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
