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

// Module: vendor.go
// Description: Vendor required functions only from pi-apps' api package to not depend on the entire api package
// SPDX-License-Identifier: GPL-3.0-or-later

package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	// Global variables for system information
	PIAppsDir string
)

// ErrorNoExit displays an error message in red but does not exit the program
func ErrorNoExit(msg string) {
	// Use the exact same ANSI sequence as the original bash script
	fmt.Fprintln(os.Stderr, "\033[91m"+msg+"\033[0m")
}

// GetPiAppsDir returns the Pi-Apps directory path
func GetPiAppsDir() string {
	// First check if the environment variable is set
	if dir := os.Getenv("PI_APPS_DIR"); dir != "" && isValidPiAppsDir(dir) {
		return dir
	}

	// Check the pi-apps directory path (old folder name for Bash based implementation)
	piAppsPath := filepath.Join(os.Getenv("HOME"), "pi-apps")
	if isValidPiAppsDir(piAppsPath) {
		return piAppsPath
	}

	// Check the pi-apps-go directory path (new folder name for Go based implementation)
	piAppsPath = filepath.Join(os.Getenv("HOME"), "pi-apps-go")
	if isValidPiAppsDir(piAppsPath) {
		return piAppsPath
	}

	// Fall back to the original path
	return PIAppsDir
}

// isValidPiAppsDir checks if a directory is a valid Pi-Apps directory
// This function implements the same safety checks as the original Bash install script
// to prevent users from accidentally installing Pi-Apps into their HOME directory
// or other user directories (like Downloads), which could lead to data loss.
func isValidPiAppsDir(dir string) bool {
	if dir == "" {
		return false
	}

	// Get the absolute path to avoid issues with symlinks and relative paths
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	// Get HOME directory for comparison
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// If we can't get HOME, we can't validate, so reject for safety
		return false
	}
	absHomeDir, err := filepath.Abs(homeDir)
	if err != nil {
		return false
	}

	// CRITICAL SAFETY CHECK: Reject if the directory is exactly the HOME directory
	// This prevents the issue described in https://github.com/Botspot/pi-apps/issues/2137
	// where users accidentally installed Pi-Apps into their Downloads folder
	if absDir == absHomeDir {
		return false
	}

	// Reject common user directories that should never be used as Pi-Apps directory
	// These are directories where users typically store personal files
	// NOTE: This only rejects the directories themselves (e.g., /home/user/Videos),
	// NOT subdirectories within them (e.g., /home/user/Videos/pi-apps is allowed).
	// This matches the original Bash script's behavior of only rejecting $HOME itself.
	commonUserDirs := []string{
		"Downloads",
		"Documents",
		"Desktop",
		"Pictures",
		"Videos",
		"Music",
		"Public",
		"Templates",
	}
	for _, userDir := range commonUserDirs {
		userDirPath := filepath.Join(absHomeDir, userDir)
		// Only reject if the directory is exactly one of these user directories
		// Subdirectories (like /home/user/Videos/pi-apps) are allowed
		if absDir == userDirPath {
			return false
		}
	}

	// Check if the directory exists and has the expected files
	// This matches the original Bash script's check: [ ! -f "${DIRECTORY}/api" ] || [ ! -f "${DIRECTORY}/gui" ]
	apiFile := filepath.Join(absDir, "api")
	guiFile := filepath.Join(absDir, "gui")
	return DirExists(absDir) && FileExists(apiFile) && FileExists(guiFile)
}

// FileExists checks if a file exists
//
//	false - file does not exist
//	true - file exists
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
//
//	false - directory does not exist
//	true - directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
