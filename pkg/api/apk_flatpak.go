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

// Module: apk_flatpak.go
// Description: Provides functions for managing flatpak apps when using the APK package manager.

//go:build apk

package api

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// FlatpakInstall installs an app using flatpak
func FlatpakInstall(app string) error {
	if app == "" {
		Error("flatpak_install(): This function is used to install a flatpak app, but nothing was specified")
		return fmt.Errorf("flatpak_install(): This function is used to install a flatpak app, but nothing was specified")
	}

	// Check if flatpak is installed
	if _, err := exec.LookPath("flatpak"); err != nil {
		// Try to install flatpak using APK
		Status("Flatpak is not installed. Installing from APK repositories...")
		if PackageAvailable("flatpak", "") {
			cmd := exec.Command("sudo", "apk", "add", "flatpak")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				Error(fmt.Sprintf("flatpak_install(): Could not install flatpak: %v", err))
				return fmt.Errorf("flatpak_install(): Could not install flatpak: %w", err)
			}
		} else {
			Error(fmt.Sprintf("flatpak_install(): Could not install %s because flatpak is not available", app))
			return fmt.Errorf("flatpak_install(): Could not install %s because flatpak is not available", app)
		}
	}

	// Check if flatpak version is new enough
	isNewEnough := PackageIsNewEnough("flatpak", "1.14.4")
	if !isNewEnough {
		Status("Flatpak version is older than required. Upgrading...")
		
		// Try to upgrade flatpak
		cmd := exec.Command("sudo", "apk", "upgrade", "flatpak")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			Warning(fmt.Sprintf("Failed to upgrade flatpak: %v", err))
			// Continue anyway - might still work
		}
	}

	// Add flathub remote
	Status("Adding Flathub remote repository...")
	err := execCommand("sudo", "flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo")
	if err != nil {
		Status("Could not add Flathub as root, trying as user...")
		// Try as user if sudo failed
		err = execCommand("flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo")
		if err != nil {
			Error(fmt.Sprintf("Failed to add Flathub remote: %v", err))
			return fmt.Errorf("flatpak failed to add flathub remote: %w", err)
		}
	}
	StatusGreen("Flathub repository added successfully")

	// Install the app
	Status(fmt.Sprintf("Installing %s from Flathub...", app))
	err = execCommand("sudo", "flatpak", "install", "flathub", app, "-y")
	if err != nil {
		Status("Could not install as root, trying as user...")
		// Try as user if sudo failed
		err = execCommand("flatpak", "install", "flathub", app, "-y")
		if err != nil {
			Error(fmt.Sprintf("Failed to install %s: %v", app, err))
			return fmt.Errorf("flatpak failed to install %s: %w", app, err)
		}
	}

	StatusGreen(fmt.Sprintf("%s installed successfully", app))

	// Handle desktop launcher visibility without reboot
	if !strings.Contains(os.Getenv("XDG_DATA_DIRS"), "/var/lib/flatpak/exports/share") {
		Status("Setting up desktop integration for immediate use...")
		appDir := "/var/lib/flatpak/exports/share/applications"
		tempDir := "/usr/share/applications/flatpak-temporary"

		// Check if there are files in the applications directory
		files, err := os.ReadDir(appDir)
		if err == nil && len(files) > 0 {
			// Check if temporary directory is empty or doesn't exist
			_, err := os.Stat(tempDir)
			if os.IsNotExist(err) || isFlatpakDirEmpty(tempDir) {
				// Create temporary directory if it doesn't exist
				if err := execCommand("sudo", "mkdir", "-p", tempDir); err != nil {
					Warning(fmt.Sprintf("Failed to create temporary directory: %v", err))
					return fmt.Errorf("failed to create temporary directory: %w", err)
				}
				// Bind mount the applications directory
				if err := execCommand("sudo", "mount", "--bind", appDir, tempDir); err != nil {
					Warning(fmt.Sprintf("Failed to bind mount applications directory: %v", err))
					return fmt.Errorf("failed to bind mount applications directory: %w", err)
				}
				Status("Desktop integration set up successfully")
			}
		}
	} else {
		// Clean up temporary directory if XDG_DATA_DIRS includes flatpak path
		Status("Cleaning up temporary desktop integration...")
		if err := execCommand("sudo", "rm", "-rf", "/usr/share/applications/flatpak-temporary"); err != nil {
			Warning(fmt.Sprintf("Failed to clean up temporary directory: %v", err))
		}
	}

	Status("Flatpak installation completed")
	return nil
}

// FlatpakUninstall uninstalls an app using flatpak
func FlatpakUninstall(app string) error {
	if app == "" {
		Error("flatpak_uninstall(): This function is used to uninstall a flatpak app, but nothing was specified")
		return fmt.Errorf("flatpak_uninstall(): This function is used to uninstall a flatpak app, but nothing was specified")
	}

	// Check if flatpak is installed
	if _, err := exec.LookPath("flatpak"); err != nil {
		// If flatpak is not installed, return success
		Status("Flatpak is not installed, nothing to uninstall")
		return nil
	}

	// Check if the app is installed
	Status("Checking if app is installed...")
	cmd := exec.Command("flatpak", "list")
	output, err := cmd.Output()
	if err != nil {
		Error(fmt.Sprintf("Failed to list installed flatpak apps: %v", err))
		return fmt.Errorf("failed to list installed flatpak apps: %w", err)
	}

	if strings.Contains(string(output), app) {
		Status(fmt.Sprintf("Uninstalling %s...", app))
		// Try to uninstall with sudo first
		err := execCommand("sudo", "flatpak", "uninstall", app, "-y")
		if err != nil {
			Status("Could not uninstall as root, trying as user...")
			// Try as user if sudo failed
			err = execCommand("flatpak", "uninstall", app, "-y")
			if err != nil {
				Error(fmt.Sprintf("Failed to uninstall %s: %v", app, err))
				return fmt.Errorf("flatpak failed to uninstall %s: %w", app, err)
			}
		}
		StatusGreen(fmt.Sprintf("%s uninstalled successfully", app))
	} else {
		Status(fmt.Sprintf("App %s is not installed, nothing to uninstall", app))
	}

	return nil
}

// FlatpakPackageInstalled checks if a specific flatpak package is installed
func FlatpakPackageInstalled(pkg string) bool {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return false
	}

	cmd := exec.Command("flatpak", "list", "--columns=application")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), pkg)
}

// IsFlatpakAppCompatibleWithArch checks if a flatpak app is compatible with the target architecture
func IsFlatpakAppCompatibleWithArch(flatpakID, targetArch string) bool {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return false
	}

	cmd := exec.Command("flatpak", "info", "--show-metadata", flatpakID)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Arch:") {
			arches := strings.Fields(strings.TrimPrefix(line, "Arch:"))
			for _, arch := range arches {
				if arch == targetArch {
					return true
				}
			}
			return false
		}
	}

	return true
}

// Helper function to check if a directory is empty
func isFlatpakDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	return err == io.EOF
}

// Helper function to run commands and capture output
func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	// Create pipes for stdout and stderr
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
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Process output and filter control sequences
	go processOutput(stdout, os.Stdout)
	go processOutput(stderr, os.Stderr)

	// Wait for the command to complete
	return cmd.Wait()
}

// Process output and filter control sequences
func processOutput(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// Filter out terminal control sequences
		line = filterControlSequences(line)
		fmt.Fprintln(w, line)
	}
}

// Function to filter out terminal control sequences
func filterControlSequences(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}




