package api

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetAppStatus gets the app's current status (installed, uninstalled, corrupted, disabled)
func GetAppStatus(app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("app_status(): requires an argument")
	}

	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Check if app status file exists
	statusFile := filepath.Join(directory, "data", "status", app)
	if FileExists(statusFile) {
		// Read the status file
		statusData, err := os.ReadFile(statusFile)
		if err != nil {
			return "", fmt.Errorf("failed to read status file: %w", err)
		}
		return strings.TrimSpace(string(statusData)), nil
	}

	// If app status file doesn't exist, assume uninstalled
	return "uninstalled", nil
}

// AppType determines if an app is a 'standard' app or a 'package' app
// 'standard' apps have install/uninstall scripts
// 'package' apps are aliases to install apt packages from existing repositories
func AppType(app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("app_type(): no app specified")
	}

	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	appDir := filepath.Join(directory, "apps", app)

	// Check if it's a package app (has packages file)
	if FileExists(filepath.Join(appDir, "packages")) {
		return "package", nil
	}

	// Check if it's a standard app (has install/uninstall scripts)
	hasUninstall := FileExists(filepath.Join(appDir, "uninstall"))
	hasInstall := FileExists(filepath.Join(appDir, "install"))
	hasInstall32 := FileExists(filepath.Join(appDir, "install-32"))
	hasInstall64 := FileExists(filepath.Join(appDir, "install-64"))

	if hasUninstall || hasInstall || hasInstall32 || hasInstall64 {
		return "standard", nil
	}

	// If neither conditional above evaluated to true, return an error
	return "", fmt.Errorf("app_type: '%s' is not a valid app type", app)
}

// PkgAppPackagesRequired returns which packages are required during installation of a package-app
// It handles the '|' separator and checks for package availability
// Returns no output if not all required packages are available
func PkgAppPackagesRequired(app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("pkgapp_packages_required(): no app specified")
	}

	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Check if app has a packages file
	packagesFile := filepath.Join(directory, "apps", app, "packages")
	if !FileExists(packagesFile) {
		return "", fmt.Errorf("pkgapp_packages_required(): This app '%s' does not have a packages file", app)
	}

	// Read the packages file
	file, err := os.Open(packagesFile)
	if err != nil {
		return "", fmt.Errorf("failed to open packages file: %w", err)
	}
	defer file.Close()

	// Process each word - packages separated by '|' are 1 word
	packages := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		word := scanner.Text()
		// Replace " | " with "|" for consistent handling
		word = strings.ReplaceAll(word, " | ", "|")

		if strings.Contains(word, "|") {
			// Handle OR dependencies (package1|package2|package3)
			pkgOptions := strings.Split(word, "|")
			found := false

			// First check for any already installed packages
			// If a package is already installed, it should be used
			for _, pkg := range pkgOptions {
				installed := PackageInstalled(pkg)
				if installed {
					packages = append(packages, pkg)
					found = true
					break
				}
			}

			// If no installed package found, check for available packages
			if !found {
				for _, pkg := range pkgOptions {
					available := PackageAvailable(pkg, "")
					if available {
						packages = append(packages, pkg)
						found = true
						break
					}
				}
			}

			// If no package in the OR is available, return empty
			if !found {
				return "", nil
			}
		} else {
			// Non-OR package - no parsing '|' separators
			available := PackageAvailable(word, "")
			if available {
				packages = append(packages, word)
			} else {
				// One package in the AND is not available, so return empty
				return "", nil
			}
		}
	}

	if len(packages) > 0 {
		return strings.Join(packages, " "), nil
	}

	return "", nil
}

// ListAppsMissingDummyDebs lists any installed apps that have had their dummy deb
// uninstalled more recently than the app was installed
// This helps track apps that might have broken package management
func ListAppsMissingDummyDebs() ([]string, error) {
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get list of installed standard apps
	installedApps, err := ListApps("installed")
	if err != nil {
		return nil, fmt.Errorf("error listing installed apps: %w", err)
	}

	standardApps, err := ListApps("standard")
	if err != nil {
		return nil, fmt.Errorf("error listing standard apps: %w", err)
	}

	// Find intersection of installed and standard apps
	var installedStandardApps []string
	for _, app := range installedApps {
		for _, stdApp := range standardApps {
			if app == stdApp {
				installedStandardApps = append(installedStandardApps, app)
				break
			}
		}
	}

	// Parse dpkg.log to get status of pi-apps packages
	dpkgLogFile := "/var/log/dpkg.log"
	if !FileExists(dpkgLogFile) {
		return nil, fmt.Errorf("dpkg log file not found: %s", dpkgLogFile)
	}

	file, err := os.Open(dpkgLogFile)
	if err != nil {
		return nil, fmt.Errorf("error opening dpkg log: %w", err)
	}
	defer file.Close()

	// Map to track package status, keeping only the most recent entry for each package
	// Key is package name, value is array [status, timestamp]
	packageStatus := make(map[string][]string)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Look for lines containing pi-apps package status changes
		if strings.Contains(line, " installed pi-apps-") || strings.Contains(line, " not-installed pi-apps-") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}

			// Format: date time status installed/not-installed package-name version
			timestamp := fields[0] + " " + fields[1]
			status := fields[3] // "installed" or "not-installed"
			packageName := fields[4]

			// Remove :all suffix if present
			packageName = strings.TrimSuffix(packageName, ":all")

			// Only keep the most recent entry for each package
			if _, exists := packageStatus[packageName]; !exists {
				packageStatus[packageName] = []string{status, timestamp}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning dpkg log: %w", err)
	}

	// List apps with missing dummy debs
	var missingDummyDebs []string

	for _, app := range installedStandardApps {
		// Convert app name to package name
		pkgName, err := AppToPkgName(app)
		if err != nil {
			continue
		}

		// Check if package status exists and is "not-installed"
		statusInfo, exists := packageStatus[pkgName]
		if !exists {
			// No dummy deb information associated with this app
			continue
		}

		if statusInfo[0] == "not-installed" {
			// Check if the dummy deb was removed after the app was installed
			// First parse the dpkg timestamp
			dpkgTime, err := time.Parse("2006-01-02 15:04:05", statusInfo[1])
			if err != nil {
				continue
			}

			// Get the app installation timestamp from the status file
			statusFile := filepath.Join(directory, "data", "status", app)
			fileInfo, err := os.Stat(statusFile)
			if err != nil {
				continue
			}

			appInstallTime := fileInfo.ModTime()

			// If the dummy deb was removed after the app was installed, there's a problem
			if dpkgTime.After(appInstallTime) {
				missingDummyDebs = append(missingDummyDebs, app)
			}
		}
	}

	return missingDummyDebs, nil
}
