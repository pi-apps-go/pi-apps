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

// Module: apt.go
// Description: Provides functions for managing APT repositories and packages.
// In order to allow multiple package managers at one, all package manager related functions (here for APT) are implemented in this file.

//go:build apt

package api

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ProtonMail/gopenpgp/v3/armor"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

// RepoAdd adds local package files to the /tmp/pi-apps-local-packages repository
func RepoAdd(files ...string) error {
	// Ensure the repo folder exists
	repoDir := "/tmp/pi-apps-local-packages"
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", repoDir, err)
	}

	// Move every mentioned deb file to the repository
	for _, file := range files {
		destPath := filepath.Join(repoDir, filepath.Base(file))

		// Check if source file exists
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("source file does not exist: %s", file)
		}

		// Read source file
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		// Write to destination
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write to %s: %w", destPath, err)
		}

		// Remove source file after successful copy
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("failed to remove source file %s after moving: %w", file, err)
		}
	}

	return nil
}

// RepoRefresh indexes the Pi-Apps local apt repository by creating a Packages file
func RepoRefresh() error {
	repoDir := "/tmp/pi-apps-local-packages"

	// Check if the repository directory exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("cannot index the repository - it's missing! %s", repoDir)
	}

	// Change to the repository directory and run apt-ftparchive
	cmd := exec.Command("apt-ftparchive", "packages", ".")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("apt-ftparchive failed to index the repository: %s\nThe Pi-Apps developers have been receiving a few of these errors recently, but we can't figure out what the problem is without your help. Could you please reach out so we can solve this?", repoDir)
		return fmt.Errorf("%s\nCommand output: %s\nError: %w", errorMsg, output, err)
	}

	// Write the Packages file
	packagesPath := filepath.Join(repoDir, "Packages")
	if err := os.WriteFile(packagesPath, output, 0644); err != nil {
		return fmt.Errorf("failed to create Packages file: %w", err)
	}

	// Check if the Packages file actually exists
	if _, err := os.Stat(packagesPath); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("apt-ftparchive failed to index the repository: %s\nThe Pi-Apps developers have been receiving a few of these errors recently, but we can't figure out what the problem is without your help. Could you please reach out so we can solve this?", repoDir)
		return fmt.Errorf("%s", errorMsg)
	}

	// Modify the Packages file - remove "./" from Filename entries
	packagesContent, err := os.ReadFile(packagesPath)
	if err != nil {
		return fmt.Errorf("failed to read Packages file: %w", err)
	}

	modifiedContent := strings.ReplaceAll(string(packagesContent), "Filename: ./", "Filename: ")

	if err := os.WriteFile(packagesPath, []byte(modifiedContent), 0644); err != nil {
		return fmt.Errorf("failed to write modified Packages file: %w", err)
	}

	// Create aptftp.conf file for setting repo origin name
	aptftpConf := []byte(`APT::FTPArchive::Release {
Origin "pi-apps-local-packages";
};`)

	if err := os.WriteFile(filepath.Join(repoDir, "aptftp.conf"), aptftpConf, 0644); err != nil {
		return fmt.Errorf("failed to create aptftp.conf file: %w", err)
	}

	// Create a source.list file for the repository
	// First remove existing source.list file if it exists
	sourceListPath := filepath.Join(repoDir, "source.list")
	os.Remove(sourceListPath)

	// Create the source.list file with the local repository entry
	sourceListContent := "deb [trusted=yes] file:/tmp/pi-apps-local-packages/ ./\n"

	// Read the system sources.list and append it
	systemSourcesList, err := os.ReadFile("/etc/apt/sources.list")
	if err != nil {
		return fmt.Errorf("failed to read system sources.list: %w", err)
	}

	sourceListContent += string(systemSourcesList)

	// Write the combined source.list file
	if err := os.WriteFile(sourceListPath, []byte(sourceListContent), 0644); err != nil {
		return fmt.Errorf("failed to create source.list file: %w", err)
	}

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
	lockFiles := []string{
		"/var/lib/dpkg/lock",
		"/var/lib/apt/lists/lock",
		"/var/cache/apt/archives/lock",
		"/var/log/unattended-upgrades/unattended-upgrades.log",
		"/var/lib/dpkg/lock-frontend",
		"/var/cache/debconf/config.dat",
	}

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

	// Try to install a non-existent package to see if apt fails due to a lock-file
	// NOTE: This check needs to be resilient to APT 3.0's UI changes, which may affect the error message format
	// APT 3.0 is on Debian 13+/Ubuntu 25.04+ which uses colors extensively for the UI and as a result partially changed the output format
	for {
		cmd := exec.Command("sudo", "-E", "apt", "-o", "DPkg::Lock::Timeout=-1", "install", "lkqecjhxwqekc")
		output, _ := cmd.CombinedOutput()
		outputStr := string(output)

		// Strip ANSI color codes from the output as APT 3.0 uses colors
		outputStr = stripAnsiCodes(outputStr)

		// Check for various lock-related messages that might appear in different APT versions
		if !strings.Contains(outputStr, "Could not get lock") &&
			!strings.Contains(outputStr, "could not get lock") &&
			!strings.Contains(outputStr, "Unable to lock") &&
			!strings.Contains(outputStr, "unable to lock") &&
			!strings.Contains(outputStr, "is locked by another process") {
			break
		}

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
	// First, strip ANSI color codes from the input
	// APT 3.0 introduces colored output, so we need to strip these codes
	// before doing any pattern matching
	input = stripAnsiCodes(input)

	unwantedPatterns := []string{
		"apt does not have a stable CLI interface.",
		"Reading package lists...",
		"Building dependency tree",
		"Reading state information...",
		"Need to get",
		"Selecting previously unselected package",
		"Preparing to unpack",
		"Setting up ",
		"Processing triggers for ",
		"^$",
		// Additional patterns for APT 3.0
		"The following packages were automatically installed",
		"Unpacking",
		"Get:",
		"Summary:",
		"Upgrading:",
		"Download size:",
		"Space needed:",
		"Space reclaimed:",
		"After this operation", // Line about disk space (present in both APT 2.x and 3.0)
		"Fetched",              // Line about download size and speed
		"Preparing to unpack",
		"Extracting templates",
		"Removing old",
	}

	lines := strings.Split(input, "\n")
	var filteredLines []string

	for _, line := range lines {
		if line == "" {
			continue // Skip empty lines
		}

		keep := true

		// Special handling for repository lines to ensure we don't miss any PPAs
		if strings.HasPrefix(line, "Hit:") ||
			strings.HasPrefix(line, "Ign:") ||
			strings.HasPrefix(line, "Get:") {
			// Always keep repository lines
			filteredLines = append(filteredLines, line)
			continue
		}

		for _, pattern := range unwantedPatterns {
			if strings.Contains(line, pattern) {
				keep = false
				break
			}
		}

		if keep {
			filteredLines = append(filteredLines, line)
		}
	}

	result := strings.Join(filteredLines, "\n")
	// If the result is not empty and the input ended with a newline, add a newline to the result too
	if result != "" && strings.HasSuffix(input, "\n") {
		result += "\n"
	}

	return result
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
	// Construct the URL to fetch the key from the keyserver
	// Most keyservers support HTTP-based key retrieval
	url := fmt.Sprintf("https://%s/pks/lookup?op=get&search=0x%s", keyserver, keyID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch key from keyserver: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keyserver returned status %d", resp.StatusCode)
	}

	keyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read key data: %w", err)
	}

	// The response might be armored, so we need to unarmor it
	_, isArmored := armor.IsPGPArmored(bytes.NewReader(keyData))
	if isArmored {
		// Try to unarmor the key data
		unarmoredData, err := armor.Unarmor(string(keyData))
		if err != nil {
			// If unarmoring fails, return the original data (might be binary)
			return keyData, nil
		}
		return unarmoredData, nil
	}

	return keyData, nil
}

// exportGPGKey exports a GPG key to binary format
// This replaces the exec.Command("gpg", "--export", key) call
func exportGPGKey(keyData []byte) ([]byte, error) {
	// Create a key from the provided data
	key, err := crypto.NewKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key: %w", err)
	}

	// Export the key in binary format
	exportedData, err := key.GetArmoredPublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get armored public key: %w", err)
	}

	// Convert armored data back to binary for keyring storage
	unarmoredData, err := armor.Unarmor(exportedData)
	if err != nil {
		return nil, fmt.Errorf("failed to unarmor exported key: %w", err)
	}

	return unarmoredData, nil
}

