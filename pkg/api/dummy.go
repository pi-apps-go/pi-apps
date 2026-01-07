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

// Module: dummy.go
// Description: Provides dummy functions for managing repositories and packages if no package manager build tag is set.
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build dummy

package api

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RepoAdd adds local package files to the /tmp/pi-apps-local-packages repository
func RepoAdd(files ...string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files specified")
	}

	// return success if no package manager build tag is set
	return nil
}

// RepoRefresh indexes the Pi-Apps local apt repository by creating a Packages file
func RepoRefresh() error {
	// return success if no package manager build tag is set
	return nil
}

// AptLockWait waits until other apt processes are finished before proceeding
func AptLockWait() error {
	// First ensure English locale is added
	AddEnglish()

	// Spawn a goroutine to notify the user after 5 seconds
	notificationDone := make(chan bool)
	notificationShown := make(chan bool)

	go func() {
		select {
		case <-time.After(5 * time.Second):
			fmt.Print(T("Waiting until APT locks are released... "))
			notificationShown <- true
		case <-notificationDone:
			return
		}
	}()

	// Check if sudo needs a password
	cmd := exec.Command("sudo", "-n", "true")
	err := cmd.Run()
	if err != nil {
		// Sudo needs a password, prompt the user
		cmd = exec.Command("sudo", "echo")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			close(notificationDone)
			return fmt.Errorf("failed to get sudo permissions: %w", err)
		}
	}

	// Wait until lock files are not in use
	lockFiles := []string{} // no lock files to check for if no package manager build tag is set

	for {
		// Check if any locks are in use with fuser
		lockInUse := false

		for _, lockFile := range lockFiles {
			cmd := exec.Command("sudo", "fuser", lockFile)
			err := cmd.Run()

			// fuser returns exit code 0 if the file is in use, 1 if not in use
			if err == nil {
				lockInUse = true
				break
			}
		}

		if !lockInUse {
			break
		}

		time.Sleep(1 * time.Second)
	}

	for {
		// return nothing if no package manager build tag is set
		time.Sleep(1 * time.Second)
	}

	// Clean up notification goroutine
	close(notificationDone)

	// If the notification was shown, print "Done"
	select {
	case <-notificationShown:
		fmt.Println(T("Done"))
	default:
		// Notification wasn't shown, do nothing
	}

	return nil
}

// LessApt filters out unwanted lines from apt output
//
// This is a helper function for apt-related operations
func LessApt(input string) string {
	// return nothing if no package manager build tag is set
	return ""
}

// stripAnsiCodes removes ANSI color and formatting codes from a string
// This is useful for processing the output of commands like apt that might include color codes
func stripAnsiCodes(s string) string {
	// ANSI escape sequences typically start with ESC (0x1B) followed by '[' and then some sequence
	// This regex matches all ANSI escape sequences used for colors and formatting
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}

// fetchGPGKeyFromKeyserver retrieves a GPG key from a keyserver using HTTP
// This replaces the exec.Command("gpg", "--recv-key", key) call
func fetchGPGKeyFromKeyserver(keyID, keyserver string) ([]byte, error) {
	// return error if no package manager build tag is set
	return nil, fmt.Errorf("failed to fetch key from keyserver: no package manager build tag is set")
}

// exportGPGKey exports a GPG key to binary format
// This replaces the exec.Command("gpg", "--export", key) call
func exportGPGKey(keyData []byte) ([]byte, error) {
	// return error if no package manager build tag is set
	return nil, fmt.Errorf("failed to export key: no package manager build tag is set")
}

// dearmorGPGKey converts armored GPG data to binary format
// This replaces the exec.Command("gpg", "--dearmor") call
func dearmorGPGKey(armoredData []byte) ([]byte, error) {
	// return error if no package manager build tag is set
	return nil, fmt.Errorf("failed to dearmor GPG data: no package manager build tag is set")
}

// AptUpdate runs an apt update with error-checking and minimal output
func AptUpdate(args ...string) error {
	// Use cyan color with reverse video styling to match the original implementation
	// \033[96m for cyan, \033[7m for reverse video, \033[27m to end reverse, \033[0m to reset all formatting
	fmt.Fprintf(os.Stderr, "\033[96m%s \033[7m<package manager update command>\033[27m...\033[0m\n", T("Running"))
	fmt.Fprintf(os.Stderr, "\033[96m%s\033[0m\n", T("<package manager update command> complete."))
	// return success if no package manager build tag is set
	return nil
}

