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

// Module: pacman.go
// Description: Provides functions for managing repositories and packages when using the Pacman package manager.

//go:build pacman

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
	if len(files) == 0 {
		return fmt.Errorf("no files specified")
	}

	// Get system architecture
	arch, err := getDpkgArchitecture()
	if err != nil {
		return fmt.Errorf("failed to get system architecture: %w", err)
	}

	// Create architecture-specific subdirectory
	// Pacman will look for files in /tmp/pi-apps-local-packages/{arch}/
	repoDir := filepath.Join("/tmp/pi-apps-local-packages", arch)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", repoDir, err)
	}

	// Move every mentioned package file to the repository
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

// RepoRefresh indexes the Pi-Apps local pacman repository by creating a database
func RepoRefresh() error {
	// Get system architecture
	arch, err := getDpkgArchitecture()
	if err != nil {
		return fmt.Errorf("failed to get system architecture: %w", err)
	}

	// Use architecture-specific subdirectory
	repoDir := filepath.Join("/tmp/pi-apps-local-packages", arch)

	// Check if the repository directory exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("cannot index the repository - it's missing! %s", repoDir)
	}

	// Check if we have any package files in the repository
	patterns := []string{"*.pkg.tar.zst", "*.pkg.tar.xz", "*.pkg.tar.gz"}
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoDir, pattern))
		if err != nil {
			return fmt.Errorf("failed to list package files: %w", err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		// No packages to index, just return success
		return nil
	}

	// Create a temporary database directory
	dbDir := filepath.Join(repoDir, "pi-apps-local.db")
	os.RemoveAll(dbDir)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Use repo-add to create the repository database
	// repo-add creates a database file in the same directory as the packages
	cmd := exec.Command("repo-add", filepath.Join(repoDir, "pi-apps-local.db.tar.gz"))
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")

	// Add all package files as arguments
	for _, file := range files {
		cmd.Args = append(cmd.Args, filepath.Base(file))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("repo-add failed to index the repository: %s\nCommand output: %s", repoDir, string(output))
		return fmt.Errorf("%s\nError: %w", errorMsg, err)
	}

	// Check if the database file was created
	dbFile := filepath.Join(repoDir, "pi-apps-local.db.tar.gz")
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("repo-add failed to create database file: %s", dbFile)
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}

