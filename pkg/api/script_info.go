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

// Module: script_info.go
// Description: Provides functions for information about apps. (analytics & app type detection)
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UserCount returns number of users for specified app
// If app is empty, returns the entire clicklist
func UserCount(app string) (string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	clicklistPath := filepath.Join(directory, "data", "clicklist")

	// Check if clicklist file is missing or older than a day
	needsUpdate := false
	fileInfo, err := os.Stat(clicklistPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, so we need to download it
			needsUpdate = true
		} else {
			return "", fmt.Errorf("error checking clicklist file: %w", err)
		}
	} else {
		// File exists, check if it's older than a day
		if time.Since(fileInfo.ModTime()).Hours() > 24 {
			needsUpdate = true
		}
	}

	// Download fresh clicklist if needed
	if needsUpdate {
		Status("Downloading latest clicklist data...")

		// Ensure the data directory exists
		dataDir := filepath.Join(directory, "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create data directory: %w", err)
		}

		// Download the clicklist file
		resp, err := http.Get("https://raw.githubusercontent.com/Botspot/pi-apps-analytics/main/clicklist")
		if err != nil {
			return "", fmt.Errorf("failed to download clicklist: %w", err)
		}
		defer resp.Body.Close()

		// Create the clicklist file
		out, err := os.Create(clicklistPath)
		if err != nil {
			return "", fmt.Errorf("failed to create clicklist file: %w", err)
		}
		defer out.Close()

		// Write the response to the file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to write clicklist file: %w", err)
		}
	}

	// Read the clicklist file
	clicklistData, err := os.ReadFile(clicklistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read clicklist file: %w", err)
	}

	clicklist := string(clicklistData)
	if clicklist == "" {
		return "", fmt.Errorf("usercount(): clicklist empty. Likely no internet connection")
	}

	// If no app specified, return the entire clicklist
	if app == "" {
		return clicklist, nil
	}

	// Parse the clicklist to find the user count for the specified app
	scanner := bufio.NewScanner(strings.NewReader(clicklist))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, " "+app) {
			// Extract the count from the beginning of the line
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], nil // Return the count
			}
		}
	}

	// No match found, return empty string
	return "", nil
}

// ScriptName returns name of install script(s) for the specified app
// Outputs: "", 'install-32', 'install-64', 'install', 'install-32 install-64'
func ScriptName(app string) (string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Ensure app is a valid app name
	appDir := filepath.Join(directory, "apps", app)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return "", fmt.Errorf("script_name: '%s' is an invalid app name.\n%s does not exist", app, appDir)
	}

	// Check which install scripts exist
	install32 := filepath.Join(appDir, "install-32")
	install64 := filepath.Join(appDir, "install-64")
	installGeneric := filepath.Join(appDir, "install")

	has32 := FileExists(install32)
	has64 := FileExists(install64)
	hasGeneric := FileExists(installGeneric)

	// Return the appropriate script name based on what exists
	if has32 && !has64 {
		return "install-32", nil
	} else if has64 && !has32 {
		return "install-64", nil
	} else if has32 && has64 {
		return "install-32 install-64", nil
	} else if hasGeneric {
		return "install", nil
	}

	// No install script found
	return "", nil
}

// ScriptNameCPU gets script name to run based on detected CPU architecture
func ScriptNameCPU(app string) (string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Ensure app is a valid app name
	allApps, err := ListApps("all")
	if err != nil {
		return "", fmt.Errorf("failed to list apps: %w", err)
	}

	found := false
	for _, a := range allApps {
		if a == app {
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("script_name_cpu: '%s' is an invalid app name", app)
	}

	// Get the current architecture using the existing function from log_diagnose.go
	arch := getArchitecture()

	// Check which script to use based on architecture
	appDir := filepath.Join(directory, "apps", app)

	// Check if architecture-specific install script exists
	if arch == "32" && FileExists(filepath.Join(appDir, "install-32")) {
		return "install-32", nil
	} else if arch == "64" && FileExists(filepath.Join(appDir, "install-64")) {
		return "install-64", nil
	} else if FileExists(filepath.Join(appDir, "install")) {
		return "install", nil
	} else if FileExists(filepath.Join(appDir, "packages")) {
		return "packages", nil
	}

	// No compatible script found
	return "", nil
}
