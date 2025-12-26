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

// Module: apk.go
// Description: Provides functions for managing APK repositories and packages.
// In order to allow multiple package managers at once, all package manager related functions (here for APK) are implemented in this file.

//go:build apk

package api

import (
	"bufio"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// getApkArchitecture gets the current system architecture from apk
func getApkArchitecture() (string, error) {
	cmd := exec.Command("apk", "--print-arch")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error running apk --print-arch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RepoAdd adds local package files to a local APK repository
func RepoAdd(files ...string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files specified")
	}

	// Get system architecture
	arch, err := getApkArchitecture()
	if err != nil {
		return fmt.Errorf("failed to get system architecture: %w", err)
	}

	// Create architecture-specific subdirectory
	// APK will look for files in /tmp/pi-apps-local-packages/{arch}/
	repoDir := filepath.Join("/tmp/pi-apps-local-packages", arch)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", repoDir, err)
	}

	// Move every mentioned apk file to the repository
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

// RepoRefresh indexes the Pi-Apps local APK repository
func RepoRefresh() error {
	// Get system architecture
	arch, err := getApkArchitecture()
	if err != nil {
		return fmt.Errorf("failed to get system architecture: %w", err)
	}

	// Use architecture-specific subdirectory
	repoDir := filepath.Join("/tmp/pi-apps-local-packages", arch)

	// Check if the repository directory exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return fmt.Errorf("cannot index the repository - it's missing! %s", repoDir)
	}

	// Check if we have any .apk files in the repository
	files, err := filepath.Glob(filepath.Join(repoDir, "*.apk"))
	if err != nil {
		return fmt.Errorf("failed to list apk files: %w", err)
	}

	if len(files) == 0 {
		// No packages to index, just return success
		return nil
	}

	// Generate a private key for signing the index (required by apk index)
	keyPath := filepath.Join(repoDir, "pi-apps-local.rsa")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Use Go's crypto/rsa to generate a private key
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("failed to generate RSA private key: %w", err)
		}
		keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to open file for private key: %w", err)
		}
		defer keyOut.Close()
		if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
			return fmt.Errorf("failed to encode private key to file: %w", err)
		}
	}

	// Extract public key
	pubKeyPath := filepath.Join(repoDir, "pi-apps-local.rsa.pub")
	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		// Read private key from file
		privKeyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}
		block, _ := pem.Decode(privKeyBytes)
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return fmt.Errorf("failed to decode PEM block as RSA PRIVATE KEY")
		}
		privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse RSA private key: %w", err)
		}
		pubASN1, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to marshal public key: %w", err)
		}
		pubOut, err := os.OpenFile(pubKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file for public key: %w", err)
		}
		defer pubOut.Close()
		if err := pem.Encode(pubOut, &pem.Block{Type: "PUBLIC KEY", Bytes: pubASN1}); err != nil {
			return fmt.Errorf("failed to encode public key to file: %w", err)
		}
	}

	// Create the package index using apk index
	indexPath := filepath.Join(repoDir, "APKINDEX.tar.gz")

	// Remove old index if it exists
	os.Remove(indexPath)

	// Get list of .apk files (expand glob pattern ourselves since exec.Command doesn't use a shell)
	apkFiles, err := filepath.Glob(filepath.Join(repoDir, "*.apk"))
	if err != nil {
		return fmt.Errorf("failed to find apk files: %w", err)
	}
	if len(apkFiles) == 0 {
		return fmt.Errorf("no .apk files found in %s", repoDir)
	}

	// Convert absolute paths to just filenames (relative to repoDir)
	var filenames []string
	for _, f := range apkFiles {
		filenames = append(filenames, filepath.Base(f))
	}

	// Build the apk index command - need to run from the directory
	// Don't use --rewrite-arch since we want to preserve the actual architecture
	indexArgs := []string{"index", "-o", "APKINDEX.tar.gz"}
	indexArgs = append(indexArgs, filenames...)
	cmd := exec.Command("apk", indexArgs...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("apk index failed to index the repository: %s\nCommand output: %s", repoDir, string(output))
		return fmt.Errorf("%s\nError: %w", errorMsg, err)
	}

	// Sign the index with abuild-sign
	// Note: abuild-sign is part of alpine-sdk package
	signCmd := exec.Command("abuild-sign", "-k", keyPath, indexPath)
	signCmd.Dir = repoDir
	if err := signCmd.Run(); err != nil {
		// If abuild-sign is not available, continue without signing
		// The repository will work with --allow-untrusted flag
		Warning("Failed to sign APK index (abuild-sign not available). Repository will require --allow-untrusted flag.")
	}

	// Add the repository to /etc/apk/repositories if not already present
	// Use the parent directory - APK will automatically append the architecture
	repoLine := "file:///tmp/pi-apps-local-packages"
	reposContent, err := os.ReadFile("/etc/apk/repositories")
	if err != nil {
		// If we can't read it, we'll try to add anyway
		reposContent = []byte{}
	}

	if !strings.Contains(string(reposContent), repoLine) {
		// Add our repository to the beginning
		tempFile, err := os.CreateTemp("", "apk-repos")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer os.Remove(tempFile.Name())

		// Write our repo first
		tempFile.WriteString(repoLine + "\n")
		// Then append existing repos
		tempFile.Write(reposContent)
		tempFile.Close()

		// Move to /etc/apk/repositories
		cmd := exec.Command("sudo", "cp", tempFile.Name(), "/etc/apk/repositories")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update /etc/apk/repositories: %w", err)
		}
	}

	return nil
}

