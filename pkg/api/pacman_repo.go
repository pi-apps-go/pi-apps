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

// Module: pacman_repo.go
// Description: Provides functions for managing repositories when using the Pacman package manager.

//go:build pacman

package api

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// AnythingInstalledFromURISuiteComponent checks if any packages from a specific repository
// (identified by URI, suite, and optional component) are currently installed.
//
// For pacman, URI typically maps to a repository name in /etc/pacman.conf
// suite and component are ignored as pacman doesn't use these concepts
//
//	false - no packages are installed from the repository
//	true - at least one package is installed from the repository
//	error - error if repository URI, suite, or component is not specified
func AnythingInstalledFromURISuiteComponent(uri, suite, component string) (bool, error) {
	Debug(fmt.Sprintf("Checking if anything is installed from %s %s %s", uri, suite, component))

	if uri == "" {
		return false, fmt.Errorf("repository URI must be specified")
	}

	// Get all installed packages
	installedPackages, err := getInstalledPackages()
	if err != nil {
		return false, fmt.Errorf("failed to get installed packages: %w", err)
	}

	// Check if any installed packages are from this repository
	// Note: suite and component are ignored for pacman
	return checkIfPackagesInstalledFromRepo(installedPackages, uri, suite, component)
}

// RemoveRepofileIfUnused removes a pacman repository configuration if nothing from that repository is currently installed.
//
// For pacman, repositories are defined in /etc/pacman.conf with [repo-name] sections
// followed by Server = lines. This function removes the entire repository section.
// If testMode is "test", it only outputs the status without removing anything.
//
//	error - error if file is not specified or testMode is not "test"
func RemoveRepofileIfUnused(file, testMode, key string) error {
	// For pacman, the file should be /etc/pacman.conf or a file in /etc/pacman.d/
	// The file parameter might be:
	// 1. A path to pacman.conf or a file in /etc/pacman.d/
	// 2. A repository name (e.g., "custom-repo")
	// 3. A URL that matches a Server = line in pacman.conf
	var repoName string
	var targetURL string
	var inUse bool
	var err error

	// Check if this is a URL first (before checking if file exists)
	if strings.Contains(file, "://") || strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
		// This looks like a URL - find the repository section that contains this URL
		targetURL = file
		repoName, inUse, err = findRepoByURL(targetURL)
		if err != nil {
			return fmt.Errorf("failed to find repository by URL: %w", err)
		}
		if repoName == "" {
			// URL not found in any repository
			if testMode == "test" {
				fmt.Fprintf(os.Stderr, "Repository with URL %s not found in pacman.conf\n", targetURL)
			}
			return nil
		}
	} else if strings.HasPrefix(file, "/etc/pacman") {
		// Return if the file does not exist
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return nil
		}
		// Extract repository name from the file or check all repositories in the file
		// The file parameter might contain a repository name or be the config file itself
		if strings.Contains(file, "[") && strings.Contains(file, "]") {
			// Extract repo name from file path if it contains [repo-name]
			start := strings.Index(file, "[")
			end := strings.Index(file, "]")
			if start != -1 && end != -1 && end > start {
				repoName = file[start+1 : end]
			}
		}

		// Check if repository is in use
		if repoName != "" {
			inUse, err = AnythingInstalledFromURISuiteComponent(repoName, "", "")
		} else {
			// Check all repositories in the file
			inUse, err = handlePacmanConfFile(file)
		}
		if err != nil {
			return fmt.Errorf("failed to check if repository is in use: %w", err)
		}
	} else {
		// Return if the file does not exist (for non-URL, non-pacman paths)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return nil
		}
		// Assume it's a repository name
		repoName = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		inUse, err = AnythingInstalledFromURISuiteComponent(repoName, "", "")
		if err != nil {
			return fmt.Errorf("failed to check if repository is in use: %w", err)
		}
	}

	if inUse {
		if testMode == "test" {
			fmt.Fprintln(os.Stderr, "At least one package is preventing the repo from being removed")
		}
		return nil
	}

	if testMode == "test" {
		fmt.Fprintf(os.Stderr, "The given repository is not in use and can be deleted:\n%s\n", file)
		return nil
	}

	// Determine which pacman.conf file to edit
	pacmanConf := "/etc/pacman.conf"
	if strings.HasPrefix(file, "/etc/pacman.d/") {
		// For files in /etc/pacman.d/, we can just remove the file
		Status(fmt.Sprintf("Removing the %s repo file as it is not being used", filepath.Base(file)))
		rmCmd := exec.Command("sudo", "rm", "-f", file)
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove repo file: %w", err)
		}
	} else {
		// For /etc/pacman.conf, we need to remove the repository section
		if repoName == "" {
			// Try to extract repo name from file parameter
			// The file might be something like "/etc/pacman.conf[custom-repo]"
			if strings.Contains(file, "[") && strings.Contains(file, "]") {
				start := strings.Index(file, "[")
				end := strings.Index(file, "]")
				if start != -1 && end != -1 && end > start {
					repoName = file[start+1 : end]
				}
			}
			if repoName == "" {
				repoName = strings.TrimSuffix(strings.TrimSuffix(filepath.Base(file), ".conf"), ".list")
			}
		}

		// If we found a repo by URL, make sure we have the repo name
		if repoName == "" && targetURL != "" {
			// This shouldn't happen, but handle it gracefully
			Warning(fmt.Sprintf("Could not determine repository name for URL %s", targetURL))
			return nil
		}

		Status(fmt.Sprintf("Removing the %s repo from %s as it is not being used", repoName, pacmanConf))

		// Read the pacman.conf file
		content, err := os.ReadFile(pacmanConf)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", pacmanConf, err)
		}

		// Parse and remove the repository section
		lines := strings.Split(string(content), "\n")
		var newLines []string
		inRepoSection := false
		removedSection := false

		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)

			// Check if this is the start of our repository section
			if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
				sectionName := strings.TrimPrefix(strings.TrimSuffix(trimmedLine, "]"), "[")
				if sectionName == repoName {
					inRepoSection = true
					removedSection = true
					continue // Skip the [repo-name] line
				} else {
					inRepoSection = false
				}
			}

			// If we're in the repository section, skip Server = and Include = lines
			if inRepoSection {
				if strings.HasPrefix(trimmedLine, "Server") || strings.HasPrefix(trimmedLine, "Include") {
					continue // Skip Server/Include lines
				}
				// If we hit a non-Server/Include line (or empty line), we've reached the end of the section
				if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
					inRepoSection = false
				} else if trimmedLine == "" {
					// Empty line might be part of the section, but we'll keep one for spacing
					// Only add if the next non-empty line isn't part of the section
					continue
				}
			}

			// Keep all other lines
			newLines = append(newLines, line)
		}

		if !removedSection {
			Warning(fmt.Sprintf("Repository section [%s] not found in %s", repoName, pacmanConf))
			return nil
		}

		// Write the modified content back
		newContent := strings.Join(newLines, "\n")
		tempFile, err := os.CreateTemp("", "pacman-conf")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(newContent); err != nil {
			tempFile.Close()
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}
		tempFile.Close()

		// Copy the temporary file to /etc/pacman.conf using sudo
		cpCmd := exec.Command("sudo", "cp", tempFile.Name(), pacmanConf)
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("failed to update %s: %w", pacmanConf, err)
		}

		// Set proper permissions
		chmodCmd := exec.Command("sudo", "chmod", "644", pacmanConf)
		chmodCmd.Run() // Ignore errors for chmod
	}

	// Remove key file if it exists
	if key != "" {
		if _, err := os.Stat(key); err == nil {
			rmKeyCmd := exec.Command("sudo", "rm", "-f", key)
			if err := rmKeyCmd.Run(); err != nil {
				// Not returning error as this is not critical
				Warning(fmt.Sprintf("Failed to remove key file %s: %s", key, err))
			}
		}
	}

	return nil
}