// AptLockWait waits until other pacman processes are finished before proceeding
func AptLockWait() error {
	// First ensure English locale is added
	AddEnglish()

	// Spawn a goroutine to notify the user after 5 seconds
	notificationDone := make(chan bool)
	notificationShown := make(chan bool)

	go func() {
		select {
		case <-time.After(5 * time.Second):
			fmt.Print(T("Waiting until pacman locks are released... "))
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
	// Pacman uses /var/lib/pacman/db.lck as its lock file
	lockFiles := []string{"/var/lib/pacman/db.lck"}

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

	// Try to run a pacman command to see if it fails due to a lock
	for {
		cmd := exec.Command("sudo", "-E", "pacman", "-Sy", "--noconfirm", "--dbpath", "/tmp/pi-apps-pacman-db-check")
		output, _ := cmd.CombinedOutput()
		outputStr := string(output)

		// Strip ANSI color codes from the output
		outputStr = stripAnsiCodes(outputStr)

		// Check for lock-related messages
		if !strings.Contains(outputStr, "could not lock database") &&
			!strings.Contains(outputStr, "failed to lock database") &&
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

// LessApt filters out unwanted lines from pacman output
//
// This is a helper function for pacman-related operations
func LessApt(input string) string {
	// First, strip ANSI color codes from the input
	input = stripAnsiCodes(input)

	unwantedPatterns := []string{
		":: Synchronizing package databases...",
		":: Starting full system upgrade...",
		":: Processing package changes...",
		":: Loading package files...",
		":: Checking for file conflicts...",
		":: Checking available disk space...",
		":: Installing packages...",
		":: Removing packages...",
		":: Upgrading packages...",
		":: Running pre-transaction hooks...",
		":: Running post-transaction hooks...",
		":: Database directory:",
		":: Retrieving packages...",
		":: Package (",
		":: Total",
		":: Proceed with installation?",
		":: Proceed with removal?",
		":: Proceed with upgrade?",
		":: Downloading",
		":: Checking",
		":: Verifying",
		"^$",
	}

	lines := strings.Split(input, "\n")
	var filteredLines []string

	for _, line := range lines {
		if line == "" {
			continue // Skip empty lines
		}

		keep := true

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

// AptUpdate runs a pacman -Sy update with error-checking and minimal output
func AptUpdate(args ...string) error {
	// Wait for pacman locks to be released first
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for pacman locks: %w", err)
	}

	// Use cyan color with reverse video styling to match the original implementation
	// \033[96m for cyan, \033[7m for reverse video, \033[27m to end reverse, \033[0m to reset all formatting
	fmt.Fprintf(os.Stderr, "\033[96m%s \033[7msudo pacman -Sy\033[27m...\033[0m\n", T("Running"))

	// Prepare the pacman -Sy command
	cmdArgs := []string{"pacman", "-Sy", "--noconfirm"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("sudo", cmdArgs...)

	// Preserve environment variables
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
		return fmt.Errorf("failed to start pacman update command: %w", err)
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
	completeOutput := outputBuffer.String()

	// Show completion message in cyan to match the original
	fmt.Fprintf(os.Stderr, "\033[96m%s\033[0m\n", T("pacman -Sy complete."))

	// Handle errors
	if err != nil {
		// Extract error messages
		var errorLines []string
		lines := strings.Split(completeOutput, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "error") ||
				strings.Contains(strings.ToLower(line), "failed") {
				errorLines = append(errorLines, line)
			}
		}

		errorMessage := strings.Join(errorLines, "\n")
		fmt.Fprintf(os.Stderr, "\033[91m%s \033[4msudo pacman -Sy\033[0m\033[39m!\n", T("Failed to run"))
		if errorMessage != "" {
			fmt.Fprintf(os.Stderr, "%s\n\033[91m%s\033[39m\n", T("Pacman reported these errors:"), errorMessage)
		}

		// Print the full output for diagnosis
		fmt.Fprintln(os.Stderr, completeOutput)

		if err != nil {
			return fmt.Errorf("pacman -Sy failed with exit code %d: %w", cmd.ProcessState.ExitCode(), err)
		}
		return fmt.Errorf("pacman -Sy failed with error messages")
	}

	return nil
}

// RepoRm removes the local pacman repository
func RepoRm() error {
	// Wait for other operations to finish before continuing
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for pacman locks: %w", err)
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

	// Remove the repository entry from /etc/pacman.conf if it exists
	// This is optional - we'll just remove the directory
	// The repository entry would need to be manually removed from pacman.conf

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

	// Remove the local repo, just in case the last operation left it in an unrecoverable state
	if err := RepoRm(); err != nil {
		return fmt.Errorf("failed to remove existing local repository: %w", err)
	}

	// Flag to track if we're using the local packages repository
	usingLocalPackages := false

	// Process packages to handle local files, URLs, and regex
	var packages []string
	for i := 0; i < len(args); i++ {
		pkg := args[i]

		// Handle local files (package path starts with /)
		if strings.HasPrefix(pkg, "/") {
			// Check if file exists
			if _, err := os.Stat(pkg); os.IsNotExist(err) {
				return fmt.Errorf(T("local package does not exist: %s"), pkg)
			}

			// Add local package to repository
			if err := RepoAdd(pkg); err != nil {
				return fmt.Errorf("failed to add local package %s to repository: %w", pkg, err)
			}

			usingLocalPackages = true

			// Extract package name from .pkg.tar.zst or .pkg.tar.xz file
			baseName := filepath.Base(pkg)
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.zst")
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.xz")
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.gz")

			// Extract package name by removing version-release-arch
			// Format: packagename-version-release-arch
			parts := strings.Split(baseName, "-")
			if len(parts) >= 3 {
				// Remove last 2 parts (release and arch)
				pkgName := strings.Join(parts[:len(parts)-2], "-")
				packages = append(packages, pkgName)
			} else {
				packages = append(packages, baseName)
			}

		} else if strings.Contains(pkg, "://") {
			// Handle URLs - download the package file
			filename := filepath.Join("/tmp", filepath.Base(strings.TrimSuffix(pkg, "/download")))

			// Add .pkg.tar.zst extension if missing
			if !strings.HasSuffix(filename, ".pkg.tar.zst") && !strings.HasSuffix(filename, ".pkg.tar.xz") && !strings.HasSuffix(filename, ".pkg.tar.gz") {
				Status(fmt.Sprintf("%s is not ending with .pkg.tar.zst/.pkg.tar.xz/.pkg.tar.gz, renaming it to '%s.pkg.tar.zst'...", filename, filename))
				filename = filename + ".pkg.tar.zst"
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

			// Add to repository
			if err := RepoAdd(filename); err != nil {
				return fmt.Errorf("failed to add downloaded package to repository: %w", err)
			}

			usingLocalPackages = true

			// Extract package name
			baseName := filepath.Base(filename)
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.zst")
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.xz")
			baseName = strings.TrimSuffix(baseName, ".pkg.tar.gz")

			parts := strings.Split(baseName, "-")
			if len(parts) >= 3 {
				pkgName := strings.Join(parts[:len(parts)-2], "-")
				packages = append(packages, pkgName)
			} else {
				packages = append(packages, baseName)
			}

		} else if strings.Contains(pkg, "*") {
			// Handle regex (expand wildcards)
			Status(fmt.Sprintf("Expanding regex in '%s'...", pkg))

			// Use pacman -Ss to search for matching packages
			searchPattern := strings.ReplaceAll(pkg, "*", ".*")
			cmd := exec.Command("pacman", "-Ss", searchPattern)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to search for packages with pattern %s: %w", pkg, err)
			}

			// Extract package names from search results
			var expandedPkgs []string
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "core/") || strings.HasPrefix(line, "extra/") || strings.HasPrefix(line, "community/") {
					// Format: repository/package-name version description
					parts := strings.Fields(line)
					if len(parts) > 0 {
						// Extract package name (remove repository prefix)
						pkgName := strings.Split(parts[0], "/")
						if len(pkgName) > 1 {
							expandedPkgs = append(expandedPkgs, pkgName[1])
						}
					}
				}
			}

			// Remove the regex package and append expanded list
			if len(expandedPkgs) > 0 {
				packages = append(packages, expandedPkgs...)
			} else {
				return fmt.Errorf(T("no packages found matching pattern: %s"), pkg)
			}

		} else {
			// Regular package name
			packages = append(packages, pkg)
		}
	}

	// Initialize local repository if needed
	if usingLocalPackages {
		if err := RepoRefresh(); err != nil {
			return fmt.Errorf("failed to refresh local repository: %w", err)
		}

		// Update pacman database to include our local repo
		if err := AptUpdate(); err != nil {
			return fmt.Errorf("failed to update pacman database: %w", err)
		}
	}

	// Wait for pacman locks
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for pacman locks: %w", err)
	}

	// Install packages with pacman
	StatusTf("Installing packages: %s", strings.Join(packages, " "))

	installArgs := []string{"pacman", "-S", "--noconfirm", "--needed"}
	installArgs = append(installArgs, packages...)
	cmd := exec.Command("sudo", installArgs...)

	// Preserve environment variables
	cmd.Env = os.Environ()

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacman install command: %w", err)
	}

	// Create a buffer to store the complete output
	var outputBuffer bytes.Buffer
	outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

	// Read output and filter it
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		line := scanner.Text()
		filteredLine := LessApt(line + "\n")
		if filteredLine != "" {
			fmt.Fprint(outputWriter, filteredLine)
		}
	}

	// Wait for the command to complete
	err = cmd.Wait()

	StatusT("Pacman finished.")

	// Get the complete output
	combinedOutput := outputBuffer.String()

	// Check for errors
	if err != nil {
		// Extract error lines
		var errorLines []string
		for _, line := range strings.Split(combinedOutput, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(strings.ToLower(line), "error") ||
				strings.Contains(strings.ToLower(line), "failed") {
				errorLines = append(errorLines, line)
			}
		}

		errorStr := strings.Join(errorLines, "\n")

		if len(errorLines) > 0 {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
			fmt.Printf("%s\n\033[91m%s\033[39m\n", T("Pacman reported these errors:"), errorStr)
		} else {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
			fmt.Printf(T("Pacman exited with error code %d\n"), cmd.ProcessState.ExitCode())
		}

		fmt.Println(combinedOutput)

		if len(errorLines) > 0 {
			return fmt.Errorf("pacman reported errors: %s", errorStr)
		}
		return fmt.Errorf("pacman install failed with exit code %d", cmd.ProcessState.ExitCode())
	}

	// Create a tracking file to remember which packages were installed for this app
	piAppsDir := GetPiAppsDir()
	trackingDir := filepath.Join(piAppsDir, "data", "installed-packages")
	if err := os.MkdirAll(trackingDir, 0755); err != nil {
		WarningTf("Failed to create tracking directory: %v", err)
	} else {
		trackingFile := filepath.Join(trackingDir, app)
		content := strings.Join(packages, "\n")
		if err := os.WriteFile(trackingFile, []byte(content), 0644); err != nil {
			WarningTf("Failed to write tracking file: %v", err)
		}
	}

	// Clean up local repository if it was used
	if usingLocalPackages {
		if err := RepoRm(); err != nil {
			return fmt.Errorf("failed to remove local repository: %w", err)
		}
	}

	StatusGreenT("Package installation complete.")
	return nil
}