// AptLockWait waits until other apk processes are finished before proceeding
func AptLockWait() error {
	// APK doesn't use locale files like APT does, so we skip AddEnglish()
	// Just set environment variables for consistent output
	os.Setenv("LANG", "C")
	os.Setenv("LC_ALL", "C")

	// Spawn a goroutine to notify the user after 5 seconds
	notificationDone := make(chan bool)
	notificationShown := make(chan bool)

	go func() {
		select {
		case <-time.After(5 * time.Second):
			fmt.Print(T("Waiting until APK locks are released... "))
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

	// APK uses /lib/apk/db/lock as its lock file
	lockFiles := []string{"/lib/apk/db/lock"}

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

// LessApt filters out unwanted lines from apk output
//
// This is a helper function for apk-related operations
func LessApt(input string) string {
	var result []string
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		// Strip ANSI codes first
		line = stripAnsiCodes(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Filter out APK-specific progress and status lines
		// These are similar to APT 1.x/2.x output patterns
		if strings.HasPrefix(line, "fetch ") ||
			strings.HasPrefix(line, "OK: ") ||
			strings.HasPrefix(line, "WARNING: ") ||
			strings.Contains(line, "Need to download") ||
			strings.Contains(line, "After this operation") ||
			strings.Contains(line, "Do you want to continue") ||
			strings.Contains(line, "Installing ") && strings.Contains(line, "(") && strings.Contains(line, ")") ||
			strings.Contains(line, ".trigger: Executing") ||
			strings.Contains(line, ".trigger: Regenerating") ||
			strings.HasPrefix(line, "Executing ") ||
			strings.HasPrefix(line, "Purging ") && strings.Contains(line, "(") {
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// stripAnsiCodes removes ANSI color and formatting codes from a string
func stripAnsiCodes(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}

// AptUpdate runs an apk update with error-checking and minimal output
func AptUpdate(args ...string) error {
	// Use cyan color with reverse video styling
	fmt.Fprintf(os.Stderr, "\033[96m%s \033[7m sudo apk update\033[27m...\033[0m\n", T("Running"))

	// Build command with optional arguments (like --allow-untrusted)
	cmdArgs := []string{"apk", "update"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("sudo", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Prepare and print enhanced error message similar to apt.go
		errorMessageTitle := T("Failed to run")
		errorMessageCommand := "sudo apk update"
		errorMessageErrors := T("APK reported these errors:")
		fmt.Fprint(os.Stderr, fmt.Sprintf("\033[91m%s \033[4m%s\033[0m\033[39m!\n", errorMessageTitle, errorMessageCommand))
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s\n\033[91m%s\033[39m\n", errorMessageErrors, err.Error()))
		return fmt.Errorf("apk update failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\033[96m%s\033[0m\n", T("apk update complete."))
	return nil
}

// RepoRm removes the local apk repository
func RepoRm() error {
	// Wait for other operations to finish before continuing
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for APK locks: %w", err)
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

	// Remove the repository entry from /etc/apk/repositories
	// Need to remove all architecture-specific entries
	reposContent, err := os.ReadFile("/etc/apk/repositories")
	if err != nil {
		// If we can't read it, nothing to remove
		return nil
	}

	// Remove our repository lines (any path containing pi-apps-local-packages)
	lines := strings.Split(string(reposContent), "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.Contains(line, "pi-apps-local-packages") {
			newLines = append(newLines, line)
		}
	}

	// Write back the modified file
	tempFile, err := os.CreateTemp("", "apk-repos")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	tempFile.WriteString(strings.Join(newLines, "\n"))
	tempFile.Close()

	cmd := exec.Command("sudo", "cp", tempFile.Name(), "/etc/apk/repositories")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update /etc/apk/repositories: %v\n", err)
	}

	return nil
}

// AppToPkgName converts an app-name to a unique, valid package-name that starts with 'pi-apps-'
func AppToPkgName(app string) (string, error) {
	// Calculate MD5 hash of the app name
	h := md5.New()
	io.WriteString(h, app)
	hashBytes := h.Sum(nil)

	// Convert the first 8 bytes to a hex string
	hashString := hex.EncodeToString(hashBytes)[:8]

	// Return the package name with the 'pi-apps-' prefix
	return fmt.Sprintf("pi-apps-%s", hashString), nil
}

// InstallPackages installs packages using APK
func InstallPackages(app string, args ...string) error {
	// Process arguments
	var packages []string
	usingLocalPackages := false

	StatusTf("Will install these packages: %s", strings.Join(args, " "))

	// Remove the local repo, just in case the last operation left it in an unrecoverable state
	if err := RepoRm(); err != nil {
		return fmt.Errorf("failed to remove existing local repository: %w", err)
	}

	// Process packages to handle local files, URLs
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

			// Extract package name from .apk file
			// APK packages are named: packagename-version-release.apk
			// We need to extract just the package name (before the version)
			baseName := filepath.Base(pkg)
			baseName = strings.TrimSuffix(baseName, ".apk")

			// Extract package name by removing version/release numbers
			// Version numbers typically start after the last alphabetic character followed by a dash and digit
			// Use regex to find where version starts: dash followed by a digit
			versionRegex := regexp.MustCompile(`-\d`)
			versionIndex := versionRegex.FindStringIndex(baseName)

			pkgName := baseName
			if versionIndex != nil {
				// Found version start, extract package name
				pkgName = baseName[:versionIndex[0]]
			}

			packages = append(packages, pkgName)

		} else if strings.Contains(pkg, "://") {
			// Handle URLs - download the .apk file
			filename := filepath.Join("/tmp", filepath.Base(strings.TrimSuffix(pkg, "/download")))

			// Add .apk extension if missing
			if !strings.HasSuffix(filename, ".apk") {
				Status(fmt.Sprintf("%s is not ending with .apk, renaming it to '%s.apk'...", filename, filename))
				filename = filename + ".apk"
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

				WarningTf("Package download failed. (Attempt %d of 3)", attempt)
				os.Remove(filename) // Clean up failed download
			}

			if !success {
				return fmt.Errorf(T("downloaded package does not exist: %s"), filename)
			}

			// Add to repository
			if err := RepoAdd(filename); err != nil {
				return fmt.Errorf(T("failed to add downloaded package to repository: %w"), err)
			}

			usingLocalPackages = true

			// Extract package name from .apk file
			baseName := filepath.Base(filename)
			baseName = strings.TrimSuffix(baseName, ".apk")

			// Extract package name by removing version/release numbers
			versionRegex := regexp.MustCompile(`-\d`)
			versionIndex := versionRegex.FindStringIndex(baseName)

			pkgName := baseName
			if versionIndex != nil {
				// Found version start, extract package name
				pkgName = baseName[:versionIndex[0]]
			}

			packages = append(packages, pkgName)

		} else if strings.Contains(pkg, "*") {
			// Handle wildcards - expand using apk search
			StatusTf("Expanding wildcard in '%s'...", pkg)

			searchPattern := strings.ReplaceAll(pkg, "*", "")
			cmd := exec.Command("apk", "search", searchPattern)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf(T("failed to search for packages with pattern %s: %w"), pkg, err)
			}

			// Extract package names from search results
			var expandedPkgs []string
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Extract package name (before the version)
				pkgName := strings.Split(line, "-")[0]
				if pkgName != "" {
					expandedPkgs = append(expandedPkgs, pkgName)
				}
			}

			if len(expandedPkgs) > 0 {
				packages = append(packages, expandedPkgs...)
			} else {
				return fmt.Errorf(T("no packages found matching pattern: %s"), pkg, err)
			}
		} else {
			// Regular package name
			packages = append(packages, pkg)
		}
	}

	// Initialize local repository if needed
	if usingLocalPackages {
		if err := RepoRefresh(); err != nil {
			return fmt.Errorf(T("failed to refresh local repository: %w"), err)
		}

		// Update APK indexes to include our local repo
		// Use --allow-untrusted flag since local packages won't be signed
		if err := AptUpdate("--allow-untrusted"); err != nil {
			return fmt.Errorf(T("failed to update APK indexes: %w"), err)
		}
	}

	StatusTf("Installing packages: %s", strings.Join(packages, " "))

	// Wait for APK locks
	if err := AptLockWait(); err != nil {
		return fmt.Errorf(T("failed to wait for APK locks: %w"), err)
	}

	// Install packages with APK - use pipes to capture and filter output
	// Add --allow-untrusted flag if using local packages (they won't be signed)
	installArgs := []string{"add", "--no-cache"}
	if usingLocalPackages {
		installArgs = append(installArgs, "--allow-untrusted")
	}
	installArgs = append(installArgs, packages...)
	cmd := exec.Command("sudo", append([]string{"apk"}, installArgs...)...)
	cmd.Env = os.Environ()

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf(T("failed to create stdout pipe: %w"), err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf(T("failed to create stderr pipe: %w"), err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(T("failed to start apk add command: %w"), err)
	}

	// Capture all output for error analysis
	var outputBuffer strings.Builder
	var bufferMutex sync.Mutex
	var wg sync.WaitGroup

	// Read and display stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			bufferMutex.Lock()
			outputBuffer.WriteString(line + "\n")
			bufferMutex.Unlock()
			fmt.Println(line)
		}
	}()

	// Read and display stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			bufferMutex.Lock()
			outputBuffer.WriteString(line + "\n")
			bufferMutex.Unlock()
			fmt.Fprintln(os.Stderr, line)
		}
	}()

	// Wait for the command to complete
	err = cmd.Wait()
	fmt.Fprintln(os.Stderr, "[DEBUG] cmd.Wait() completed, err:", err)

	// Wait for output readers to finish
	wg.Wait()
	fmt.Fprintln(os.Stderr, "[DEBUG] wg.Wait() completed")

	StatusTf("APK finished.")

	combinedOutput := outputBuffer.String()
	fmt.Fprintln(os.Stderr, "[DEBUG] Output buffer length:", len(combinedOutput))

	// Extract error lines from APK output
	// APK uses "ERROR:" prefix for errors
	// Check for errors even if exit code is 0, as APK sometimes returns 0 on failure
	var errorLines []string
	for _, line := range strings.Split(combinedOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ERROR:") ||
			strings.Contains(line, "fetch") && strings.Contains(line, "error") ||
			strings.Contains(line, "unable to select packages") {
			errorLines = append(errorLines, line)
		}
	}
	fmt.Fprintln(os.Stderr, "[DEBUG] Error lines found:", len(errorLines))

	// Check for errors - either from exit code or from error messages in output
	if err != nil || len(errorLines) > 0 {
		// Handle error cases
		errorStr := strings.Join(errorLines, "\n")

		if len(errorLines) > 0 {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
			fmt.Printf(T("%s\n\033[91m%s\033[39m\n"), T("APK reported these errors:"), errorStr)
		} else {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to install the packages!"))
			fmt.Printf(T("APK exited with error code %d\n"), cmd.ProcessState.ExitCode())
		}

		fmt.Println(combinedOutput)

		if len(errorLines) > 0 {
			return fmt.Errorf("apk reported errors: %s", errorStr)
		}
		return fmt.Errorf("apk add failed with exit code %d", cmd.ProcessState.ExitCode())
	}

	// Create a tracking file to remember which packages were installed for this app
	// This helps with uninstallation later
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

	StatusGreenT("Package installation complete.")
	return nil
}

// PurgePackages allows dependencies of the specified app to be removed
func PurgePackages(app string, isUpdate bool) error {
	StatusTf("Allowing packages required by the %s app to be uninstalled", app)

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
	// APK will error if we try to delete a package that doesn't exist
	var installedPackages []string
	for _, pkg := range packages {
		// Check if package is installed
		checkCmd := exec.Command("apk", "info", "-e", pkg)
		if err := checkCmd.Run(); err == nil {
			// Package is installed
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

	// Wait for APK locks
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for APK locks: %w", err)
	}

	// Remove packages with APK - use pipes to capture and filter output
	delArgs := append([]string{"del"}, installedPackages...)
	cmd := exec.Command("sudo", append([]string{"apk"}, delArgs...)...)
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
		return fmt.Errorf("failed to start apk del command: %w", err)
	}

	// Capture all output for error analysis
	var outputBuffer strings.Builder
	var bufferMutex sync.Mutex
	var wg sync.WaitGroup

	// Read and display stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			bufferMutex.Lock()
			outputBuffer.WriteString(line + "\n")
			bufferMutex.Unlock()
			fmt.Println(line)
		}
	}()

	// Read and display stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			bufferMutex.Lock()
			outputBuffer.WriteString(line + "\n")
			bufferMutex.Unlock()
			fmt.Fprintln(os.Stderr, line)
		}
	}()

	// Wait for the command to complete
	err = cmd.Wait()

	// Wait for output readers to finish
	wg.Wait()

	StatusT("APK finished.")

	combinedOutput := outputBuffer.String()

	// Extract error lines from APK output
	// Check for errors even if exit code is 0, as APK sometimes returns 0 on failure
	var errorLines []string
	for _, line := range strings.Split(combinedOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ERROR:") ||
			strings.Contains(line, "unable to select packages") ||
			strings.Contains(line, "World entry") && strings.Contains(line, "not found") {
			errorLines = append(errorLines, line)
		}
	}

	// Check for errors - either from exit code or from error messages in output
	if err != nil || len(errorLines) > 0 {
		// Handle error cases
		errorStr := strings.Join(errorLines, "\n")

		if len(errorLines) > 0 {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
			fmt.Printf("%s\n\033[91m%s\033[39m\n", T("APK reported these errors:"), errorStr)
		} else {
			fmt.Printf("\033[91m%s\033[39m\n", T("Failed to uninstall the packages!"))
			fmt.Printf(T("APK exited with error code %d\n"), cmd.ProcessState.ExitCode())
		}

		fmt.Println(combinedOutput)

		if len(errorLines) > 0 {
			return fmt.Errorf("apk reported errors: %s", errorStr)
		}
		return fmt.Errorf("apk del failed with exit code %d", cmd.ProcessState.ExitCode())
	}

	// Remove the tracking file
	if err := os.Remove(trackingFile); err != nil {
		WarningTf("Failed to remove tracking file: %v", err)
	}

	StatusGreenT("All packages have been purged successfully.")
	return nil
}

