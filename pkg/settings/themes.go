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

// Module: themes.go
// Description: Theme detection and handling for App List Style setting
//
// This module handles GTK3 theme detection and application for Pi-Apps.
// Unlike the original bash implementation that used YAD with theme prefixes,
// this version uses direct GTK3 bindings and applies themes via the GTK_THEME
// environment variable for better integration with the desktop environment.

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateThemeOptions generates the available theme options for App List Style
// This mimics the theme detection logic from the original bash script
func (sw *SettingsWindow) generateThemeOptions() []string {
	var themes []string

	// Start with default theme (no GTK_THEME override)
	themes = append(themes, "default")

	// Define theme directories to search
	themeDirs := []string{
		"/usr/share/themes",
		filepath.Join(os.Getenv("HOME"), ".local/share/themes"),
		filepath.Join(os.Getenv("HOME"), ".themes"),
	}

	// Search for GTK3 themes
	gtk3Themes := make(map[string]bool) // Use map to avoid duplicates

	for _, themeDir := range themeDirs {
		if !fileExists(themeDir) {
			continue
		}

		dirs, err := os.ReadDir(themeDir)
		if err != nil {
			continue
		}

		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}

			themeName := dir.Name()
			if themeName == "Default" {
				continue
			}

			themePath := filepath.Join(themeDir, themeName)

			// Check if this theme has GTK3 support
			if hasGTK3Support(themePath) {
				if !gtk3Themes[themeName] {
					// Use actual theme name without "yad-" prefix since we're using GTK3 directly
					themes = append(themes, themeName)
					gtk3Themes[themeName] = true
				}
			}
		}
	}

	// Add xlunch preset themes (these are different display modes, not GTK themes)
	themes = append(themes, "xlunch-dark")
	themes = append(themes, "xlunch-dark-3d")
	themes = append(themes, "xlunch-light-3d")

	return themes
}

// hasGTK3Support checks if a theme directory contains GTK3 theme files
func hasGTK3Support(themePath string) bool {
	files, err := os.ReadDir(themePath)
	if err != nil {
		return false
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "gtk-3.") {
			return true
		}
	}

	return false
}

// processAppListStyleSetting handles special processing for the App List Style setting
func (sw *SettingsWindow) processAppListStyleSetting(setting *Setting) {
	if setting.Name == "App List Style" {
		// Replace the default values with dynamically generated theme options
		setting.Values = sw.generateThemeOptions()

		// Ensure current value is valid, if not set to first available
		validCurrent := false
		for _, value := range setting.Values {
			if value == setting.Current {
				validCurrent = true
				break
			}
		}

		if !validCurrent && len(setting.Values) > 0 {
			setting.Current = setting.Values[0]
		}
	}
}

// applyThemeToCurrentWindow applies a theme to the current settings window
func (sw *SettingsWindow) applyThemeToCurrentWindow(themeName string) {
	if themeName == "" || themeName == "default" {
		// Reset to default theme
		os.Unsetenv("GTK_THEME")
		return
	}

	// Skip xlunch themes as they are not GTK themes
	if strings.HasPrefix(themeName, "xlunch-") {
		return
	}

	// Apply GTK theme via environment variable
	os.Setenv("GTK_THEME", themeName)
}

// GetThemeEnvironmentForLaunch returns the environment variables needed to launch
// a GUI application with the specified theme
func GetThemeEnvironmentForLaunch(themeName string) []string {
	env := os.Environ()

	// Remove any existing GTK_THEME
	var filteredEnv []string
	for _, envVar := range env {
		if !strings.HasPrefix(envVar, "GTK_THEME=") {
			filteredEnv = append(filteredEnv, envVar)
		}
	}

	// Add GTK_THEME if it's not default and not xlunch
	if themeName != "" && themeName != "default" && !strings.HasPrefix(themeName, "xlunch-") {
		filteredEnv = append(filteredEnv, "GTK_THEME="+themeName)
	}

	return filteredEnv
}

// isXlunchTheme checks if a theme name represents an xlunch display mode
// TODO: this is not used anywhere and giving warnings in gopls, either remove or use it in detecting themes
func isXlunchTheme(themeName string) bool {
	return strings.HasPrefix(themeName, "xlunch-")
}

// isGTKTheme checks if a theme name represents a GTK theme
// TODO: this is not used anywhere and giving warnings in gopls, either remove or use it in detecting themes
func isGTKTheme(themeName string) bool {
	return themeName != "" && !isXlunchTheme(themeName)
}

// GetCurrentAppListStyle reads the current App List Style setting from disk
func GetCurrentAppListStyle() (string, error) {
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	settingPath := filepath.Join(directory, "data", "settings", "App List Style")
	if !fileExists(settingPath) {
		return "default", nil // Return default if no setting exists
	}

	content, err := os.ReadFile(settingPath)
	if err != nil {
		return "default", err
	}

	theme := strings.TrimSpace(string(content))
	if theme == "" {
		return "default", nil
	}

	return theme, nil
}

// GetCurrentThemeEnvironment returns environment variables for the current theme
// This is a convenience function for other packages to use
func GetCurrentThemeEnvironment() ([]string, error) {
	currentTheme, err := GetCurrentAppListStyle()
	if err != nil {
		return os.Environ(), err // Return default environment on error
	}

	return GetThemeEnvironmentForLaunch(currentTheme), nil
}
