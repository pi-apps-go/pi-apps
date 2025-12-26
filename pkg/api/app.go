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

// Module: app.go
// Description: Provides functions for managing apps.

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// getManageBinary returns the correct manage binary path, checking for multi-call binary first
func getManageBinary(directory string) (string, []string) {
	if multiCallBinary := os.Getenv("PI_APPS_MULTI_CALL_BINARY"); multiCallBinary != "" {
		// Use multi-call binary: multi-call-pi-apps manage [args...]
		return multiCallBinary, []string{"manage"}
	}
	// Use separate binary: manage [args...]
	return filepath.Join(directory, "manage"), []string{}
}

// AppStatus returns the current status of an app: installed, uninstalled, etc.
// It also handles deprecated apps that may have been removed from the apps directory
func AppStatus(app string) (string, error) {
	// Get the Pi-Apps directory
	directory := GetPiAppsDir()
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("app_status: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Check if app exists
	appDir := filepath.Join(directory, "apps", app)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		// If app doesn't exist, check if it's a deprecated app
		if IsDeprecatedApp(app) {
			// For deprecated apps, check status file
			statusFile := filepath.Join(directory, "data", "status", app)
			if _, err := os.Stat(statusFile); os.IsNotExist(err) {
				return "uninstalled", nil
			}
			// Read the status file
			statusBytes, err := os.ReadFile(statusFile)
			if err != nil {
				return "", fmt.Errorf("app_status: failed to read status file: %w", err)
			}
			status := strings.TrimSpace(string(statusBytes))
			return status, nil
		}
		return "", fmt.Errorf("app_status: app %s does not exist", app)
	}

	// Check if the app has a status file
	statusFile := filepath.Join(directory, "data", "status", app)
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		return "uninstalled", nil
	}

	// Read the status file
	statusBytes, err := os.ReadFile(statusFile)
	if err != nil {
		return "", fmt.Errorf("app_status: failed to read status file: %w", err)
	}

	status := strings.TrimSpace(string(statusBytes))
	return status, nil
}

// storeDeprecatedAppData stores the uninstall script and icons for a deprecated app
// so it can be uninstalled later even after the app directory is removed
func storeDeprecatedAppData(app, removalArch, message string) error {
	directory := GetPiAppsDir()
	if directory == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	appDir := filepath.Join(directory, "apps", app)
	deprecatedDir := filepath.Join(directory, "data", "deprecated-apps", app)

	// Create deprecated app directory
	if err := os.MkdirAll(deprecatedDir, 0755); err != nil {
		return fmt.Errorf("failed to create deprecated app directory: %w", err)
	}

	// Store uninstall script if it exists
	uninstallScript := filepath.Join(appDir, "uninstall")
	if _, err := os.Stat(uninstallScript); err == nil {
		destUninstall := filepath.Join(deprecatedDir, "uninstall")
		if err := CopyFile(uninstallScript, destUninstall); err != nil {
			return fmt.Errorf("failed to copy uninstall script: %w", err)
		}
		// Make it executable
		if err := os.Chmod(destUninstall, 0755); err != nil {
			return fmt.Errorf("failed to make uninstall script executable: %w", err)
		}
	}

	// Store icon files if they exist
	icon24 := filepath.Join(appDir, "icon-24.png")
	icon64 := filepath.Join(appDir, "icon-64.png")
	if _, err := os.Stat(icon24); err == nil {
		destIcon24 := filepath.Join(deprecatedDir, "icon-24.png")
		if err := CopyFile(icon24, destIcon24); err != nil {
			return fmt.Errorf("failed to copy icon-24.png: %w", err)
		}
	}
	if _, err := os.Stat(icon64); err == nil {
		destIcon64 := filepath.Join(deprecatedDir, "icon-64.png")
		if err := CopyFile(icon64, destIcon64); err != nil {
			return fmt.Errorf("failed to copy icon-64.png: %w", err)
		}
	}

	// Store metadata
	metadata := fmt.Sprintf("app=%s\nremovalArch=%s\nmessage=%s\n", app, removalArch, message)
	metadataFile := filepath.Join(deprecatedDir, "metadata")
	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// IsDeprecatedApp checks if an app is deprecated and returns true if it is
func IsDeprecatedApp(app string) bool {
	directory := GetPiAppsDir()
	if directory == "" {
		return false
	}
	metadataFile := filepath.Join(directory, "data", "deprecated-apps", app, "metadata")
	_, err := os.Stat(metadataFile)
	return err == nil
}

// GetDeprecatedAppUninstallScript returns the path to the stored uninstall script for a deprecated app
func GetDeprecatedAppUninstallScript(app string) (string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}
	uninstallScript := filepath.Join(directory, "data", "deprecated-apps", app, "uninstall")
	if _, err := os.Stat(uninstallScript); os.IsNotExist(err) {
		return "", fmt.Errorf("uninstall script not found for deprecated app %s", app)
	}
	return uninstallScript, nil
}