// dearmorGPGKey converts armored GPG data to binary format
// This replaces the exec.Command("gpg", "--dearmor") call
func dearmorGPGKey(armoredData []byte) ([]byte, error) {
	// Check if the data is already unarmored (binary)
	_, isArmored := armor.IsPGPArmored(bytes.NewReader(armoredData))
	if !isArmored {
		return armoredData, nil
	}

	// Unarmor the data
	unarmoredData, err := armor.Unarmor(string(armoredData))
	if err != nil {
		return nil, fmt.Errorf("failed to dearmor GPG data: %w", err)
	}

	return unarmoredData, nil
}

// AptUpdate runs an apt update with error-checking and minimal output
func AptUpdate(args ...string) error {
	// Wait for APT locks to be released first
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for APT locks: %w", err)
	}

	// Use cyan color with reverse video styling to match the original implementation
	// \033[96m for cyan, \033[7m for reverse video, \033[27m to end reverse, \033[0m to reset all formatting
	fmt.Fprintf(os.Stderr, "\033[96m%s \033[7msudo apt update\033[27m...\033[0m\n", T("Running"))

	// Prepare the apt update command with provided arguments
	// Use the original LANG that was set by the user, not the one modified by i18n
	lang := getOriginalLang()

	var aptArgs []string

	if lang != "" {
		// Set locale variables before sudo for proper APT localization
		aptArgs = append([]string{"-E", "LANG=" + lang, "LC_ALL=" + lang, "LC_MESSAGES=" + lang, "apt-get", "update", "--allow-releaseinfo-change"}, args...)
	} else {
		aptArgs = append([]string{"-E", "apt-get", "update", "--allow-releaseinfo-change"}, args...)
	}

	cmd := exec.Command("sudo", aptArgs...)

	// Preserve environment variables for proper locale handling
	cmd.Env = os.Environ()

	// Set up pipes for stdout and stderr to capture output in real-time
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create a combined reader for both stdout and stderr
	outputReader := io.MultiReader(stdout, stderr)

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start apt update command: %w", err)
	}

	// Create a buffer to store the complete output
	var outputBuffer bytes.Buffer
	outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

	// Read output and buffer it to process complete lines
	scanner := bufio.NewScanner(outputReader)
	var buffer string
	for scanner.Scan() {
		line := scanner.Text()

		// Add line to our processing buffer
		buffer += line + "\n"

		// If we have a complete line or multiple lines, process them
		if strings.Contains(buffer, "\n") {
			lines := strings.Split(buffer, "\n")
			// Process all complete lines
			for i := 0; i < len(lines)-1; i++ {
				if lines[i] != "" {
					filteredLine := LessApt(lines[i] + "\n")
					if filteredLine != "" {
						fmt.Fprint(outputWriter, filteredLine)
					}
				}
			}
			// Keep any partial line for the next iteration
			buffer = lines[len(lines)-1]
		}
	}

	// Process any remaining content in the buffer
	if buffer != "" {
		filteredLine := LessApt(buffer)
		if filteredLine != "" {
			fmt.Fprint(outputWriter, filteredLine)
		}
	}

	// Wait for the command to complete
	err = cmd.Wait()

	// Process output to show helpful messages
	// Strip color codes first to ensure reliable pattern matching
	completeOutput := outputBuffer.String()
	strippedOutput := stripAnsiCodes(completeOutput)

	// Show completion message in cyan to match the original
	fmt.Fprintf(os.Stderr, "\033[96m%s\033[0m\n", T("apt update complete."))

	// Check for autoremovable packages messages (both APT 2.x and 3.0 formats)
	if strings.Contains(strippedOutput, "autoremove to remove them") ||
		strings.Contains(strippedOutput, "can be autoremoved") {
		// Use direct ANSI codes for exact matching with the original
		fmt.Printf("\033[33m%s\033[39m %s \033[4msudo a\033[0mp\033[4mt autoremove\033[0m.\n",
			T("Some packages are unnecessary."), T("Please consider running"))
	}

	// Check for upgradeable packages messages (both APT 2.x and 3.0 formats)
	if strings.Contains(strippedOutput, "packages can be upgraded") ||
		strings.Contains(strippedOutput, "can be upgraded") ||
		strings.Contains(strippedOutput, "upgradable") {
		fmt.Printf("\033[33m%s\033[39m %s \033[4msudo a\033[0mp\033[4mt full-u\033[0mpg\033[4mrade\033[0m.\n",
			T("Some packages can be upgraded."), T("Please consider running"))
	} else if strings.Contains(strippedOutput, "package can be upgraded") ||
		strings.Contains(strippedOutput, "is upgradable") {
		fmt.Printf("\033[33m%s\033[39m %s \033[4msudo a\033[0mp\033[4mt full-u\033[0mpg\033[4mrade\033[0m.\n",
			T("One package can be upgraded."), T("Please consider running"))
	}

	// Handle errors
	if err != nil || strings.Contains(completeOutput, "Err:") || strings.Contains(completeOutput, "E:") {
		// Extract error messages
		var errorLines []string
		lines := strings.Split(completeOutput, "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "E:") || strings.HasPrefix(strings.TrimSpace(line), "Err:") {
				errorLines = append(errorLines, line)
			}
		}

		errorMessage := strings.Join(errorLines, "\n")
		fmt.Fprintf(os.Stderr, "\033[91m%s \033[4msudo apt update\033[0m\033[39m!\n", T("Failed to run"))
		fmt.Fprintf(os.Stderr, "%s\n\033[91m%s\033[39m\n", T("APT reported these errors:"), errorMessage)

		// Print the full output for diagnosis
		fmt.Fprintln(os.Stderr, completeOutput)

		if err != nil {
			return fmt.Errorf("apt update failed with exit code %d: %w", cmd.ProcessState.ExitCode(), err)
		}
		return fmt.Errorf("apt update failed with error messages")
	}

	return nil
}

// RepoRm removes the local apt repository
func RepoRm() error {
	// Wait for other operations to finish before continuing
	// This helps solve cases when the pi-apps local repository was removed unexpectedly by a second process
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for APT locks: %w", err)
	}

	repoPath := "/tmp/pi-apps-local-packages"

	// Try to remove as current user first
	err := os.RemoveAll(repoPath)
	if err != nil {
		// If that fails, try with sudo
		cmd := exec.Command("sudo", "rm", "-rf", repoPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to remove the local repository: %s: %w", repoPath, err)
		}
	}

	// Also remove broken symbolic link to /tmp/pi-apps-local-packages/./Packages
	cmd := exec.Command("sudo", "rm", "-f", "/var/lib/apt/lists/_tmp_pi-apps-local-packages_._Packages")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove broken symbolic link: %w", err)
	}

	return nil
}