// GetIconFromPackage finds the largest icon file (png or svg) installed by a package
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

	// Collect all files from all packages using apk info -L
	var iconFiles []string
	for _, pkg := range allPackages {
		cmd := exec.Command("apk", "info", "-L", pkg)
		cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
		output, err := cmd.Output()
		if err != nil {
			// Continue even if there's an error, as some packages might not be installed
			continue
		}

		// Parse output and filter for icon files
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == pkg+" contains:" {
				continue
			}

			// APK info -L output is relative paths, so prepend /
			if !strings.HasPrefix(line, "/") {
				line = "/" + line
			}

			// Check if it's a PNG or SVG file in the right directories
			if (strings.HasSuffix(line, ".png") || strings.HasSuffix(line, ".svg")) &&
				(strings.Contains(line, "/icons/") || strings.Contains(line, "/pixmaps/")) &&
				!strings.Contains(line, "/symbolic/") {
				iconFiles = append(iconFiles, line)
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

// UbuntuPPAInstaller sets up a PPA (not applicable for APK-based systems)
func UbuntuPPAInstaller(ppaName string) error {
	return fmt.Errorf("PPAs are not supported on APK-based systems")
}

// DebianPPAInstaller sets up a PPA (not applicable for APK-based systems)
func DebianPPAInstaller(ppaName, ppaDist, key string) error {
	return fmt.Errorf("PPAs are not supported on APK-based systems")
}

// AddExternalRepo adds an external APK repository
func AddExternalRepo(reponame, pubkeyurl, uris, suites, components string, additionalOptions ...string) error {
	// Exit if reponame or uri contains space
	if strings.Contains(reponame, " ") || strings.Contains(uris, " ") {
		return fmt.Errorf("add_external_repo: provided reponame or uris contains a space")
	}

	// For APK, the URI is the complete repository URL
	// APK repositories format: http://example.com/alpine/v3.18/main
	// We ignore suites and components as they're APT-specific concepts

	// If pubkeyurl is provided, download and install the public key
	if pubkeyurl != "" {
		fmt.Println(T("add_external_repo: checking 3rd party pubkeyurl validity"))
		resp, err := http.Get(pubkeyurl)
		if err != nil {
			return fmt.Errorf(T("add_external_repo: failed to reach pubkeyurl: %w"), err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf(T("add_external_repo: pubkeyurl returned status code %d"), resp.StatusCode)
		}

		// Download the public key
		keyData, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf(T("add_external_repo: failed to read key data: %w"), err)
		}

		// APK keys go in /etc/apk/keys/
		// Generate a filename based on the key content (use SHA1 hash)
		h := sha1.New()
		h.Write(keyData)
		keyHash := hex.EncodeToString(h.Sum(nil))[:16]

		keyDir := "/etc/apk/keys"
		keyFile := filepath.Join(keyDir, fmt.Sprintf("%s-%s.rsa.pub", reponame, keyHash))

		// Ensure keys directory exists
		if _, err := os.Stat(keyDir); os.IsNotExist(err) {
			cmd := exec.Command("sudo", "mkdir", "-p", keyDir)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf(T("add_external_repo: failed to create keys directory: %w"), err)
			}
		}

		// Write the key to a temporary file
		tempFile, err := os.CreateTemp("", "apk-key")
		if err != nil {
			return fmt.Errorf(T("add_external_repo: failed to create temporary file: %w"), err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.Write(keyData); err != nil {
			return fmt.Errorf(T("add_external_repo: failed to write key data: %w"), err)
		}
		tempFile.Close()

		// Move the key to the keys directory
		cmd := exec.Command("sudo", "cp", tempFile.Name(), keyFile)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(T("add_external_repo: failed to install key: %w"), err)
		}
	}

	// Add the repository to /etc/apk/repositories
	// Read existing repositories
	reposContent, err := os.ReadFile("/etc/apk/repositories")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(T("add_external_repo: failed to read /etc/apk/repositories: %w"), err)
	}

	// Check if repository already exists
	if strings.Contains(string(reposContent), uris) {
		fmt.Printf(T("Repository %s already exists in /etc/apk/repositories\n"), uris)
		return nil
	}

	// Prepare the new repository line
	// Add a comment to identify this was added by Pi-Apps
	repoLine := fmt.Sprintf("# Added by Pi-Apps: %s\n%s\n", reponame, uris)

	// Create a temporary file with the updated content
	tempFile, err := os.CreateTemp("", "apk-repos")
	if err != nil {
		return fmt.Errorf(T("add_external_repo: failed to create temporary file: %w"), err)
	}
	defer os.Remove(tempFile.Name())

	// Write existing content first
	if len(reposContent) > 0 {
		tempFile.Write(reposContent)
		// Ensure there's a newline before our addition
		if !strings.HasSuffix(string(reposContent), "\n") {
			tempFile.WriteString("\n")
		}
	}

	// Add our repository
	tempFile.WriteString(repoLine)
	tempFile.Close()

	// Move to /etc/apk/repositories
	cmd := exec.Command("sudo", "cp", tempFile.Name(), "/etc/apk/repositories")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(T("add_external_repo: failed to update /etc/apk/repositories: %w"), err)
	}

	return nil
}