// RepoRm removes the local apt repository
func RepoRm() error {
	// return success if no package manager build tag is set
	return nil
}

// AppToPkgName converts an app-name to a unique, valid package-name that starts with 'pi-apps-'
//
//	"" - error if app is not specified
//	packageName - package name
//	error - error if app is not specified
func AppToPkgName(app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("no app-name specified")
	}

	// Calculate MD5 hash of the app name using native Go crypto package
	h := md5.New()
	io.WriteString(h, app)
	hashBytes := h.Sum(nil)

	// Convert the first 8 bytes to a hex string
	hashString := hex.EncodeToString(hashBytes)[:8]

	// Return the package name with the 'pi-apps-' prefix and the first 8 characters of the MD5 hash
	return fmt.Sprintf("pi-apps-%s", hashString), nil
}

// InstallPackages installs packages and makes them dependencies of the specified app
// Supports package names, regex, local files, and URLs
//
//	"" - error if app is not specified
//	error - error if app is not specified
func InstallPackages(app string, args ...string) error {
	if app == "" {
		return fmt.Errorf("install_packages function can only be used by apps to install packages (the app variable was not set)")
	}

	StatusT(Tf("Will install these packages: %s", strings.Join(args, " ")))

	// return success if no package manager build tag is set
	StatusGreenT(T("Package installation complete."))
	return nil
}

// Helper functions for InstallPackages

// extractPackageInfo parses dpkg-deb -I output to get package name, version, and architecture
func extractPackageInfo(output string) (name, version, arch string) {
	// return empty strings if no package manager build tag is set
	return "", "", ""
}

// parsePackageVersion extracts the version from apt-cache policy output
func parsePackageVersion(output string) string {
	// return empty string if no package manager build tag is set
	return ""
}

// sortAndDeduplicate sorts packages and removes duplicates, maintaining version constraints
func sortAndDeduplicate(packages []string) string {
	// Use a map to track unique package names
	pkgMap := make(map[string]string)

	for _, pkg := range packages {
		// Split package name and version constraint
		parts := strings.SplitN(pkg, " ", 2)
		pkgName := parts[0]
		pkgVersion := ""
		if len(parts) > 1 {
			pkgVersion = parts[1]
		}

		// Store the package with its version constraint
		pkgMap[pkgName] = pkgVersion
	}

	// Convert map to sorted list
	var uniquePkgs []string
	for name, version := range pkgMap {
		if version != "" {
			uniquePkgs = append(uniquePkgs, name+" "+version)
		} else {
			uniquePkgs = append(uniquePkgs, name)
		}
	}

	sort.Strings(uniquePkgs)

	// Join with commas and spaces
	return strings.Join(uniquePkgs, ", ")
}

// PurgePackages allows dependencies of the specified app to be autoremoved
// This is a Go implementation of the original bash purge_packages function
func PurgePackages(app string, isUpdate bool) error {
	Status(Tf("Allowing packages required by the %s app to be uninstalled", app))

	// return success if no package manager build tag is set
	StatusGreenT("All packages have been purged successfully.")
	return nil
}

// GetIconFromPackage finds the largest icon file (png or svg) installed by a package
// This is a Go implementation of the original bash get_icon_from_package function
func GetIconFromPackage(packages ...string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("get_icon_from_package requires at least one apt package name")
	}

	// return error if no package manager build tag is set
	return "", fmt.Errorf("no suitable icon files found")
}

// UbuntuPPAInstaller sets up a PPA on an Ubuntu-based distro
// This is a Go implementation of the original bash ubuntu_ppa_installer function
func UbuntuPPAInstaller(ppaName string) error {
	// return success if no package manager build tag is set
	return nil
}

// DebianPPAInstaller sets up a PPA on a Debian-based distro
// This is a Go implementation of the original bash debian_ppa_installer function
func DebianPPAInstaller(ppaName, ppaDist, key string) error {
	// return success if no package manager build tag is set
	return nil
}

