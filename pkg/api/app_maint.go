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

// Module: app_maint.go
// Description: Provides functions for maintaining the app list, package app status, and app icons.

package api

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/davidbyttow/govips/v2/vips"
)

// GenerateAppIcons converts the given image into icon-24.png and icon-64.png files for the specified app
//
// This implementation uses the govips library for image processing and preserves the original aspect ratio
// of the image when resizing, similar to how ImageMagick would handle it in the bash implementation
func GenerateAppIcons(iconPath, appName string) error {
	if iconPath == "" {
		return fmt.Errorf("generate_app_icons(): icon field empty")
	}
	if appName == "" {
		return fmt.Errorf("generate_app_icons(): app field empty")
	}

	// Get the PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Create the app directory if it doesn't exist
	appDir := filepath.Join(directory, "apps", appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	// Initialize govips
	vips.Startup(nil)
	defer vips.Shutdown()

	// Load the source image
	image, err := vips.NewImageFromFile(iconPath)
	if err != nil {
		return fmt.Errorf("error reading source image: %w", err)
	}
	defer image.Close()

	// Get original dimensions
	originalWidth := image.Width()
	originalHeight := image.Height()

	// Create a 24x24 icon (preserving aspect ratio)
	icon24Path := filepath.Join(appDir, "icon-24.png")

	// Clone the image for 24x24 processing
	image24, err := image.Copy()
	if err != nil {
		return fmt.Errorf("error copying image for 24x24 processing: %w", err)
	}
	defer image24.Close()

	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		err = image24.Resize(24.0/float64(originalHeight), vips.KernelLanczos3)
	} else {
		// Image is taller than wide, constrain by width
		err = image24.Resize(24.0/float64(originalWidth), vips.KernelLanczos3)
	}

	if err != nil {
		return fmt.Errorf("error resizing image to 24px: %w", err)
	}

	// Export as PNG
	image24bytes, _, err := image24.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("error exporting 24x24 icon: %w", err)
	}

	if err := os.WriteFile(icon24Path, image24bytes, 0644); err != nil {
		return fmt.Errorf("error saving 24x24 icon: %w", err)
	}

	// Create a 64x64 icon (preserving aspect ratio)
	icon64Path := filepath.Join(appDir, "icon-64.png")

	// Clone the original image for 64x64 processing
	image64, err := image.Copy()
	if err != nil {
		return fmt.Errorf("error copying image for 64x64 processing: %w", err)
	}
	defer image64.Close()

	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		err = image64.Resize(64.0/float64(originalHeight), vips.KernelLanczos3)
	} else {
		// Image is taller than wide, constrain by width
		err = image64.Resize(64.0/float64(originalWidth), vips.KernelLanczos3)
	}

	if err != nil {
		return fmt.Errorf("error resizing image to 64px: %w", err)
	}

	// Export as PNG
	image64bytes, _, err := image64.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("error exporting 64x64 icon: %w", err)
	}

	if err := os.WriteFile(icon64Path, image64bytes, 0644); err != nil {
		return fmt.Errorf("error saving 64x64 icon: %w", err)
	}

	return nil
}