// RmExternalRepo removes an external APK repository
func RmExternalRepo(reponame string, force bool) error {
	// Exit if reponame contains space
	if strings.Contains(reponame, " ") {
		return fmt.Errorf("rm_external_repo: provided reponame contains a space")
	}

	// Read /etc/apk/repositories
	reposContent, err := os.ReadFile("/etc/apk/repositories")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("rm_external_repo: failed to read /etc/apk/repositories: %w", err)
	}

	// Find and remove lines related to this repository
	lines := strings.Split(string(reposContent), "\n")
	var newLines []string
	skipNext := false
	commentMarker := fmt.Sprintf("# Added by Pi-Apps: %s", reponame)

	for _, line := range lines {
		if strings.Contains(line, commentMarker) {
			// Skip this comment and the next line (the actual repo URL)
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}
		newLines = append(newLines, line)
	}

	// If nothing was removed, check if we need to force remove by checking for reponame in URLs
	if len(newLines) == len(lines) && force {
		newLines = []string{}
		for _, line := range lines {
			// Skip lines that might contain the reponame
			if !strings.Contains(line, reponame) {
				newLines = append(newLines, line)
			}
		}
	}

	// Write the modified content back
	tempFile, err := os.CreateTemp("", "apk-repos")
	if err != nil {
		return fmt.Errorf("rm_external_repo: failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	tempFile.WriteString(strings.Join(newLines, "\n"))
	tempFile.Close()

	cmd := exec.Command("sudo", "cp", tempFile.Name(), "/etc/apk/repositories")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rm_external_repo: failed to update /etc/apk/repositories: %w", err)
	}

	// Remove associated keys from /etc/apk/keys/
	keyPattern := filepath.Join("/etc/apk/keys", fmt.Sprintf("%s-*.rsa.pub", reponame))
	keys, err := filepath.Glob(keyPattern)
	if err == nil && len(keys) > 0 {
		for _, key := range keys {
			cmd := exec.Command("sudo", "rm", "-f", key)
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove key %s: %v\n", key, err)
			}
		}
	}

	return nil
}