// GetDeprecatedAppIcon returns the path to the stored icon for a deprecated app
// Returns icon-64.png if available, otherwise icon-24.png, or empty string if neither exists
func GetDeprecatedAppIcon(app string) string {
	directory := GetPiAppsDir()
	if directory == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return ""
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}
	icon64 := filepath.Join(directory, "data", "deprecated-apps", app, "icon-64.png")
	if _, err := os.Stat(icon64); err == nil {
		return icon64
	}
	icon24 := filepath.Join(directory, "data", "deprecated-apps", app, "icon-24.png")
	if _, err := os.Stat(icon24); err == nil {
		return icon24
	}
	return ""
}

// removeDeprecatedAppEntries removes the deprecated app directory and all its contents
// This should be called after a deprecated app is successfully uninstalled
func removeDeprecatedAppEntries(app string) error {
	directory := GetPiAppsDir()
	if directory == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	deprecatedDir := filepath.Join(directory, "data", "deprecated-apps", app)
	if _, err := os.Stat(deprecatedDir); os.IsNotExist(err) {
		// Directory doesn't exist, nothing to clean up
		return nil
	}

	// Remove the entire deprecated app directory
	if err := os.RemoveAll(deprecatedDir); err != nil {
		return fmt.Errorf("failed to remove deprecated app directory: %w", err)
	}

	return nil
}

