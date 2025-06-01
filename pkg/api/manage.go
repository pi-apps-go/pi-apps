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

// Module: manage.go
// Description: Provides functions for managing app installations.

package api

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Action represents the type of operation to be performed on an app
type Action string

const (
	ActionInstall   Action = "install"
	ActionUninstall Action = "uninstall"
	ActionUpdate    Action = "update"
	ActionRefresh   Action = "refresh"
)

// AnsiStripWriter wraps an io.Writer and strips ANSI escape sequences before writing
type AnsiStripWriter struct {
	writer io.Writer
	ansiRe *regexp.Regexp
}

// NewAnsiStripWriter creates a new AnsiStripWriter
func NewAnsiStripWriter(w io.Writer) *AnsiStripWriter {
	return &AnsiStripWriter{
		writer: w,
		ansiRe: regexp.MustCompile(`\x1b\[?[0-9;]*[a-zA-Z]`),
	}
}

// Write implements io.Writer interface, stripping ANSI codes before writing
func (w *AnsiStripWriter) Write(p []byte) (n int, err error) {
	// Strip ANSI escape sequences
	cleaned := w.ansiRe.ReplaceAll(p, []byte{})
	// Write the cleaned content to the underlying writer
	_, err = w.writer.Write(cleaned)
	// Return the original length to satisfy the io.Writer interface
	return len(p), err
}