// AdoptiumInstaller sets up Java/JDK for APK systems
// Note: Adoptium (Eclipse Temurin) doesn't provide APK packages, so we install OpenJDK from Alpine repos
func AdoptiumInstaller() error {
	// Alpine Linux provides OpenJDK in its official repositories
	// Check which JDK versions are available and install the latest LTS

	StatusT("Checking available OpenJDK versions in Alpine repositories...")

	// Common OpenJDK packages in Alpine: openjdk8, openjdk11, openjdk17, openjdk21
	// Try to find the latest available LTS version
	jdkVersions := []string{"openjdk25", "openjdk21", "openjdk17", "openjdk11", "openjdk8"}

	var availableJDK string
	for _, jdk := range jdkVersions {
		if PackageAvailable(jdk, "") {
			availableJDK = jdk
			break
		}
	}

	if availableJDK == "" {
		return fmt.Errorf("no OpenJDK package found in Alpine repositories")
	}

	StatusTf("Installing %s from Alpine repositories (Adoptium/Temurin not available for APK systems)...", availableJDK)

	// Install the JDK
	cmd := exec.Command("sudo", "apk", "add", "--no-cache", availableJDK)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", availableJDK, err)
	}

	StatusGreenTf("%s installed successfully", availableJDK)

	// Optionally set JAVA_HOME
	// Find the java installation directory
	javaPath, err := exec.Command("which", "java").Output()
	if err == nil && len(javaPath) > 0 {
		// Try to determine JAVA_HOME from the java binary location
		// Alpine typically installs Java in /usr/lib/jvm/
		javaHomeHint := "/usr/lib/jvm/default-jvm"
		if _, err := os.Stat(javaHomeHint); err == nil {
			StatusTf("Java installed. JAVA_HOME can be set to: %s", javaHomeHint)
		}
	}

	return nil
}