// AppToPkgName converts an app-name to a unique, valid package-name that starts with 'pi-apps-'
//
//	"" - error if app is not specified
//	packageName - package name
//	error - error if app is not specified
func AppToPkgName(app string) (string, error) {
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
	// Extract apt flags and process package list
	var aptFlags []string
	var packages []string
	var repoSelection string

	// Process arguments to handle -t flags and collect packages
	for i := 0; i < len(args); i++ {
		if args[i] == "-t" && i+1 < len(args) {
			repoSelection = args[i+1]
			aptFlags = append(aptFlags, "-t", args[i+1])
			i++ // Skip the next argument as it's the repo name
		} else {
			packages = append(packages, args[i])
		}
	}

	StatusT(Tf("Will install these packages: %s", strings.Join(packages, " ")))

	// Remove the local repo, just in case the last operation left it in an unrecoverable state
	if err := RepoRm(); err != nil {
		return fmt.Errorf("failed to remove existing local repository: %w", err)
	}

	// Flag to track if we're using the local packages repository
	usingLocalPackages := false

	// Process packages to handle local files, URLs, and regex
	for i := 0; i < len(packages); i++ {
		pkg := packages[i]

		// Handle local files (package path starts with /)
		if strings.HasPrefix(pkg, "/") {
			// Check if file exists
			if _, err := os.Stat(pkg); os.IsNotExist(err) {
				return fmt.Errorf(T("local package does not exist: %s"), pkg)
			}

			// Get package info using dpkg-deb
			cmd := exec.Command("dpkg-deb", "-I", pkg)
			cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf(T("failed to get package info from %s: %w"), pkg, err)
			}

			// Parse the output to get package name, version, and architecture
			pkgName, pkgVersion, pkgArch := extractPackageInfo(string(output))
			if pkgName == "" {
				return fmt.Errorf(T("failed to determine package name for file: %s"), pkg)
			}
			if pkgVersion == "" {
				return fmt.Errorf(T("failed to determine package version for file: %s"), pkg)
			}
			if pkgArch == "" {
				return fmt.Errorf(T("failed to determine package architecture for file: %s"), pkg)
			}

			// Add architecture suffix if it's a foreign architecture
			cmd = exec.Command("dpkg", "--print-architecture")
			cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
			currentArch, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get current architecture: %w", err)
			}

			currentArchStr := strings.TrimSpace(string(currentArch))
			if pkgArch != currentArchStr && pkgArch != "all" {
				pkgName = fmt.Sprintf("%s:%s", pkgName, pkgArch)
			}

			// Add local package to repository
			if err := RepoAdd(pkg); err != nil {
				return fmt.Errorf("failed to add local package %s to repository: %w", pkg, err)
			}

			usingLocalPackages = true

			// Replace package filename with name of package and version specification
			packages[i] = fmt.Sprintf("%s (>= %s)", pkgName, pkgVersion)

		} else if strings.Contains(pkg, "://") {
			// Handle URLs
			// Extract filename from URL
			filename := filepath.Join("/tmp", filepath.Base(strings.TrimSuffix(pkg, "/download")))

			// Add .deb extension if missing
			if !strings.HasSuffix(filename, ".deb") {
				Status(fmt.Sprintf("%s is not ending with .deb, renaming it to '%s.deb'...", filename, filename))
				filename = filename + ".deb"
			}

			// Download the file with retry
			success := false
			for attempt := 1; attempt <= 3; attempt++ {
				cmd := exec.Command("wget", "-O", filename, pkg)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()

				if err == nil {
					if _, statErr := os.Stat(filename); statErr == nil {
						success = true
						break
					}
				}

				Warning(fmt.Sprintf("Package download failed. (Attempt %d of 3)", attempt))
				os.Remove(filename) // Clean up failed download
			}

			if !success {
				return fmt.Errorf(T("downloaded package does not exist: %s"), filename)
			}

			// Get package info
			cmd := exec.Command("dpkg-deb", "-I", filename)
			cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get package info from %s: %w", filename, err)
			}

			// Parse the output
			pkgName, pkgVersion, pkgArch := extractPackageInfo(string(output))
			if pkgName == "" {
				return fmt.Errorf("failed to determine package name for file: %s", filename)
			}
			if pkgVersion == "" {
				return fmt.Errorf("failed to determine package version for file: %s", filename)
			}
			if pkgArch == "" {
				return fmt.Errorf("failed to determine package architecture for file: %s", filename)
			}

			// Add architecture suffix if needed
			cmd = exec.Command("dpkg", "--print-architecture")
			cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
			currentArch, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get current architecture: %w", err)
			}

			currentArchStr := strings.TrimSpace(string(currentArch))
			if pkgArch != currentArchStr && pkgArch != "all" {
				pkgName = fmt.Sprintf("%s:%s", pkgName, pkgArch)
			}

			// Add to repository
			if err := RepoAdd(filename); err != nil {
				return fmt.Errorf("failed to add downloaded package to repository: %w", err)
			}

			usingLocalPackages = true

			// Replace URL with package name and version
			packages[i] = fmt.Sprintf("%s (>= %s)", pkgName, pkgVersion)

		} else if strings.Contains(pkg, "*") {
			// Handle regex (expand wildcards)
			Status(fmt.Sprintf("Expanding regex in '%s'...", pkg))

			// Use apt-cache to search for matching packages
			searchPattern := strings.ReplaceAll(pkg, "*", "")
			cmd := exec.Command("apt-cache", "search", pkg)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to search for packages with pattern %s: %w", pkg, err)
			}

			// Extract package names from search results
			var expandedPkgs []string
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				fields := strings.SplitN(line, " ", 2)
				if len(fields) > 0 && strings.Contains(fields[0], searchPattern) {
					expandedPkgs = append(expandedPkgs, fields[0])
				}
			}

			// Remove the regex package and append expanded list
			if len(expandedPkgs) > 0 {
				// Remove the current package with regex
				packages = append(packages[:i], packages[i+1:]...)
				// Append the expanded packages
				packages = append(packages, expandedPkgs...)
				// Adjust index since we modified the slice
				i--
			} else {
				return fmt.Errorf(T("no packages found matching pattern: %s"), pkg)
			}

		} else if repoSelection != "" {
			// Handle packages from specific repo with version
			cmd := exec.Command("apt-cache", "policy", "-t", repoSelection, pkg)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get package version for %s from repo %s: %w", pkg, repoSelection, err)
			}

			// Parse output to get version
			pkgVersion := parsePackageVersion(string(output))
			pkgVersion = strings.Split(pkgVersion, "+")[0] // Remove "+..." suffix

			if pkgVersion == "" {
				return fmt.Errorf("failed to get package version for %s, apt-cache output was %s", pkg, string(output))
			}

			// Add version specification
			packages[i] = fmt.Sprintf("%s (>= %s)", pkg, pkgVersion)
		}
	}

	// Verify no regex, URLs, or file paths remain
	for _, pkg := range packages {
		if strings.Contains(pkg, "*") {
			return fmt.Errorf(T("failed to remove all regex from package list: %s"), strings.Join(packages, "\n"))
		}
		if strings.Contains(pkg, "://") {
			return fmt.Errorf(T("failed to remove all URLs from package list: %s"), strings.Join(packages, "\n"))
		}
		if strings.Contains(pkg, "/") && !strings.Contains(pkg, " (>=") {
			return fmt.Errorf(T("failed to remove all filenames from package list: %s"), strings.Join(packages, "\n"))
		}
	}

	// Initialize local repository if needed
	if usingLocalPackages {
		if err := RepoRefresh(); err != nil {
			return fmt.Errorf("failed to refresh local repository: %w", err)
		}

		// Add source list to apt flags
		aptFlags = append(aptFlags, "-o", "Dir::Etc::SourceList=/tmp/pi-apps-local-packages/source.list")
	}

	// Create a unique package name using app_to_pkgname
	pkgName, err := AppToPkgName(app)
	if err != nil {
		return fmt.Errorf("failed to create package name for app %s: %w", app, err)
	}

	StatusTf("Creating an empty apt-package to install the necessary apt packages...\nIt will be named: %s", pkgName)

	// Check if package is already installed and get its dependencies
	pkgInstalled := PackageInstalled(pkgName)
	var existingDeps string

	if pkgInstalled {
		deps, err := PackageDependencies(pkgName)
		if err != nil {
			return fmt.Errorf("failed to get dependencies for existing package %s: %w", pkgName, err)
		}

		existingDeps = strings.Join(deps, ", ")
		StatusTf("The %s package is already installed. Inheriting its dependencies: %s", pkgName, existingDeps)

		// Add existing dependencies to packages list
		if existingDeps != "" {
			packages = append(packages, strings.Split(strings.ReplaceAll(existingDeps, ", ", ","), ",")...)
		}
	}

	// Create temporary directory for the dummy package
	pkgDir := filepath.Join("/tmp", pkgName)
	os.RemoveAll(pkgDir)
	os.RemoveAll(pkgDir + ".deb")

	if err := os.MkdirAll(filepath.Join(pkgDir, "DEBIAN"), 0755); err != nil {
		return fmt.Errorf("failed to create package directory: %w", err)
	}

	// Sort packages and remove duplicates
	uniquePkgs := sortAndDeduplicate(packages)

	// Create control file
	controlContent := fmt.Sprintf(`Maintainer: Pi-Apps Go team
Name: %s
Description: %s
Version: 1.0
Architecture: all
Priority: optional
Section: custom
Depends: %s
Package: %s
`, app, Tf("Dummy package created by pi-apps go to install dependencies for the '%s' app", app), uniquePkgs, pkgName)

	controlFile := filepath.Join(pkgDir, "DEBIAN", "control")
	if err := os.WriteFile(controlFile, []byte(controlContent), 0644); err != nil {
		return fmt.Errorf("failed to create control file: %w", err)
	}

	// Set proper permissions
	cmd := exec.Command("sudo", "chmod", "-R", "00755", pkgDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions on package directory: %w", err)
	}

	// Show the Depends line to user
	depends := ""
	for _, line := range strings.Split(controlContent, "\n") {
		if strings.HasPrefix(line, "Depends: ") {
			depends = line
			break
		}
	}
	fmt.Println(depends)

	// Check if package is already installed with identical control file
	if pkgInstalled {
		// Get current package info
		pkgInfo, err := PackageInfo(pkgName)
		if err != nil {
			return fmt.Errorf("failed to get package info: %w", err)
		}

		// Remove Status line and sort for comparison
		pkgInfoLines := strings.Split(pkgInfo, "\n")
		var filteredPkgInfo []string
		for _, line := range pkgInfoLines {
			if !strings.HasPrefix(line, "Status: ") {
				filteredPkgInfo = append(filteredPkgInfo, line)
			}
		}
		sort.Strings(filteredPkgInfo)

		// Sort control file lines
		controlLines := strings.Split(controlContent, "\n")
		sort.Strings(controlLines)

		// Compare
		if strings.Join(filteredPkgInfo, "\n") == strings.Join(controlLines, "\n") {
			fmt.Printf(T("%s is already installed and no changes would be made. Skipping...\n"), pkgName)

			// Clean up
			os.RemoveAll(pkgDir)
			os.RemoveAll(pkgDir + ".deb")

			// Remove local repo if it was used
			if usingLocalPackages {
				if err := RepoRm(); err != nil {
					return fmt.Errorf("failed to remove local repository: %w", err)
				}
			}

			StatusGreenT("Package installation complete.")
			return nil
		}
	}

	// Build the deb package
	cmd = exec.Command("dpkg-deb", "--build", pkgDir)
	output, err := cmd.CombinedOutput()
	if err != nil || !FileExists(pkgDir+".deb") {
		fmt.Println(string(output))
		return fmt.Errorf("failed to create dummy deb %s: %w", pkgName, err)
	}

	// Check if local repo still exists
	if usingLocalPackages && !FileExists("/tmp/pi-apps-local-packages/Packages") {
		return fmt.Errorf("%s", T("user error: the /tmp/pi-apps-local-packages folder went missing while installing packages - this usually happens if you try to install several apps at the same time in multiple terminals"))
	}

	// Run apt update and install with retry loop
	for i := range 5 {
		// Run apt update
		if err := AptUpdate(aptFlags...); err != nil {
			return err
		}

		// Install dummy deb
		StatusTf("Installing the %s package...", pkgName)

		if err := AptLockWait(); err != nil {
			return fmt.Errorf("failed to wait for APT locks: %w", err)
		}

		// Create command for apt install
		lang := os.Getenv("LANG")
		var installArgs []string

		if lang != "" {
			// Set locale variables before sudo for proper APT localization
			installArgs = []string{"-E", "LANG=" + lang, "LC_ALL=" + lang, "LC_MESSAGES=" + lang, "apt-get", "-o", "DPkg::Lock::Timeout=-1", "install", "-fy", "--no-install-recommends", "--allow-downgrades"}
		} else {
			installArgs = []string{"-E", "apt-get", "-o", "DPkg::Lock::Timeout=-1", "install", "-fy", "--no-install-recommends", "--allow-downgrades"}
		}
		installArgs = append(installArgs, aptFlags...)
		installArgs = append(installArgs, pkgDir+".deb")

		cmd = exec.Command("sudo", installArgs...)

		// Preserve environment variables for proper locale handling
		cmd.Env = os.Environ()

		// Set up pipes for stdout and stderr to capture output in real-time
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// Create a combined reader for both stdout and stderr
		outputReader := io.MultiReader(stdout, stderr)

		// Start the command
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start apt install command: %w", err)
		}

		// Create a buffer to store the complete output
		var outputBuffer bytes.Buffer
		outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

		// Read output and buffer it to process complete lines
		scanner := bufio.NewScanner(outputReader)
		var buffer string
		for scanner.Scan() {
			line := scanner.Text()

			// Add line to our processing buffer
			buffer += line + "\n"

			// If we have a complete line or multiple lines, process them
			if strings.Contains(buffer, "\n") {
				lines := strings.Split(buffer, "\n")
				// Process all complete lines
				for i := 0; i < len(lines)-1; i++ {
					if lines[i] != "" {
						filteredLine := LessApt(lines[i] + "\n")
						if filteredLine != "" {
							fmt.Fprint(outputWriter, filteredLine)
						}
					}
				}
				// Keep any partial line for the next iteration
				buffer = lines[len(lines)-1]
			}
		}

		// Process any remaining content in the buffer
		if buffer != "" {
			filteredLine := LessApt(buffer)
			if filteredLine != "" {
				fmt.Fprint(outputWriter, filteredLine)
			}
		}

		// Wait for the command to complete
		err = cmd.Wait()

		StatusT("Apt finished.")

		// Get the complete output
		combinedOutput := outputBuffer.String()

		// Check if local repo was lost
		if usingLocalPackages && !FileExists("/var/lib/apt/lists/_tmp_pi-apps-local-packages_._Packages") && i < 4 {
			WarningTf("Local packages failed to install because another apt update process erased apt's knowledge of the pi-apps local repository.\nTrying again... (attempt %d of 5)", i+1)
			continue
		}

		// Check for errors
		if err != nil {
			// Extract error lines
			var errorLines []string
			for _, line := range strings.Split(combinedOutput, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "E:") || strings.HasPrefix(line, "Err:") {
					errorLines = append(errorLines, line)
				}
			}

			// Handle error cases
			errorStr := strings.Join(errorLines, "\n")

			if len(errorLines) == 0 {
				fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
				fmt.Printf(T("User error: Apt exited with a failed exitcode (%d) and no error (E/Err) output. "+
					"This could indicate system corruption (eg: storage corruption or unstable overclocking).\n"), cmd.ProcessState.ExitCode())
				return fmt.Errorf(T("apt exited with error code %d and no error output"), cmd.ProcessState.ExitCode())
			} else {
				fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
				fmt.Printf("%s\n\033[91m%s\033[39m\n", T("The APT reported these errors:"), errorStr)

				// Debug output for local repository issues
				if usingLocalPackages && !FileExists("/tmp/pi-apps-local-packages/Packages") {
					fmt.Println("User error: Uh-oh, the /tmp/pi-apps-local-packages folder went missing while installing packages." +
						"\nThis usually happens if you try to install several apps at the same time in multiple terminals.")
				} else if usingLocalPackages && (strings.Contains(combinedOutput, "but it is not installable") ||
					strings.Contains(combinedOutput, "but it is not going to be installed") ||
					strings.Contains(combinedOutput, "but .* is to be installed")) {

					fmt.Printf("\033[91m%s\033[39m", T("The Pi-Apps Local Repository was being used, and a package seemed to not be available. Here's the Packages file:"))
					packagesContent, _ := os.ReadFile("/tmp/pi-apps-local-packages/Packages")
					fmt.Println(string(packagesContent))

					fmt.Println(T("Attempting apt --dry-run installation of the problematic package(s) for debugging purposes:"))

					// Extract problematic packages
					var problemPackages []string
					var pattern = regexp.MustCompile(`Depends: ([^ ]*) but it is not (installable|going to be installed)|(but .* is to be installed)`)
					matches := pattern.FindAllStringSubmatch(combinedOutput, -1)

					for _, match := range matches {
						if len(match) > 1 && match[1] != "" {
							problemPackages = append(problemPackages, match[1])
						}
					}

					if len(problemPackages) > 0 {
						// Run dry-run install
						dryRunArgs := []string{"-E", "apt-get", "install", "-fy", "--no-install-recommends", "--allow-downgrades", "--dry-run"}
						dryRunArgs = append(dryRunArgs, aptFlags...)
						dryRunArgs = append(dryRunArgs, problemPackages...)

						dryRunCmd := exec.Command("sudo", dryRunArgs...)
						dryRunOutput, _ := dryRunCmd.CombinedOutput()
						fmt.Println(string(dryRunOutput))
					}

					fmt.Println(T("Printing apt-cache policy output for debugging purposes:"))
					policyCmd := exec.Command("apt-cache", "policy")
					policyOutput, _ := policyCmd.CombinedOutput()
					fmt.Println(string(policyOutput))
				}

				return fmt.Errorf("apt reported errors: %s", errorStr)
			}
		}

		break // If we get here, installation succeeded
	}

	// Clean up
	os.Remove(pkgDir + ".deb")
	os.RemoveAll(pkgDir)

	// Remove the local repository if it was used
	if usingLocalPackages {
		if err := RepoRm(); err != nil {
			return fmt.Errorf("failed to remove local repository: %w", err)
		}
	}

	StatusGreenT(T("Package installation complete."))
	return nil
}

