package api

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RepoAdd adds local package files to the /tmp/pi-apps-local-packages repository
// This is a Go implementation of the original bash repo_add function
func RepoAdd(files ...string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files specified")
	}

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
// This is a Go implementation of the original bash repo_refresh function
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
		return fmt.Errorf(errorMsg)
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
// This is a Go implementation of the original bash apt_lock_wait function
func AptLockWait() error {
	// First ensure English locale is added
	AddEnglish()

	// Spawn a goroutine to notify the user after 5 seconds
	notificationDone := make(chan bool)
	notificationShown := make(chan bool)

	go func() {
		select {
		case <-time.After(5 * time.Second):
			fmt.Print("Waiting until APT locks are released... ")
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
	for {
		cmd := exec.Command("sudo", "-E", "apt", "install", "lkqecjhxwqekc")
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
		fmt.Println("Done")
	default:
		// Notification wasn't shown, do nothing
	}

	return nil
}

// LessApt filters out unwanted lines from apt output
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

// AptUpdate runs an apt update with error-checking and minimal output
// This is a Go implementation of the original bash apt_update function
func AptUpdate(args ...string) error {
	// Wait for APT locks to be released first
	if err := AptLockWait(); err != nil {
		return fmt.Errorf("failed to wait for APT locks: %w", err)
	}

	// Use cyan color with reverse video styling to match the original implementation
	// \033[96m for cyan, \033[7m for reverse video, \033[27m to end reverse, \033[0m to reset all formatting
	fmt.Fprintf(os.Stderr, "\033[96mRunning \033[7msudo apt update\033[27m...\033[0m\n")

	// Prepare the apt update command with provided arguments
	aptArgs := append([]string{"-E", "apt", "update", "--allow-releaseinfo-change"}, args...)
	cmd := exec.Command("sudo", aptArgs...)

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

	// Show completion message in cyan to match the original
	fmt.Fprintln(os.Stderr, "\033[96mapt update complete.\033[0m")

	// Process output to show helpful messages about package status
	// Strip color codes first to ensure reliable pattern matching
	completeOutput := outputBuffer.String()
	strippedOutput := stripAnsiCodes(completeOutput)

	// Check for autoremovable packages messages (both APT 2.x and 3.0 formats)
	if strings.Contains(strippedOutput, "autoremove to remove them") ||
		strings.Contains(strippedOutput, "can be autoremoved") {
		// Use direct ANSI codes for exact matching with the original
		fmt.Println("\033[33mSome packages are unnecessary.\033[39m Please consider running \033[4msudo a\033[0mp\033[4mt autoremove\033[0m.")
	}

	// Check for upgradeable packages messages (both APT 2.x and 3.0 formats)
	if strings.Contains(strippedOutput, "packages can be upgraded") ||
		strings.Contains(strippedOutput, "can be upgraded") ||
		strings.Contains(strippedOutput, "upgradable") {
		fmt.Println("\033[33mSome packages can be upgraded.\033[39m Please consider running \033[4msudo a\033[0mp\033[4mt full-u\033[0mpg\033[4mrade\033[0m.")
	} else if strings.Contains(strippedOutput, "package can be upgraded") ||
		strings.Contains(strippedOutput, "is upgradable") {
		fmt.Println("\033[33mOne package can be upgraded.\033[39m Please consider running \033[4msudo a\033[0mp\033[4mt full-u\033[0mpg\033[4mrade\033[0m.")
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
		fmt.Fprintf(os.Stderr, "\033[91mFailed to run \033[4msudo apt update\033[0m\033[39m!\n")
		fmt.Fprintf(os.Stderr, "APT reported these errors:\n\033[91m%s\033[39m\n", errorMessage)

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
// This is a Go implementation of the original bash repo_rm function
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
// This is a Go implementation of the original bash app_to_pkgname function
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
// This is a Go implementation of the original bash install_packages function
func InstallPackages(app string, args ...string) error {
	if app == "" {
		return fmt.Errorf("install_packages function can only be used by apps to install packages (the app variable was not set)")
	}

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

	Status("Will install these packages: " + strings.Join(packages, " "))

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
				return fmt.Errorf("local package does not exist: %s", pkg)
			}

			// Get package info using dpkg-deb
			cmd := exec.Command("dpkg-deb", "-I", pkg)
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to get package info from %s: %w", pkg, err)
			}

			// Parse the output to get package name, version, and architecture
			pkgName, pkgVersion, pkgArch := extractPackageInfo(string(output))
			if pkgName == "" {
				return fmt.Errorf("failed to determine package name for file: %s", pkg)
			}
			if pkgVersion == "" {
				return fmt.Errorf("failed to determine package version for file: %s", pkg)
			}
			if pkgArch == "" {
				return fmt.Errorf("failed to determine package architecture for file: %s", pkg)
			}

			// Add architecture suffix if it's a foreign architecture
			cmd = exec.Command("dpkg", "--print-architecture")
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
				return fmt.Errorf("downloaded package does not exist: %s", filename)
			}

			// Get package info
			cmd := exec.Command("dpkg-deb", "-I", filename)
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
				return fmt.Errorf("no packages found matching pattern: %s", pkg)
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
			return fmt.Errorf("failed to remove all regex from package list: %s", strings.Join(packages, "\n"))
		}
		if strings.Contains(pkg, "://") {
			return fmt.Errorf("failed to remove all URLs from package list: %s", strings.Join(packages, "\n"))
		}
		if strings.Contains(pkg, "/") && !strings.Contains(pkg, " (>=") {
			return fmt.Errorf("failed to remove all filenames from package list: %s", strings.Join(packages, "\n"))
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

	Status("Creating an empty apt-package to install the necessary apt packages...\nIt will be named: " + pkgName)

	// Check if package is already installed and get its dependencies
	pkgInstalled := PackageInstalled(pkgName)
	var existingDeps string

	if pkgInstalled {
		deps, err := PackageDependencies(pkgName)
		if err != nil {
			return fmt.Errorf("failed to get dependencies for existing package %s: %w", pkgName, err)
		}

		existingDeps = strings.Join(deps, ", ")
		Status("The " + pkgName + " package is already installed. Inheriting its dependencies: " + existingDeps)

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
	controlContent := fmt.Sprintf(`Maintainer: Pi-Apps team
Name: %s
Description: Dummy package created by pi-apps to install dependencies for the '%s' app
Version: 1.0
Architecture: all
Priority: optional
Section: custom
Depends: %s
Package: %s
`, app, app, uniquePkgs, pkgName)

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
			fmt.Printf("%s is already installed and no changes would be made. Skipping...\n", pkgName)

			// Clean up
			os.RemoveAll(pkgDir)
			os.RemoveAll(pkgDir + ".deb")

			// Remove local repo if it was used
			if usingLocalPackages {
				if err := RepoRm(); err != nil {
					return fmt.Errorf("failed to remove local repository: %w", err)
				}
			}

			StatusGreen("Package installation complete.")
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
		return fmt.Errorf("user error: Uh-oh, the /tmp/pi-apps-local-packages folder went missing while installing packages." +
			"\nThis usually happens if you try to install several apps at the same time in multiple terminals.")
	}

	// Run apt update and install with retry loop
	for i := 0; i < 5; i++ {
		// Run apt update
		if err := AptUpdate(aptFlags...); err != nil {
			return err
		}

		// Install dummy deb
		Status("Installing the " + pkgName + " package...")

		if err := AptLockWait(); err != nil {
			return fmt.Errorf("failed to wait for APT locks: %w", err)
		}

		// Create command for apt install
		installArgs := []string{"-E", "apt", "install", "-fy", "--no-install-recommends", "--allow-downgrades"}
		installArgs = append(installArgs, aptFlags...)
		installArgs = append(installArgs, pkgDir+".deb")

		cmd = exec.Command("sudo", installArgs...)

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

		Status("Apt finished.")

		// Get the complete output
		combinedOutput := outputBuffer.String()

		// Check if local repo was lost
		if usingLocalPackages && !FileExists("/var/lib/apt/lists/_tmp_pi-apps-local-packages_._Packages") && i < 4 {
			Warning(fmt.Sprintf("Local packages failed to install because another apt update process erased apt's knowledge of the pi-apps local repository.\nTrying again... (attempt %d of 5)", i+1))
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
				fmt.Println("\033[91mFailed to install the packages!\033[39m")
				fmt.Printf("User error: Apt exited with a failed exitcode (%d) and no error (E/Err) output. "+
					"This could indicate system corruption (eg: storage corruption or unstable overclocking).\n", cmd.ProcessState.ExitCode())
				return fmt.Errorf("apt exited with error code %d and no error output", cmd.ProcessState.ExitCode())
			} else {
				fmt.Println("\033[91mFailed to install the packages!\033[39m")
				fmt.Printf("The APT reported these errors:\n\033[91m%s\033[39m\n", errorStr)

				// Debug output for local repository issues
				if usingLocalPackages && !FileExists("/tmp/pi-apps-local-packages/Packages") {
					fmt.Println("User error: Uh-oh, the /tmp/pi-apps-local-packages folder went missing while installing packages." +
						"\nThis usually happens if you try to install several apps at the same time in multiple terminals.")
				} else if usingLocalPackages && (strings.Contains(combinedOutput, "but it is not installable") ||
					strings.Contains(combinedOutput, "but it is not going to be installed") ||
					strings.Contains(combinedOutput, "but .* is to be installed")) {

					fmt.Println("\033[91mThe Pi-Apps Local Repository was being used, and a package seemed to not be available. Here's the Packages file:\033[39m")
					packagesContent, _ := os.ReadFile("/tmp/pi-apps-local-packages/Packages")
					fmt.Println(string(packagesContent))

					fmt.Println("Attempting apt --dry-run installation of the problematic package(s) for debugging purposes:\n")

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

					fmt.Println("Printing apt-cache policy output for debugging purposes:\n")
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

	StatusGreen("Package installation complete.")
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
	if app == "" {
		return fmt.Errorf("purge_packages function can only be used by apps to install packages (the app variable was not set)")
	}

	Status("Allowing packages required by the " + app + " app to be uninstalled")

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

		fmt.Printf("These packages were: %s\n", strings.Join(deps, ", "))
		Status("Purging the " + pkgName + " package...")

		// Wait for APT locks
		if err := AptLockWait(); err != nil {
			return fmt.Errorf("failed to wait for APT locks: %w", err)
		}

		// Create command for apt purge
		var purgeArgs []string
		if isUpdate {
			// Skip --autoremove for faster updates
			purgeArgs = []string{"-E", "apt", "purge", "-y", pkgName}
		} else {
			// Normal case, use --autoremove
			purgeArgs = []string{"-E", "apt", "purge", "-y", pkgName, "--autoremove"}
		}

		cmd := exec.Command("sudo", purgeArgs...)

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

		Status("Apt finished.")

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

			fmt.Println("\033[91mFailed to uninstall the packages!\033[39m")
			fmt.Printf("The APT reported these errors:\n\033[91m%s\033[39m\n", errorStr)
			fmt.Println(combinedOutput)

			return fmt.Errorf("apt reported errors: %s", errorStr)
		}
	} else {
		// Check for legacy installed-packages file
		installDataDir := os.Getenv("DIRECTORY")
		if installDataDir == "" {
			installDataDir = "/home/pi/pi-apps" // Default location
		}

		legacyPkgFile := filepath.Join(installDataDir, "data", "installed-packages", app)

		if FileExists(legacyPkgFile) {
			Warning("Using the old implementation - an installed-packages file instead of a dummy deb")

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
				Status("Legacy package file is empty. Nothing to do.")
				// Remove the legacy file
				os.Remove(legacyPkgFile)
				StatusGreen("All packages have been purged successfully.")
				return nil
			}

			// Convert to slice for the command
			packages := strings.Split(pkgList, " ")

			// Wait for APT locks
			if err := AptLockWait(); err != nil {
				return fmt.Errorf("failed to wait for APT locks: %w", err)
			}

			// Create command for apt purge with real-time output
			purgeArgs := append([]string{"-E", "apt", "purge", "-y"}, packages...)
			cmd := exec.Command("sudo", purgeArgs...)

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

			Status("Apt finished.")

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

				fmt.Println("\033[91mFailed to uninstall the packages!\033[39m")
				fmt.Printf("The APT reported these errors:\n\033[91m%s\033[39m\n", errorStr)
				fmt.Println(combinedOutput)

				return fmt.Errorf("apt reported errors: %s", errorStr)
			}
		} else {
			Status("The " + pkgName + " package is not installed so there's nothing to do.")
		}
	}

	// Clean up the installed-packages file
	installDataDir := os.Getenv("DIRECTORY")
	if installDataDir == "" {
		installDataDir = "/home/pi/pi-apps" // Default location
	}

	legacyPkgFile := filepath.Join(installDataDir, "data", "installed-packages", app)
	if FileExists(legacyPkgFile) {
		os.Remove(legacyPkgFile)
	}

	StatusGreen("All packages have been purged successfully.")
	return nil
}