// Helper functions for InstallPackages

// extractPackageInfo parses pacman package info to get package name, version, and architecture
func extractPackageInfo(output string) (name, version, arch string) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				name = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				version = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "Architecture") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				arch = strings.TrimSpace(parts[1])
			}
		}
	}

	return
}

// parsePackageVersion extracts the version from pacman -Si output
func parsePackageVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
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

// PurgePackages allows dependencies of the specified app to be removed
// This is a Go implementation of the original bash purge_packages function
func PurgePackages(app string, isUpdate bool) error {
	Status(Tf("Allowing packages required by the %s app to be uninstalled", app))

	// Get PI_APPS_DIR
	piAppsDir := GetPiAppsDir()

	// Check for tracking file
	trackingFile := filepath.Join(piAppsDir, "data", "installed-packages", app)

	if !FileExists(trackingFile) {
		// No tracking file, nothing to purge
		StatusT("No packages tracking file found. Nothing to purge.")
		StatusGreenT("All packages have been purged successfully.")
		return nil
	}

	// Read the package list from the tracking file
	pkgData, err := os.ReadFile(trackingFile)
	if err != nil {
		return fmt.Errorf("failed to read package tracking file: %w", err)
	}

	// Convert newlines to spaces and clean up
	pkgList := strings.TrimSpace(string(pkgData))
	pkgList = strings.ReplaceAll(pkgList, "\n", " ")

	// Remove multiple spaces
	for strings.Contains(pkgList, "  ") {
		pkgList = strings.ReplaceAll(pkgList, "  ", " ")
	}

	if pkgList == "" {
		StatusT("Package tracking file is empty. Nothing to do.")
		// Remove the tracking file
		os.Remove(trackingFile)
		StatusGreenT("All packages have been purged successfully.")
		return nil
	}

	// Convert to slice
	packages := strings.Split(pkgList, " ")

	fmt.Print(Tf("These packages were: %s\n", strings.Join(packages, ", ")))
	StatusT("Purging packages...")

	// Filter out packages that aren't actually installed
	var installedPackages []string
	for _, pkg := range packages {
		if PackageInstalled(pkg) {
			installedPackages = append(installedPackages, pkg)
		}
	}

	if len(installedPackages) == 0 {
		StatusT("No packages are currently installed. Nothing to do.")
		// Remove the tracking file
		os.Remove(trackingFile)
		StatusGreenT("All packages have been purged successfully.")
		return nil
	}

	// Wait for pacman locks
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for pacman locks: %w", err)
	}

	// Remove packages with pacman
	delArgs := []string{"pacman", "-R", "--noconfirm"}
	if !isUpdate {
		// Add --nosave to remove configuration files too
		delArgs = append(delArgs, "--nosave")
	}
	delArgs = append(delArgs, installedPackages...)
	cmd := exec.Command("sudo", delArgs...)
	cmd.Env = os.Environ()

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pacman remove command: %w", err)
	}

	// Create a buffer to store the complete output
	var outputBuffer bytes.Buffer
	outputWriter := io.MultiWriter(os.Stderr, &outputBuffer)

	// Read output and filter it
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		line := scanner.Text()
		filteredLine := LessApt(line + "\n")
		if filteredLine != "" {
			fmt.Fprint(outputWriter, filteredLine)
		}
	}

	// Wait for the command to complete
	err = cmd.Wait()

	StatusT("Pacman finished.")

	combinedOutput := outputBuffer.String()

	// Check for errors
	if err != nil {
		// Extract error lines
		var errorLines []string
		for _, line := range strings.Split(combinedOutput, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(strings.ToLower(line), "error") ||
				strings.Contains(strings.ToLower(line), "failed") {
				errorLines = append(errorLines, line)
			}
		}

		errorStr := strings.Join(errorLines, "\n")

		if len(errorLines) > 0 {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
			fmt.Printf("%s\n\033[91m%s\033[39m\n", T("Pacman reported these errors:"), errorStr)
		} else {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
			fmt.Printf(T("Pacman exited with error code %d\n"), cmd.ProcessState.ExitCode())
		}

		fmt.Println(combinedOutput)

		if len(errorLines) > 0 {
			return fmt.Errorf("pacman reported errors: %s", errorStr)
		}
		return fmt.Errorf("pacman remove failed with exit code %d", cmd.ProcessState.ExitCode())
	}

	// Remove the tracking file
	if err := os.Remove(trackingFile); err != nil {
		WarningTf("Failed to remove tracking file: %v", err)
	}

	StatusGreenT("All packages have been purged successfully.")
	return nil
}

