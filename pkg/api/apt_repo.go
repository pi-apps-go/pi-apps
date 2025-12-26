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

// Module: apt_repo.go
// Description: Provides functions for managing APT repositories.

//go:build apt

package api

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// AnythingInstalledFromURISuiteComponent checks if any packages from a specific APT repository
// (identified by URI, suite, and optional component) are currently installed.
//
//	false - no packages are installed from the repository
//	true - at least one package is installed from the repository
//	error - error if repository URI, suite, or component is not specified
func AnythingInstalledFromURISuiteComponent(uri, suite, component string) (bool, error) {
	Debug(fmt.Sprintf("Checking if anything is installed from %s %s %s", uri, suite, component))

	// Clean URI by removing protocol and trailing slashes
	cleanURI := strings.TrimSuffix(regexp.MustCompile(`.*://`).ReplaceAllString(uri, ""), "/")
	cleanURI = strings.ReplaceAll(cleanURI, "/", "_")

	// Clean suite by removing trailing slashes
	cleanSuite := strings.TrimSuffix(suite, "/")
	cleanSuite = strings.ReplaceAll(cleanSuite, "/", "_")

	// Construct filepath pattern based on presence of component
	var filepathPattern string
	if component == "" {
		filepathPattern = fmt.Sprintf("/var/lib/apt/lists/%s_%s_", cleanURI, cleanSuite)
	} else {
		cleanComponent := strings.TrimSuffix(component, "/")
		cleanComponent = strings.ReplaceAll(cleanComponent, "/", "_")
		filepathPattern = fmt.Sprintf("/var/lib/apt/lists/%s_dists_%s_%s_", cleanURI, cleanSuite, cleanComponent)
	}

	Debug(filepathPattern)

	// Find all relevant package list files
	matches, err := filepath.Glob(filepathPattern + "*_Packages")
	if err != nil {
		return false, fmt.Errorf("failed to find package lists: %w", err)
	}

	Debug(strings.Join(matches, "\n"))

	if len(matches) == 0 {
		return false, nil
	}

	// Get list of installed packages
	installedPackages, err := getInstalledPackages()
	if err != nil {
		return false, fmt.Errorf("failed to get installed packages: %w", err)
	}

	// For each repo file, check if any packages are installed from it
	for _, repoFile := range matches {
		packagesInRepo, err := getPackagesInRepo(repoFile)
		if err != nil {
			return false, fmt.Errorf("failed to get packages in repo %s: %w", repoFile, err)
		}

		// Find intersection of installed packages and packages in repo
		var packagesToCheck []string
		for _, pkg := range packagesInRepo {
			if slices.Contains(installedPackages, pkg) {
				packagesToCheck = append(packagesToCheck, pkg)
			}
		}

		if len(packagesToCheck) == 0 {
			continue
		}

		// Check if any of these packages are installed from this repo
		isInstalled, err := checkIfPackagesInstalledFromRepo(packagesToCheck, uri, suite, component)
		if err != nil {
			return false, fmt.Errorf("failed to check if packages are installed from repo: %w", err)
		}

		if isInstalled {
			return true, nil
		}
	}

	return false, nil
}