// Helper functions for InstallPackages

// extractPackageInfo parses dpkg-deb -I output to get package name, version, and architecture
func extractPackageInfo(output string) (name, version, arch string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Package:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "Package:"))
		} else if strings.HasPrefix(line, "Version:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
		} else if strings.HasPrefix(line, "Architecture:") {
			arch = strings.TrimSpace(strings.TrimPrefix(line, "Architecture:"))
		}
	}

	return
}

// parsePackageVersion extracts the version from apt-cache policy output
func parsePackageVersion(output string) string {
	lines := strings.Split(output, "\n")
	var version string

	for i, line := range lines {
		if strings.Contains(line, "Candidate:") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "  Candidate:"))
			break
		}

		// Also try to find version in the version table
		if strings.Contains(line, "***") && i+1 < len(lines) {
			parts := strings.Fields(lines[i+1])
			if len(parts) > 0 {
				version = parts[0]
				break
			}
		}
	}

	return version
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

	// Create a unique package name using app_to_pkgname
	pkgName, err := AppToPkgName(app)
	if err != nil {
		return fmt.Errorf("failed to create package name for app %s: %w", app, err)
	}

	// Check if the dummy package is installed
	if PackageInstalled(pkgName) {
		// Get package dependencies to show the user
		deps, err := PackageDependencies(pkgName)
		if err != nil {
			return fmt.Errorf("failed to get dependencies for package %s: %w", pkgName, err)
		}

		fmt.Print(Tf("These packages were: %s\n", strings.Join(deps, ", ")))
		Status(Tf("Purging the %s package...", pkgName))

		// Wait for APT locks
		if err := AptLockWait(); err != nil {
			return fmt.Errorf("failed to wait for APT locks: %w", err)
		}

		// Create command for apt purge
		lang := os.Getenv("LANG")
		var purgeArgs []string

		if lang != "" {
			// Set locale variables before sudo for proper APT localization
			if isUpdate {
				// Skip --autoremove for faster updates
				purgeArgs = []string{"-E", "LANG=" + lang, "LC_ALL=" + lang, "LC_MESSAGES=" + lang, "apt-get", "purge", "-y", pkgName}
			} else {
				// Normal case, use --autoremove
				purgeArgs = []string{"-E", "LANG=" + lang, "LC_ALL=" + lang, "LC_MESSAGES=" + lang, "apt-get", "purge", "-y", pkgName, "--autoremove"}
			}
		} else {
			if isUpdate {
				// Skip --autoremove for faster updates
				purgeArgs = []string{"-E", "apt-get", "purge", "-y", pkgName}
			} else {
				// Normal case, use --autoremove
				purgeArgs = []string{"-E", "apt-get", "purge", "-y", pkgName, "--autoremove"}
			}
		}

		cmd := exec.Command("sudo", purgeArgs...)

		// Preserve environment variables for proper locale handling
		cmd.Env = os.Environ()

		// Set up pipes for stdout and stderr to capture output in real-time
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// Create a combined reader for both stdout and stderr
		outputReader := io.MultiReader(stdout, stderr)

		// Start the command
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start apt purge command: %w", err)
		}

		// Create a buffer to store the complete output
		var outputBuffer bytes.Buffer
		outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

		// Read output and buffer it to process complete lines
		scanner := bufio.NewScanner(outputReader)
		var buffer string
		for scanner.Scan() {
			line := scanner.Text()

			// Add line to our processing buffer
			buffer += line + "\n"

			// If we have a complete line or multiple lines, process them
			if strings.Contains(buffer, "\n") {
				lines := strings.Split(buffer, "\n")
				// Process all complete lines
				for i := 0; i < len(lines)-1; i++ {
					if lines[i] != "" {
						filteredLine := LessApt(lines[i] + "\n")
						if filteredLine != "" {
							fmt.Fprint(outputWriter, filteredLine)
						}
					}
				}
				// Keep any partial line for the next iteration
				buffer = lines[len(lines)-1]
			}
		}

		// Process any remaining content in the buffer
		if buffer != "" {
			filteredLine := LessApt(buffer)
			if filteredLine != "" {
				fmt.Fprint(outputWriter, filteredLine)
			}
		}

		// Wait for the command to complete
		err = cmd.Wait()

		Status(T("Apt finished."))

		// Get complete output
		combinedOutput := outputBuffer.String()

		// Check for errors
		if err != nil {
			// Extract error lines
			var errorLines []string
			for _, line := range strings.Split(combinedOutput, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "E:") || strings.HasPrefix(line, "Err:") {
					errorLines = append(errorLines, line)
				}
			}

			// Handle error cases
			errorStr := strings.Join(errorLines, "\n")

			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
			fmt.Printf("%s\n\033[91m%s\033[39m\n", T("The APT reported these errors:"), errorStr)
			fmt.Println(combinedOutput)

			return fmt.Errorf("apt reported errors: %s", errorStr)
		}
	} else {
		// Check for legacy installed-packages file
		installDataDir := GetPiAppsDir()
		if installDataDir == "" {
			return fmt.Errorf("failed to get PI_APPS_DIR")
		}

		legacyPkgFile := filepath.Join(installDataDir, "data", "installed-packages", app)

		if FileExists(legacyPkgFile) {
			WarningT(T("Using the old implementation - an installed-packages file instead of a dummy deb"))

			// Read the package list from the file
			pkgData, err := os.ReadFile(legacyPkgFile)
			if err != nil {
				return fmt.Errorf("failed to read legacy package file: %w", err)
			}

			// Convert newlines to spaces and clean up extra spaces
			pkgList := strings.TrimSpace(string(pkgData))
			pkgList = strings.ReplaceAll(pkgList, "\n", " ")

			// Remove multiple spaces
			for strings.Contains(pkgList, "  ") {
				pkgList = strings.ReplaceAll(pkgList, "  ", " ")
			}

			if pkgList == "" {
				StatusT(T("Legacy package file is empty. Nothing to do."))
				// Remove the legacy file
				os.Remove(legacyPkgFile)
				StatusGreenT("All packages have been purged successfully.")
				return nil
			}

			// Convert to slice for the command
			packages := strings.Split(pkgList, " ")

			// Wait for APT locks
			if err := AptLockWait(); err != nil {
				return fmt.Errorf("failed to wait for APT locks: %w", err)
			}

			// Create command for apt purge with real-time output
			lang := os.Getenv("LANG")
			var purgeArgs []string

			if lang != "" {
				// Set locale variables before sudo for proper APT localization
				purgeArgs = append([]string{"-E", "LANG=" + lang, "LC_ALL=" + lang, "LC_MESSAGES=" + lang, "apt-get", "purge", "-y"}, packages...)
			} else {
				purgeArgs = append([]string{"-E", "apt-get", "purge", "-y"}, packages...)
			}

			cmd := exec.Command("sudo", purgeArgs...)

			// Preserve environment variables for proper locale handling
			cmd.Env = os.Environ()

			// Set up pipes for stdout and stderr to capture output in real-time
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create stdout pipe: %w", err)
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return fmt.Errorf("failed to create stderr pipe: %w", err)
			}

			// Create a combined reader for both stdout and stderr
			outputReader := io.MultiReader(stdout, stderr)

			// Start the command
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start apt purge command: %w", err)
			}

			// Create a buffer to store the complete output
			var outputBuffer bytes.Buffer
			outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

			// Read output and buffer it to process complete lines
			scanner := bufio.NewScanner(outputReader)
			var buffer string
			for scanner.Scan() {
				line := scanner.Text()

				// Add line to our processing buffer
				buffer += line + "\n"

				// If we have a complete line or multiple lines, process them
				if strings.Contains(buffer, "\n") {
					lines := strings.Split(buffer, "\n")
					// Process all complete lines
					for i := 0; i < len(lines)-1; i++ {
						if lines[i] != "" {
							filteredLine := LessApt(lines[i] + "\n")
							if filteredLine != "" {
								fmt.Fprint(outputWriter, filteredLine)
							}
						}
					}
					// Keep any partial line for the next iteration
					buffer = lines[len(lines)-1]
				}
			}

			// Process any remaining content in the buffer
			if buffer != "" {
				filteredLine := LessApt(buffer)
				if filteredLine != "" {
					fmt.Fprint(outputWriter, filteredLine)
				}
			}

			// Wait for the command to complete
			err = cmd.Wait()

			Status(T("Apt finished."))

			// Check for errors
			if err != nil {
				// Get complete output
				combinedOutput := outputBuffer.String()

				// Extract error lines
				var errorLines []string
				for _, line := range strings.Split(combinedOutput, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "E:") || strings.HasPrefix(line, "Err:") {
						errorLines = append(errorLines, line)
					}
				}

				errorStr := strings.Join(errorLines, "\n")

				fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
				fmt.Printf("%s\n\033[91m%s\033[39m\n", T("The APT reported these errors:"), errorStr)
				fmt.Println(combinedOutput)

				return fmt.Errorf("apt reported errors: %s", errorStr)
			}
		} else {
			StatusT(Tf("The %s package is not installed so there's nothing to do.", pkgName))
		}
	}

	// Clean up the installed-packages file
	installDataDir := GetPiAppsDir()
	if installDataDir == "" {
		return fmt.Errorf("failed to get PI_APPS_DIR")
	}

	legacyPkgFile := filepath.Join(installDataDir, "data", "installed-packages", app)
	if FileExists(legacyPkgFile) {
		os.Remove(legacyPkgFile)
	}

	StatusGreenT("All packages have been purged successfully.")
	return nil
}