// GetIconFromPackage finds the largest icon file (png or svg) installed by a package
// This is a Go implementation of the original bash get_icon_from_package function
func GetIconFromPackage(packages ...string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("get_icon_from_package requires at least one pacman package name")
	}

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
			// Split by any commas and extract the package name
			parts := strings.Split(dep, ",")

			for _, part := range parts {
				// Extract just the package name
				pkgName := strings.TrimSpace(part)
				pkgName = strings.Fields(pkgName)[0] // Keep only the first word

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

	// Collect all files from all packages using pacman -Ql or pacman -Fl
	var iconFiles []string
	for _, pkg := range allPackages {
		// Try installed packages first
		cmd := exec.Command("pacman", "-Ql", pkg)
		cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
		output, err := cmd.Output()
		if err != nil {
			// Package not installed, try sync database
			cmd = exec.Command("pacman", "-Fl", pkg)
			cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
			output, err = cmd.Output()
			if err != nil {
				// Continue even if there's an error
				continue
			}
		}

		// Parse output and filter for icon files
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Extract file path (last field)
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			filePath := parts[len(parts)-1]

			// Check if it's a PNG or SVG file in the right directories
			if (strings.HasSuffix(filePath, ".png") || strings.HasSuffix(filePath, ".svg")) &&
				(strings.Contains(filePath, "/icons/") || strings.Contains(filePath, "/pixmaps/")) &&
				!strings.Contains(filePath, "/symbolic/") {
				iconFiles = append(iconFiles, filePath)
			}
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

	if len(fileSizes) == 0 {
		return "", fmt.Errorf("no suitable icon files found")
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

// ensureYayInstalled ensures yay (AUR helper) is installed
// If yay is not installed, it will be cloned from AUR, built, and installed
func ensureYayInstalled() error {
	// Check if yay is already installed
	if commandExists("yay") {
		return nil
	}

	StatusT("yay (AUR helper) is not installed. Installing yay...")

	// Create a temporary directory for building yay
	tmpDir := filepath.Join("/tmp", "yay-build")
	os.RemoveAll(tmpDir) // Clean up any existing directory
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up after we're done

	// Clone yay from AUR
	cloneCmd := exec.Command("git", "clone", "https://aur.archlinux.org/yay.git", tmpDir)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone yay from AUR: %w", err)
	}

	// Build yay using makepkg (run as regular user, not sudo)
	makepkgCmd := exec.Command("makepkg", "-s", "--noconfirm")
	makepkgCmd.Dir = tmpDir
	makepkgCmd.Stdout = os.Stdout
	makepkgCmd.Stderr = os.Stderr
	if err := makepkgCmd.Run(); err != nil {
		return fmt.Errorf("failed to build yay: %w", err)
	}

	// Find the built package file
	pkgFiles, err := filepath.Glob(filepath.Join(tmpDir, "yay-*.pkg.tar.zst"))
	if err != nil || len(pkgFiles) == 0 {
		// Try .pkg.tar.xz as fallback
		pkgFiles, err = filepath.Glob(filepath.Join(tmpDir, "yay-*.pkg.tar.xz"))
		if err != nil || len(pkgFiles) == 0 {
			return fmt.Errorf("failed to find built yay package file")
		}
	}

	// Install the package using sudo pacman
	installCmd := exec.Command("sudo", "pacman", "-U", "--noconfirm", pkgFiles[0])
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install yay: %w", err)
	}

	// Verify yay is now installed
	if !commandExists("yay") {
		return fmt.Errorf("yay installation completed but command not found")
	}

	StatusGreenT("yay installed successfully")
	return nil
}

// UbuntuPPAInstaller installs an AUR package (equivalent to Ubuntu PPA)
// On Arch Linux, PPAs are replaced by AUR packages
// This function installs the package from AUR using yay or makepkg
func UbuntuPPAInstaller(ppaName string) error {
	// On Arch, treat ppaName as an AUR package name
	// Format is typically "owner/repo" which matches AUR package naming
	aurPackage := ppaName

	// Check if package is already installed
	if PackageInstalled(aurPackage) {
		Status(Tf("Skipping %s AUR package, already installed", aurPackage))
		return nil
	}

	Status(Tf("Installing %s from AUR", aurPackage))

	// Ensure yay is installed
	if err := ensureYayInstalled(); err != nil {
		return err
	}

	// Install the AUR package using yay
	installCmd := exec.Command("yay", "-S", "--noconfirm", aurPackage)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf(T("failed to install AUR package %s: %w"), aurPackage, err)
	}

	Status(Tf("Successfully installed %s from AUR", aurPackage))
	return nil
}