// RefreshPkgAppStatus updates the status of a package-app
//
// # If a package is installed, mark the app as installed
//
// # If a package is not installed but available, mark the app as uninstalled
//
// # If a package is not available, mark the app as hidden
func RefreshPkgAppStatus(appName string, packageName string) error {
	if appName == "" {
		return fmt.Errorf("refresh_pkgapp_status(): no app specified")
	}

	// Get the PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// If packageName is not specified, get the first available package from the packages file
	if packageName == "" {
		pkgs, err := PkgAppPackagesRequired(appName)
		if err != nil {
			return fmt.Errorf("error getting required packages: %w", err)
		}

		// If pkgs is empty, this means no packages are available
		if pkgs == "" {
			// Mark the app as hidden
			Debug(fmt.Sprintf("Marking %s as hidden", appName))
			err := RunCategoryEdit(appName, "hidden")
			if err != nil {
				return fmt.Errorf("error marking app as hidden: %w", err)
			}
			return nil
		}

		// Take the first package from the list
		packageName = strings.Fields(pkgs)[0]
	}

	// Check if the package is installed
	installed := PackageInstalled(packageName)

	// Get the current status of the app
	status, err := GetAppStatus(appName)
	if err != nil {
		Debug(fmt.Sprintf("Error getting app status: %v", err))
	}

	if installed {
		// If the package is installed, mark the app as installed
		if status != "installed" {
			Debug(fmt.Sprintf("Marking %s as installed", appName))
			statusDir := filepath.Join(directory, "data", "status")
			if err := os.MkdirAll(statusDir, 0755); err != nil {
				return fmt.Errorf("error creating status directory: %w", err)
			}

			// Write "installed" to the status file
			statusFile := filepath.Join(statusDir, appName)
			if err := os.WriteFile(statusFile, []byte("installed"), 0644); err != nil {
				return fmt.Errorf("error writing status file: %w", err)
			}

			// Send analytics
			go func() {
				ShlinkLink(appName, "install")
			}()
		}
	} else {
		// The package is not installed but available
		if status != "uninstalled" {
			Debug(fmt.Sprintf("Marking %s as uninstalled", appName))
			// Remove the status file to mark it as uninstalled
			statusFile := filepath.Join(directory, "data", "status", appName)
			_ = os.Remove(statusFile) // Ignore error if file doesn't exist

			// Send analytics
			go func() {
				ShlinkLink(appName, "uninstall")
			}()
		}
	}

	// Check if the app is currently hidden but should be unhidden
	categoryOverridesFile := filepath.Join(directory, "data", "category-overrides")
	if FileExists(categoryOverridesFile) {
		// Check if app is marked as hidden in the category-overrides file
		file, err := os.Open(categoryOverridesFile)
		if err == nil {
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if line == fmt.Sprintf("%s|hidden", appName) {
					// App is hidden but should be unhidden
					Debug(fmt.Sprintf("Unhiding %s as its packages are now available", appName))

					// Get the original category from categories file
					cat, err := getOriginalCategory(directory, appName)
					if err != nil {
						Debug(fmt.Sprintf("Error getting original category: %v", err))
						cat = "Other" // Default to "Other" if there's an error
					}

					// Move app to original category
					err = RunCategoryEdit(appName, cat)
					if err != nil {
						return fmt.Errorf("error unhiding app: %w", err)
					}
					break
				}
			}
		}
	}

	return nil
}

// RunCategoryEdit sets the category for an app
func RunCategoryEdit(appName, category string) error {
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Edit the category
	err := EditAppCategory(appName, category)

	if err != nil {
		return fmt.Errorf("error running categoryedit: %w", err)
	}

	return nil
}

// getOriginalCategory gets the original category of an app from the categories file
func getOriginalCategory(directory, appName string) (string, error) {
	categoriesFile := filepath.Join(directory, "etc", "categories")
	if !FileExists(categoriesFile) {
		return "", fmt.Errorf("categories file does not exist")
	}

	file, err := os.Open(categoriesFile)
	if err != nil {
		return "", fmt.Errorf("error opening categories file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) >= 2 && parts[0] == appName {
			return parts[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning categories file: %w", err)
	}

	return "", fmt.Errorf("app not found in categories file")
}

// RefreshAppList forces regeneration of the app list
func RefreshAppList() error {
	// Get the PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get the current GUI mode
	guiModeFile := filepath.Join(directory, "data", "settings", "App List Style")
	if !FileExists(guiModeFile) {
		return fmt.Errorf("app list style setting not found")
	}

	guiMode, err := os.ReadFile(guiModeFile)
	if err != nil {
		return fmt.Errorf("error reading app list style: %w", err)
	}

	// Delete preload directory
	preloadDir := filepath.Join(directory, "data", "preload")
	err = os.RemoveAll(preloadDir)
	if err != nil {
		return fmt.Errorf("error removing preload directory: %w", err)
	}

	// Run the preload-daemon
	cmd := exec.Command(filepath.Join(directory, "etc", "preload-daemon"),
		string(guiMode), "once")
	cmd.Stdout = nil // Discard output
	cmd.Stderr = nil

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting preload-daemon: %w", err)
	}

	// We don't wait for the command to complete, as it's running in the background
	Debug("Preload daemon started to refresh app list")

	return nil
}
