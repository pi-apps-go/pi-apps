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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/h2non/bimg"
)

// GenerateAppIcons converts the given image into icon-24.png and icon-64.png files for the specified app
// This implementation uses the bimg library for image processing and preserves the original aspect ratio
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

	// Read the source image
	buffer, err := bimg.Read(iconPath)
	if err != nil {
		return fmt.Errorf("error reading source image: %w", err)
	}

	// Get original dimensions
	size, err := bimg.NewImage(buffer).Size()
	if err != nil {
		return fmt.Errorf("error getting source image size: %w", err)
	}

	originalWidth := size.Width
	originalHeight := size.Height

	// Create a 24x24 icon using bimg (preserving aspect ratio)
	icon24Path := filepath.Join(appDir, "icon-24.png")

	var options24 bimg.Options
	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		newWidth := int(24.0 * float64(originalWidth) / float64(originalHeight))
		options24 = bimg.Options{
			Width:   newWidth,
			Height:  24,
			Crop:    false,
			Enlarge: true,
			Type:    bimg.PNG,
		}
	} else {
		// Image is taller than wide, constrain by width
		newHeight := int(24.0 * float64(originalHeight) / float64(originalWidth))
		options24 = bimg.Options{
			Width:   24,
			Height:  newHeight,
			Crop:    false,
			Enlarge: true,
			Type:    bimg.PNG,
		}
	}

	newImage24, err := bimg.NewImage(buffer).Process(options24)
	if err != nil {
		return fmt.Errorf("error creating 24x24 icon: %w", err)
	}

	if err := bimg.Write(icon24Path, newImage24); err != nil {
		return fmt.Errorf("error saving 24x24 icon: %w", err)
	}

	// Create a 64x64 icon using bimg (preserving aspect ratio)
	icon64Path := filepath.Join(appDir, "icon-64.png")

	var options64 bimg.Options
	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		newWidth := int(64.0 * float64(originalWidth) / float64(originalHeight))
		options64 = bimg.Options{
			Width:   newWidth,
			Height:  64,
			Crop:    false,
			Enlarge: true,
			Type:    bimg.PNG,
		}
	} else {
		// Image is taller than wide, constrain by width
		newHeight := int(64.0 * float64(originalHeight) / float64(originalWidth))
		options64 = bimg.Options{
			Width:   64,
			Height:  newHeight,
			Crop:    false,
			Enlarge: true,
			Type:    bimg.PNG,
		}
	}

	newImage64, err := bimg.NewImage(buffer).Process(options64)
	if err != nil {
		return fmt.Errorf("error creating 64x64 icon: %w", err)
	}

	if err := bimg.Write(icon64Path, newImage64); err != nil {
		return fmt.Errorf("error saving 64x64 icon: %w", err)
	}

	return nil
}

// RefreshPkgAppStatus updates the status of a package-app
// If a package is installed, mark the app as installed
// If a package is not installed but available, mark the app as uninstalled
// If a package is not available, mark the app as hidden
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

// RunCategoryEdit runs the categoryedit script to set the category for an app
func RunCategoryEdit(appName, category string) error {
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Run the categoryedit script
	cmd := exec.Command(filepath.Join(directory, "etc", "categoryedit"), appName, category)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running categoryedit: %s - %w", stderr.String(), err)
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

// RefreshAllPkgAppStatus updates the status of all package-apps
func RefreshAllPkgAppStatus() error {
	// Get the PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get all package-apps
	packageApps, err := ListApps("package")
	if err != nil {
		return fmt.Errorf("error listing package apps: %w", err)
	}

	// Get system architecture
	dpkgArch, err := getDpkgArchitecture()
	if err != nil {
		return fmt.Errorf("error getting dpkg architecture: %w", err)
	}

	// Gather all packages needed by package apps
	var allPackages []string
	for _, app := range packageApps {
		packagesFile := filepath.Join(directory, "apps", app, "packages")
		if !FileExists(packagesFile) {
			continue
		}

		packages, err := readPackagesFile(packagesFile)
		if err != nil {
			Debug(fmt.Sprintf("Error reading packages for %s: %v", app, err))
			continue
		}

		allPackages = append(allPackages, packages...)
	}

	// Format packages for apt-cache policy
	var formattedPackages []string
	for _, pkg := range allPackages {
		formattedPackages = append(formattedPackages, fmt.Sprintf("%s:%s", pkg, dpkgArch))
	}

	// Get policy info for all packages at once
	aptCacheOutput, err := getAptCachePolicy(formattedPackages)
	if err != nil {
		return fmt.Errorf("error getting apt-cache policy: %w", err)
	}

	// Get dpkg status for all packages
	dpkgStatus, err := getDpkgStatus(allPackages)
	if err != nil {
		return fmt.Errorf("error getting dpkg status: %w", err)
	}

	// Use the collected data to refresh status for each package app
	for _, app := range packageApps {
		// We'll update through our own implementation rather than calling RefreshPkgAppStatus
		// to avoid re-querying package status information
		err := refreshPackageAppStatusWithCache(app, aptCacheOutput, dpkgStatus, directory)
		if err != nil {
			Debug(fmt.Sprintf("Error refreshing status for %s: %v", app, err))
		}
	}

	return nil
}

// getDpkgArchitecture gets the current system architecture from dpkg
func getDpkgArchitecture() (string, error) {
	cmd := exec.Command("dpkg", "--print-architecture")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running dpkg --print-architecture: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// readPackagesFile reads and parses packages from a packages file
func readPackagesFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading packages file: %w", err)
	}

	var packages []string
	for _, line := range strings.Split(string(data), "\n") {
		// Handle alternative packages (package1 | package2)
		alternativePackages := strings.Split(line, "|")
		for _, pkg := range alternativePackages {
			for _, p := range strings.Fields(pkg) {
				if p != "" {
					packages = append(packages, p)
				}
			}
		}
	}

	return packages, nil
}