// DebianPPAInstaller installs an AUR package (equivalent to Debian PPA)
// On Arch Linux, PPAs are replaced by AUR packages
// ppaDist and key parameters are ignored as they're not applicable to AUR
func DebianPPAInstaller(ppaName, ppaDist, key string) error {
	// On Arch, treat ppaName as an AUR package name
	// ppaDist and key are ignored (not applicable to AUR)
	aurPackage := ppaName

	// Check if package is already installed
	if PackageInstalled(aurPackage) {
		Status(Tf("Skipping %s AUR package, already installed", aurPackage))
		return nil
	}

	Status(Tf("Installing %s from AUR", aurPackage))

	// Ensure yay is installed
	if err := ensureYayInstalled(); err != nil {
		return err
	}

	// Install the AUR package using yay
	installCmd := exec.Command("yay", "-S", "--noconfirm", aurPackage)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf(T("failed to install AUR package %s: %w"), aurPackage, err)
	}

	Status(Tf("Successfully installed %s from AUR", aurPackage))
	return nil
}

// AddExternalRepo adds an external package manager repository
func AddExternalRepo(reponame, pubkeyurl, uris, suites, components string, additionalOptions ...string) error {
	// Exit if reponame or uri or suite contains space
	if strings.Contains(reponame, " ") || strings.Contains(uris, " ") || strings.Contains(suites, " ") {
		return fmt.Errorf("add_external_repo: provided reponame, uris, or suites contains a space")
	}

	// Check if pubkeyurl is provided and valid
	if pubkeyurl != "" {
		Status(T("add_external_repo: checking 3rd party pubkeyurl validity"))
		resp, err := http.Get(pubkeyurl)
		if err != nil {
			return fmt.Errorf("add_external_repo: failed to reach pubkeyurl: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("add_external_repo: pubkeyurl returned status code %d", resp.StatusCode)
		}

		// Download the key
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

		// Import the key using pacman-key
		// Write key to temporary file first
		tempKeyFile, err := os.CreateTemp("", "pacman-key-*.asc")
		if err != nil {
			return fmt.Errorf("add_external_repo: failed to create temporary key file: %w", err)
		}
		defer os.Remove(tempKeyFile.Name())

		if _, err := tempKeyFile.Write(keyData); err != nil {
			tempKeyFile.Close()
			return fmt.Errorf("add_external_repo: failed to write key data: %w", err)
		}
		tempKeyFile.Close()

		// Import the key
		Status(T("add_external_repo: importing GPG key"))
		importCmd := exec.Command("sudo", "pacman-key", "--add", tempKeyFile.Name())
		importCmd.Stdout = os.Stdout
		importCmd.Stderr = os.Stderr
		if err := importCmd.Run(); err != nil {
			return fmt.Errorf("add_external_repo: failed to import key: %w", err)
		}

		// Extract key ID from the key data to sign it
		// Try to parse the key to get its fingerprint
		key, err := crypto.NewKey(keyData)
		if err == nil {
			fingerprint := key.GetFingerprint()
			if fingerprint != "" {
				// Sign the key (trust it locally)
				Status(T("add_external_repo: signing GPG key"))
				signCmd := exec.Command("sudo", "pacman-key", "--lsign-key", fingerprint)
				signCmd.Stdout = os.Stdout
				signCmd.Stderr = os.Stderr
				if err := signCmd.Run(); err != nil {
					// Non-fatal, but log it
					Warning(fmt.Sprintf("add_external_repo: failed to sign key (non-fatal): %v", err))
				}
			}
		}
	}

	// Check if repository already exists in pacman.conf
	pacmanConf := "/etc/pacman.conf"
	content, err := os.ReadFile(pacmanConf)
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to read %s: %w", pacmanConf, err)
	}

	// Check if repository section already exists
	if strings.Contains(string(content), fmt.Sprintf("[%s]", reponame)) {
		Status(Tf("Repository [%s] already exists in %s", reponame, pacmanConf))
		return nil
	}

	// Prepare the repository section
	// Format: Server = <uris>/$repo/os/$arch
	// Note: suites and components are APT-specific and ignored for pacman
	repoSection := fmt.Sprintf("\n[%s]\n", reponame)
	repoSection += fmt.Sprintf("Server = %s/$repo/os/$arch\n", strings.TrimSuffix(uris, "/"))

	// Add additional options if provided
	for _, option := range additionalOptions {
		repoSection += fmt.Sprintf("%s\n", option)
	}

	// Append the repository section to pacman.conf
	// Read existing content
	lines := strings.Split(string(content), "\n")

	// Find a good place to insert (after the last repository section or at the end)
	insertPos := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		trimmedLine := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
			// Found the last repository section, insert after it
			insertPos = i + 1
			// Find the end of this section (next [section] or end of file)
			for j := i + 1; j < len(lines); j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextTrimmed, "[") && strings.HasSuffix(nextTrimmed, "]") {
					insertPos = j
					break
				}
			}
			break
		}
	}

	// Insert the new repository section
	newLines := make([]string, 0, len(lines)+10)
	newLines = append(newLines, lines[:insertPos]...)
	newLines = append(newLines, "")
	newLines = append(newLines, strings.Split(strings.TrimSuffix(repoSection, "\n"), "\n")...)
	newLines = append(newLines, lines[insertPos:]...)

	// Write to temporary file
	tempFile, err := os.CreateTemp("", "pacman-conf")
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	newContent := strings.Join(newLines, "\n")
	if _, err := tempFile.WriteString(newContent); err != nil {
		tempFile.Close()
		return fmt.Errorf("add_external_repo: failed to write to temporary file: %w", err)
	}
	tempFile.Close()

	// Copy the temporary file to /etc/pacman.conf using sudo
	Status(Tf("add_external_repo: adding repository [%s] to %s", reponame, pacmanConf))
	cpCmd := exec.Command("sudo", "cp", tempFile.Name(), pacmanConf)
	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("add_external_repo: failed to update %s: %w", pacmanConf, err)
	}

	// Set proper permissions
	chmodCmd := exec.Command("sudo", "chmod", "644", pacmanConf)
	chmodCmd.Run() // Ignore errors for chmod

	// Update pacman database
	if err := AptUpdate(); err != nil {
		return fmt.Errorf("add_external_repo: failed to update pacman database: %w", err)
	}

	return nil
}

