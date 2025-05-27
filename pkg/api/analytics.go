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

// Module: analytics.go
// Description: Provides functions for sending anonymous analytics data to the analytics webhook.

package api

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BitlyLink is a compatibility function that redirects to ShlinkLink
//
// It's maintained for backward compatibility with scripts that might use it
func BitlyLink(app, trigger string) error {
	return ShlinkLink(app, trigger)
}

// ShlinkLink sends anonymous analytics data when an app is installed or uninstalled
// to track app popularity. No personally identifiable information is sent.
func ShlinkLink(app, trigger string) error {
	// Run in a goroutine to avoid blocking the caller
	go func() {
		// Validate inputs
		if app == "" {
			Error("ShlinkLink(): requires an app argument")
			return
		}
		if trigger == "" {
			Error("ShlinkLink(): requires a trigger argument")
			return
		}

		// Check if analytics are enabled
		directory := os.Getenv("PI_APPS_DIR")
		if directory == "" {
			Error("ShlinkLink(): PI_APPS_DIR environment variable not set")
			return
		}

		settingsPath := filepath.Join(directory, "data", "settings", "Enable analytics")
		settingsData, err := os.ReadFile(settingsPath)
		if err == nil && strings.TrimSpace(string(settingsData)) == "No" {
			// Analytics are disabled
			return
		}

		// Get device information
		model, socID := getModel()
		machineID := getHashedFileContent("/etc/machine-id")
		serialNumber := getHashedFileContent("/sys/firmware/devicetree/base/serial-number")
		osName := getOSName()
		arch := getArchitecture()

		// Sanitize app name for URL
		sanitizedApp := sanitizeAppName(app)

		// Create the URL
		url := fmt.Sprintf("https://analytics.pi-apps.io/pi-apps-%s-%s/track", trigger, sanitizedApp)

		// Create the user agent string
		userAgent := fmt.Sprintf("Pi-Apps Raspberry Pi app store; %s; %s; %s; %s; %s; %s",
			model, socID, machineID, serialNumber, osName, arch)

		// Make the request
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			Debug(fmt.Sprintf("ShlinkLink: Error creating request: %v", err))
			return
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "image/gif")

		resp, err := client.Do(req)
		if err != nil {
			Debug(fmt.Sprintf("ShlinkLink: Error making request: %v", err))
			return
		}
		defer resp.Body.Close()

		// We don't need to do anything with the response
	}()

	return nil
}

// Helper functions

// getModel returns the device model and SOC_ID
//
//	model - device model
//	socID - SOC_ID
func getModel() (string, string) {
	var model, socID string

	// Read /proc/cpuinfo file
	cpuInfo, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", ""
	}

	// Convert to string and split into lines
	lines := strings.Split(string(cpuInfo), "\n")

	// Find model and hardware info
	for _, line := range lines {
		if strings.HasPrefix(line, "Model") {
			model = strings.TrimPrefix(line, "Model")
			model = strings.TrimPrefix(model, ":")
			model = strings.TrimSpace(model)
			model = strings.Trim(model, `"';`)
		} else if strings.HasPrefix(line, "Hardware") {
			socID = strings.TrimPrefix(line, "Hardware")
			socID = strings.TrimPrefix(socID, ":")
			socID = strings.TrimSpace(socID)
			socID = strings.Trim(socID, `"';`)
		}
	}

	return model, socID
}

// getHashedFileContent reads a file and returns its SHA1 hash if the file exists and has content
//
//	"" - file does not exist or has no content
//	hash - file exists and has content
func getHashedFileContent(filePath string) string {
	// Check if file exists and has content
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() == 0 {
		return ""
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	// Calculate SHA1 hash
	hash := sha1.New()
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// getOSName returns the OS name and version
func getOSName() string {
	// Read the os-release file
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}

	// Convert to string and split into lines
	lines := strings.Split(string(content), "\n")

	var osID, osVersion string

	// Parse each line
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			osID = strings.TrimPrefix(line, "ID=")
			osID = strings.Trim(osID, `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			osVersion = strings.TrimPrefix(line, "VERSION_ID=")
			osVersion = strings.Trim(osVersion, `"`)
		}
	}

	osName := fmt.Sprintf("%s %s", osID, osVersion)
	// Capitalize first letter
	if len(osName) > 0 {
		osName = strings.ToUpper(osName[:1]) + osName[1:]
	}

	return osName
}

// sanitizeAppName removes any non-alphanumeric characters from the app name
func sanitizeAppName(appName string) string {
	// Remove spaces
	noSpaces := strings.ReplaceAll(appName, " ", "")

	// Keep only alphanumeric characters
	reg := regexp.MustCompile("[^a-zA-Z0-9]")
	return reg.ReplaceAllString(noSpaces, "")
}