// ManageApp handles installation, uninstallation, or updating of an app
func ManageApp(action Action, appName string, isUpdate bool) error {
	// Get PI_APPS_DIR environment variable
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Validate the app exists
	appDir := filepath.Join(piAppsDir, "apps", appName)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return fmt.Errorf("app %s does not exist", appName)
	}

	// Check if app is disabled before installation
	appStatus, err := GetAppStatus(appName)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}
	if action == ActionInstall && appStatus == "disabled" {
		fmt.Printf("Not installing the %s app. IT IS DISABLED.\n", appName)
		return nil
	}

	// Check internet connection if installing
	if action == ActionInstall {
		if err := CheckInternetConnection(); err != nil {
			return fmt.Errorf("no internet connection: %w", err)
		}
	}

	// Set up logging
	logDir := filepath.Join(piAppsDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFilename := fmt.Sprintf("%s-incomplete-%s.log", action, appName)
	logPath := filepath.Join(logDir, logFilename)

	// If log file already exists with another status, create a new one with a suffix
	if _, err := os.Stat(logPath); err == nil {
		// File already exists, add a number to the filename
		i := 1
		for {
			newLogPath := fmt.Sprintf("%s%d", logPath, i)
			if _, err := os.Stat(newLogPath); os.IsNotExist(err) {
				logPath = newLogPath
				break
			}
			i++
		}
	}

	// Create log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Write to log file (plain text) and stdout (colored)
	fmt.Fprintf(logFile, "%s %sing %s...\n\n", time.Now().Format("2006-01-02 15:04:05"), action, appName)
	Status(fmt.Sprintf("%sing \033[1m%s\033[22m...", action, appName))

	// Check if system is supported
	supported, supportMessage := IsAppSupportedOnSystem(appName)
	if !supported {
		// Add ANSI color codes to match the original Bash implementation
		warningPrefix := "\033[93m\033[5m◢◣\033[25m\033[0m \033[93mWARNING:\033[0m \033[93mYOUR SYSTEM IS UNSUPPORTED:\033[0m\n"
		// Also format the message in yellow like in the original
		formattedMessage := "\033[93m" + supportMessage + "\033[0m\n"
		disabledMsg := "\033[103m\033[30mThe ability to send error reports has been disabled.\033[39m\033[49m\n"
		waitingMsg := "\033[103m\033[30mWaiting 10 seconds... (To cancel, press Ctrl+C or close this terminal)\033[39m\033[49m\n"

		// Write colored messages to stdout (terminal)
		Warning(fmt.Sprintf("%s%s%s%s", warningPrefix, formattedMessage, disabledMsg, waitingMsg))

		// Write plain text to log file (no color codes)
		fmt.Fprintf(logFile, "WARNING: YOUR SYSTEM IS UNSUPPORTED:\n%s\n", supportMessage)
		fmt.Fprintf(logFile, "The ability to send error reports has been disabled.\n")
		fmt.Fprintf(logFile, "Waiting 10 seconds... (To cancel, press Ctrl+C or close this terminal)\n")

		// We don't show a GUI dialog here - that's handled by the CLI tools with the -gui flag
		time.Sleep(10 * time.Second)
	}

	// Determine script to run or package to install/uninstall
	var cmd *exec.Cmd
	appType, err := AppType(appName)
	if err != nil {
		return fmt.Errorf("failed to determine app type: %w", err)
	}

	if appType == "standard" {
		// Standard app with scripts
		var scriptName string
		if action == ActionInstall {
			scriptName = GetScriptNameForCPU(appName)
			if scriptName == "" {
				return fmt.Errorf("no suitable install script found for %s", appName)
			}
		} else if action == ActionUninstall {
			scriptName = "uninstall"
		}

		scriptPath := filepath.Join(appDir, scriptName)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return fmt.Errorf("script %s does not exist for app %s", scriptPath, appName)
		}

		// Make script executable
		os.Chmod(scriptPath, 0755)

		// Set up command
		cmd = exec.Command(scriptPath)

		// Set up environment variables for the script
		env := os.Environ()
		env = append(env, fmt.Sprintf("PI_APPS_DIR=%s", getPiAppsDir()))
		env = append(env, fmt.Sprintf("app=%s", appName))
		env = append(env, "DEBIAN_FRONTEND=noninteractive")

		if isUpdate {
			env = append(env, "script_input=update")
		}

		cmd.Env = env
	} else if appType == "package" {
		// Package-based app
		packages, err := PkgAppPackagesRequired(appName)
		if err != nil {
			return fmt.Errorf("failed to get required packages: %w", err)
		}

		if packages == "" {
			return fmt.Errorf("no installable packages specified for app %s", appName)
		}

		// Set up apt command
		aptAction := "install"
		if action == ActionUninstall {
			aptAction = "purge --autoremove"
		}

		// For package type apps, use apt
		cmd = exec.Command("sudo", "apt", aptAction, "-yf")
		cmd.Args = append(cmd.Args, strings.Fields(packages)...)
	} else {
		return fmt.Errorf("unknown app type: %s", appType)
	}

	// Set command working directory to user's home
	cmd.Dir = os.Getenv("HOME")

	// Create ANSI-stripping writer for log file to avoid escape codes in logs
	ansiStripLogWriter := NewAnsiStripWriter(logFile)
	// Connect command output to log file with ANSI stripped
	cmd.Stdout = ansiStripLogWriter
	cmd.Stderr = ansiStripLogWriter

	// Run the command
	err = cmd.Run()

	// Determine success or failure
	if err != nil {
		// Write plain text to log file (no color codes)
		fmt.Fprintf(logFile, "\nFailed to %s %s!\n", action, appName)
		fmt.Fprintf(logFile, "Need help? Copy the ENTIRE terminal output or take a screenshot.\n")
		fmt.Fprintf(logFile, "Please ask on Github: https://github.com/pi-apps-go/pi-apps/issues/new/choose\n")
		fmt.Fprintf(logFile, "Or on Discord: https://discord.gg/RXSTvaUvuu\n")

		// Write colored messages to stdout (terminal) matching the original bash formatting
		fmt.Printf("\n\033[91mFailed to %s %s!\033[39m\n", action, appName)
		fmt.Printf("\033[40m\033[93m\033[5m◢◣\033[25m\033[39m\033[49m\033[93mNeed help? Copy the \033[1mENTIRE\033[0m\033[49m\033[93m terminal output or take a screenshot.\n")
		fmt.Printf("Please ask on Github: \033[94m\033[4mhttps://github.com/pi-apps-go/pi-apps/issues/new/choose\033[24m\033[93m\n")
		fmt.Printf("Or on Discord: \033[94m\033[4mhttps://discord.gg/RXSTvaUvuu\033[0m\n")

		// Format the log file to add device information (consistent with bash version)
		formatErr := FormatLogfile(logPath)
		if formatErr != nil {
			fmt.Printf("Warning: failed to format log file %s: %v\n", logPath, formatErr)
		}

		// Rename log file to indicate failure
		newLogPath := strings.Replace(logPath, "-incomplete-", "-fail-", 1)
		os.Rename(logPath, newLogPath)

		// If app is script-type, set status to corrupted if the error is not system, internet, or package related
		if appType == "standard" {
			// Use log_diagnose to determine error type and set appropriate status
			diagnosis, err := LogDiagnose(logPath, true)
			if diagnosis.ErrorType == "system" || diagnosis.ErrorType == "internet" || diagnosis.ErrorType == "package" {
				SetAppStatus(appName, "failed")
			} else {
				SetAppStatus(appName, "corrupted")
			}
			if err != nil {
				ErrorNoExit("Unable to detect error type, setting it as corrupted")
				SetAppStatus(appName, "failed")
			}
		}

		// Extract exit code from error if available
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command failed: exit code %d", exitError.ExitCode())
		}
		return fmt.Errorf("command failed: %v", err)
	}

	// Success
	fmt.Fprintf(logFile, "\n%s %sed successfully.\n", action, appName)
	StatusGreen(fmt.Sprintf("%s %sed successfully.", action, appName))

	// Format the log file to add device information (consistent with bash version)
	formatErr := FormatLogfile(logPath)
	if formatErr != nil {
		fmt.Printf("Warning: failed to format log file %s: %v\n", logPath, formatErr)
	}

	// Rename log file to indicate success
	newLogPath := strings.Replace(logPath, "-incomplete-", "-success-", 1)
	os.Rename(logPath, newLogPath)

	// Set app status
	SetAppStatus(appName, string(action)+"ed")

	// If package-type app, refresh its status
	if appType == "package" {
		RefreshPackageAppStatus(appName)
	}

	return nil
}

