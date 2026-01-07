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

// Module: list.go
// Description: Provides functions for listing apps.
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ListApps lists apps based on the specified filter
// Filters include: installed, uninstalled, corrupted, cpu_installable, hidden, visible,
// online, online_only, local, local_only, all, package, standard, have_status, missing_status, disabled
func ListApps(filter string) ([]string, error) {
	// Get the directory from environment variable
	directory := GetPiAppsDir()
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Default case: local apps (all local apps)
	if filter == "" || filter == "local" {
		apps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to list local apps: %w", err)
		}
		sort.Strings(apps)
		return apps, nil
	}

	switch filter {
	case "all":
		// Combined list of apps, both online and local with duplicates removed
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		// Combine and remove duplicates
		return ListUnion(localApps, onlineApps), nil

	case "installed":
		// List apps that are installed
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get installed apps: %w", err)
		}
		return installedApps, nil

	case "uninstalled":
		// List apps that are uninstalled
		uninstalledApps, err := getAppsWithStatus(directory, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get uninstalled apps: %w", err)
		}
		return uninstalledApps, nil

	case "corrupted":
		// List apps that are corrupted
		corruptedApps, err := getCorruptedApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get corrupted apps: %w", err)
		}
		return corruptedApps, nil

	case "disabled":
		// List apps that are disabled
		disabledApps, err := getAppsWithStatusContent(directory, "disabled")
		if err != nil {
			return nil, fmt.Errorf("failed to get disabled apps: %w", err)
		}
		return disabledApps, nil

	case "have_status":
		// List apps that have a status file
		statusApps, err := getAppsWithStatusFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get apps with status files: %w", err)
		}
		return statusApps, nil

	case "missing_status":
		// List apps that don't have a status file
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		statusApps, err := getAppsWithStatusFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get apps with status files: %w", err)
		}

		return ListSubtract(localApps, statusApps), nil

	case "cpu_installable":
		// List apps that can be installed on the device's OS architecture (32-bit or 64-bit)
		return getCPUInstallableApps(directory)

	case "package":
		// List apps that have a "packages" file
		return getAppsWithFile(directory, "packages")

	case "standard":
		// List apps that have scripts
		return getStandardApps(directory)

	case "hidden":
		// List apps that are in the hidden category
		hiddenApps, err := getCategoryApps(directory, "hidden")
		if err != nil {
			return nil, fmt.Errorf("failed to get hidden apps: %w", err)
		}
		return hiddenApps, nil

	case "visible":
		// List apps that are in any category but 'hidden'
		allCategories, err := readCategoryFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to read category files: %w", err)
		}

		var visibleApps []string
		for app, category := range allCategories {
			if category != "hidden" {
				visibleApps = append(visibleApps, app)
			}
		}

		// Subtract disabled apps if needed
		// disabled, err := getAppsWithStatusContent(directory, "disabled")
		// if err != nil {
		//     return nil, fmt.Errorf("failed to get disabled apps: %w", err)
		// }
		// return ListSubtract(visibleApps, disabled), nil

		sort.Strings(visibleApps)
		return visibleApps, nil

	case "online":
		// List apps that exist on the online git repo
		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}
		return onlineApps, nil

	case "online_only":
		// List apps that exist only on the git repo, and not locally
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		return ListSubtract(onlineApps, localApps), nil

	case "local_only":
		// List apps that exist only locally, and not on the git repo
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		return ListSubtract(localApps, onlineApps), nil

	default:
		// Check if the filter is a category name
		categoryApps, err := getCategoryApps(directory, filter)
		if err == nil {
			// Successfully found a category
			return categoryApps, nil
		}

		return nil, fmt.Errorf("unknown filter: %s", filter)
	}
}

