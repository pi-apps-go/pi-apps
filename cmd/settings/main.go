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

// Module: main.go
// Description: Main entry point for the Pi-Apps Settings application

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/pi-apps-go/pi-apps/pkg/api"
	"github.com/pi-apps-go/pi-apps/pkg/settings"
)

func main() {
	// runtime crashes can happen (keep in mind Pi-Apps Go is ALPHA software)
	// so add a handler to log those runtime errors to save them to a log file
	// this option can be disabled by specifying DISABLE_ERROR_HANDLING to true
	// Edit: nevermind, cgo crashes are not handled by this handler

	logger := log.New(os.Stderr, "pi-apps-settings: ", log.LstdFlags)
	errorHandling := os.Getenv("DISABLE_ERROR_HANDLING")
	if errorHandling != "true" {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace as a string
				buf := make([]byte, 1024*1024)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				logger.Printf("Panic recovered: %v", r)

				// Format the full crash report
				crashReport := fmt.Sprintf(
					"Pi-Apps Go has encountered a error and had to shutdown.\n\nReason: %v\n\nStack trace:\n%s",
					r,
					stackTrace,
				)

				// Display the error to the user
				api.ErrorNoExit(crashReport)

				// later put a function to write it to the log file in the logs folder
				os.Exit(1)
			}
		}()
	}
	api.Init()
	if err := settings.Main(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