// RemoveRepofileIfUnused removes a sources.list.d file if nothing from that repository is currently installed.
//
// If testMode is "test", it only outputs the status without removing anything.
//
//	error - error if file is not specified or testMode is not "test"
func RemoveRepofileIfUnused(file, testMode, key string) error {
	// Return if the file does not exist
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	}

	// Determine file type and process accordingly
	fileExt := filepath.Ext(file)
	switch fileExt {
	case ".list":
		inUse, err := handleListFile(file)
		if err != nil {
			return fmt.Errorf("failed to process list file: %w", err)
		}

		if inUse {
			if testMode == "test" {
				fmt.Fprintln(os.Stderr, "At least one package is preventing the repo from being removed")
			}
			return nil
		}
	case ".sources":
		inUse, err := handleSourcesFile(file)
		if err != nil {
			return fmt.Errorf("failed to process sources file: %w", err)
		}

		if inUse {
			if testMode == "test" {
				fmt.Fprintln(os.Stderr, "At least one package is preventing the repo from being removed")
			}
			return nil
		}
	default:
		return fmt.Errorf("%s was not of apt list or sources type", file)
	}

	if testMode == "test" {
		fmt.Fprintf(os.Stderr, "The given repository is not in use and can be deleted:\n%s\n", file)
		return nil
	}

	repoName := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(file), ".list"), ".sources")
	Status(fmt.Sprintf("Removing the %s repo as it is not being used", repoName))

	// Remove the repo file
	if err := os.Remove(file); err != nil {
		return fmt.Errorf("failed to remove repo file: %w", err)
	}

	// Remove key file if it exists
	if key != "" {
		if _, err := os.Stat(key); err == nil {
			if err := os.Remove(key); err != nil {
				// Not returning error as this is not critical
				Warning(fmt.Sprintf("Failed to remove key file %s: %s", key, err))
			}
		}
	}

	return nil
}

// Helper function to handle .list files
func handleListFile(file string) (bool, error) {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", file, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(fileContent)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "deb ") {
			continue
		}

		// Extract URI, suite, and components
		line = strings.TrimPrefix(line, "deb ")

		// Remove apt options like [arch=amd64,signed-by=/etc/apt/keyrings/key.gpg]
		aptOptionsRegex := regexp.MustCompile(`\[.*?\]`)
		line = aptOptionsRegex.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		uri := fields[0]
		suite := fields[1]
		var components []string

		if len(fields) > 2 {
			components = fields[2:]
		}

		if len(components) == 0 {
			inUse, err := AnythingInstalledFromURISuiteComponent(uri, suite, "")
			if err != nil {
				return false, fmt.Errorf("failed to check if anything is installed from %s %s: %w", uri, suite, err)
			}

			if inUse {
				return true, nil
			}
		} else {
			for _, component := range components {
				inUse, err := AnythingInstalledFromURISuiteComponent(uri, suite, component)
				if err != nil {
					return false, fmt.Errorf("failed to check if anything is installed from %s %s %s: %w", uri, suite, component, err)
				}

				if inUse {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// Helper function to handle .sources files
func handleSourcesFile(file string) (bool, error) {
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return false, fmt.Errorf("failed to read file %s: %w", file, err)
	}

	lines := strings.Split(string(fileContent), "\n")

	// Find empty lines that separate stanzas
	emptyLines := []int{}
	for i, line := range lines {
		if line == "" || regexp.MustCompile(`^\s+$`).MatchString(line) || regexp.MustCompile(`^\t+$`).MatchString(line) {
			emptyLines = append(emptyLines, i)
		}
	}

	// Always add the last line
	emptyLines = append(emptyLines, len(lines))

	Debug(fmt.Sprintf("Empty lines: %v", emptyLines))

	// Parse each stanza
	lineStart := 0
	for _, lineEnd := range emptyLines {
		stanza := lines[lineStart:lineEnd]

		// Skip if Enabled: no
		enabled := true
		for _, line := range stanza {
			if regexp.MustCompile(`(?i)^Enabled:\s*no`).MatchString(line) {
				enabled = false
				break
			}
		}

		if !enabled {
			lineStart = lineEnd + 1
			continue
		}

		// Extract URIs, Suites, and Components
		var uris, suites, components []string

		for _, line := range stanza {
			if strings.HasPrefix(line, "#") {
				continue
			}

			if regexp.MustCompile(`(?i)^URIs:`).MatchString(line) {
				uriLine := regexp.MustCompile(`(?i)^URIs:`).ReplaceAllString(line, "")
				uriLine = strings.TrimSpace(uriLine)
				uris = strings.Fields(uriLine)
			} else if regexp.MustCompile(`(?i)^Suites:`).MatchString(line) {
				suiteLine := regexp.MustCompile(`(?i)^Suites:`).ReplaceAllString(line, "")
				suiteLine = strings.TrimSpace(suiteLine)
				suites = strings.Fields(suiteLine)
			} else if regexp.MustCompile(`(?i)^Components:`).MatchString(line) {
				componentLine := regexp.MustCompile(`(?i)^Components:`).ReplaceAllString(line, "")
				componentLine = strings.TrimSpace(componentLine)
				components = strings.Fields(componentLine)
			}
		}

		// Check if anything is installed from any combination
		for _, uri := range uris {
			for _, suite := range suites {
				if len(components) == 0 {
					inUse, err := AnythingInstalledFromURISuiteComponent(uri, suite, "")
					if err != nil {
						return false, fmt.Errorf("failed to check if anything is installed from %s %s: %w", uri, suite, err)
					}

					if inUse {
						return true, nil
					}
				} else {
					for _, component := range components {
						inUse, err := AnythingInstalledFromURISuiteComponent(uri, suite, component)
						if err != nil {
							return false, fmt.Errorf("failed to check if anything is installed from %s %s %s: %w", uri, suite, component, err)
						}

						if inUse {
							return true, nil
						}
					}
				}
			}
		}

		lineStart = lineEnd + 1
	}

	return false, nil
}

// Helper function to get the list of all installed packages
func getInstalledPackages() ([]string, error) {
	statusFile, err := os.Open("/var/lib/dpkg/status")
	if err != nil {
		return nil, fmt.Errorf("failed to open status file: %w", err)
	}
	defer statusFile.Close()

	var installedPackages []string
	var packageName string
	var isInstalled bool

	scanner := bufio.NewScanner(statusFile)
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			// End of package entry
			if isInstalled && packageName != "" {
				installedPackages = append(installedPackages, packageName)
			}

			packageName = ""
			isInstalled = false
			continue
		case strings.HasPrefix(line, "Package: "):
			packageName = strings.TrimPrefix(line, "Package: ")
		case line == "Status: install ok installed":
			isInstalled = true
		}
	}

	// Handle the last package entry
	if isInstalled && packageName != "" {
		installedPackages = append(installedPackages, packageName)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading status file: %w", err)
	}

	return installedPackages, nil
}

