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
// Description: Multi-call binary that provides all Pi-Apps functionality in one executable

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// Build-time variables
var (
	BuildDate string
	GitCommit string
)

func main() {
	// runtime crashes can happen (keep in mind Pi-Apps Go is ALPHA software)
	// so add a handler to log those runtime errors to save them to a log file
	// this option can be disabled by specifying DISABLE_ERROR_HANDLING to true

	errorHandling := os.Getenv("DISABLE_ERROR_HANDLING")
	if errorHandling != "true" {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace as a string
				stackTrace := string(debug.Stack())

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

	// Initialize API
	api.Init()

	// Determine which binary we're emulating based on argv[0]
	programName := filepath.Base(os.Args[0])

	// Handle symlinks and different calling conventions
	switch strings.ToLower(programName) {
	case "api", "api-go", "pi-apps-api":
		runAPI()
	case "gui", "pi-apps-gui":
		runGUI()
	case "manage", "pi-apps-manage":
		runManage()
	case "settings", "pi-apps-settings":
		runSettings()
	case "updater", "pi-apps-updater":
		runUpdater()
	case "multi-call-pi-apps":
		// If called directly, check first argument for mode
		if len(os.Args) > 1 {
			mode := strings.ToLower(os.Args[1])
			// Set up os.Args properly for each sub-function
			// Each sub-function expects os.Args[0] to be the binary name
			originalArgs := os.Args
			switch mode {
			case "api":
				os.Args = append([]string{"api"}, originalArgs[2:]...)
				runAPI()
			case "gui":
				os.Args = append([]string{"gui"}, originalArgs[2:]...)
				runGUI()
			case "manage":
				os.Args = append([]string{"manage"}, originalArgs[2:]...)
				runManage()
			case "daemon-terminal":
				// Special case for daemon-terminal mode - pass all args to manage
				os.Args = append([]string{"manage"}, originalArgs[1:]...)
				runManage()
			case "settings":
				os.Args = append([]string{"settings"}, originalArgs[2:]...)
				runSettings()
			case "updater":
				os.Args = append([]string{"updater"}, originalArgs[2:]...)
				runUpdater()
			default:
				printUsage()
				os.Exit(1)
			}
		} else {
			printUsage()
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	// for debugging add the passed in args
	api.Status("Passed in args: " + strings.Join(os.Args, " "))
	api.Status("Pi-Apps Multi-Call Binary")
	api.Status("Usage:")
	api.Status("  multi-call-pi-apps <mode> [args...]")
	api.Status("  Or create symlinks: api, gui, manage, settings, updater")
	api.Status("")
	api.Status("Available modes:")
	api.Status("  api      - Pi-Apps API interface")
	api.Status("  gui      - Pi-Apps GUI")
	api.Status("  manage   - Pi-Apps management tool")
	api.Status("  settings - Pi-Apps settings")
	api.Status("  updater  - Pi-Apps updater")
}
