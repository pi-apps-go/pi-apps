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

// Module: settings.go
// Description: Provides a native GTK3 settings interface for Pi-Apps using GOTK3 bindings

// general TODO: add plugin section as we are going to allow users to add plugins to Pi-Apps Go thanks to the plugin package

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// SettingsWindow represents the main settings window
type SettingsWindow struct {
	window     *gtk.Window
	notebook   *gtk.Notebook
	directory  string
	settings   map[string]*Setting
	comboBoxes map[string]*gtk.ComboBoxText
}

// Setting represents a configuration setting
type Setting struct {
	Name        string
	Description string
	Values      []string
	Current     string
	Tooltip     string
}

// NewSettingsWindow creates and initializes a new settings window
func NewSettingsWindow() (*SettingsWindow, error) {

	// Set approprative name
	glib.SetPrgname("Pi-Apps Settings")

	// Initialize GTK
	gtk.Init(nil)

	// Get PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	sw := &SettingsWindow{
		directory:  directory,
		settings:   make(map[string]*Setting),
		comboBoxes: make(map[string]*gtk.ComboBoxText),
	}

	// Load settings from files
	if err := sw.loadSettings(); err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	// Apply current App List Style theme if available
	if appListSetting, exists := sw.settings["App List Style"]; exists {
		sw.applyThemeToCurrentWindow(appListSetting.Current)
	}

	// Create the main window
	if err := sw.createWindow(); err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	return sw, nil
}

// Show displays the settings window
func (sw *SettingsWindow) Show() {
	sw.window.ShowAll()
}

// Run starts the GTK main loop
func (sw *SettingsWindow) Run() {
	gtk.Main()
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// loadSettings loads all settings from the setting-params directory
func (sw *SettingsWindow) loadSettings() error {
	settingParamsDir := filepath.Join(sw.directory, "etc", "setting-params")

	// Ensure settings directory exists
	settingsDir := filepath.Join(sw.directory, "data", "settings")
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
		paramPath := filepath.Join(settingParamsDir, settingName)

		// Read parameter file
		content, err := os.ReadFile(paramPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		lines := strings.Split(string(content), "\n")
		setting := &Setting{
			Name:   settingName,
			Values: []string{},
		}

		// Parse content
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") {
				// This is the description/tooltip
				if setting.Description == "" {
					setting.Description = strings.TrimPrefix(line, "#")
					setting.Tooltip = setting.Description
				} else {
					setting.Tooltip += "\n" + strings.TrimPrefix(line, "#")
				}
			} else {
				// This is a value option
				setting.Values = append(setting.Values, line)
			}
		}

		// Read current value
		currentPath := filepath.Join(settingsDir, settingName)
		if fileExists(currentPath) {
			currentBytes, err := os.ReadFile(currentPath)
			if err == nil {
				setting.Current = strings.TrimSpace(string(currentBytes))
			}
		}

		// If no current value, use first available option
		if setting.Current == "" && len(setting.Values) > 0 {
			setting.Current = setting.Values[0]
			// Save default value
			if err := os.WriteFile(currentPath, []byte(setting.Current), 0644); err != nil {
				return fmt.Errorf("failed to write default setting: %w", err)
			}
		}

		// Special processing for App List Style to generate theme options
		sw.processAppListStyleSetting(setting)

		sw.settings[settingName] = setting
	}

	return nil
}

// createWindow creates and configures the main settings window
func (sw *SettingsWindow) createWindow() error {
	var err error

	// Create main window
	sw.window, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("failed to create window: %w", err)
	}

	// Configure window
	sw.window.SetTitle("Pi-Apps Settings")
	sw.window.SetDefaultSize(600, 550)
	sw.window.SetPosition(gtk.WIN_POS_CENTER)
	sw.window.SetResizable(true)

	// Set window icon
	iconPath := filepath.Join(sw.directory, "icons", "settings.png")
	if fileExists(iconPath) {
		sw.window.SetIconFromFile(iconPath)
	}

	// Connect close signal
	sw.window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Create main container
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return fmt.Errorf("failed to create main box: %w", err)
	}
	mainBox.SetMarginTop(15)
	mainBox.SetMarginBottom(15)
	mainBox.SetMarginStart(15)
	mainBox.SetMarginEnd(15)

	// Create notebook for tabbed interface
	sw.notebook, err = gtk.NotebookNew()
	if err != nil {
		return fmt.Errorf("failed to create notebook: %w", err)
	}

	// Add settings tab
	if err := sw.createSettingsTab(); err != nil {
		return fmt.Errorf("failed to create settings tab: %w", err)
	}

	// Add actions tab
	if err := sw.createActionsTab(); err != nil {
		return fmt.Errorf("failed to create actions tab: %w", err)
	}

	// Create button box with better alignment
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return fmt.Errorf("failed to create button box: %w", err)
	}
	buttonBox.SetHAlign(gtk.ALIGN_END)
	buttonBox.SetMarginTop(10)

	// Add buttons
	if err := sw.createButtons(buttonBox); err != nil {
		return fmt.Errorf("failed to create buttons: %w", err)
	}

	// Pack everything
	mainBox.PackStart(sw.notebook, true, true, 0)
	mainBox.PackStart(buttonBox, false, false, 0)
	sw.window.Add(mainBox)

	return nil
}