// Helper function to get the list of packages in a repo file
func getPackagesInRepo(repoFile string) ([]string, error) {
	file, err := os.Open(repoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open repo file: %w", err)
	}
	defer file.Close()

	var packages []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Package: ") {
			packageName := strings.TrimSpace(strings.TrimPrefix(line, "Package: "))
			packages = append(packages, packageName)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading repo file: %w", err)
	}

	return packages, nil
}

// Helper function to check if any packages are installed from a specific repo
func checkIfPackagesInstalledFromRepo(packages []string, uri, suite, component string) (bool, error) {
	if len(packages) == 0 {
		return false, nil
	}

	// Prepare the apt-cache policy command
	cmd := exec.Command("apt-cache", append([]string{"policy"}, packages...)...)
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to run apt-cache policy: %w", err)
	}

	// Clean URI for comparison
	cleanURI := strings.TrimSuffix(regexp.MustCompile(`.*://`).ReplaceAllString(uri, ""), "/")

	// Prepare the pattern to look for in the apt-cache output
	var pattern string
	if component == "" {
		pattern = fmt.Sprintf("%s %s", cleanURI, suite)
	} else {
		pattern = fmt.Sprintf("%s %s/%s", cleanURI, suite, component)
	}

	// Look for lines matching the pattern and check if they are installed versions
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.Contains(line, pattern) && i > 0 {
			// Check if the previous line indicates this is the installed version
			if strings.Contains(lines[i-1], "***") {
				return true, nil
			}
		}
	}

	return false, nil
}
