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

// Module: apk_repo.go
// Description: Provides functions for managing APK repositories.
// SPDX-License-Identifier: GPL-3.0-or-later
//go:build apk

package api

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"gitlab.alpinelinux.org/alpine/go/repository"
)

// AnythingInstalledFromURISuiteComponent checks if any packages from a specific repository
// are currently installed.
//
// Note: APK uses a different repository structure than APT, so suite and component
// parameters are ignored for APK (they're APT-specific).
func AnythingInstalledFromURISuiteComponent(uri, suite, component string) (bool, error) {
	if uri == "" {
		Error("AnythingInstalledFromURISuiteComponent: A repository uri must be specified.")
		return false, fmt.Errorf("repository uri must be specified")
	}

	Debug(fmt.Sprintf("Checking if anything is installed from %s", uri))

	// Get all installed packages
	installedPackages, err := getInstalledPackages()
	if err != nil {
		return false, fmt.Errorf("failed to get installed packages: %w", err)
	}

	// Check if any installed packages are from this repository
	// Note: APK doesn't use suite/component, so we ignore those parameters
	return checkIfPackagesInstalledFromRepo(installedPackages, uri, suite, component)
}

// RemoveRepofileIfUnused removes a repository file if nothing from that repository is currently installed.
//
// Note: APK uses /etc/apk/repositories file format, different from APT's sources.list.d
func RemoveRepofileIfUnused(file, testMode, key string) error {
	if file == "" {
		Error("RemoveRepofileIfUnused: no repository file specified!")
		return fmt.Errorf("no repository file specified")
	}

	Debug(fmt.Sprintf("Checking if repository file %s can be removed", file))

	// Read the repository file
	content, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			Debug(fmt.Sprintf("Repository file %s does not exist, nothing to remove", file))
			return nil
		}
		return fmt.Errorf("failed to read repository file: %w", err)
	}

	// Parse repository URLs from the file
	lines := strings.Split(string(content), "\n")
	var repoURLs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		repoURLs = append(repoURLs, line)
	}

	if len(repoURLs) == 0 {
		Debug(fmt.Sprintf("Repository file %s contains no repositories, removing", file))
		if testMode != "yes" {
			return os.Remove(file)
		}
		Debug(fmt.Sprintf("[TEST MODE] Would remove empty repository file: %s", file))
		return nil
	}

	// Check if any packages from these repositories are installed
	installedPackages, err := getInstalledPackages()
	if err != nil {
		return fmt.Errorf("failed to get installed packages: %w", err)
	}

	for _, repoURL := range repoURLs {
		Debug(fmt.Sprintf("Checking if packages from %s are installed", repoURL))

		// Check if anything is installed from this repository
		hasInstalled, err := checkIfPackagesInstalledFromRepo(installedPackages, repoURL, "", "")
		if err != nil {
			// If we can't check, be conservative and don't remove
			Debug(fmt.Sprintf("Could not check repository %s: %v", repoURL, err))
			continue
		}

		if hasInstalled {
			Debug(fmt.Sprintf("Repository %s has packages installed, not removing %s", repoURL, file))
			return nil
		}
	}

	// No packages from any of these repositories are installed, safe to remove
	Debug(fmt.Sprintf("No packages installed from %s, removing", file))

	if testMode == "yes" {
		Debug(fmt.Sprintf("[TEST MODE] Would remove repository file: %s", file))
		if key != "" {
			Debug(fmt.Sprintf("[TEST MODE] Would remove key: %s", key))
		}
		return nil
	}

	// Remove the repository file
	if err := os.Remove(file); err != nil {
		return fmt.Errorf("failed to remove repository file: %w", err)
	}

	// Remove associated key if specified
	if key != "" {
		keyPath := "/etc/apk/keys/" + key
		if _, err := os.Stat(keyPath); err == nil {
			Debug(fmt.Sprintf("Removing associated key: %s", keyPath))
			if err := os.Remove(keyPath); err != nil {
				// Log but don't fail if key removal fails
				Warning(fmt.Sprintf("Failed to remove key %s: %v", keyPath, err))
			}
		}
	}

	Debug(fmt.Sprintf("Successfully removed repository file: %s", file))
	return nil
}