// PackageInstalled checks if a package is installed
func PackageInstalled(packageName string) bool {
	// APK uses: apk info -e <package>
	cmd := exec.Command("apk", "info", "-e", packageName)
	err := cmd.Run()
	return err == nil
}

// PackageAvailable determines if the specified package exists in a repository
func PackageAvailable(packageName string, dpkgArch string) bool {
	// Special handling for "init" package check
	// APK systems don't have an "init" package, but use various init systems
	if packageName == "init" {
		// Check for common init systems on APK-based distributions
		initSystems := []string{
			"openrc",  // Alpine Linux default
			"dinit",   // Chimera Linux
			"s6",      // s6-based systems
			"busybox", // Alpine minimal (provides init)
			"runit",   // runit-based systems
		}

		for _, initPkg := range initSystems {
			if PackageInstalled(initPkg) {
				return true
			}
		}

		// If none are installed but available, check availability
		for _, initPkg := range initSystems {
			cmd := exec.Command("apk", "search", "-e", initPkg)
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				return true
			}
		}

		return false
	}

	// APK uses: apk search -e <package>
	cmd := exec.Command("apk", "search", "-e", packageName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return len(output) > 0
}

// PackageDependencies outputs the list of dependencies for the specified package
func PackageDependencies(packageName string) ([]string, error) {
	if packageName == "" {
		Error("PackageDependencies(): no package specified!")
		return nil, fmt.Errorf("no package specified")
	}

	// Use apk info -R to get dependencies
	cmd := exec.Command("apk", "info", "-R", packageName)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies for package %s: %w", packageName, err)
	}

	// Parse the output - apk info -R lists dependencies one per line
	var deps []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != packageName {
			deps = append(deps, line)
		}
	}

	// Return as a single comma-separated string (like APT version does)
	if len(deps) > 0 {
		return []string{strings.Join(deps, ", ")}, nil
	}

	return []string{}, nil
}