// GetIconFromPackage finds the largest icon file (png or svg) installed by a package
// This is a Go implementation of the original bash get_icon_from_package function
func GetIconFromPackage(packages ...string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("get_icon_from_package requires at least one apt package name")
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
	if ppaName == "" {
		return fmt.Errorf("ubuntu_ppa_installer(): This function is used to add a ppa to a ubuntu based install but a required input argument was missing")
	}

	// Prepare ppaGrep for checking if the PPA is already added
	ppaGrep := ppaName
	if !strings.HasSuffix(ppaName, "/") {
		ppaGrep += "/"
	}

	// Check if PPA is already added
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(TARGET_OF)")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check if PPA is already added: %w", err)
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
		Status("Skipping " + ppaName + " PPA, already added")
	} else {
		Status("Adding " + ppaName + " PPA")

		// Add the PPA
		cmd = exec.Command("sudo", "add-apt-repository", "ppa:"+ppaName, "-y")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add PPA: %w", err)
		}

		// Update APT
		if err := AptUpdate(); err != nil {
			return fmt.Errorf("failed to update APT after adding PPA: %w", err)
		}
	}

	// Check if PPA .list filename exists under the current distro codename
	// On a distro upgrade, the .list filename is not updated and add-apt-repository can re-use the old filename

	// Get the OS codename
	osCodename, err := getOSCodename()
	if err != nil {
		return fmt.Errorf("failed to get OS codename: %w", err)
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
				return fmt.Errorf("failed to rename PPA file: %w", err)
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
	if ppaName == "" || ppaDist == "" || key == "" {
		return fmt.Errorf("debian_ppa_installer(): This function is used to add a ppa to a debian based install but a required input argument was missing")
	}

	// Prepare ppaGrep for checking if the PPA is already added
	ppaGrep := ppaName
	if !strings.HasSuffix(ppaName, "/") {
		ppaGrep = ppaName + "/ubuntu " + ppaDist
	}

	// Check if PPA is already added
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(TARGET_OF)")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check if PPA is already added: %w", err)
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
		Status("Skipping " + ppaName + " PPA, already added")
	} else {
		Status("Adding " + ppaName + " PPA")

		// Create the .list file
		ppaOwner := strings.Split(ppaName, "/")[0]
		ppaRepo := strings.Split(ppaName, "/")[1]
		listFilename := fmt.Sprintf("/etc/apt/sources.list.d/%s-ubuntu-%s-%s.list", ppaOwner, ppaRepo, ppaDist)

		// Add repository to sources.list.d
		cmd = exec.Command("sudo", "tee", listFilename)
		cmd.Stdin = strings.NewReader(fmt.Sprintf("deb https://ppa.launchpadcontent.net/%s/ubuntu %s main", ppaName, ppaDist))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add repository to sources.list: %w", err)
		}

		// Add GPG key
		cmd = exec.Command("sudo", "apt-key", "adv", "--keyserver", "hkp://keyserver.ubuntu.com:80", "--recv-keys", key)
		if err := cmd.Run(); err != nil {
			// Remove the list file if key addition fails
			removeCmd := exec.Command("sudo", "rm", "-f", listFilename)
			removeCmd.Run() // Ignore errors from removal

			return fmt.Errorf("failed to sign the %s PPA: %w", ppaName, err)
		}

		// Update APT
		if err := AptUpdate(); err != nil {
			return fmt.Errorf("failed to update APT after adding PPA: %w", err)
		}
	}

	return nil
}

