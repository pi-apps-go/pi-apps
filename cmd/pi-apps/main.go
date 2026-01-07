// Copyright (C) 2026 pi-apps-go contributors
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

// Description: Main entry point for the Pi-Apps terminal plugin implementation
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pi-apps-go/pi-apps/pkg/api"
	"github.com/pi-apps-go/pi-apps/pkg/builder"
)

func main() {
	// Parse command line flags
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	helpFlag := flag.Bool("help", false, "Show help message")
	versionFlag := flag.Bool("version", false, "Show version information")
	logoFlag := flag.Bool("logo", false, "Display the Pi-Apps logo")
	flag.Parse()

	// Set debug mode if specified
	if *debugFlag {
		api.SetDebugMode(true)
	}

	// Initialize plugins
	if err := builder.InitializePlugins(); err != nil {
		fmt.Printf("Error initializing plugins: %v\n", err)
		os.Exit(1)
	}

	// Handle help flag
	if *helpFlag {
		printUsage()
		return
	}

	// Handle version flag
	if *versionFlag {
		fmt.Println("Pi-Apps Go Edition v0.1.0")
		return
	}

	// Handle logo flag
	if *logoFlag {
		fmt.Print(api.GenerateLogo())
		return
	}

	// If no arguments were provided, start the Pi-Apps GUI as usual
	if flag.NArg() == 0 {
		// obtain the Pi-Apps directory
		api.Init()
		piAppsDir := api.GetPiAppsDir()
		if piAppsDir == "" {
			fmt.Println("Error: PI_APPS_DIR environment variable is not set")
			os.Exit(1)
		}

		guiPath := piAppsDir + "/gui"

		// Check if gui-demo exists
		if _, err := os.Stat(guiPath); os.IsNotExist(err) {
			fmt.Printf("Error: GUI executable not found at %s\n", guiPath)
			os.Exit(1)
		}

		cmd := exec.Command(guiPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error launching GUI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Get the command and arguments
	command := flag.Arg(0)
	args := flag.Args()[1:]

	// Execute the requested command
	switch strings.ToLower(command) {
	case "install":
		// Handle installation
		if len(args) < 1 {
			fmt.Println("Error: No app specified for installation")
			fmt.Println("Usage: pi-apps install <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		if err := api.InstallPackages(appName); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "uninstall", "remove":
		// Handle uninstallation
		if len(args) < 1 {
			fmt.Println("Error: No app specified for removal")
			fmt.Println("Usage: pi-apps uninstall/remove <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		if err := api.PurgePackages(appName, false); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "update":
		// Handle update
		if err := api.AptUpdate(); err != nil {
			fmt.Printf("Error updating package lists: %v\n", err)
			os.Exit(1)
		}

	case "upgrade":
		// Handle upgrade
		if err := api.AptUpdate("--upgrade"); err != nil {
			fmt.Printf("Error upgrading packages: %v\n", err)
			os.Exit(1)
		}

	case "list":
		// List installed Pi-Apps apps (default behavior)
		api.Init()
		apps, err := api.ListApps("installed")
		if err != nil {
			fmt.Printf("Error listing installed apps: %v\n", err)
			os.Exit(1)
		}
		if len(apps) == 0 {
			fmt.Println("No apps are currently installed.")
		} else {
			fmt.Printf("Installed apps (%d):\n", len(apps))
			for _, app := range apps {
				fmt.Println(app)
			}
		}
		return

	case "search":
		// Search for Pi-Apps apps
		if len(args) < 1 {
			fmt.Println("Error: No search query specified")
			fmt.Println("Usage: pi-apps search <query>")
			os.Exit(1)
		}

		api.Init()
		query := args[0]
		fmt.Printf("Searching for apps containing: %s\n", query)

		// Get all available apps
		apps, err := api.ListApps("all")
		if err != nil {
			fmt.Printf("Error listing apps: %v\n", err)
			os.Exit(1)
		}

		// Filter apps that match the search query
		var matchedApps []string
		queryLower := strings.ToLower(query)

		for _, app := range apps {
			// Search in app name and description
			if strings.Contains(strings.ToLower(app), queryLower) {
				matchedApps = append(matchedApps, app)
				continue
			}

			// Also search in description
			piAppsDir := api.GetPiAppsDir()
			descFile := piAppsDir + "/apps/" + app + "/description"
			if content, err := os.ReadFile(descFile); err == nil {
				if strings.Contains(strings.ToLower(string(content)), queryLower) {
					matchedApps = append(matchedApps, app)
				}
			}
		}

		if len(matchedApps) == 0 {
			fmt.Printf("No apps found matching '%s'\n", query)
		} else {
			fmt.Printf("Found %d apps matching '%s':\n", len(matchedApps), query)
			for _, app := range matchedApps {
				// Show app name and status
				status, _ := api.GetAppStatus(app)
				fmt.Printf("  %s (%s)\n", app, status)
			}
		}
		return

	case "show":
		// Show app details
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: pi-apps show <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		piAppsDir := api.GetPiAppsDir()

		// Check if app exists
		appDir := piAppsDir + "/apps/" + appName
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			fmt.Printf("Error: App '%s' not found\n", appName)
			os.Exit(1)
		}

		// Get app status
		status, err := api.GetAppStatus(appName)
		if err != nil {
			status = "unknown"
		}

		// Get app description
		descFile := piAppsDir + "/apps/" + appName + "/description"
		description := "Description unavailable"
		if content, err := os.ReadFile(descFile); err == nil {
			description = string(content)
		}

		// Get app website
		websiteFile := piAppsDir + "/apps/" + appName + "/website"
		website := "Not specified"
		if content, err := os.ReadFile(websiteFile); err == nil {
			if w := strings.TrimSpace(string(content)); w != "" {
				website = w
			}
		}

		// Display app details
		fmt.Printf("App: %s\n", appName)
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Website: %s\n", website)
		fmt.Printf("\nDescription:\n%s\n", description)
		return
	case "gui":
		// Start the Pi-Apps GUI
		api.Init()
		piAppsDir := api.GetPiAppsDir()
		if piAppsDir == "" {
			fmt.Println("Error: PI_APPS_DIR environment variable is not set")
			os.Exit(1)
		}
		guiPath := piAppsDir + "/gui-demo"
		cmd := exec.Command(guiPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error launching GUI: %v\n", err)
			os.Exit(1)
		}
		return
	case "list-all":
		// List all apps
		api.Init()
		apps, err := api.ListApps("all")
		if err != nil {
			fmt.Printf("Error listing apps: %v\n", err)
			os.Exit(1)
		}
		if len(apps) == 0 {
			fmt.Println("No apps available.")
		} else {
			fmt.Printf("All available apps (%d):\n", len(apps))
			for _, app := range apps {
				fmt.Println(app)
			}
		}
		return

	case "list-installed":
		// List installed apps
		api.Init()
		apps, err := api.ListApps("installed")
		if err != nil {
			fmt.Printf("Error listing installed apps: %v\n", err)
			os.Exit(1)
		}
		if len(apps) == 0 {
			fmt.Println("No apps are currently installed.")
		} else {
			fmt.Printf("Installed apps (%d):\n", len(apps))
			for _, app := range apps {
				fmt.Println(app)
			}
		}
		return

	case "list-uninstalled":
		// List uninstalled apps
		api.Init()
		apps, err := api.ListApps("uninstalled")
		if err != nil {
			fmt.Printf("Error listing uninstalled apps: %v\n", err)
			os.Exit(1)
		}
		if len(apps) == 0 {
			fmt.Println("All available apps are installed.")
		} else {
			fmt.Printf("Uninstalled apps (%d):\n", len(apps))
			for _, app := range apps {
				fmt.Println(app)
			}
		}
		return

	case "list-corrupted":
		// List corrupted apps
		api.Init()
		apps, err := api.ListApps("corrupted")
		if err != nil {
			fmt.Printf("Error listing corrupted apps: %v\n", err)
			os.Exit(1)
		}
		if len(apps) == 0 {
			fmt.Println("No corrupted apps found.")
		} else {
			fmt.Printf("Corrupted apps (%d):\n", len(apps))
			for _, app := range apps {
				fmt.Println(app)
			}
		}
		return

	case "status":
		// Show app status
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: pi-apps status <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		status, err := api.GetAppStatus(appName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s: %s\n", appName, status)
		return

	case "description", "desc":
		// Show app description
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: pi-apps description <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		piAppsDir := api.GetPiAppsDir()
		descFile := piAppsDir + "/apps/" + appName + "/description"

		if _, err := os.Stat(descFile); os.IsNotExist(err) {
			fmt.Printf("Error: App '%s' not found or has no description\n", appName)
			os.Exit(1)
		}

		content, err := os.ReadFile(descFile)
		if err != nil {
			fmt.Printf("Error reading description: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Description for %s:\n%s\n", appName, string(content))
		return

	case "website":
		// Show app website
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: pi-apps website <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		piAppsDir := api.GetPiAppsDir()
		websiteFile := piAppsDir + "/apps/" + appName + "/website"

		if _, err := os.Stat(websiteFile); os.IsNotExist(err) {
			fmt.Printf("Error: App '%s' not found or has no website specified\n", appName)
			os.Exit(1)
		}

		content, err := os.ReadFile(websiteFile)
		if err != nil {
			fmt.Printf("Error reading website: %v\n", err)
			os.Exit(1)
		}
		website := strings.TrimSpace(string(content))
		if website == "" {
			fmt.Printf("No website specified for %s\n", appName)
		} else {
			fmt.Printf("Website for %s: %s\n", appName, website)
		}
		return
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Pi-Apps Go Terminal plugin - Usage:")
	fmt.Println("  pi-apps <command> [options]")
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("  install <app>            Install an app from the Pi-Apps repository")
	fmt.Println("  uninstall/remove <app>   Uninstall an app from the Pi-Apps repository")
	fmt.Println("  update                   Update package lists")
	fmt.Println("  upgrade                  Upgrade all packages")
	fmt.Println("")
	fmt.Println("App Information:")
	fmt.Println("  list                     List installed apps")
	fmt.Println("  list-all                 List all available apps")
	fmt.Println("  list-installed           List installed apps")
	fmt.Println("  list-uninstalled         List uninstalled apps")
	fmt.Println("  list-corrupted           List corrupted apps")
	fmt.Println("  search <query>           Search for apps by name or description")
	fmt.Println("  show <app>               Show detailed app information")
	fmt.Println("  status <app>             Show app installation status")
	fmt.Println("  description/desc <app>   Show app description")
	fmt.Println("  website <app>            Show app website")
	fmt.Println("")
	fmt.Println("Interface:")
	fmt.Println("  gui                      Start the Pi-Apps GUI")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --debug                  Enable debug output")
	fmt.Println("  --help                   Show this help message")
	fmt.Println("  --version                Show version information")
	fmt.Println("  --logo                   Display the Pi-Apps logo")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  pi-apps list")
	fmt.Println("  pi-apps install Firefox")
	fmt.Println("  pi-apps status Firefox")
	fmt.Println("  pi-apps search browser")
	fmt.Println("  pi-apps show Firefox")
}