// PackageInstalledVersion returns the installed version of the specified package
func PackageInstalledVersion(packageName string) (string, error) {
	if packageName == "" {
		Error("PackageInstalledVersion(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// Use apk info to get the installed version
	// apk info outputs: packagename-version description
	cmd := exec.Command("apk", "info", "-e", packageName)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(T("package %s is not installed"), packageName)
	}

	// Parse output - format is typically: packagename-version
	line := strings.TrimSpace(string(output))
	if line == "" {
		return "", fmt.Errorf(T("package %s is not installed"), packageName)
	}

	// Extract version from packagename-version format
	// Find the last dash to separate package name from version
	parts := strings.Split(line, "-")
	if len(parts) >= 2 {
		// The version is typically the last part or last two parts (version-release)
		// Try to find where the version starts by removing the package name
		version := strings.TrimPrefix(line, packageName+"-")
		if version != line {
			return version, nil
		}
	}

	// Fallback: use apk info with -a flag to get detailed info
	cmd = exec.Command("apk", "info", "-a", packageName)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf(T("package %s is not installed"), packageName)
	}

	// Look for the version line in the output
	for _, infoLine := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(infoLine, packageName+"-") && strings.Contains(infoLine, " description:") {
			// Extract version from "packagename-version description: ..."
			parts := strings.SplitN(infoLine, " ", 2)
			if len(parts) > 0 {
				version := strings.TrimPrefix(parts[0], packageName+"-")
				return version, nil
			}
		}
	}

	return "", fmt.Errorf(T("package %s is not installed"), packageName)
}