// Helper function to get the OS codename
func getOSCodename() (string, error) {
	// Try to get OS codename from lsb_release
	cmd := exec.Command("lsb_release", "-cs")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// Fallback: Try to parse /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/os-release: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		// Look for VERSION_CODENAME
		if strings.HasPrefix(line, "VERSION_CODENAME=") {
			return strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"'"), nil
		}
	}

	return "", fmt.Errorf("could not determine OS codename")
}

// AddExternalRepo adds an external apt repository and its gpg key
// Follows https://wiki.debian.org/DebianRepository/UseThirdParty specification with deb822 format
func AddExternalRepo(reponame, pubkeyurl, uris, suites, components string, additionalOptions ...string) error {
	// Check if all needed vars are set
	if reponame == "" {
		return fmt.Errorf("add_external_repo: reponame not set")
	}
	if uris == "" {
		return fmt.Errorf("add_external_repo: uris not set")
	}
	if suites == "" {
		return fmt.Errorf("add_external_repo: suites not set")
	}
	if pubkeyurl == "" {
		return fmt.Errorf("add_external_repo: pubkeyurl not set")
	}

	// Exit if reponame or uri or suite contains space
	if strings.Contains(reponame, " ") || strings.Contains(uris, " ") || strings.Contains(suites, " ") {
		return fmt.Errorf("add_external_repo: provided reponame, uris, or suites contains a space")
	}

	// Check if links are valid
	fmt.Println("add_external_repo: checking 3rd party pubkeyurl validity")
	cmd := exec.Command("wget", "--spider", pubkeyurl)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add_external_repo: pubkeyurl isn't a valid link: %w", err)
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

	// Use a pipe to download the key and save it as a gpg keyring
	downloadCmd := exec.Command("wget", "-qO-", pubkeyurl)
	downloadOut, err := downloadCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("add_external_repo: failed to create stdout pipe: %w", err)
	}

	gpgCmd := exec.Command("sudo", "gpg", "--dearmor", "-o", keyringFile)
	gpgCmd.Stdin = downloadOut

	if err := downloadCmd.Start(); err != nil {
		return fmt.Errorf("add_external_repo: failed to start download command: %w", err)
	}

	if err := gpgCmd.Run(); err != nil {
		// Clean up if gpg command fails
		exec.Command("sudo", "rm", "-f", sourcesFile).Run()
		exec.Command("sudo", "rm", "-f", keyringFile).Run()
		return fmt.Errorf("add_external_repo: download from specified pubkeyurl failed: %w", err)
	}

	if err := downloadCmd.Wait(); err != nil {
		// Clean up if download command fails
		exec.Command("sudo", "rm", "-f", sourcesFile).Run()
		exec.Command("sudo", "rm", "-f", keyringFile).Run()
		return fmt.Errorf("add_external_repo: download from specified pubkeyurl failed: %w", err)
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
	if reponame == "" {
		return fmt.Errorf("rm_external_repo: reponame not provided")
	}

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
	// Get the OS codename
	osCodename, err := getOSCodename()
	if err != nil {
		return fmt.Errorf("failed to determine OS codename: %w", err)
	}

	// Determine if the OS is supported with a specific repository configuration
	switch osCodename {
	case "bionic", "focal", "jammy", "noble", "buster", "bullseye", "bookworm":
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

// PipxInstall installs packages using pipx, handling various distro and Python version requirements
// This is a Go implementation of the original bash pipx_install function
func PipxInstall(packages ...string) error {
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for pipx installation")
	}

	// Use "pipx" as the app name for tracking dependencies
	appName := "pipx"

	// Check if pipx is available with a new enough version (>= 1.0.0)
	pipxAvailable := PackageAvailable("pipx", "")

	pipxNewEnough := false
	if pipxAvailable {
		pipxNewEnough = PackageIsNewEnough("pipx", "1.0.0")
	}

	if pipxAvailable && pipxNewEnough {
		// Install pipx from package manager if it's available and new enough
		if err := InstallPackages(appName, "pipx", "python3-venv"); err != nil {
			return fmt.Errorf("failed to install pipx and python3-venv: %w", err)
		}
	} else {
		// Check Python version to determine installation method
		python3NewEnough := PackageIsNewEnough("python3", "3.7")

		if python3NewEnough {
			// Python 3.7+ is available, install pipx using pip
			if err := InstallPackages(appName, "python3-venv"); err != nil {
				return fmt.Errorf("failed to install python3-venv: %w", err)
			}

			fmt.Println("Installing pipx with pip...")
			cmd := exec.Command("sudo", "-H", "python3", "-m", "pip", "install", "--upgrade", "pipx")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install pipx with pip: %w", err)
			}
		} else {
			// Check if Python 3.8 is available
			python38Available := PackageAvailable("python3.8", "")

			if python38Available {
				// Install Python 3.8 and its venv package
				if err := InstallPackages(appName, "python3.8", "python3.8-venv"); err != nil {
					return fmt.Errorf("failed to install python3.8 and python3.8-venv: %w", err)
				}

				fmt.Println("Installing pipx with pip using Python 3.8...")
				cmd := exec.Command("sudo", "-H", "python3.8", "-m", "pip", "install", "--upgrade", "pipx")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to install pipx with pip using python3.8: %w", err)
				}
			} else {
				// No suitable Python version found
				return fmt.Errorf("pipx is not available on your distro and so cannot install %s to python venv", strings.Join(packages, " "))
			}
		}
	}

	// Check if pipx command exists after installation
	fmt.Println("Verifying pipx installation...")
	checkCmd := exec.Command("which", "pipx")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("pipx installation failed: command not found after installation")
	}

	// Install the requested packages with pipx
	fmt.Printf("Installing %s with pipx...\n", strings.Join(packages, ", "))

	// Create the pipx install command with environment variables
	cmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx install %s",
			strings.Join(packages, " ")))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s with pipx: %w", strings.Join(packages, " "), err)
	}

	fmt.Printf("Successfully installed %s with pipx\n", strings.Join(packages, ", "))
	return nil
}

// PipxUninstall uninstalls packages that were installed using pipx
// This is a Go implementation of the original bash pipx_uninstall function
func PipxUninstall(packages ...string) error {
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for pipx uninstallation")
	}

	// Check if pipx command exists
	checkCmd := exec.Command("which", "pipx")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("pipx is not installed: command not found")
	}

	// Uninstall the requested packages with pipx
	fmt.Printf("Uninstalling %s with pipx...\n", strings.Join(packages, ", "))

	// Create the pipx uninstall command with environment variables
	cmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx uninstall %s",
			strings.Join(packages, " ")))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall %s with pipx: %w", strings.Join(packages, " "), err)
	}

	fmt.Printf("Successfully uninstalled %s with pipx\n", strings.Join(packages, ", "))
	return nil
}