// getAptCachePolicy runs apt-cache policy for the specified packages
func getAptCachePolicy(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	cmd := exec.Command("apt-cache", append([]string{"policy"}, packages...)...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running apt-cache policy: %w", err)
	}

	return string(output), nil
}

// getDpkgStatus gets the status of the specified packages from dpkg
func getDpkgStatus(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	// Read the dpkg status file
	statusData, err := os.ReadFile("/var/lib/dpkg/status")
	if err != nil {
		return "", fmt.Errorf("error reading dpkg status file: %w", err)
	}

	// Split the status file into package sections
	sections := strings.Split(string(statusData), "\n\n")
	var result strings.Builder

	// Create a map for quick package lookup
	packageMap := make(map[string]bool)
	for _, pkg := range packages {
		packageMap[pkg] = true
	}

	// Process each section
	for _, section := range sections {
		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		// Check if this section is for one of our packages
		if strings.HasPrefix(lines[0], "Package: ") {
			pkgName := strings.TrimPrefix(lines[0], "Package: ")
			if packageMap[pkgName] {
				// Include this section and the next 2 lines
				result.WriteString(section)
				result.WriteString("\n\n")
			}
		}
	}

	return result.String(), nil
}

// refreshPackageAppStatusWithCache refreshes a single package-app status using cached apt and dpkg data
func refreshPackageAppStatusWithCache(appName, aptCacheOutput, dpkgStatus, directory string) error {
	packagesFile := filepath.Join(directory, "apps", appName, "packages")
	if !FileExists(packagesFile) {
		return fmt.Errorf("packages file does not exist for %s", appName)
	}

	// Read the packages file to get the list of packages
	packages, err := readPackagesFile(packagesFile)
	if err != nil {
		return fmt.Errorf("error reading packages file: %w", err)
	}

	// Check if any package is installed
	var installed bool
	var availablePackage string

	for _, pkg := range packages {
		// Check if installed
		if isPackageInstalledFromStatus(pkg, dpkgStatus) {
			installed = true
			availablePackage = pkg
			break
		}

		// Check if available in repository
		if isPackageAvailableFromPolicy(pkg, aptCacheOutput) {
			availablePackage = pkg
		}
	}

	// If no package is available, mark as hidden
	if availablePackage == "" {
		// Mark the app as hidden
		Debug(fmt.Sprintf("Marking %s as hidden", appName))
		err := RunCategoryEdit(appName, "hidden")
		if err != nil {
			return fmt.Errorf("error marking app as hidden: %w", err)
		}
		return nil
	}

	// Get current app status
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

// isPackageInstalledFromStatus checks if a package is installed by looking at the dpkg status
func isPackageInstalledFromStatus(packageName, dpkgStatus string) bool {
	// Look for the package and check if it's installed
	packageSection := fmt.Sprintf("Package: %s\n", packageName)
	index := strings.Index(dpkgStatus, packageSection)
	if index == -1 {
		return false
	}

	// Check the status in the few lines after the package name
	statusSection := dpkgStatus[index : index+200] // Look at a reasonable section after the package name
	return strings.Contains(statusSection, "Status: install ok installed")
}

// isPackageAvailableFromPolicy checks if a package is available in repositories
func isPackageAvailableFromPolicy(packageName, aptCacheOutput string) bool {
	// Look for the package and check if there's a candidate
	packageSection := fmt.Sprintf("%s:", packageName)
	index := strings.Index(aptCacheOutput, packageSection)
	if index == -1 {
		return false
	}

	// Check a reasonable section after the package name for Candidate line
	sectionEnd := index + 300
	if sectionEnd > len(aptCacheOutput) {
		sectionEnd = len(aptCacheOutput)
	}
	sectionText := aptCacheOutput[index:sectionEnd]

	// Package is available if there's a Candidate line that's not "(none)"
	candidateLine := regexp.MustCompile(`(?m)^  Candidate: (.+)$`).FindStringSubmatch(sectionText)
	return len(candidateLine) > 1 && candidateLine[1] != "(none)"
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