// GetIconFromPackage finds the largest icon file (png or svg) installed by a package
// This is a Go implementation of the original bash get_icon_from_package function
func GetIconFromPackage(packages ...string) (string, error) {
	// Find dependencies that start with the same name as the original packages
	var extraPackages []string
	for _, pkg := range packages {
		// Get dependencies for the package
		deps, err := PackageDependencies(pkg)
		if err != nil {
			// Continue if we can't get dependencies for a package
			continue
		}

		// Filter dependencies that start with the package name
		for _, dep := range deps {
			// Split by any commas and pipes and extract the package name
			parts := strings.FieldsFunc(dep, func(r rune) bool {
				return r == ',' || r == '|'
			})

			for _, part := range parts {
				// Extract just the package name
				pkgName := strings.TrimSpace(part)
				pkgName = strings.Split(pkgName, " ")[0] // Keep only the part before any space

				// Check if it starts with the original package name
				if strings.HasPrefix(pkgName, pkg) {
					extraPackages = append(extraPackages, pkgName)
				}
			}
		}
	}

	// Remove duplicates from extraPackages
	uniqueExtraPackages := make(map[string]bool)
	for _, pkg := range extraPackages {
		uniqueExtraPackages[pkg] = true
	}

	extraPackages = []string{}
	for pkg := range uniqueExtraPackages {
		extraPackages = append(extraPackages, pkg)
	}

	// Combine original packages and extra packages
	allPackages := append(packages, extraPackages...)

	// Run dpkg-query to list all files installed by the packages
	args := append([]string{"-L"}, allPackages...)
	cmd := exec.Command("dpkg-query", args...)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		// Continue even if there's an error, as the bash version ignores errors
	}

	// Filter for .png and .svg files in /icons/ or /pixmaps/ directories, excluding /symbolic/
	var iconFiles []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if it's a PNG or SVG file in the right directories
		if (strings.HasSuffix(line, ".png") || strings.HasSuffix(line, ".svg")) &&
			(strings.Contains(line, "/icons/") || strings.Contains(line, "/pixmaps/")) &&
			!strings.Contains(line, "/symbolic/") {
			iconFiles = append(iconFiles, line)
		}
	}

	if len(iconFiles) == 0 {
		return "", fmt.Errorf("no suitable icon files found")
	}

	// Get file sizes
	var fileSizes []struct {
		Path string
		Size int64
	}

	for _, file := range iconFiles {
		info, err := os.Stat(file)
		if err != nil {
			// Skip files that can't be accessed
			continue
		}

		fileSizes = append(fileSizes, struct {
			Path string
			Size int64
		}{
			Path: file,
			Size: info.Size(),
		})
	}

	// Sort by size in descending order
	sort.Slice(fileSizes, func(i, j int) bool {
		return fileSizes[i].Size > fileSizes[j].Size
	})

	// Return the path of the largest file
	if len(fileSizes) > 0 {
		return fileSizes[0].Path, nil
	}

	return "", fmt.Errorf("no suitable icon files found")
}