// RmExternalRepo removes an external package manager repository
// If force is true, it removes the repo regardless of whether it's in use
func RmExternalRepo(reponame string, force bool) error {
	// Exit if reponame contains space
	if strings.Contains(reponame, " ") {
		return fmt.Errorf("rm_external_repo: provided reponame contains a space")
	}

	pacmanConf := "/etc/pacman.conf"

	if force {
		// Force remove the repository section from pacman.conf
		content, err := os.ReadFile(pacmanConf)
		if err != nil {
			return fmt.Errorf("rm_external_repo: failed to read %s: %w", pacmanConf, err)
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
				if sectionName == reponame {
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
					// Empty line might be part of the section, skip it
					continue
				}
			}

			// Keep all other lines
			newLines = append(newLines, line)
		}

		if !removedSection {
			// Repository not found, that's okay
			return nil
		}

		// Write the modified content back
		newContent := strings.Join(newLines, "\n")
		tempFile, err := os.CreateTemp("", "pacman-conf")
		if err != nil {
			return fmt.Errorf("rm_external_repo: failed to create temporary file: %w", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(newContent); err != nil {
			tempFile.Close()
			return fmt.Errorf("rm_external_repo: failed to write to temporary file: %w", err)
		}
		tempFile.Close()

		// Copy the temporary file to /etc/pacman.conf using sudo
		Status(Tf("rm_external_repo: removing repository [%s] from %s", reponame, pacmanConf))
		cpCmd := exec.Command("sudo", "cp", tempFile.Name(), pacmanConf)
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("rm_external_repo: failed to update %s: %w", pacmanConf, err)
		}

		// Set proper permissions
		chmodCmd := exec.Command("sudo", "chmod", "644", pacmanConf)
		chmodCmd.Run() // Ignore errors for chmod
	} else {
		// Check if repository is still in use before removing
		// Use RemoveRepofileIfUnused with the repository name
		if err := RemoveRepofileIfUnused(reponame, "", ""); err != nil {
			return fmt.Errorf("rm_external_repo: %w", err)
		}
	}

	return nil
}