// RemoveDeprecatedApp prompts a user to uninstall a deprecated pi-apps application
// This is a Go implementation of the original bash remove_deprecated_app function
// It now stores the uninstall script and icons so the app can be uninstalled later
func RemoveDeprecatedApp(app, removalArch, message string) error {
	// Get the Pi-Apps directory
	directory := GetPiAppsDir()
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("remove_deprecated_app: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Get the app status
	appStatus, err := GetAppStatus(app)
	if err != nil {
		// If app doesn't exist, it might already be removed, but we can still mark it as deprecated
		// Store the deprecated app data anyway
		if err := storeDeprecatedAppData(app, removalArch, message); err != nil {
			return fmt.Errorf("failed to store deprecated app data: %w", err)
		}
		return nil
	}

	// Get the system architecture using unsafe.Sizeof
	arch := fmt.Sprintf("%d", unsafe.Sizeof(uintptr(0))*8)

	// Check if the app directory exists
	appDir := filepath.Join(directory, "apps", app)
	appDirExists := true
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		appDirExists = false
	}

	// Store deprecated app data (uninstall script, icons, metadata) before removing
	if appDirExists {
		if err := storeDeprecatedAppData(app, removalArch, message); err != nil {
			return fmt.Errorf("failed to store deprecated app data: %w", err)
		}
	}

	// Determine if we should prompt based on the conditions
	var shouldPrompt bool
	var text string

	switch {
	case removalArch != "" && appDirExists && arch == removalArch && appStatus == "installed":
		shouldPrompt = true
		if message != "" {
			text = Tf("Pi-Apps has deprecated %s for %s-bit OSs which you currently have installed.\n\n%s\n\nWould you like to uninstall it now or leave it installed? You can uninstall it later from Pi-Apps if needed.", app, removalArch, message)
		} else {
			text = Tf("Pi-Apps has deprecated %s for %s-bit OSs which you currently have installed.\nWould you like to uninstall it now or leave it installed? You can uninstall it later from Pi-Apps if needed.", app, removalArch)
		}
	case removalArch == "" && appDirExists && appStatus == "installed":
		shouldPrompt = true
		if message != "" {
			text = Tf("Pi-Apps has deprecated %s which you currently have installed.\n\n%s\n\nWould you like to uninstall it now or leave it installed? You can uninstall it later from Pi-Apps if needed.", app, message)
		} else {
			text = Tf("Pi-Apps has deprecated %s which you currently have installed.\nWould you like to uninstall it now or leave it installed? You can uninstall it later from Pi-Apps if needed.", app)
		}
	}

	// If we should prompt, show the dialog and process response
	if shouldPrompt {
		output, err := UserInputFunc(text, T("Uninstall now"), T("Leave installed"))
		if err != nil {
			return fmt.Errorf("remove_deprecated_app: failed to get user input: %w", err)
		}

		// If user chose to uninstall, run the uninstall command
		if output == T("Uninstall now") {
			manageBinary, baseArgs := getManageBinary(directory)
			args := append(baseArgs, "-uninstall", app)
			uninstallCmd := exec.Command(manageBinary, args...)
			uninstallCmd.Stdout = os.Stdout
			uninstallCmd.Stderr = os.Stderr
			if err := uninstallCmd.Run(); err != nil {
				return fmt.Errorf("remove_deprecated_app: failed to uninstall app: %w", err)
			}
		}
	}

	// Clean up files based on removal architecture
	if removalArch != "" {
		// Remove per-architecture script regardless of the current arch
		installScript := filepath.Join(directory, "apps", app, "install-"+removalArch)
		if _, err := os.Stat(installScript); err == nil {
			os.Remove(installScript)
		}

		// Remove unified-architecture script in case the new version has a per-architecture script
		unifiedScript := filepath.Join(directory, "apps", app, "install")
		if _, err := os.Stat(unifiedScript); err == nil {
			os.Remove(unifiedScript)
		}
	} else {
		// Only remove folder if the desired removal arch is unset (so remove on all architectures)
		if appDirExists {
			os.RemoveAll(appDir)
		}
	}

	return nil
}

// TerminalManage is a wrapper for executing app management actions
// This is a Go implementation of the original bash terminal_manage function
func TerminalManage(action, app string) error {
	// Forward to the multi-version with a single action
	return TerminalManageMulti(fmt.Sprintf("%s %s", action, app))
}

