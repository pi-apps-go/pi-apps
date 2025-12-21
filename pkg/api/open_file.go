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

// Module: open_file.go
// Description: Provides functions for opening files in the user's preferred text editor.

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OpenFile opens the specified file in the user's preferred text editor
//
// It reads the preferred editor from PI_APPS_DIR/data/settings/Preferred text editor
// and falls back to nano if the preferred editor is not available or fails
func OpenFile(filePath string) error {
	// Get the Pi-Apps directory
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		// Fallback to nano if we can't get Pi-Apps directory
		return openWithNano(filePath)
	}

	// Read the preferred text editor setting
	settingsFile := filepath.Join(piAppsDir, "data", "settings", "Preferred text editor")
	preferredEditor := ""

	if data, err := os.ReadFile(settingsFile); err == nil {
		preferredEditor = strings.TrimSpace(string(data))
	}

	// If no preferred editor is set or file doesn't exist, fallback to nano
	if preferredEditor == "" {
		return openWithNano(filePath)
	}
	// Try to open with the preferred editor
	var cmd *exec.Cmd
	switch preferredEditor {
	case "geany":
		go func() {
			cmd := exec.Command("geany", filePath)
			cmd.Start()
		}()
	case "mousepad":
		go func() {
			cmd := exec.Command("mousepad", filePath)
			cmd.Start()
		}()
	case "leafpad":
		go func() {
			cmd := exec.Command("leafpad", filePath)
			cmd.Start()
		}()
	case "nano":
		return openWithNano(filePath)
	case "Visual Studio Code":
		cmd = exec.Command("code", filePath)
		if err := cmd.Start(); err != nil {
			// If preferred editor fails, fallback to nano
			return openWithNano(filePath)
		}
	case "VSCodium":
		cmd = exec.Command("codium", filePath)
		if err := cmd.Start(); err != nil {
			// If preferred editor fails, fallback to nano
			return openWithNano(filePath)
		}
	default:
		// If unknown editor, try to use it as a command directly
		cmd = exec.Command(preferredEditor, filePath)
		if err := cmd.Start(); err != nil {
			// If preferred editor fails, fallback to nano
			return openWithNano(filePath)
		}
	}

	return nil
}

// openWithNano opens the file with nano editor
func openWithNano(filePath string) error {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Spawn nano in a separate goroutine to allow multiple sessions
	go func() {
		// Quote the file path to handle spaces properly
		TerminalRun(fmt.Sprintf("nano %s", filePath), "Editing "+filepath.Base(filePath))
	}()

	return nil
}
