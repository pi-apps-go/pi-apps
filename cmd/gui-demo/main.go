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

// Demo program for testing the Pi-Apps GUI implementation

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/botspot/pi-apps/pkg/gui"
	"github.com/charmbracelet/log"
)

// Build-time variables
var (
	BuildDate string
	GitCommit string
)

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportCaller:    true,
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
})

func main() {
	var (
		directory = flag.String("directory", "", "Pi-Apps directory (defaults to PI_APPS_DIR env var)")
		mode      = flag.String("mode", "", "GUI mode: gtk, yad-default, xlunch-dark, etc.")
		help      = flag.Bool("help", false, "Show help message")
		version   = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *help {
		fmt.Println("Pi-Apps GUI")
		fmt.Println("Usage: gui-demo [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Environment Variables:")
		fmt.Println("  PI_APPS_DIR  Path to Pi-Apps directory")
		fmt.Println()
		fmt.Println("GUI Modes:")
		fmt.Println("  default      Auto-detect best interface (GTK3 if available, fallback to bash)")
		fmt.Println("  gtk          Native GTK3 interface")
		fmt.Println("  native       Same as gtk")
		fmt.Println("  yad-default  YAD-based interface (compatibility, deprecated)")
		fmt.Println("  xlunch-dark  XLunch dark theme")
		fmt.Println()
		return
	}

	// Handle version flag
	if *version {
		fmt.Println("Pi-Apps GUI binary runtime (rolling release)")
		if BuildDate != "" {
			api.Status(fmt.Sprintf("Built on %s", BuildDate))
		} else {
			api.ErrorNoExit("Build date not available")
		}
		if GitCommit != "" {
			api.Status(fmt.Sprintf("Git commit: %s", GitCommit))
			account, repo := api.GetGitUrl()
			if account != "" && repo != "" {
				api.Status(fmt.Sprintf("Link to commit: https://github.com/%s/%s/commit/%s", account, repo, GitCommit))
			}
		} else {
			api.ErrorNoExit("Git commit hash not available")
		}
		return
	}

	// Set default directory if not provided
	if *directory == "" {
		*directory = os.Getenv("PI_APPS_DIR")
		if *directory == "" {
			logger.Fatal("PI_APPS_DIR environment variable not set and no directory specified")
		}
	}

	// Set default mode
	if *mode == "" {
		*mode = "default"
	}

	fmt.Println(api.GenerateLogo())
	properties := logger.With("compiled-on", BuildDate, "git-commit", GitCommit, "mode", *mode)
	properties.Info("Starting Pi-Apps GUI Demo...")

	// Create GUI configuration
	config := gui.GUIConfig{
		Directory: *directory,
		GuiMode:   *mode,
	}

	// Create and initialize GUI
	app, err := gui.NewGUI(config)
	if err != nil {
		log.Fatalf("Failed to create GUI: %v", err)
	}

	if err := app.Initialize(); err != nil {
		log.Fatalf("Failed to initialize GUI: %v", err)
	}

	// Ensure cleanup on exit
	defer app.Cleanup()

	// Run the GUI
	if err := app.Run(); err != nil {
		log.Fatalf("Failed to run GUI: %v", err)
	}
}