// InstallApp installs the specified app
func InstallApp(appName string) error {
	// Validate app exists
	if !IsValidApp(appName) {
		return fmt.Errorf("app '%s' does not exist", appName)
	}

	// Check if already installed
	if IsAppInstalled(appName) {
		return fmt.Errorf("app '%s' is already installed", appName)
	}

	// Get app type
	appType, err := GetAppType(appName)
	if err != nil {
		return fmt.Errorf("failed to determine app type: %v", err)
	}

	// Handle app installation based on app type
	switch appType {
	case "package":
		return installPackageApp(appName)
	case "standard":
		err := installScriptApp(appName)
		return err
	default:
		return fmt.Errorf("unsupported app type: %s", appType)
	}
}

// UninstallApp uninstalls the specified app
func UninstallApp(appName string) error {
	// Validate app exists
	if !IsValidApp(appName) {
		return fmt.Errorf("app '%s' does not exist", appName)
	}

	// Check if already uninstalled (allow uninstall for corrupted apps)
	appStatus, err := GetAppStatus(appName)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}
	if appStatus == "uninstalled" {
		return fmt.Errorf("app '%s' is not installed", appName)
	}
	// Note: corrupted apps are allowed to be uninstalled

	// Get app type
	appType, err := GetAppType(appName)
	if err != nil {
		return fmt.Errorf("failed to determine app type: %v", err)
	}

	// Handle app uninstallation based on app type
	switch appType {
	case "package":
		return uninstallPackageApp(appName)
	case "standard":
		return uninstallScriptApp(appName)
	default:
		return fmt.Errorf("unsupported app type: %s", appType)
	}
}

// UpdateApp updates the specified app (reinstalls it)
func UpdateApp(appName string) error {
	// Validate app exists
	if !IsValidApp(appName) {
		return fmt.Errorf("app '%s' does not exist", appName)
	}

	// Check if already uninstalled (allow update for corrupted apps)
	appStatus, err := GetAppStatus(appName)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}
	if appStatus == "uninstalled" {
		return fmt.Errorf("app '%s' is not installed", appName)
	}
	// Note: corrupted apps are allowed to be updated

	// Get app type
	appType, err := GetAppType(appName)
	if err != nil {
		return fmt.Errorf("failed to determine app type: %v", err)
	}

	// Handle app update based on app type
	switch appType {
	case "package":
		// For package-based apps, this is essentially a reinstall
		err = uninstallPackageApp(appName)
		if err != nil {
			return fmt.Errorf("failed to uninstall app during update: %v", err)
		}
		return installPackageApp(appName)
	case "standard":
		// For script-based apps, run the update script if it exists, otherwise reinstall
		updateScriptPath := filepath.Join(getPiAppsDir(), "apps", appName, "update")
		if _, err := os.Stat(updateScriptPath); err == nil {
			return runAppScript(appName, "update")
		}

		// No update script, so uninstall and reinstall
		err = uninstallScriptApp(appName)
		if err != nil {
			return fmt.Errorf("failed to uninstall app during update: %v", err)
		}
		return installScriptApp(appName)
	default:
		return fmt.Errorf("unsupported app type: %s", appType)
	}
}