// TerminalManageMulti executes multiple app management actions in the Pi-Apps environment
// This is a Go implementation of the original bash terminal_manage_multi function
func TerminalManageMulti(queue string) error {
	// Get the Pi-Apps directory
	directory := GetPiAppsDir()
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("terminal_manage_multi: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Check if a daemon is already running by checking the pid file AND queue pipe
	// Just checking PID isn't enough because that PID might belong to a different process after reboot
	daemonPidFile := filepath.Join(directory, "data", "manage-daemon", "pid")
	daemonQueuePipe := filepath.Join(directory, "data", "manage-daemon", "queue")

	daemonRunning := false
	if _, err := os.Stat(daemonPidFile); err == nil {
		// Check if queue pipe also exists (indicates a real daemon)
		if info, err := os.Stat(daemonQueuePipe); err == nil && (info.Mode()&os.ModeNamedPipe) != 0 {
			// Read the PID from the file
			pidBytes, err := os.ReadFile(daemonPidFile)
			if err == nil {
				pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
				if err == nil {
					// Check if the process exists using os.FindProcess
					process, err := os.FindProcess(pid)
					if err == nil {
						// Send signal 0 to check if process is running
						if err := process.Signal(syscall.Signal(0)); err == nil {
							// All checks passed - daemon is actually running
							daemonRunning = true
						}
					}
				}
			}
		}
	}

	if daemonRunning {
		// Daemon is running, send the queue to it and exit
		manageBinary, baseArgs := getManageBinary(directory)
		args := append(baseArgs, "-daemon", queue)
		daemonCmd := exec.Command(manageBinary, args...)
		daemonCmd.Stdout = os.Stdout
		daemonCmd.Stderr = os.Stderr

		if err := daemonCmd.Run(); err != nil {
			return fmt.Errorf("terminal_manage_multi: failed to send queue to daemon: %w", err)
		}

		return nil
	}

	// If we reached here, there's no active daemon or the PID file doesn't exist
	// We'll run the daemon with our queue
	manageBinary, baseArgs := getManageBinary(directory)
	args := append(baseArgs, "-daemon", queue)
	daemonCmd := exec.Command(manageBinary, args...)
	daemonCmd.Stdout = os.Stdout
	daemonCmd.Stderr = os.Stderr

	if err := daemonCmd.Run(); err != nil {
		return fmt.Errorf("terminal_manage_multi: failed to run daemon: %w", err)
	}

	// Refresh the app list if there's a pipe
	pipeEnv := os.Getenv("pipe")
	if pipeEnv != "" {
		// Check if the pipe exists
		if _, err := os.Stat(pipeEnv); err == nil {
			// Write form feed character to the pipe
			pipeFile, err := os.OpenFile(pipeEnv, os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to open pipe: %w", err)
			}
			defer pipeFile.Close()

			// Write form feed character to clear the screen
			if _, err := pipeFile.WriteString("\f"); err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to write to pipe: %w", err)
			}

			// Get the app list style
			prefix := os.Getenv("prefix")
			styleFile := filepath.Join(directory, "data", "settings", "App List Style")
			styleBytes, err := os.ReadFile(styleFile)
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to read app list style: %w", err)
			}

			style := string(styleBytes)

			// Reload the app list via the gui
			preloadCmd := exec.Command(filepath.Join(directory, "gui"), "-mode", "preload-daemon-once", style, prefix)
			preloadOutput, err := preloadCmd.Output()
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to run preload: %w", err)
			}

			// Write the preload output to the pipe
			if _, err := pipeFile.Write(preloadOutput); err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to write preload output to pipe: %w", err)
			}
		}
	}

	return nil
}

// RefreshApp refreshes an app by copying its files from the update directory to the main directory
func RefreshApp(app string) error {
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Check if app exists in update directory
	updateAppDir := filepath.Join(directory, "update", "pi-apps", "apps", app)
	if !FileExists(updateAppDir) {
		return fmt.Errorf("app '%s' not found in update directory", app)
	}

	// Check if app exists in main directory
	mainAppDir := filepath.Join(directory, "apps", app)
	if !FileExists(mainAppDir) {
		return fmt.Errorf("app '%s' not found in main directory", app)
	}

	// Copy all files from update directory to main directory
	err := filepath.Walk(updateAppDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == updateAppDir {
			return nil
		}

		// Get relative path from update directory
		relPath, err := filepath.Rel(updateAppDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}

		// Construct destination path
		destPath := filepath.Join(mainAppDir, relPath)

		if info.IsDir() {
			// Create directory if it doesn't exist
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("error creating directory %s: %w", destPath, err)
			}
		} else {
			// Copy file
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("error copying file %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error refreshing app: %w", err)
	}

	return nil
}

// UpdateFile updates a specific file in the Pi-Apps system
func UpdateFile(filePath string) error {
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Check if file exists in update directory
	updateFilePath := filepath.Join(directory, "update", "pi-apps", filePath)
	if !FileExists(updateFilePath) {
		return fmt.Errorf("file '%s' not found in update directory", filePath)
	}

	// Check if file exists in main directory
	mainFilePath := filepath.Join(directory, filePath)
	if !FileExists(mainFilePath) {
		return fmt.Errorf("file '%s' not found in main directory", filePath)
	}

	// Copy file from update directory to main directory
	if err := copyFile(updateFilePath, mainFilePath); err != nil {
		return fmt.Errorf("error updating file: %w", err)
	}

	return nil
}