// Helper function to handle pacman.conf files
func handlePacmanConfFile(file string) (bool, error) {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", file, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(fileContent)))
	var currentRepo string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check for repository section [repo-name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentRepo = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			// Skip special sections
			if currentRepo == "options" {
				currentRepo = ""
			}
			continue
		}

		// If we're in a repository section, check if anything is installed from it
		if currentRepo != "" {
			inUse, err := AnythingInstalledFromURISuiteComponent(currentRepo, "", "")
			if err != nil {
				return false, fmt.Errorf("failed to check if anything is installed from %s: %w", currentRepo, err)
			}

			if inUse {
				return true, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading file: %w", err)
	}

	return false, nil
}

// Helper function to find a repository section by URL
// Returns the repository name, whether it's in use, and any error
func findRepoByURL(url string) (string, bool, error) {
	pacmanConf := "/etc/pacman.conf"
	content, err := os.ReadFile(pacmanConf)
	if err != nil {
		return "", false, fmt.Errorf("failed to read %s: %w", pacmanConf, err)
	}

	lines := strings.Split(string(content), "\n")
	var currentRepo string
	var foundRepo string

	// Normalize the URL for comparison (remove trailing slashes, etc.)
	normalizedURL := strings.TrimSuffix(strings.TrimSpace(url), "/")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			continue
		}

		// Check for repository section [repo-name]
		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
			currentRepo = strings.TrimPrefix(strings.TrimSuffix(trimmedLine, "]"), "[")
			// Skip special sections
			if currentRepo == "options" {
				currentRepo = ""
			}
			continue
		}

		// Check if this is a Server = line in the current repository section
		if currentRepo != "" && strings.HasPrefix(trimmedLine, "Server") {
			// Extract URL from Server = line
			// Format: Server = <url>/$repo/os/$arch
			parts := strings.SplitN(trimmedLine, "=", 2)
			if len(parts) >= 2 {
				serverURL := strings.TrimSpace(parts[1])
				normalizedServerURL := strings.TrimSuffix(strings.TrimSpace(serverURL), "/")

				// Check if the URL matches (exact match or contains the target URL)
				if normalizedServerURL == normalizedURL || strings.Contains(normalizedServerURL, normalizedURL) || strings.Contains(normalizedURL, normalizedServerURL) {
					foundRepo = currentRepo
					break
				}
			}
		}
	}

	if foundRepo == "" {
		return "", false, nil
	}

	// Check if this repository is in use
	inUse, err := AnythingInstalledFromURISuiteComponent(foundRepo, "", "")
	return foundRepo, inUse, err
}