// AdoptiumInstaller sets up Adoptium JDK using AUR package
// This installs yay (AUR helper) if needed, then installs jdk-temurin from AUR
func AdoptiumInstaller() error {
	// Ensure yay is installed
	if err := ensureYayInstalled(); err != nil {
		return err
	}

	// Try to find and install the latest available jdk-temurin version
	// Try versions in order: jdk-temurin, jdk21-temurin, jdk17-temurin, jdk11-temurin, jdk8-temurin
	StatusT("Checking available jdk-temurin versions in AUR...")
	jdkVersions := []string{"jdk-temurin", "jdk21-temurin", "jdk17-temurin", "jdk11-temurin", "jdk8-temurin"}

	var availableJDK string
	for _, jdk := range jdkVersions {
		// Check if package is available in AUR using yay
		checkCmd := exec.Command("yay", "-Si", jdk)
		if err := checkCmd.Run(); err == nil {
			availableJDK = jdk
			break
		}
	}

	if availableJDK == "" {
		return fmt.Errorf("no jdk-temurin package found in AUR")
	}

	// Check if already installed
	if PackageInstalled(availableJDK) {
		StatusTf("%s is already installed", availableJDK)
		return nil
	}

	StatusTf("Installing %s from AUR...", availableJDK)

	// Install jdk-temurin using yay
	installCmd := exec.Command("yay", "-S", "--noconfirm", availableJDK)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", availableJDK, err)
	}

	StatusGreenTf("%s installed successfully", availableJDK)

	// Optionally set JAVA_HOME
	// Find the java installation directory
	javaPath, err := exec.Command("which", "java").Output()
	if err == nil && len(javaPath) > 0 {
		// Try to determine JAVA_HOME from the java binary location
		// Arch typically installs JDK in /usr/lib/jvm/
		javaHomeHint := "/usr/lib/jvm/temurin-" + strings.TrimPrefix(availableJDK, "jdk-temurin-")
		if availableJDK == "jdk-temurin" {
			javaHomeHint = "/usr/lib/jvm/temurin"
		}
		if _, err := os.Stat(javaHomeHint); err == nil {
			StatusTf("Java installed. JAVA_HOME can be set to: %s", javaHomeHint)
		} else {
			// Try to find any temurin directory
			entries, err := os.ReadDir("/usr/lib/jvm/")
			if err == nil {
				for _, entry := range entries {
					if strings.Contains(entry.Name(), "temurin") {
						StatusTf("Java installed. JAVA_HOME can be set to: /usr/lib/jvm/%s", entry.Name())
						break
					}
				}
			}
		}
	}

	return nil
}