// InstallIfNotInstalled installs the app only if not already installed
func InstallIfNotInstalled(appName string) error {
	// Validate app exists
	if !IsValidApp(appName) {
		return fmt.Errorf("app '%s' does not exist", appName)
	}

	// Check if already installed
	if IsAppInstalled(appName) {
		fmt.Printf("App '%s' is already installed, skipping installation\n", appName)
		return nil
	}

	// Not installed, so install it
	return InstallApp(appName)
}

// CheckInternetConnection checks if the internet is available
func CheckInternetConnection() error {
	cmd := exec.Command("wget", "--spider", "https://github.com")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("github.com failed to respond: %w", err)
	}
	return nil
}

// SetAppStatus sets the status of an app (installed, uninstalled, corrupted, disabled)
func SetAppStatus(appName, status string) error {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	statusDir := filepath.Join(piAppsDir, "data", "status")
	os.MkdirAll(statusDir, 0755)

	statusFile := filepath.Join(statusDir, appName)
	return os.WriteFile(statusFile, []byte(status), 0644)
}

// RefreshPackageAppStatus refreshes the status of a package-based app
func RefreshPackageAppStatus(appName string) error {
	packages, err := PkgAppPackagesRequired(appName)
	if err != nil {
		return fmt.Errorf("failed to get required packages: %w", err)
	}

	// If no packages are specified, assume uninstalled
	if packages == "" {
		return SetAppStatus(appName, "uninstalled")
	}

	// Check if all packages are installed
	allInstalled := true
	for _, pkg := range strings.Fields(packages) {
		if !PackageInstalled(pkg) {
			allInstalled = false
			break
		}
	}

	if allInstalled {
		return SetAppStatus(appName, "installed")
	} else {
		return SetAppStatus(appName, "uninstalled")
	}
}

// GetScriptNameForCPU determines which install script to use based on the current CPU architecture
func GetScriptNameForCPU(appName string) string {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return ""
	}

	appDir := filepath.Join(piAppsDir, "apps", appName)

	// Check available scripts
	hasInstall := FileExists(filepath.Join(appDir, "install"))
	hasInstall32 := FileExists(filepath.Join(appDir, "install-32"))
	hasInstall64 := FileExists(filepath.Join(appDir, "install-64"))

	// Determine architecture (32-bit or 64-bit)
	is64bit := Is64BitOS()

	// Choose the appropriate script based on architecture and available scripts
	if is64bit {
		if hasInstall64 {
			return "install-64"
		} else if hasInstall {
			return "install"
		} else if hasInstall32 {
			// Fall back to 32-bit script if that's all that's available
			return "install-32"
		}
	} else {
		// On 32-bit systems
		if hasInstall32 {
			return "install-32"
		} else if hasInstall {
			return "install"
		}
		// Don't fall back to 64-bit script on 32-bit systems
	}

	return "" // No suitable script found
}

// Is64BitOS checks if the current OS is 64-bit
func Is64BitOS() bool {
	// Run 'uname -m' to get architecture
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	arch := strings.TrimSpace(string(output))
	return strings.HasSuffix(arch, "64") || arch == "aarch64"
}

// IsSystemSupportedMessage returns a message explaining why the system might not be supported
func IsSystemSupportedMessage() (string, error) {
	status, err := IsSystemSupported()
	if err != nil {
		return "", err
	}

	// Just return the message directly from the status
	return status.Message, nil
}

// ValidateApps checks a list of apps to ensure they exist and can be managed
func ValidateApps(action Action, appList []string) ([]string, error) {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	var validApps []string
	for _, app := range appList {
		appDir := filepath.Join(piAppsDir, "apps", app)
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			fmt.Printf("Invalid app '%s'. Cannot %s it.\n", app, action)
			continue
		}

		// Additional validation could be added here

		validApps = append(validApps, app)
	}

	return validApps, nil
}

// MultiBatchOperation performs the specified action on multiple apps
func MultiBatchOperation(action Action, appList []string) error {
	validApps, err := ValidateApps(action, appList)
	if err != nil {
		return err
	}

	if len(validApps) == 0 {
		return fmt.Errorf("no valid apps to %s", action)
	}

	failedApps := []string{}
	for _, app := range validApps {
		fmt.Printf("Processing %s: %s\n", action, app)
		if err := ManageApp(action, app, false); err != nil {
			fmt.Printf("Failed to %s %s: %v\n", action, app, err)
			failedApps = append(failedApps, app)
		}
	}

	if len(failedApps) > 0 {
		return fmt.Errorf("failed to %s the following apps: %s", action, strings.Join(failedApps, ", "))
	}

	return nil
}