// AddExternalRepo adds an external package manager repository
func AddExternalRepo(reponame, pubkeyurl, uris, suites, components string, additionalOptions ...string) error {
	// Exit if reponame or uri or suite contains space
	if strings.Contains(reponame, " ") || strings.Contains(uris, " ") || strings.Contains(suites, " ") {
		return fmt.Errorf("add_external_repo: provided reponame, uris, or suites contains a space")
	}

	// return success if no package manager build tag is set
	return nil
}

// RmExternalRepo removes an external package manager repository
// If force is true, it removes the repo regardless of whether it's in use
func RmExternalRepo(reponame string, force bool) error {
	// Exit if reponame contains space
	if strings.Contains(reponame, " ") {
		return fmt.Errorf("rm_external_repo: provided reponame contains a space")
	}

	// return success if no package manager build tag is set
	return nil
}

// AdoptiumInstaller sets up the Adoptium repository based on the OS codename
// This is a Go implementation of the original bash adoptium_installer function
func AdoptiumInstaller() error {
	// return success if no package manager build tag is set
	return nil
}

// PackageInstalled checks if a package is installed
func PackageInstalled(packageName string) bool {
	// return false if no package manager build tag is set
	return false
}

// PackageAvailable determines if the specified package exists in a local repository
func PackageAvailable(packageName string, dpkgArch string) bool {
	// return false if no package manager build tag is set
	return false
}

// PackageDependencies outputs the list of dependencies for the specified package
//
//	[]string - list of dependencies
//	error - error if package is not specified
func PackageDependencies(packageName string) ([]string, error) {
	// return empty slice if no package manager build tag is set
	return []string{}, nil
}

// PackageInstalledVersion returns the installed version of the specified package
//
//	"" - package is not installed
//	version - package is installed
func PackageInstalledVersion(packageName string) (string, error) {
	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("package %s is not installed", packageName)
}

// PackageLatestVersion returns the latest available version of the specified package
//
//	"" - package is not available
//	version - package is available
func PackageLatestVersion(packageName string, repo ...string) (string, error) {
	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("package %s is not available", packageName)
}

// RefreshAllPkgAppStatus updates the status of all package-apps
func RefreshAllPkgAppStatus() error {
	// Get the PI_APPS_DIR environment variable
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// return success if no package manager build tag is set
	return nil
}

// getDpkgArchitecture gets the current system architecture from dpkg
//
//	architecture - system architecture
//	error - error if dpkg is not installed
func getDpkgArchitecture() (string, error) {
	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("failed to get dpkg architecture: no package manager build tag is set")
}

// getAptCachePolicy runs the package manager's policy command for the specified packages
//
//	"" - no packages specified
//	packageManagerCachePolicyOutput - package manager cache policy output
//	error - error if apt-cache policy fails
func getAptCachePolicy(packages []string) (string, error) {
	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("failed to get apt cache policy: no package manager build tag is set")
}

// getDpkgStatus gets the status of the specified packages from dpkg
//
//	"" - no packages specified
//	dpkgStatus - dpkg status
//	error - error if dpkg status fails
func getDpkgStatus(packages []string) (string, error) {
	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("failed to get dpkg status: no package manager build tag is set")
}

// refreshPackageAppStatusWithCache refreshes a single package-app status using cached apt and dpkg data
func refreshPackageAppStatusWithCache(appName, aptCacheOutput, dpkgStatus, directory string) error {
	// return success if no package manager build tag is set
	return nil
}

// isPackageInstalledFromStatus checks if a package is installed by looking at the dpkg status
//
//	packageName - package name
//	dpkgStatus - dpkg status
//	false - package is not installed
//	true - package is installed
func isPackageInstalledFromStatus(packageName, dpkgStatus string) bool {
	// return false if no package manager build tag is set
	return false
}

// isPackageAvailableFromPolicy checks if a package is available in repositories
func isPackageAvailableFromPolicy(packageName, aptCacheOutput string) bool {
	// return false if no package manager build tag is set
	return false
}

// PackageInfo lists everything the package manager knows about the specified package
func PackageInfo(packageName string) (string, error) {
	// Validate package name to prevent package manager errors with spaces or invalid characters
	if strings.ContainsAny(packageName, " \t\n\r") {
		return "", fmt.Errorf("package name '%s' contains invalid characters (spaces or whitespace)", packageName)
	}

	// return empty string if no package manager build tag is set
	return "", fmt.Errorf("failed to get package info: no package manager build tag is set")
}