// UbuntuPPAInstaller sets up a PPA on an Ubuntu-based distro
// This is a Go implementation of the original bash ubuntu_ppa_installer function
func UbuntuPPAInstaller(ppaName string) error {
	// Prepare ppaGrep for checking if the PPA is already added
	ppaGrep := ppaName
	if !strings.HasSuffix(ppaName, "/") {
		ppaGrep += "/"
	}

	// Check if PPA is already added
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(TARGET_OF)")
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(T("failed to check if PPA is already added: %w"), err)
	}

	ppaAdded := false
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "deb" && strings.Contains(fields[0]+" "+fields[1], ppaGrep) {
			ppaAdded = true
			break
		}
	}

	if ppaAdded {
		Status(Tf("Skipping %s PPA, already added", ppaName))
	} else {
		Status(Tf("Adding %s PPA", ppaName))

		// Add the PPA
		cmd = exec.Command("sudo", "add-apt-repository", "ppa:"+ppaName, "-y")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(T("failed to add PPA: %w"), err)
		}

		// Update APT
		if err := AptUpdate(); err != nil {
			return fmt.Errorf(T("failed to update APT after adding PPA: %w"), err)
		}
	}

	// Check if PPA .list filename exists under the current distro codename
	// On a distro upgrade, the .list filename is not updated and add-apt-repository can re-use the old filename

	// Get the OS codename
	osCodename := VERSION_CODENAME
	if osCodename == "" {
		return fmt.Errorf("failed to determine OS codename: OS codename is empty")
	}

	// Format the standard filename
	ppaOwner := strings.Split(ppaName, "/")[0]
	ppaRepo := strings.Split(ppaName, "/")[1]
	standardFilename := fmt.Sprintf("/etc/apt/sources.list.d/%s-ubuntu-%s-%s.list", ppaOwner, ppaRepo, osCodename)

	// Check if the standard filename exists
	if _, err := os.Stat(standardFilename); os.IsNotExist(err) {
		// Check if there are any matching list files
		pattern := fmt.Sprintf("/etc/apt/sources.list.d/%s-ubuntu-%s-*.list", ppaOwner, ppaRepo)
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			// Get the first matching file
			originalFilename := matches[0]

			// Rename to the standard filename
			cmd = exec.Command("sudo", "mv", originalFilename, standardFilename)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf(T("failed to rename PPA file: %w"), err)
			}

			// Remove related distUpgrade and save files
			cmd = exec.Command("sudo", "rm", "-f", originalFilename+".distUpgrade")
			cmd.Run() // Ignore errors

			cmd = exec.Command("sudo", "rm", "-f", originalFilename+".save")
			cmd.Run() // Ignore errors
		}
	}

	return nil
}