// MultiInstall installs multiple apps
func MultiInstall(appList []string) error {
	return MultiBatchOperation(ActionInstall, appList)
}

// MultiUninstall uninstalls multiple apps
func MultiUninstall(appList []string) error {
	return MultiBatchOperation(ActionUninstall, appList)
}

// Helper functions

// installPackageApp installs a package-based app
func installPackageApp(appName string) error {
	// Show colored status message
	Status(fmt.Sprintf("Installing \033[1m%s\033[22m...", appName))

	packageListPath := filepath.Join(getPiAppsDir(), "apps", appName, "packages")

	// Read packages list
	packageListBytes, err := os.ReadFile(packageListPath)
	if err != nil {
		return fmt.Errorf("failed to read packages list: %v", err)
	}

	packageList := strings.TrimSpace(string(packageListBytes))
	packages := strings.Fields(packageList)

	// Install packages with sudo
	cmd := exec.Command("sudo", append([]string{"apt-get", "install", "-y"}, packages...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install packages: %v", err)
	}

	// Show success message
	StatusGreen(fmt.Sprintf("Installed %s successfully.", appName))

	// Mark app as installed
	return markAppAsInstalled(appName)
}

// uninstallPackageApp uninstalls a package-based app
func uninstallPackageApp(appName string) error {
	// Show colored status message
	Status(fmt.Sprintf("Uninstalling \033[1m%s\033[22m...", appName))

	packageListPath := filepath.Join(getPiAppsDir(), "apps", appName, "packages")

	// Read packages list
	packageListBytes, err := os.ReadFile(packageListPath)
	if err != nil {
		return fmt.Errorf("failed to read packages list: %v", err)
	}

	packageList := strings.TrimSpace(string(packageListBytes))
	packages := strings.Fields(packageList)

	// Uninstall packages with sudo
	cmd := exec.Command("sudo", append([]string{"apt-get", "remove", "-y"}, packages...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall packages: %v", err)
	}

	// Show success message
	StatusGreen(fmt.Sprintf("Uninstalled %s successfully.", appName))

	// Mark app as uninstalled
	return markAppAsUninstalled(appName)
}

// installScriptApp installs a script-based app
func installScriptApp(appName string) error {
	err := runAppScript(appName, "install")
	return err
}

// uninstallScriptApp uninstalls a script-based app
func uninstallScriptApp(appName string) error {
	return runAppScript(appName, "uninstall")
}

// runAppScript runs a script for an app (install, uninstall, update)
func runAppScript(appName, scriptName string) error {
	// Get PI_APPS_DIR environment variable
	piAppsDir := getPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Set up logging
	logDir := filepath.Join(piAppsDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFilename := fmt.Sprintf("%s-incomplete-%s.log", scriptName, appName)
	logPath := filepath.Join(logDir, logFilename)

	// If log file already exists with another status, create a new one with a suffix
	if _, err := os.Stat(logPath); err == nil {
		// File already exists, add a number to the filename
		i := 1
		for {
			newLogPath := fmt.Sprintf("%s%d", logPath, i)
			if _, err := os.Stat(newLogPath); os.IsNotExist(err) {
				logPath = newLogPath
				break
			}
			i++
		}
	}

	// Create log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Write to log file (plain text) and stdout (colored)
	fmt.Fprintf(logFile, "%s %sing %s...\n\n", time.Now().Format("2006-01-02 15:04:05"), scriptName, appName)
	Status(fmt.Sprintf("%sing \033[1m%s\033[22m...", strings.Title(scriptName), appName))

	scriptPath := filepath.Join(getPiAppsDir(), "apps", appName, scriptName)

	// Check if script exists
	if _, err := os.Stat(scriptPath); err != nil {
		// If specific script doesn't exist, try architecture-specific versions for install
		if scriptName == "install" {
			// Check for install-32 and install-64
			install32Path := filepath.Join(getPiAppsDir(), "apps", appName, "install-32")
			install64Path := filepath.Join(getPiAppsDir(), "apps", appName, "install-64")

			// Get system architecture
			arch, err := GetSystemArchitecture()
			if err != nil {
				return err
			}

			// Choose the appropriate script based on architecture
			is32Bit := arch == "armv7l" || arch == "armv6l" || arch == "i386" || arch == "x86" || arch == "riscv32" // riscv32 is not supported by Go but still there for future support
			is64Bit := arch == "aarch64" || arch == "arm64" || arch == "x86_64" || arch == "amd64" || arch == "riscv64"

			if is32Bit {
				// 32-bit architecture
				if _, err := os.Stat(install32Path); err == nil {
					scriptPath = install32Path
				} else {
					return fmt.Errorf("install script does not exist for app '%s' on 32-bit architecture", appName)
				}
			} else if is64Bit {
				// 64-bit architecture
				if _, err := os.Stat(install64Path); err == nil {
					scriptPath = install64Path
				} else if _, err := os.Stat(install32Path); err == nil {
					// Fallback to 32-bit if 64-bit specific script doesn't exist
					scriptPath = install32Path
				} else {
					return fmt.Errorf("install script does not exist for app '%s' on 64-bit architecture", appName)
				}
			} else {
				return fmt.Errorf("unsupported architecture: %s", arch)
			}
		} else {
			return fmt.Errorf("%s script does not exist for app '%s'", scriptName, appName)
		}
	}

	fmt.Printf("Running script: %s\n", scriptPath)
	fmt.Fprintf(logFile, "Running script: %s\n", scriptPath)

	// Make script executable if it's not already
	err = os.Chmod(scriptPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	// Get API wrapper path
	apiBashWrapper := filepath.Join(piAppsDir, "api")
	if _, err := os.Stat(apiBashWrapper); os.IsNotExist(err) {
		return fmt.Errorf("API bash wrapper not found at %s",
			filepath.Join(piAppsDir, "api"))
	}

	// Create a temp script that sources the API and then runs the original script
	tempDir, err := os.MkdirTemp("", "pi-apps-script")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Read the original script
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %v", err)
	}

	// Check if we need sudo (for system operations)
	needsSudo := scriptName == "install" || scriptName == "uninstall"

	// Set up the app directory
	appDir := filepath.Join(getPiAppsDir(), "apps", appName)

	// Get the path to the api-go binary
	apiDir := filepath.Dir(apiBashWrapper)
	apiBinaryPath := filepath.Join(apiDir, "api-go")

	// Create a symlink to api-go in the temp directory
	tempApiBinaryPath := filepath.Join(tempDir, "api-go")
	if err := os.Symlink(apiBinaryPath, tempApiBinaryPath); err != nil {
		return fmt.Errorf("failed to create symlink to api-go: %v", err)
	}

	// Create a new script with the API environment explicitly set
	newScriptContent := fmt.Sprintf(`#!/bin/bash

# Set the path to api-go binary explicitly
export GO_API_BIN="%s"
export PI_APPS_DIR="%s"

# Source the API with explicit binary location
source "%s"

# Change to the app directory
cd "%s"

# Original script content follows
%s`, apiBinaryPath, piAppsDir, apiBashWrapper, appDir, string(scriptContent))

	// Write the new script to a temporary file
	tempScriptPath := filepath.Join(tempDir, "temp_script.sh")
	err = os.WriteFile(tempScriptPath, []byte(newScriptContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp script: %v", err)
	}

	var cmd *exec.Cmd
	if needsSudo {
		cmd = exec.Command("sudo", "-E", tempScriptPath)
	} else {
		cmd = exec.Command(tempScriptPath)
	}

	// Create ANSI-stripping writer for log file to avoid escape codes in logs
	ansiStripLogWriter := NewAnsiStripWriter(logFile)
	// Connect command output to both log file (with ANSI stripped) and stdout (with ANSI preserved)
	multiWriter := io.MultiWriter(ansiStripLogWriter, os.Stdout)
	cmd.Stdout = multiWriter
	cmd.Stderr = multiWriter
	cmd.Dir = appDir

	// Set environment variables that scripts might need
	env := os.Environ()
	env = append(env, fmt.Sprintf("PI_APPS_DIR=%s", getPiAppsDir()))
	env = append(env, fmt.Sprintf("app=%s", appName)) // Use lowercase 'app' to match bash API
	env = append(env, "DEBIAN_FRONTEND=noninteractive")

	// Add script_input=update if this is an update operation
	if scriptName == "update" || strings.Contains(scriptName, "update") {
		env = append(env, "script_input=update")
	}

	cmd.Env = env

	// Run the command
	err = cmd.Run()

	// Determine success or failure
	if err != nil {
		// Write plain text to log file (no color codes)
		fmt.Fprintf(logFile, "\nFailed to %s %s!\n", scriptName, appName)
		fmt.Fprintf(logFile, "Need help? Copy the ENTIRE terminal output or take a screenshot.\n")
		fmt.Fprintf(logFile, "Please ask on Github: https://github.com/pi-apps-go/pi-apps/issues/new/choose\n")
		fmt.Fprintf(logFile, "Or on Discord: https://discord.gg/RXSTvaUvuu\n")

		// Write colored messages to stdout (terminal) matching the original bash formatting
		fmt.Printf("\n\033[91mFailed to %s %s!\033[39m\n", scriptName, appName)
		fmt.Printf("\033[40m\033[93m\033[5m◢◣\033[25m\033[39m\033[49m\033[93mNeed help? Copy the \033[1mENTIRE\033[0m\033[49m\033[93m terminal output or take a screenshot.\n")
		fmt.Printf("Please ask on Github: \033[94m\033[4mhttps://github.com/pi-apps-go/pi-apps/issues/new/choose\033[24m\033[93m\n")
		fmt.Printf("Or on Discord: \033[94m\033[4mhttps://discord.gg/RXSTvaUvuu\033[0m\n")

		// Format the log file to add device information (consistent with bash version)
		err := FormatLogfile(logPath)
		if err != nil {
			fmt.Printf("Warning: failed to format log file %s: %v\n", logPath, err)
		}

		// Rename log file to indicate failure
		newLogPath := strings.Replace(logPath, "-incomplete-", "-fail-", 1)
		os.Rename(logPath, newLogPath)

		// For script-type apps, set status to corrupted if the error is not system, internet, or package related
		appType, typeErr := GetAppType(appName)
		if typeErr == nil && appType == "standard" {
			// Use log_diagnose to determine error type and set appropriate status
			diagnosis, diagErr := LogDiagnose(newLogPath, true)
			if diagErr == nil && (diagnosis.ErrorType == "system" || diagnosis.ErrorType == "internet" || diagnosis.ErrorType == "package") {
				SetAppStatus(appName, "failed")
			} else {
				SetAppStatus(appName, "corrupted")
			}
			if diagErr != nil {
				ErrorNoExit("Unable to detect error type, setting it as failed")
				SetAppStatus(appName, "failed")
			}
		}

		// Extract exit code from error if available
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command failed: exit code %d", exitError.ExitCode())
		}
		return fmt.Errorf("command failed: %v", err)
	}

	// Success
	fmt.Fprintf(logFile, "\n%s %sed successfully.\n", scriptName, appName)
	StatusGreen(fmt.Sprintf("%sed %s successfully.", strings.Title(scriptName), appName))

	// Format the log file to add device information (consistent with bash version)
	err = FormatLogfile(logPath)
	if err != nil {
		fmt.Printf("Warning: failed to format log file %s: %v\n", logPath, err)
	}

	// Rename log file to indicate success
	newLogPath := strings.Replace(logPath, "-incomplete-", "-success-", 1)
	os.Rename(logPath, newLogPath)

	// Display success message consistently for both package and script apps
	if scriptName == "install" {
		return markAppAsInstalled(appName)
	} else if scriptName == "uninstall" {
		return markAppAsUninstalled(appName)
	}

	return nil
}

// IsValidApp checks if an app exists in the Pi-Apps directory
func IsValidApp(appName string) bool {
	appDir := filepath.Join(getPiAppsDir(), "apps", appName)
	if _, err := os.Stat(appDir); err != nil {
		return false
	}
	return true
}

// IsAppInstalled checks if an app is installed
func IsAppInstalled(appName string) bool {
	statusFile := filepath.Join(getPiAppsDir(), "data", "status", appName)
	content, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(content))
	return status == "installed"
}

// GetAppType determines the type of the app (package or standard)
func GetAppType(appName string) (string, error) {
	packageListPath := filepath.Join(getPiAppsDir(), "apps", appName, "packages")
	installPath := filepath.Join(getPiAppsDir(), "apps", appName, "install")
	install32Path := filepath.Join(getPiAppsDir(), "apps", appName, "install-32")
	install64Path := filepath.Join(getPiAppsDir(), "apps", appName, "install-64")

	// Check if it has a packages file
	if _, err := os.Stat(packageListPath); err == nil {
		return "package", nil
	}

	// Check if it has any install script
	if _, err := os.Stat(installPath); err == nil {
		return "standard", nil
	}

	// Check for install-32 script
	if _, err := os.Stat(install32Path); err == nil {
		return "standard", nil
	}

	// Check for install-64 script
	if _, err := os.Stat(install64Path); err == nil {
		return "standard", nil
	}

	return "", errors.New("cannot determine app type")
}

// markAppAsInstalled marks an app as installed in the status directory
func markAppAsInstalled(appName string) error {
	statusDir := filepath.Join(getPiAppsDir(), "data", "status")

	// Create status directory if it doesn't exist
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		err = os.MkdirAll(statusDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create status directory: %v", err)
		}
	}

	statusFile := filepath.Join(statusDir, appName)
	return os.WriteFile(statusFile, []byte("installed"), 0644)
}

// markAppAsUninstalled marks an app as uninstalled in the status directory
func markAppAsUninstalled(appName string) error {
	statusFile := filepath.Join(getPiAppsDir(), "data", "status", appName)

	// If the status file exists, remove it
	if _, err := os.Stat(statusFile); err == nil {
		return os.Remove(statusFile)
	}

	// If file doesn't exist, we're good (it's already "uninstalled")
	return nil
}

// getPiAppsDir returns the Pi-Apps directory from environment variable
func getPiAppsDir() string {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		// Default to a reasonable location if env var not set
		homeDir, err := os.UserHomeDir()
		if err == nil {
			piAppsDir = filepath.Join(homeDir, "pi-apps")
		} else {
			piAppsDir = "/home/pi/pi-apps"
		}
	}
	return piAppsDir
}