// PackageInstalled checks if a package is installed
func PackageInstalled(packageName string) bool {
	// Use pacman -Q to check if the package is installed
	// Force English locale to ensure consistent output
	cmd := exec.Command("pacman", "-Q", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	err := cmd.Run()
	return err == nil
}

// PackageAvailable determines if the specified package exists in a repository
func PackageAvailable(packageName string, dpkgArch string) bool {
	// Special handling for "init" package check

	if packageName == "init" {
		// Arch systems by default use systemd, however other Arch like systems (like Artix Linux) use different init systems
		// so check for those also
		initSystems := []string{
			"openrc",  // Artix Linux default
			"systemd", // Arch Linux default
			"dinit",   // Selectable as init option in Artix Linux
			"s6",      // Selectable as init option in Artix Linux
			"runit",   // Selectable as init option in Artix Linux
		}

		for _, initPkg := range initSystems {
			if PackageInstalled(initPkg) {
				return true
			}
		}

		// If none are installed but available, check availability
		for _, initPkg := range initSystems {
			cmd := exec.Command("pacman", "-Ss", initPkg)
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				return true
			}
		}
		return false
	}

	// Use pacman -Ss to search for the package in repositories
	// Force English locale to ensure consistent output
	cmd := exec.Command("pacman", "-Ss", "^"+packageName+"$")
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if output contains the package name
	outputStr := string(output)
	return strings.Contains(outputStr, packageName+" ")
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

	// Extract the Depends On line from package info
	var deps []string
	for _, line := range strings.Split(info, "\n") {
		if after, ok := strings.CutPrefix(line, "Depends On"); ok {
			// Return the entire dependency line, which includes version requirements
			depLine := strings.TrimSpace(after)
			if depLine != "" {
				// Split by spaces and filter out empty strings
				depsList := strings.Fields(depLine)
				return depsList, nil
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
	// Use pacman -Q to get the installed version
	// Force English locale to ensure consistent output
	cmd := exec.Command("pacman", "-Q", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(T("package %s is not installed"), packageName)
	}

	// Parse output: "package-name version"
	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return parts[1], nil
	}

	return "", fmt.Errorf(T("package %s is not installed"), packageName)
}

// PackageLatestVersion returns the latest available version of the specified package
//
//	"" - package is not available
//	version - package is available
func PackageLatestVersion(packageName string, repo ...string) (string, error) {
	// Use pacman -Si to get the latest available version
	// Force English locale to ensure consistent output
	cmd := exec.Command("pacman", "-Si", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	// Parse output to find Version line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Version") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				version := strings.TrimSpace(parts[1])
				return version, nil
			}
		}
	}

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

	// Get system architecture (for future use)
	_, err = getDpkgArchitecture()
	if err != nil {
		return fmt.Errorf("error getting pacman architecture: %w", err)
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

	// Get availability info for all packages at once
	pacmanCacheOutput, err := getAptCachePolicy(allPackages)
	if err != nil {
		return fmt.Errorf("error getting pacman cache info: %w", err)
	}

	// Get installed package status
	pacmanInstalledStatus, err := getDpkgStatus(allPackages)
	if err != nil {
		return fmt.Errorf("error getting pacman installed status: %w", err)
	}

	// Use the collected data to refresh status for each package app
	for _, app := range packageApps {
		// Update through our own implementation rather than calling RefreshPkgAppStatus
		// to avoid re-querying package status information
		err := refreshPackageAppStatusWithCache(app, pacmanCacheOutput, pacmanInstalledStatus, directory)
		if err != nil {
			Debug(fmt.Sprintf("Error refreshing status for %s: %v", app, err))
		}
	}

	return nil
}

// getDpkgArchitecture gets the current system architecture from pacman
//
//	architecture - system architecture
//	error - error if pacman is not installed
func getDpkgArchitecture() (string, error) {
	// redirect to GetSystemArchitecture() (pacman does not have a direct command to get the architecture)
	return GetSystemArchitecture()
}

// getAptCachePolicy runs pacman -Si for the specified packages
//
//	"" - no packages specified
//	packageManagerCachePolicyOutput - package manager cache policy output
//	error - error if pacman -Si fails
func getAptCachePolicy(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	var allOutput strings.Builder

	// Query each package individually
	for _, pkg := range packages {
		cmd := exec.Command("pacman", "-Si", pkg)
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
		output, err := cmd.Output()
		if err != nil {
			// Package not available, continue
			continue
		}
		allOutput.WriteString(string(output))
		allOutput.WriteString("\n")
	}

	return allOutput.String(), nil
}

// getDpkgStatus gets the status of the specified packages from pacman
//
//	"" - no packages specified
//	dpkgStatus - pacman status
//	error - error if pacman -Qi fails
func getDpkgStatus(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	var allOutput strings.Builder

	// Query each package individually
	for _, pkg := range packages {
		cmd := exec.Command("pacman", "-Qi", pkg)
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
		output, err := cmd.Output()
		if err != nil {
			// Package not installed, continue
			continue
		}
		allOutput.WriteString(string(output))
		allOutput.WriteString("\n")
	}

	return allOutput.String(), nil
}

// refreshPackageAppStatusWithCache refreshes a single package-app status using cached pacman data
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

// isPackageInstalledFromStatus checks if a package is installed by looking at the pacman status
//
//	packageName - package name
//	dpkgStatus - pacman status
//	false - package is not installed
//	true - package is installed
func isPackageInstalledFromStatus(packageName, dpkgStatus string) bool {
	// Look for the package and check if it's installed
	packageSection := fmt.Sprintf("Name            : %s\n", packageName)
	index := strings.Index(dpkgStatus, packageSection)
	if index == -1 {
		return false
	}

	// Check a reasonable section after the package name
	sectionEnd := index + 200
	if sectionEnd > len(dpkgStatus) {
		sectionEnd = len(dpkgStatus)
	}
	statusSection := dpkgStatus[index:sectionEnd]

	// If we found the package name, it's installed
	return strings.Contains(statusSection, packageName)
}

// isPackageAvailableFromPolicy checks if a package is available in repositories
func isPackageAvailableFromPolicy(packageName, aptCacheOutput string) bool {
	// Look for the package and check if there's a version
	packageSection := fmt.Sprintf("Name            : %s\n", packageName)
	index := strings.Index(aptCacheOutput, packageSection)
	if index == -1 {
		return false
	}

	// Check a reasonable section after the package name for Version line
	sectionEnd := index + 300
	if sectionEnd > len(aptCacheOutput) {
		sectionEnd = len(aptCacheOutput)
	}
	sectionText := aptCacheOutput[index:sectionEnd]

	// Package is available if there's a Version line
	return strings.Contains(sectionText, "Version")
}

// PackageInfo lists everything the package manager knows about the specified package
func PackageInfo(packageName string) (string, error) {
	// Validate package name to prevent package manager errors with spaces or invalid characters
	if strings.ContainsAny(packageName, " \t\n\r") {
		return "", fmt.Errorf("package name '%s' contains invalid characters (spaces or whitespace)", packageName)
	}

	// Use pacman -Si to get package information
	// Force English locale to ensure consistent output
	cmd := exec.Command("pacman", "-Si", packageName)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	output, err := cmd.Output()
	if err != nil {
		// Try -Qi for installed packages
		cmd = exec.Command("pacman", "-Qi", packageName)
		cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get package info: %w", err)
		}
	}

	return string(output), nil
}