// DebianPPAInstaller sets up a PPA on a Debian-based distro
// This is a Go implementation of the original bash debian_ppa_installer function
func DebianPPAInstaller(ppaName, ppaDist, key string) error {
	// Prepare ppaGrep for checking if the PPA is already added
	ppaGrep := ppaName
	if !strings.HasSuffix(ppaName, "/") {
		ppaGrep = ppaName + "/ubuntu " + ppaDist
	}

	// Check if PPA is already added
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(TARGET_OF)")
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf(T("failed to check if PPA is already added: %w"), err)
	}

	ppaAdded := false
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "deb" && strings.Contains(fields[0]+" "+fields[1], ppaGrep) {
			ppaAdded = true
			break
		}
	}

	if ppaAdded {
		Status(Tf("Skipping %s PPA, already added", ppaName))
	} else {
		Status(Tf("Adding %s PPA", ppaName))

		// Parse owner and repo from ppaName
		parts := strings.SplitN(ppaName, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf(T("invalid PPA name format: %s"), ppaName)
		}
		ppaOwner, ppaRepo := parts[0], parts[1]

		// Prepare filenames
		keyringName := fmt.Sprintf("%s-ubuntu-%s-%s.gpg", ppaOwner, ppaRepo, ppaDist)
		keyringPath := fmt.Sprintf("/etc/apt/keyrings/%s", keyringName)
		listFilename := fmt.Sprintf("/etc/apt/sources.list.d/%s-ubuntu-%s-%s.list", ppaOwner, ppaRepo, ppaDist)
		tmpKeyring := "/tmp/keyring.gpg"

		// Remove any temporary gpg keyring
		os.Remove(tmpKeyring)

		// Fetch the key from keyserver using native Go implementation
		keyData, err := fetchGPGKeyFromKeyserver(key, "keyserver.ubuntu.com")
		if err != nil {
			return fmt.Errorf(T("Failed to retrieve the PPA signing key: %w"), err)
		}

		// Ensure keyring folder exists
		cmd = exec.Command("sudo", "mkdir", "-p", "/etc/apt/keyrings/")
		if err := cmd.Run(); err != nil {
			os.Remove(tmpKeyring)
			return fmt.Errorf(T("Unable to create the keyring folder: %w"), err)
		}

		// Export the key to binary format and write to keyring file
		exportedKeyData, err := exportGPGKey(keyData)
		if err != nil {
			return fmt.Errorf(T("Failed to export key for the %s PPA: %w"), ppaName, err)
		}

		// Write the exported key data to the keyring file
		keyringFile, err := os.Create(keyringPath)
		if err != nil {
			return fmt.Errorf(T("Failed to create keyring file for the %s PPA: %w"), ppaName, err)
		}
		defer keyringFile.Close()

		if _, err := keyringFile.Write(exportedKeyData); err != nil {
			os.Remove(keyringPath) // Clean up on failure
			return fmt.Errorf(T("Failed to write key to keyring for the %s PPA: %w"), ppaName, err)
		}

		// Write the sources.list.d entry with signed-by
		sourcesLine := fmt.Sprintf("deb [signed-by=%s] https://ppa.launchpadcontent.net/%s/ubuntu %s main", keyringPath, ppaName, ppaDist)
		cmd = exec.Command("sudo", "tee", listFilename)
		cmd.Stdin = strings.NewReader(sourcesLine)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(T("Failed to add repository to sources.list: %w"), err)
		}

		// Update APT
		if err := AptUpdate(); err != nil {
			return fmt.Errorf(T("failed to update APT after adding PPA: %w"), err)
		}
	}

	return nil
}

// AddExternalRepo adds an external apt repository and its gpg key
// Follows https://wiki.debian.org/DebianRepository/UseThirdParty specification with deb822 format
func AddExternalRepo(reponame, pubkeyurl, uris, suites, components string, additionalOptions ...string) error {
	// Exit if reponame or uri or suite contains space
	if strings.Contains(reponame, " ") || strings.Contains(uris, " ") || strings.Contains(suites, " ") {
		return fmt.Errorf("add_external_repo: provided reponame, uris, or suites contains a space")
	}

	// Check if links are valid
	fmt.Println("add_external_repo: checking 3rd party pubkeyurl validity")
	resp, err := http.Get(pubkeyurl)
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to reach pubkeyurl: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("add_external_repo: pubkeyurl returned status code %d", resp.StatusCode)
	}

	// Make apt keyring directory if it doesn't exist
	if _, err := os.Stat("/usr/share/keyrings"); os.IsNotExist(err) {
		mkdirCmd := exec.Command("sudo", "mkdir", "-p", "/usr/share/keyrings")
		if err := mkdirCmd.Run(); err != nil {
			return fmt.Errorf("add_external_repo: failed to create apt keyring directory: %w", err)
		}
	}

	// Check if .list file already exists and remove it
	listFile := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", reponame)
	if _, err := os.Stat(listFile); err == nil {
		rmCmd := exec.Command("sudo", "rm", "-f", listFile)
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("add_external_repo: failed to remove conflicting .list file: %w", err)
		}
	}

	// Check if .sources file already exists and remove it
	sourcesFile := fmt.Sprintf("/etc/apt/sources.list.d/%s.sources", reponame)
	if _, err := os.Stat(sourcesFile); err == nil {
		rmCmd := exec.Command("sudo", "rm", "-f", sourcesFile)
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("add_external_repo: failed to remove conflicting .sources file: %w", err)
		}
	}

	// Download gpg key from specified url
	keyringFile := fmt.Sprintf("/usr/share/keyrings/%s-archive-keyring.gpg", reponame)
	if _, err := os.Stat(keyringFile); err == nil {
		rmCmd := exec.Command("sudo", "rm", "-f", keyringFile)
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("add_external_repo: failed to remove existing keyring file: %w", err)
		}
	}

	// Download the key and dearmor it using native Go implementation
	keyResp, err := http.Get(pubkeyurl)
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to download key from %s: %w", pubkeyurl, err)
	}
	defer keyResp.Body.Close()

	if keyResp.StatusCode != http.StatusOK {
		return fmt.Errorf("add_external_repo: failed to download key, got status %d", keyResp.StatusCode)
	}

	keyData, err := io.ReadAll(keyResp.Body)
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to read key data: %w", err)
	}

	// Dearmor the key data
	dearmoredKeyData, err := dearmorGPGKey(keyData)
	if err != nil {
		// Clean up on failure
		os.Remove(sourcesFile)
		os.Remove(keyringFile)
		return fmt.Errorf("add_external_repo: failed to dearmor key data: %w", err)
	}

	// Write the dearmored key data to the keyring file
	keyringFileHandle, err := os.Create(keyringFile)
	if err != nil {
		// Clean up on failure
		os.Remove(sourcesFile)
		os.Remove(keyringFile)
		return fmt.Errorf("add_external_repo: failed to create keyring file: %w", err)
	}
	defer keyringFileHandle.Close()

	if _, err := keyringFileHandle.Write(dearmoredKeyData); err != nil {
		// Clean up on failure
		os.Remove(sourcesFile)
		os.Remove(keyringFile)
		return fmt.Errorf("add_external_repo: failed to write keyring file: %w", err)
	}

	// Create the .sources file
	// First, create the basic content
	content := fmt.Sprintf("Types: deb\nURIs: %s\nSuites: %s\n", uris, suites)

	// Add components if provided
	if components != "" {
		content += fmt.Sprintf("Components: %s\n", components)
	}

	// Add additional options if provided
	for _, option := range additionalOptions {
		content += fmt.Sprintf("%s\n", option)
	}

	// Add the Signed-By line
	content += fmt.Sprintf("Signed-By: %s\n", keyringFile)

	// Write the content to a temporary file
	tempFile, err := os.CreateTemp("", "apt-sources")
	if err != nil {
		// Clean up
		exec.Command("sudo", "rm", "-f", keyringFile).Run()
		return fmt.Errorf("add_external_repo: failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(content); err != nil {
		// Clean up
		exec.Command("sudo", "rm", "-f", keyringFile).Run()
		return fmt.Errorf("add_external_repo: failed to write to temporary file: %w", err)
	}
	tempFile.Close()

	// Move the temporary file to the final location
	mvCmd := exec.Command("sudo", "cp", tempFile.Name(), sourcesFile)
	if err := mvCmd.Run(); err != nil {
		// Clean up
		exec.Command("sudo", "rm", "-f", keyringFile).Run()
		return fmt.Errorf("add_external_repo: failed to move temporary file to sources location: %w", err)
	}

	// Set the correct permissions
	chownCmd := exec.Command("sudo", "chown", "root:root", sourcesFile)
	if err := chownCmd.Run(); err != nil {
		// Not failing for this, just log the error
		fmt.Fprintf(os.Stderr, "Warning: failed to set ownership of sources file: %v\n", err)
	}

	chmodCmd := exec.Command("sudo", "chmod", "644", sourcesFile)
	if err := chmodCmd.Run(); err != nil {
		// Not failing for this, just log the error
		fmt.Fprintf(os.Stderr, "Warning: failed to set permissions of sources file: %v\n", err)
	}

	return nil
}