// Helper function to get the list of all installed packages
func getInstalledPackages() ([]string, error) {
	// Use pacman -Q to get all installed packages
	cmd := exec.Command("pacman", "-Q")
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get installed packages: %w", err)
	}

	var packages []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format is "package-name version"
		parts := strings.Fields(line)
		if len(parts) > 0 {
			packages = append(packages, parts[0])
		}
	}

	return packages, nil
}

// Helper function to get the list of packages in a repo file
// For pacman, this queries the repository using pacman -Sl
func getPackagesInRepo(repoName string) ([]string, error) {
	// Use pacman -Sl to list all packages in a repository
	cmd := exec.Command("pacman", "-Sl", repoName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get packages in repository %s: %w", repoName, err)
	}

	var packages []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format is "repository package-name version"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			packages = append(packages, parts[1])
		}
	}

	return packages, nil
}

// Helper function to check if any packages are installed from a specific repo
func checkIfPackagesInstalledFromRepo(packages []string, uri, suite, component string) (bool, error) {
	if len(packages) == 0 {
		return false, nil
	}

	// Clean URI for comparison
	// URI might be a repository name (e.g., "core", "extra") or a full URL
	cleanURI := strings.TrimSuffix(regexp.MustCompile(`.*://`).ReplaceAllString(uri, ""), "/")
	cleanURI = strings.ToLower(strings.TrimSpace(cleanURI))

	// Extract repository name from URI if it's a URL
	// Format: Server = <url>/$repo/os/$arch
	// We need to extract the repo name part
	repoNameFromURI := cleanURI
	if strings.Contains(cleanURI, "/") {
		// Try to extract repo name from URL pattern
		parts := strings.Split(cleanURI, "/")
		for i, part := range parts {
			// Look for patterns like $repo/os/$arch
			if i+2 < len(parts) && parts[i+1] == "os" {
				repoNameFromURI = part
				break
			}
		}
	}

	// For pacman, we check the repository of each installed package
	// using pacman -Qi to get repository information
	for _, pkg := range packages {
		// Use pacman -Qi to get package info (for installed packages)
		cmd := exec.Command("pacman", "-Qi", pkg)
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
		output, err := cmd.Output()
		if err != nil {
			// Package not installed, skip
			continue
		}

		// Parse output to find Repository line
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Repository") {
				// Format: "Repository      : core" or "Repository      : extra"
				parts := strings.SplitN(line, ":", 2)
				if len(parts) >= 2 {
					repo := strings.ToLower(strings.TrimSpace(parts[1]))

					// Check if the repository matches
					// Match by repository name (e.g., "core", "extra", "community")
					if repo == cleanURI || repo == repoNameFromURI {
						return true, nil
					}

					// Also check if URI contains the repo name or vice versa
					if strings.Contains(repo, cleanURI) || strings.Contains(cleanURI, repo) {
						return true, nil
					}
				}
				break
			}
		}
	}

	return false, nil
}
