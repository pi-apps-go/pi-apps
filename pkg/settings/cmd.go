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

// Module: cmd.go
// Description: Command-line interface and entry points for the settings package

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/botspot/pi-apps/pkg/api"
)

// Main entry point for settings, equivalent to the original bash script
func Main() error {
	// Get PI_APPS_DIR environment variable
	directory := api.GetPiAppsDir()
	if directory == "" {
		api.ErrorNoExit("PI_APPS_DIR environment variable not set")
		return nil
	}

	// Initialize internationalization
	if err := InitializeI18n(); err != nil {
		// Log error but continue - translations will fall back to English
		fmt.Printf("Warning: failed to initialize i18n: %v\n", err)
	}

	// Parse command line arguments
	args := os.Args[1:] // Skip program name

	// Handle special commands
	if len(args) > 0 {
		switch args[0] {
		case "refresh":
			return RefreshSettings()
		case "revert":
			return RevertSettings()
		default:
			return fmt.Errorf("unknown command: %s", args[0])
		}
	}

	// Create desktop entry if it doesn't exist
	if err := createDesktopEntry(); err != nil {
		fmt.Println(Tf("Warning: failed to create desktop entry: %v", err))
	}

	// Create and show the settings window
	window, err := NewSettingsWindow()
	if err != nil {
		return fmt.Errorf("failed to create settings window: %w", err)
	}

	window.Show()
	window.Run()

	return nil
}

// RefreshSettings creates default settings files if they don't exist
func RefreshSettings() error {
	directory := api.GetPiAppsDir()
	if directory == "" {
		api.ErrorNoExit(T("PI_APPS_DIR environment variable not set"))
		return nil
	}

	settingParamsDir := filepath.Join(directory, "etc", "setting-params")
	settingsDir := filepath.Join(directory, "data", "settings")

	// Ensure settings directory exists
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Read all setting parameter files
	files, err := os.ReadDir(settingParamsDir)
	if err != nil {
		return fmt.Errorf("failed to read setting-params directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		settingName := file.Name()
		settingPath := filepath.Join(settingsDir, settingName)

		// Only create if doesn't exist or is empty
		if !fileExists(settingPath) || isEmpty(settingPath) {
			// Read parameter file to get default value
			paramPath := filepath.Join(settingParamsDir, settingName)
			content, err := os.ReadFile(paramPath)
			if err != nil {
				continue
			}

			// Find first non-comment line as default
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					// Write default value
					if err := os.WriteFile(settingPath, []byte(line), 0644); err != nil {
						fmt.Println(Tf("Warning: failed to write default for %s: %v", settingName, err))
					}
					break
				}
			}
		}
	}

	return nil
}

// RevertSettings overwrites all settings with defaults
func RevertSettings() error {
	directory := api.GetPiAppsDir()
	if directory == "" {
		api.ErrorNoExit(T("PI_APPS_DIR environment variable not set"))
		return nil
	}

	settingParamsDir := filepath.Join(directory, "etc", "setting-params")
	settingsDir := filepath.Join(directory, "data", "settings")

	// Ensure settings directory exists
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Read all setting parameter files
	files, err := os.ReadDir(settingParamsDir)
	if err != nil {
		return fmt.Errorf("failed to read setting-params directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		settingName := file.Name()
		settingPath := filepath.Join(settingsDir, settingName)

		// Read parameter file to get default value
		paramPath := filepath.Join(settingParamsDir, settingName)
		content, err := os.ReadFile(paramPath)
		if err != nil {
			continue
		}

		// Find first non-comment line as default
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				// Write default value (overwrite existing)
				if err := os.WriteFile(settingPath, []byte(line), 0644); err != nil {
					fmt.Println(Tf("Warning: failed to revert setting %s: %v", settingName, err))
				}
				break
			}
		}
	}

	return nil
}

// createDesktopEntry creates the desktop entry for Pi-Apps Settings
func createDesktopEntry() error {
	directory := api.GetPiAppsDir()
	if directory == "" {
		api.ErrorNoExit(T("PI_APPS_DIR environment variable not set"))
		return nil
	}

	home := os.Getenv("HOME")
	if home == "" {
		api.ErrorNoExit(T("HOME environment variable not set"))
		return nil
	}

	desktopPath := filepath.Join(home, ".local", "share", "applications", "pi-apps-settings.desktop")

	// Check if already exists
	if fileExists(desktopPath) {
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		return fmt.Errorf("failed to create applications directory: %w", err)
	}

	// Create desktop entry content
	content := fmt.Sprintf(`[Desktop Entry]
Name=%s
Comment=%s
Exec=%s/settings
Icon=%s/icons/settings.png
Terminal=false
StartupWMClass=Pi-Apps-Settings
Type=Application
Categories=Settings;
StartupNotify=true
`, T("Pi-Apps Settings"), T("Configure Pi-Apps or create an App"), directory, directory)

	// Write desktop file
	if err := os.WriteFile(desktopPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write desktop entry: %w", err)
	}

	fmt.Println(T("Creating Settings menu button"))
	return nil
}

// isEmpty checks if a file is empty
func isEmpty(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	return len(strings.TrimSpace(string(content))) == 0
}