// RmExternalRepo removes an external apt repository and its gpg key
// If force is true, it removes the repo regardless of whether it's in use
func RmExternalRepo(reponame string, force bool) error {
	// Exit if reponame contains space
	if strings.Contains(reponame, " ") {
		return fmt.Errorf("rm_external_repo: provided reponame contains a space")
	}

	// Always remove deprecated .list file if present
	listFile := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", reponame)
	if _, err := os.Stat(listFile); err == nil {
		rmCmd := exec.Command("sudo", "rm", "-f", listFile)
		if err := rmCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove deprecated list file: %v\n", err)
		}
	}

	// Exit gracefully if .sources file does not exist
	sourcesFile := fmt.Sprintf("/etc/apt/sources.list.d/%s.sources", reponame)
	if _, err := os.Stat(sourcesFile); os.IsNotExist(err) {
		return nil
	}

	keyringFile := fmt.Sprintf("/usr/share/keyrings/%s-archive-keyring.gpg", reponame)

	if force {
		// Force remove the keyring and sources files
		if _, err := os.Stat(keyringFile); err == nil {
			rmKeyCmd := exec.Command("sudo", "rm", "-f", keyringFile)
			if err := rmKeyCmd.Run(); err != nil {
				return fmt.Errorf("rm_external_repo: removal of %s-archive-keyring.gpg failed: %w", reponame, err)
			}
		}

		rmSourcesCmd := exec.Command("sudo", "rm", "-f", sourcesFile)
		if err := rmSourcesCmd.Run(); err != nil {
			return fmt.Errorf("rm_external_repo: removal of %s.sources failed: %w", reponame, err)
		}
	} else {
		// Check if repository is still in use before removing
		if err := RemoveRepofileIfUnused(sourcesFile, "", keyringFile); err != nil {
			return fmt.Errorf("rm_external_repo: %w", err)
		}
	}

	return nil
}

// AdoptiumInstaller sets up the Adoptium repository based on the OS codename
// This is a Go implementation of the original bash adoptium_installer function
func AdoptiumInstaller() error {
	var err error
	// Get the OS codename
	osCodename := VERSION_CODENAME
	if osCodename == "" {
		return fmt.Errorf("failed to determine OS codename: OS codename is empty")
	}

	// Determine if the OS is supported with a specific repository configuration
	switch osCodename {
	case "bionic", "focal", "jammy", "noble", "buster", "bullseye", "bookworm", "trixie":
		// For supported OS versions, use the OS-specific repository
		err = AddExternalRepo(
			"adoptium",
			"https://adoptium.jfrog.io/artifactory/api/security/keypair/default-gpg-key/public",
			"https://adoptium.jfrog.io/artifactory/deb",
			osCodename,
			"main",
		)
	default:
		// For other OS versions, fall back to bionic configuration
		// All supported adoptium OSs use the same debs, so the target specified doesn't actually matter
		err = AddExternalRepo(
			"adoptium",
			"https://adoptium.jfrog.io/artifactory/api/security/keypair/default-gpg-key/public",
			"https://adoptium.jfrog.io/artifactory/deb",
			"bionic",
			"main",
		)
	}

	if err != nil {
		return fmt.Errorf("failed to add Adoptium repository: %w", err)
	}

	// Update the package lists
	if err := AptUpdate(); err != nil {
		// Clean up the repository if apt update fails
		RmExternalRepo("adoptium", true)
		return fmt.Errorf("failed to perform apt update after adding Adoptium repository: %w", err)
	}

	return nil
}

// PackageInstalled checks if a package is installed
func PackageInstalled(packageName string) bool {
	// Use dpkg to check if the package is installed
	// Force English locale to ensure consistent error message parsing
	cmd := exec.Command("dpkg", "-s", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	if err := cmd.Run(); err != nil {
		// Check if it's a specific dpkg error about package not being installed
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			if strings.Contains(stderr, "is not installed and no information is available") {
				return false
			}
		}
		return false
	}

	return true
}

// PackageAvailable determines if the specified package exists in a local repository
func PackageAvailable(packageName string, dpkgArch string) bool {
	// If dpkgArch is not specified, get the current architecture
	if dpkgArch == "" {
		cmd := exec.Command("dpkg", "--print-architecture")
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
		output, err := cmd.Output()
		if err != nil {
			Debug("Error getting dpkg architecture: " + err.Error())
			return false
		}
		dpkgArch = strings.TrimSpace(string(output))
	}

	// Use apt-cache to check if package is available
	// Force English locale to ensure consistent output parsing
	cmd := exec.Command("apt-cache", "policy", packageName+":"+dpkgArch)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		Debug("Error checking if package is available: " + err.Error())
		return false
	}

	// Check if the output contains "Unable to locate package" even with exit code 0
	outputStr := string(output)
	if strings.Contains(outputStr, "Unable to locate package") {
		return false
	}

	// Parse the output to see if a candidate version is available
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Candidate:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			candidate := strings.TrimSpace(parts[1])
			// Package is available if candidate is not empty and not "(none)"
			return candidate != "" && candidate != "(none)"
		}
	}

	// If no Candidate line found, package is not available
	return false
}

// PackageDependencies outputs the list of dependencies for the specified package
//
//	[]string - list of dependencies
//	error - error if package is not specified
func PackageDependencies(packageName string) ([]string, error) {
	// Get package info like the original implementation
	info, err := PackageInfo(packageName)
	if err != nil {
		return nil, err
	}

	// Extract the Depends line from package info
	var deps []string
	for _, line := range strings.Split(info, "\n") {
		if after, ok := strings.CutPrefix(line, "Depends:"); ok {
			// Return the entire dependency line, which includes version requirements
			depLine := strings.TrimSpace(after)
			if depLine != "" {
				return []string{depLine}, nil
			}
			break
		}
	}

	return deps, nil
}

// PackageInstalledVersion returns the installed version of the specified package
//
//	"" - package is not installed
//	version - package is installed
func PackageInstalledVersion(packageName string) (string, error) {
	// Use dpkg to get the installed version
	// Force English locale to ensure consistent output format
	cmd := exec.Command("dpkg-query", "-W", "-f=${Version}", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(T("package %s is not installed"), packageName)
	}

	return strings.TrimSpace(string(output)), nil
}

// PackageLatestVersion returns the latest available version of the specified package
//
//	"" - package is not available
//	version - package is available
func PackageLatestVersion(packageName string, repo ...string) (string, error) {
	// Optional repo selection flags
	var additionalFlags []string
	if len(repo) >= 2 && repo[0] == "-t" {
		additionalFlags = []string{"-t", repo[1]}
	}

	// Get the latest version using apt-cache policy
	// Force English locale to ensure consistent output parsing
	var cmd *exec.Cmd
	if len(additionalFlags) > 0 {
		cmd = exec.Command("apt-cache", append([]string{"policy"}, append(additionalFlags, packageName)...)...)
	} else {
		cmd = exec.Command("apt-cache", "policy", packageName)
	}
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	outputStr := string(output)

	// Check if the package cannot be located
	if strings.Contains(outputStr, "N: Unable to locate package "+packageName) {
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	// Parse the output to extract the Candidate version
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Candidate:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			version := strings.TrimSpace(parts[1])
			// If candidate is "(none)", the package is not available
			if version == "(none)" {
				return "", fmt.Errorf("package %s is not available", packageName)
			}
			return version, nil
		}
	}

	// If no Candidate line found, package is not available
	return "", fmt.Errorf("package %s is not available", packageName)
}

// RefreshAllPkgAppStatus updates the status of all package-apps
func RefreshAllPkgAppStatus() error {
	// Get the PI_APPS_DIR environment variable
	directory := GetPiAppsDir()
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
//
//	architecture - system architecture
//	error - error if dpkg is not installed
func getDpkgArchitecture() (string, error) {
	cmd := exec.Command("dpkg", "--print-architecture")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running dpkg --print-architecture: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getAptCachePolicy runs apt-cache policy for the specified packages
//
//	"" - no packages specified
//	aptCacheOutput - apt cache output
//	error - error if apt-cache policy fails
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
//
//	"" - no packages specified
//	dpkgStatus - dpkg status
//	error - error if dpkg status fails
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
					cat, err := getOriginalCategory(appName)
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
//
//	packageName - package name
//	dpkgStatus - dpkg status
//	false - package is not installed
//	true - package is installed
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

// PackageInfo lists everything dpkg knows about the specified package
func PackageInfo(packageName string) (string, error) {
	// Validate package name to prevent dpkg errors with spaces or invalid characters
	if strings.ContainsAny(packageName, " \t\n\r") {
		return "", fmt.Errorf("package name '%s' contains invalid characters (spaces or whitespace)", packageName)
	}

	// We'll directly use exec.Command to get package info since syspkg doesn't
	// seem to have a direct method for detailed package info
	// Force English locale to ensure consistent error message parsing
	cmd := exec.Command("dpkg", "-s", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's a specific dpkg error about package not being installed
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			if strings.Contains(stderr, "is not installed and no information is available") {
				return "", fmt.Errorf(T("package '%s' is not installed and no information is available"), packageName)
			}
			// for debugging purposes show the output of the command
			Debug("Output of dpkg -s " + packageName + ": " + string(stderr))
		}
		return "", fmt.Errorf("failed to get package info: %w", err)
	}

	return string(output), nil
}