// ListIntersect returns a list of items that appear in both list1 and list2 (exact matches only)
func ListIntersect(list1, list2 []string) []string {
	// Create a map from list2 for faster lookups
	list2Map := make(map[string]bool)
	for _, item := range list2 {
		list2Map[item] = true
	}

	// Find items in list1 that are also in list2
	var result []string
	for _, item := range list1 {
		if list2Map[item] {
			result = append(result, item)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListIntersectPartial returns a list of items from list1 that have a partial match in list2
func ListIntersectPartial(list1, list2 []string) []string {
	var result []string
	for _, item1 := range list1 {
		for _, item2 := range list2 {
			if strings.Contains(item1, item2) {
				result = append(result, item1)
				break
			}
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListSubtract returns a list of items from list1 that don't appear in list2 (exact matches only)
func ListSubtract(list1, list2 []string) []string {
	// Create a map from list2 for faster lookups
	list2Map := make(map[string]bool)
	for _, item := range list2 {
		list2Map[item] = true
	}

	// Find items in list1 that are not in list2
	var result []string
	for _, item := range list1 {
		if !list2Map[item] {
			result = append(result, item)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListSubtractPartial returns a list of items from list1 that don't have a partial match in list2
func ListSubtractPartial(list1, list2 []string) []string {
	var result []string
	for _, item1 := range list1 {
		hasMatch := false
		for _, item2 := range list2 {
			if strings.Contains(item1, item2) {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			result = append(result, item1)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListUnion returns a combined list with duplicates removed
func ListUnion(list1, list2 []string) []string {
	// Create a map to track seen items
	seen := make(map[string]bool)
	for _, item := range list1 {
		seen[item] = true
	}
	for _, item := range list2 {
		seen[item] = true
	}

	// Create a result list from the map keys
	var result []string
	for item := range seen {
		result = append(result, item)
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// Helper functions

// listLocalApps lists all locally available apps
func listLocalApps(directory string) ([]string, error) {
	appsDir := filepath.Join(directory, "apps")

	var apps []string
	err := filepath.WalkDir(appsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root apps directory
		if path == appsDir {
			return nil
		}

		// Only process directories directly under apps/
		if d.IsDir() && filepath.Dir(path) == appsDir {
			// Check if the directory contains an 'install' script, 'packages' file, or 'flatpak_packages' file
			// This ensures that only valid applications are listed
			installScript := filepath.Join(path, "install")
			packagesFile := filepath.Join(path, "packages")
			flatpakPackagesFile := filepath.Join(path, "flatpak_packages")

			if checkFileExists(installScript) || checkFileExists(packagesFile) || checkFileExists(flatpakPackagesFile) {
				apps = append(apps, filepath.Base(path))
			}

			// Don't descend into subdirectories
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return apps, nil
}

// listOnlineApps lists all apps available in the remote repository
func listOnlineApps(directory string) ([]string, error) {
	updateDir := filepath.Join(directory, "update", "pi-apps", "apps")

	// If update directory exists, use it
	if checkFileExists(updateDir) {
		var apps []string
		err := filepath.WalkDir(updateDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip the root apps directory
			if path == updateDir {
				return nil
			}

			// Only process directories directly under apps/
			if d.IsDir() && filepath.Dir(path) == updateDir {
				apps = append(apps, filepath.Base(path))
				// Don't descend into subdirectories
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil {
			return nil, err
		}

		return apps, nil
	}

	// For now, return local apps as a fallback
	// In a real implementation, this would fetch from the remote repository
	return listLocalApps(directory)
}

// getAppsWithStatus returns a list of apps with the specified status (installed or uninstalled)
func getAppsWithStatus(directory string, wantInstalled bool) ([]string, error) {
	// Get all local apps
	allApps, err := listLocalApps(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	// Also include deprecated apps that have status files
	deprecatedDir := filepath.Join(directory, "data", "deprecated-apps")
	if entries, err := os.ReadDir(deprecatedDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				appName := entry.Name()
				// Check if this deprecated app has a status file
				statusFile := filepath.Join(directory, "data", "status", appName)
				if _, err := os.Stat(statusFile); err == nil {
					// Add to allApps if not already present
					found := false
					for _, app := range allApps {
						if app == appName {
							found = true
							break
						}
					}
					if !found {
						allApps = append(allApps, appName)
					}
				}
			}
		}
	}

	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
	}

	var filteredApps []string
	for _, app := range allApps {
		// Check if the app has the expected status
		isInstalled := checkAppInstalled(directory, app)
		if (wantInstalled && isInstalled) || (!wantInstalled && !isInstalled) {
			filteredApps = append(filteredApps, app)
		}
	}

	sort.Strings(filteredApps)
	return filteredApps, nil
}

// getAppsWithStatusContent returns a list of apps with the specified status content
func getAppsWithStatusContent(directory string, statusContent string) ([]string, error) {
	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
		return []string{}, nil
	}

	// For each status file, check its content
	var matchingApps []string
	err := filepath.WalkDir(statusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root status directory
		if path == statusDir {
			return nil
		}

		// Only process files directly under status/
		if !d.IsDir() && filepath.Dir(path) == statusDir {
			// Read the file content
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files with errors
			}

			// Check if the content matches
			if strings.TrimSpace(string(content)) == statusContent {
				matchingApps = append(matchingApps, filepath.Base(path))
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(matchingApps)
	return matchingApps, nil
}

// getCorruptedApps returns a list of corrupted apps
func getCorruptedApps(directory string) ([]string, error) {
	// Get all local apps
	allApps, err := listLocalApps(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var corruptedApps []string
	for _, app := range allApps {
		appDir := filepath.Join(directory, "apps", app)

		// Check if install and uninstall scripts exist
		installScript := filepath.Join(appDir, "install")
		uninstallScript := filepath.Join(appDir, "uninstall")

		// App is corrupted if either script is missing
		if !checkFileExists(installScript) || !checkFileExists(uninstallScript) {
			corruptedApps = append(corruptedApps, app)
			continue
		}

		// App is corrupted if the icon is missing
		iconFile := filepath.Join(appDir, "icon.png")
		if !checkFileExists(iconFile) {
			corruptedApps = append(corruptedApps, app)
			continue
		}

		// App is corrupted if the description is missing
		descFile := filepath.Join(appDir, "description")
		if !checkFileExists(descFile) {
			corruptedApps = append(corruptedApps, app)
			continue
		}
	}

	sort.Strings(corruptedApps)
	return corruptedApps, nil
}

// getAppsWithStatusFiles returns a list of apps that have status files
func getAppsWithStatusFiles(directory string) ([]string, error) {
	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
		return []string{}, nil
	}

	var apps []string
	err := filepath.WalkDir(statusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root status directory
		if path == statusDir {
			return nil
		}

		// Only process files directly under status/
		if !d.IsDir() && filepath.Dir(path) == statusDir {
			apps = append(apps, filepath.Base(path))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(apps)
	return apps, nil
}

// shouldSkipDirectory checks if a directory should be skipped during filesystem walking
func shouldSkipDirectory(_ string, d fs.DirEntry) bool {
	if !d.IsDir() {
		return false
	}

	dirName := d.Name()

	// Skip CMake build directories and other problematic directories
	skipPatterns := []string{
		"build",
		"CMakeFiles",
		".git",
		"node_modules",
		"__pycache__",
		".cache",
		"tmp",
		"temp",
	}

	for _, pattern := range skipPatterns {
		if dirName == pattern || strings.HasPrefix(dirName, pattern) {
			return true
		}
	}

	return false
}

// getCPUInstallableApps returns a list of apps that can be installed on the current CPU
func getCPUInstallableApps(directory string) ([]string, error) {
	// Get system architecture using multiple methods for better compatibility
	arch := getSystemArchitecture()

	var appNames []string
	appPath := filepath.Join(directory, "apps")

	// Find apps with install script, install-XX script, or packages file
	err := filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip permission denied errors for problematic directories
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		// Skip problematic directories entirely
		if shouldSkipDirectory(path, d) {
			return fs.SkipDir
		}

		if !d.IsDir() {
			fileName := d.Name()
			appName := filepath.Base(filepath.Dir(path))

			// Check for different types of install possibilities:
			// 1. Generic install script (works on all architectures)
			// 2. Packages file (works on all architectures)
			// 3. Architecture-specific install script matching current architecture
			// 4. Flatpak packages file (works on all architectures that support Flatpak)
			if fileName == "install" || fileName == "packages" || fileName == "flatpak_packages" {
				// For flatpak packages, also check architecture compatibility
				if fileName == "flatpak_packages" {
					flatpakPackageContent, err := os.ReadFile(path)
					if err != nil {
						// logger.Warn(fmt.Sprintf("Failed to read flatpak_packages file for %s: %v", appName, err))
						return nil // Skip this app if we can't read the file
					}
					flatpakIDs := strings.Fields(string(flatpakPackageContent))
					allCompatible := true
					for _, id := range flatpakIDs {
						if !IsFlatpakAppCompatibleWithArch(id, arch) {
							allCompatible = false
							break
						}
					}
					if allCompatible {
						appNames = append(appNames, appName)
					}
				} else {
					appNames = append(appNames, appName)
				}
			} else if isArchitectureSpecificInstallScript(fileName, arch) {
				appNames = append(appNames, appName)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	// Remove duplicates
	uniqueApps := make(map[string]bool)
	var result []string
	for _, app := range appNames {
		if !uniqueApps[app] {
			uniqueApps[app] = true
			result = append(result, app)
		}
	}

	sort.Strings(result)
	return result, nil
}

// getSystemArchitecture returns the system architecture in Pi-Apps format
func getSystemArchitecture() string {
	// Try getconf LONG_BIT first (most reliable for 32/64 bit detection)
	cmd := exec.Command("getconf", "LONG_BIT")
	if output, err := cmd.Output(); err == nil {
		bits := strings.TrimSpace(string(output))
		switch bits {
		case "64":
			return "64"
		case "32":
			return "32"
		}
	}

	// Fallback to uname -m
	cmd = exec.Command("uname", "-m")
	if output, err := cmd.Output(); err == nil {
		arch := strings.TrimSpace(string(output))
		// Convert various architecture names to Pi-Apps format (32/64)
		switch arch {
		case "aarch64", "arm64", "x86_64", "amd64", "riscv64":
			return "64"
		case "armv7l", "armv6l", "i386", "i686", "armhf", "riscv32":
			return "32"
		}
	}

	// Final fallback - assume 64-bit as most modern systems are 64-bit
	return "64"
}

// isArchitectureSpecificInstallScript checks if a filename is an architecture-specific install script for the current arch
func isArchitectureSpecificInstallScript(fileName, currentArch string) bool {
	// Check for install-32 and install-64 naming convention
	expectedScript := fmt.Sprintf("install-%s", currentArch)
	return fileName == expectedScript
}

// getAppsWithFile returns a list of apps that have the specified file
func getAppsWithFile(directory string, fileName string) ([]string, error) {
	var appNames []string
	appPath := filepath.Join(directory, "apps")

	err := filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip permission denied errors for problematic directories
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		// Skip problematic directories entirely
		if shouldSkipDirectory(path, d) {
			return fs.SkipDir
		}

		if !d.IsDir() && d.Name() == fileName {
			// Get app name (parent directory name)
			appName := filepath.Base(filepath.Dir(path))
			appNames = append(appNames, appName)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	sort.Strings(appNames)
	return appNames, nil
}

// getStandardApps returns a list of apps that have scripts
func getStandardApps(directory string) ([]string, error) {
	var appNames []string
	appPath := filepath.Join(directory, "apps")

	err := filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip permission denied errors for problematic directories
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		// Skip problematic directories entirely
		if shouldSkipDirectory(path, d) {
			return fs.SkipDir
		}

		if !d.IsDir() {
			fileName := d.Name()
			if fileName == "install" || fileName == "install-32" || fileName == "install-64" || fileName == "uninstall" {
				// Get app name (parent directory name)
				appName := filepath.Base(filepath.Dir(path))
				appNames = append(appNames, appName)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	// Remove duplicates
	uniqueApps := make(map[string]bool)
	var result []string
	for _, app := range appNames {
		if !uniqueApps[app] {
			uniqueApps[app] = true
			result = append(result, app)
		}
	}

	sort.Strings(result)
	return result, nil
}

// getCategoryApps returns a list of apps in the specified category
func getCategoryApps(directory string, category string) ([]string, error) {
	categoryFile := filepath.Join(directory, "data", "categories", category)

	// Make sure the category file exists
	if !checkFileExists(categoryFile) {
		// For categories that don't exist yet, create a directory
		categoryDir := filepath.Join(directory, "data", "categories")
		if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
			if err := os.MkdirAll(categoryDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create categories directory: %w", err)
			}
		}

		// Create an empty category file
		if err := os.WriteFile(categoryFile, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("failed to create category file: %w", err)
		}

		// Return an empty list
		return []string{}, nil
	}

	// Read app names from category file
	data, err := os.ReadFile(categoryFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read category file: %w", err)
	}

	var apps []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		appName := scanner.Text()
		if appName != "" {
			apps = append(apps, appName)
		}
	}

	sort.Strings(apps)
	return apps, nil
}

// readCategoryFiles reads all app category assignments
func readCategoryFiles(directory string) (map[string]string, error) {
	categoryDir := filepath.Join(directory, "data", "categories")

	// Make sure the category directory exists
	if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
		// Create the categories directory
		if err := os.MkdirAll(categoryDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create categories directory: %w", err)
		}

		// Return an empty map
		return make(map[string]string), nil
	}

	// Read all category files
	entries, err := os.ReadDir(categoryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read category directory: %w", err)
	}

	result := make(map[string]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			categoryFilePath := filepath.Join(categoryDir, entry.Name())
			categoryName := entry.Name()

			// Read app names from category file
			data, err := os.ReadFile(categoryFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read category file %s: %w", categoryName, err)
			}

			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				appName := scanner.Text()
				if appName != "" {
					result[appName] = categoryName
				}
			}
		}
	}

	return result, nil
}

// checkAppInstalled checks if an app is installed
func checkAppInstalled(directory, app string) bool {
	statusFile := filepath.Join(directory, "data", "status", app)
	return checkFileExists(statusFile)
}

// checkFileExists checks if a file exists
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadCategoryFiles generates a combined categories-list from several sources:
// category-overrides, device-specific overrides, global categories file, and unlisted apps. Format: "app|category"
func ReadCategoryFiles(directory string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	// First, clean up category-overrides file by removing apps that no longer exist
	// (matching bash behavior: remove app category if app folder not found)
	userOverridesFile := filepath.Join(directory, "data", "category-overrides")
	if checkFileExists(userOverridesFile) {
		data, err := os.ReadFile(userOverridesFile)
		if err == nil {
			var validLines []string
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					validLines = append(validLines, line)
					continue
				}

				parts := strings.Split(line, "|")
				if len(parts) >= 1 {
					appName := strings.TrimSpace(parts[0])
					appDir := filepath.Join(directory, "apps", appName)
					// Only keep the line if the app directory exists
					if checkFileExists(appDir) {
						validLines = append(validLines, line)
					}
				}
			}
			// Write back the cleaned file if any lines were removed
			if len(validLines) != len(strings.Split(string(data), "\n")) {
				os.WriteFile(userOverridesFile, []byte(strings.Join(validLines, "\n")+"\n"), 0644)
			}
		}
	}

	// First read category-overrides file (user overrides take precedence)
	if checkFileExists(userOverridesFile) {
		data, err := os.ReadFile(userOverridesFile)
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				parts := strings.Split(line, "|")
				if len(parts) >= 2 {
					appName := strings.TrimSpace(parts[0])
					categoryName := strings.TrimSpace(parts[1])
					if appName != "" && !seen[appName] {
						result = append(result, appName+"|"+categoryName)
						seen[appName] = true
					}
				}
			}
		}
	}

	// Then read device-specific category overrides (from embedded structured data)
	for _, assignment := range getDeviceCategoryOverrides() {
		if assignment.AppName != "" && !seen[assignment.AppName] {
			result = append(result, assignment.AppName+"|"+assignment.Category)
			seen[assignment.AppName] = true
		}
	}

	// Then read global categories (from embedded structured data)
	for _, assignment := range embeddedGlobalCategories {
		if assignment.AppName != "" && !seen[assignment.AppName] {
			result = append(result, assignment.AppName+"|"+assignment.Category)
			seen[assignment.AppName] = true
		}
	}

	// Also read individual category files from data/categories directory (compatibility)
	categoriesDir := filepath.Join(directory, "data", "categories")
	if checkFileExists(categoriesDir) {
		entries, err := os.ReadDir(categoriesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				categoryName := entry.Name()
				categoryFile := filepath.Join(categoriesDir, categoryName)

				// Read the apps in this category
				data, err := os.ReadFile(categoryFile)
				if err != nil {
					continue
				}

				scanner := bufio.NewScanner(strings.NewReader(string(data)))
				for scanner.Scan() {
					appName := strings.TrimSpace(scanner.Text())
					if appName == "" {
						continue
					}

					if !seen[appName] {
						result = append(result, appName+"|"+categoryName)
						seen[appName] = true
					}
				}
			}
		}
	}

	// Add all local apps that don't have a category yet
	localApps, err := listLocalApps(directory)
	if err == nil {
		for _, app := range localApps {
			if !seen[app] {
				result = append(result, app+"|")
				seen[app] = true
			}
		}
	}

	return result, nil
}

// AppPrefixCategory lists all apps in a category with format "category/app",
// or if category is left blank, then list the full structure of all categories
func AppPrefixCategory(directory, category string) ([]string, error) {
	var result []string

	// Get the "Show apps" setting
	showAppsSetting := ""
	settingsFile := filepath.Join(directory, "data", "settings", "Show apps")
	if checkFileExists(settingsFile) {
		data, err := os.ReadFile(settingsFile)
		if err == nil {
			showAppsSetting = strings.TrimSpace(string(data))
		}
	}

	// Prepare filter function based on settings
	filterApps := func(apps []string) ([]string, error) {
		switch showAppsSetting {
		case "standard":
			// If only showing standard apps, hide package apps
			packageApps, err := getAppsWithFile(directory, "packages")
			if err != nil {
				return nil, err
			}

			// Format package apps with wildcard prefix for partial matching
			var formattedPackageApps []string
			for _, app := range packageApps {
				formattedPackageApps = append(formattedPackageApps, ".*/"+app)
			}

			return ListSubtractPartial(apps, formattedPackageApps), nil
		case "packages":
			// If only showing package apps, hide standard apps
			standardApps, err := getStandardApps(directory)
			if err != nil {
				return nil, err
			}

			// Format standard apps with wildcard prefix for partial matching
			var formattedStandardApps []string
			for _, app := range standardApps {
				formattedStandardApps = append(formattedStandardApps, ".*/"+app)
			}

			return ListSubtractPartial(apps, formattedStandardApps), nil
		default:
			// Default case: don't filter
			return apps, nil
		}
	}

	// Get hidden apps
	hiddenApps, err := getCategoryApps(directory, "hidden")
	if err != nil {
		return nil, err
	}

	switch category {
	case "Deprecated":
		// Show special "Deprecated" category
		deprecatedDir := filepath.Join(directory, "data", "deprecated-apps")
		var deprecatedApps []string
		if entries, err := os.ReadDir(deprecatedDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// Check if it has a metadata file (confirms it's a deprecated app)
					metadataFile := filepath.Join(deprecatedDir, entry.Name(), "metadata")
					if _, err := os.Stat(metadataFile); err == nil {
						deprecatedApps = append(deprecatedApps, entry.Name())
					}
				}
			}
		}

		// Format deprecated apps with category prefix
		for _, app := range deprecatedApps {
			result = append(result, "Deprecated/"+app)
		}
	case "Installed":
		// Show special "Installed" category - don't filter it
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(installedApps, hiddenApps)

		// Format installed apps with category prefix
		for _, app := range filteredApps {
			result = append(result, "Installed/"+app)
		}
	case "Packages":
		// Show special "Packages" category
		packageApps, err := getAppsWithFile(directory, "packages")
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(packageApps, hiddenApps)

		// Format package apps with category prefix
		for _, app := range filteredApps {
			result = append(result, "Packages/"+app)
		}
	case "All Apps":
		// Show special "All Apps" category
		cpuInstallableApps, err := getCPUInstallableApps(directory)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(cpuInstallableApps, hiddenApps)

		// Format apps with category prefix
		var formattedApps []string
		for _, app := range filteredApps {
			formattedApps = append(formattedApps, "All Apps/"+app)
		}

		// Apply filter based on settings
		filteredResult, err := filterApps(formattedApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredResult...)
	case "":
		// Show all categories

		// First, get regular categories
		categoryEntries, err := ReadCategoryFiles(directory)
		if err != nil {
			return nil, err
		}

		// Map of categories to apps
		categories := make(map[string][]string)
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			appName := parts[0]
			categoryName := ""
			if len(parts) > 1 {
				categoryName = parts[1]
			}

			// Skip hidden apps
			if containsApp(hiddenApps, appName) {
				continue
			}

			if categoryName != "" && categoryName != "hidden" {
				categories[categoryName] = append(categories[categoryName], appName)
			}
		}

		// Format apps with category prefix
		var formattedCategoryApps []string
		for categoryName, apps := range categories {
			for _, appName := range apps {
				formattedCategoryApps = append(formattedCategoryApps, categoryName+"/"+appName)
			}
		}

		// Apply filter based on settings
		filteredCategoryApps, err := filterApps(formattedCategoryApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredCategoryApps...)

		// Add special "Installed" category - don't filter it
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredInstalledApps := ListSubtract(installedApps, hiddenApps)

		for _, app := range filteredInstalledApps {
			result = append(result, "Installed/"+app)
		}

		// Add special "Packages" category if not standard mode
		if showAppsSetting != "standard" {
			packageApps, err := getAppsWithFile(directory, "packages")
			if err != nil {
				return nil, err
			}

			// Filter out hidden apps
			filteredPackageApps := ListSubtract(packageApps, hiddenApps)

			for _, app := range filteredPackageApps {
				result = append(result, "Packages/"+app)
			}
		}

		// Add special "All Apps" category
		cpuInstallableApps, err := getCPUInstallableApps(directory)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredAllApps := ListSubtract(cpuInstallableApps, hiddenApps)

		// Format apps with category prefix
		var formattedAllApps []string
		for _, app := range filteredAllApps {
			formattedAllApps = append(formattedAllApps, "All Apps/"+app)
		}

		// Apply filter based on settings
		filteredAllAppsResult, err := filterApps(formattedAllApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredAllAppsResult...)

		// Add special "Deprecated" category if there are any deprecated apps
		deprecatedDir := filepath.Join(directory, "data", "deprecated-apps")
		if entries, err := os.ReadDir(deprecatedDir); err == nil {
			hasDeprecatedApps := false
			for _, entry := range entries {
				if entry.IsDir() {
					metadataFile := filepath.Join(deprecatedDir, entry.Name(), "metadata")
					if _, err := os.Stat(metadataFile); err == nil {
						hasDeprecatedApps = true
						break
					}
				}
			}
			if hasDeprecatedApps {
				// Add "Deprecated/" as a category directory
				result = append(result, "Deprecated/")
			}
		}
	default:
		// Show apps in specific category
		categoryEntries, err := ReadCategoryFiles(directory)
		if err != nil {
			return nil, err
		}

		// Get apps in the specified category
		var appsInCategory []string
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			appName := parts[0]
			categoryName := ""
			if len(parts) > 1 {
				categoryName = parts[1]
			}

			// Skip hidden apps
			if containsApp(hiddenApps, appName) {
				continue
			}

			if categoryName == category {
				appsInCategory = append(appsInCategory, appName)
			}
		}

		// Format apps with category prefix
		var formattedCategoryApps []string
		for _, appName := range appsInCategory {
			formattedCategoryApps = append(formattedCategoryApps, category+"/"+appName)
		}

		// Apply filter based on settings
		filteredResult, err := filterApps(formattedCategoryApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredResult...)
	}

	sort.Strings(result)
	return result, nil
}

// Helper function to check if a string is in a slice
func containsApp(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