// GetSystemArchitecture returns the current system architecture
func GetSystemArchitecture() (string, error) {
	// Try runtime.GOARCH first for Go's built-in architecture detection
	arch := runtime.GOARCH

	// Map Go's architecture names to our expected format
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	case "arm":
		// For ARM, we need to determine if it's 32 or 64 bit
		cmd := exec.Command("uname", "-m")
		output, err := cmd.Output()
		if err == nil {
			detectedArch := strings.TrimSpace(string(output))
			if detectedArch == "aarch64" || detectedArch == "arm64" {
				arch = "aarch64"
			} else if detectedArch == "armv7l" || detectedArch == "armv6l" {
				arch = detectedArch
			}
		}
	case "arm64":
		arch = "aarch64"
	case "riscv64":
		arch = "riscv64"
	case "riscv32": // Go does not support riscv32 as a build target, but still there for future support
		arch = "riscv32"
	}

	return arch, nil
}

// IsAppSupportedOnSystem checks if the app supports the current system
func IsAppSupportedOnSystem(appName string) (bool, string) {
	arch, err := GetSystemArchitecture()
	if err != nil {
		return false, fmt.Sprintf("Failed to determine system architecture: %v", err)
	}

	// Check if app directory exists
	appDir := filepath.Join(getPiAppsDir(), "apps", appName)
	if _, err := os.Stat(appDir); err != nil {
		return false, fmt.Sprintf("App directory does not exist: %v", err)
	}

	// Check the architecture specific scripts
	is32bit := arch == "armv7l" || arch == "armv6l" || arch == "i386" || arch == "riscv32" // Go does not support riscv32 as a build target, but still there for future support
	is64bit := arch == "aarch64" || arch == "arm64" || arch == "x86_64" || arch == "riscv64"

	// Generic install script
	installPath := filepath.Join(appDir, "install")
	if _, err := os.Stat(installPath); err == nil {
		return true, ""
	}

	// Architecture specific scripts
	install32Path := filepath.Join(appDir, "install-32")
	install64Path := filepath.Join(appDir, "install-64")

	if is32bit {
		if _, err := os.Stat(install32Path); err == nil {
			return true, ""
		}
		return false, fmt.Sprintf("This app doesn't support 32-bit systems (%s)", arch)
	} else if is64bit {
		if _, err := os.Stat(install64Path); err == nil {
			return true, ""
		}
		// 64-bit systems can run 32-bit apps
		if _, err := os.Stat(install32Path); err == nil {
			return true, ""
		}
		return false, fmt.Sprintf("This app doesn't support 64-bit systems (%s)", arch)
	}

	return false, fmt.Sprintf("This app doesn't support your system architecture (%s)", arch)
}