// PackageLatestVersion returns the latest available version of the specified package
func PackageLatestVersion(packageName string, repo ...string) (string, error) {
	if packageName == "" {
		Error("PackageLatestVersion(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// APK doesn't have a direct equivalent to apt-cache policy
	// We use apk search -e for exact match, or apk list for more details

	// Try apk list first (more detailed, shows version)
	cmd := exec.Command("apk", "list", packageName)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		// Package not found
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	// Parse output - format is: packagename-version arch {repository}
	// Example: "curl-8.5.0-r0 x86_64 {main}"
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract the package-version part (before the architecture)
		parts := strings.Fields(line)
		if len(parts) > 0 {
			pkgVerStr := parts[0]
			// Remove package name to get version
			version := strings.TrimPrefix(pkgVerStr, packageName+"-")
			if version != pkgVerStr {
				return version, nil
			}
		}
	}

	// Fallback: try apk search with exact match
	cmd = exec.Command("apk", "search", "-e", packageName)
	cmd.Env = append(os.Environ(), "LANG=C")
	output, err = cmd.Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	// Parse search output - format is packagename-version
	line := strings.TrimSpace(string(output))
	if line == "" {
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	// Extract version
	version := strings.TrimPrefix(line, packageName+"-")
	if version == line {
		// Couldn't extract version properly
		return "", fmt.Errorf("package %s is not available", packageName)
	}

	return version, nil
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
	_, err = getApkArchitecture()
	if err != nil {
		return fmt.Errorf("error getting apk architecture: %w", err)
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
	apkListOutput, err := getApkListInfo(allPackages)
	if err != nil {
		return fmt.Errorf("error getting apk list info: %w", err)
	}

	// Get installed package status
	apkInstalledStatus, err := getApkInstalledStatus(allPackages)
	if err != nil {
		return fmt.Errorf("error getting apk installed status: %w", err)
	}

	// Use the collected data to refresh status for each package app
	for _, app := range packageApps {
		// Update through our own implementation rather than calling RefreshPkgAppStatus
		// to avoid re-querying package status information
		err := refreshPackageAppStatusWithCache(app, apkListOutput, apkInstalledStatus, directory)
		if err != nil {
			Debug(fmt.Sprintf("Error refreshing status for %s: %v", app, err))
		}
	}

	return nil
}

// getApkListInfo runs apk list for the specified packages
func getApkListInfo(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	var allOutput strings.Builder

	// APK list doesn't accept multiple packages at once, so query them individually
	// But we can batch them with apk search
	for _, pkg := range packages {
		cmd := exec.Command("apk", "list", pkg)
		cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
		output, err := cmd.Output()
		if err != nil {
			// Package not available, continue
			continue
		}
		allOutput.WriteString(string(output))
	}

	return allOutput.String(), nil
}

// getApkInstalledStatus gets the status of the specified packages from apk
func getApkInstalledStatus(packages []string) (string, error) {
	if len(packages) == 0 {
		return "", nil
	}

	var allOutput strings.Builder

	// Check which packages are installed using apk info -e
	for _, pkg := range packages {
		cmd := exec.Command("apk", "info", "-e", pkg)
		cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
		output, err := cmd.Output()
		if err != nil {
			// Package not installed, continue
			continue
		}
		allOutput.WriteString(fmt.Sprintf("Package: %s\n", pkg))
		allOutput.WriteString("Status: installed\n")
		allOutput.WriteString(fmt.Sprintf("Version: %s\n", strings.TrimSpace(string(output))))
		allOutput.WriteString("\n")
	}

	return allOutput.String(), nil
}

// refreshPackageAppStatusWithCache refreshes a single package-app status using cached apk data
func refreshPackageAppStatusWithCache(appName, apkListOutput, apkInstalledStatus, directory string) error {
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
		if isPackageInstalledFromStatus(pkg, apkInstalledStatus) {
			installed = true
			availablePackage = pkg
			break
		}

		// Check if available in repository
		if isPackageAvailableFromList(pkg, apkListOutput) {
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

// isPackageInstalledFromStatus checks if a package is installed by looking at the apk status
func isPackageInstalledFromStatus(packageName, apkInstalledStatus string) bool {
	// Look for the package and check if it's installed
	packageSection := fmt.Sprintf("Package: %s\n", packageName)
	index := strings.Index(apkInstalledStatus, packageSection)
	if index == -1 {
		return false
	}

	// Check the status in the few lines after the package name
	sectionEnd := index + 200
	if sectionEnd > len(apkInstalledStatus) {
		sectionEnd = len(apkInstalledStatus)
	}
	statusSection := apkInstalledStatus[index:sectionEnd]
	return strings.Contains(statusSection, "Status: installed")
}

// isPackageAvailableFromList checks if a package is available in repositories
func isPackageAvailableFromList(packageName, apkListOutput string) bool {
	// APK list output format: packagename-version arch {repository}
	// We just need to check if the package name appears at the start of a line
	lines := strings.Split(apkListOutput, "\n")
	for _, line := range lines {
		// Check if line starts with packagename-
		if strings.HasPrefix(line, packageName+"-") {
			return true
		}
	}
	return false
}

// PackageInfo lists everything the package manager knows about the specified package
func PackageInfo(packageName string) (string, error) {
	if packageName == "" {
		Error("PackageInfo(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// Validate package name
	if strings.ContainsAny(packageName, " \t\n\r") {
		return "", fmt.Errorf("package name '%s' contains invalid characters (spaces or whitespace)", packageName)
	}

	// APK uses: apk info <package>
	cmd := exec.Command("apk", "info", packageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get package info: %w", err)
	}

	return string(output), nil
}

// Helper functions

// sortAndDeduplicate sorts packages and removes duplicates
func sortAndDeduplicate(packages []string) string {
	pkgMap := make(map[string]string)

	for _, pkg := range packages {
		parts := strings.SplitN(pkg, " ", 2)
		pkgName := parts[0]
		pkgVersion := ""
		if len(parts) > 1 {
			pkgVersion = parts[1]
		}

		pkgMap[pkgName] = pkgVersion
	}

	var uniquePkgs []string
	for name, version := range pkgMap {
		if version != "" {
			uniquePkgs = append(uniquePkgs, name+" "+version)
		} else {
			uniquePkgs = append(uniquePkgs, name)
		}
	}

	sort.Strings(uniquePkgs)

	return strings.Join(uniquePkgs, ", ")
}