// Helper function to get the list of all installed packages
func getInstalledPackages() ([]string, error) {
	// APK stores installed packages in /lib/apk/db/installed
	// We can also use `apk info` command for simplicity

	cmd := exec.Command("apk", "info")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run apk info: %w", err)
	}

	var packages []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			packages = append(packages, line)
		}
	}

	return packages, nil
}

// Helper function to get packages from a repository URL
func getPackagesFromRepoURL(repoURL string) ([]string, error) {
	// Construct APKINDEX URL
	indexURL := repoURL
	if !strings.HasSuffix(repoURL, "APKINDEX.tar.gz") {
		indexURL = strings.TrimSuffix(repoURL, "/") + "/APKINDEX.tar.gz"
	}

	// Download APKINDEX
	resp, err := http.Get(indexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch APKINDEX from %s: %w", indexURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch APKINDEX: HTTP %d", resp.StatusCode)
	}

	// Create a ReadCloser wrapper
	bodyReadCloser := io.NopCloser(resp.Body)

	// Parse APKINDEX using Alpine library
	apkindex, err := repository.IndexFromArchive(bodyReadCloser)
	if err != nil {
		return nil, fmt.Errorf("failed to parse APKINDEX: %w", err)
	}

	// Extract package names
	var packages []string
	for _, pkg := range apkindex.Packages {
		packages = append(packages, pkg.Name)
	}

	return packages, nil
}

// Helper function to check if any packages are installed from a specific repo
func checkIfPackagesInstalledFromRepo(packages []string, uri, suite, component string) (bool, error) {
	if len(packages) == 0 {
		return false, nil
	}

	// APK stores package origin in /lib/apk/db/installed
	// Parse the database to check package origins

	installedFile, err := os.Open("/lib/apk/db/installed")
	if err != nil {
		return false, fmt.Errorf("failed to open APK database: %w", err)
	}
	defer installedFile.Close()

	// Build a map of package names to their origins
	packageOrigins := make(map[string]string)

	scanner := bufio.NewScanner(installedFile)
	var currentPackage string
	var currentOrigin string

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line = end of package entry
		if line == "" {
			if currentPackage != "" && currentOrigin != "" {
				packageOrigins[currentPackage] = currentOrigin
			}
			currentPackage = ""
			currentOrigin = ""
			continue
		}

		// P: Package name
		if strings.HasPrefix(line, "P:") {
			currentPackage = strings.TrimPrefix(line, "P:")
		}

		// o: Origin (repository name)
		if strings.HasPrefix(line, "o:") {
			currentOrigin = strings.TrimPrefix(line, "o:")
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading APK database: %w", err)
	}

	// Try to fetch packages from the repository using Alpine library
	// This gives us accurate package availability for the specific repo
	repoPackages, err := getPackagesFromRepoURL(uri)
	if err != nil {
		// If we can't fetch the repo, fall back to basic origin checking
		Debug(fmt.Sprintf("Could not fetch repo packages from %s: %v", uri, err))

		// Basic origin checking as fallback
		cleanURI := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(uri, "https://"), "http://"), "/")

		for _, pkg := range packages {
			if origin, exists := packageOrigins[pkg]; exists {
				if strings.Contains(cleanURI, origin) || strings.Contains(origin, "pi-apps") {
					return true, nil
				}
			}
		}

		return false, nil
	}

	// Build a map of packages in this repository
	repoPackageMap := make(map[string]bool)
	for _, repoPackage := range repoPackages {
		repoPackageMap[repoPackage] = true
	}

	// Check if any of the requested packages are both:
	// 1. Available in the repository
	// 2. Actually installed on the system
	for _, pkg := range packages {
		if _, exists := packageOrigins[pkg]; exists {
			// Package is installed
			if repoPackageMap[pkg] {
				// Package is also in the repository
				return true, nil
			}
		}
	}

	return false, nil
}
