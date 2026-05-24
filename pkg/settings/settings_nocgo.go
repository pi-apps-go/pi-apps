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

// Module: settings_nocgo.go
// Description: Provides a TUI settings interface for Pi-Apps using Charm stack terminal UI if cgo is disabled
// SPDX-License-Identifier: GPL-3.0-or-later

// general TODO: add plugin section as we are going to allow users to add plugins to Pi-Apps Go thanks to the plugin package

//go:build !cgo

package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SettingsWindow represents the main settings window
type SettingsWindow struct {
	directory string
	settings  map[string]*Setting
}

// Setting represents a configuration setting
type Setting struct {
	Name        string
	Description string
	Values      []string
	Current     string
	Tooltip     string
}

// SettingDefinition defines the structure of a setting with its metadata
type SettingDefinition struct {
	Name           string   // Setting name (e.g., "App List Style")
	Description    string   // Single-line or multi-line description
	AcceptedValues []string // List of valid values for this setting
	DefaultValue   string   // Default value (typically first in AcceptedValues)
}

// Embedded setting definitions - structured Go-native configuration
var (
	embeddedSettingDefinitions = []SettingDefinition{
		{
			Name:           "App List Style",
			Description:    "Pi-Apps can display the apps as a compact list (GTK 3 via gotk3), or as a group of larger icons. (xlunch like interface)",
			AcceptedValues: []string{"yad-default", "yad-light", "yad-dark", "xlunch-dark", "xlunch-dark-3d", "xlunch-light-3d"},
			DefaultValue:   "yad-default",
		},
		{
			Name:           "Check for updates",
			Description:    "How often should Pi-Apps check for app updates and refresh Pi-Apps on boot?",
			AcceptedValues: []string{"Daily", "Always", "Weekly", "Never"},
			DefaultValue:   "Daily",
		},
		{
			Name:           "Enable analytics",
			Description:    "Analytics are used to count the number of installs for each app.\nEach app is associated with a shlink link. During an install, that link is \"clicked\". The total number of clicks is used to calculate how many users each app has.\nThis information cannot possibly be used to identify you, or any personal information about you.",
			AcceptedValues: []string{"Yes", "No"},
			DefaultValue:   "Yes",
		},
		{
			Name:           "Preferred text editor",
			Description:    "Specify which text editor to use when editing install scripts",
			AcceptedValues: []string{"geany", "mousepad", "leafpad", "nano", "Visual Studio Code", "VSCodium"},
			DefaultValue:   "geany",
		},
		{
			Name:           "Show apps",
			Description:    "Most apps use scripts to install software from places like Github or Sourceforge.\nBut other apps can already be easily installed from Add/Remove Software. These apps are simply a shortcut to install apt-packages.\nThis option allows you to selectively show one type of app or the other, or both types.",
			AcceptedValues: []string{"All", "packages", "standard"},
			DefaultValue:   "All",
		},
		{
			Name:           "Show Edit button",
			Description:    "When viewing an App's details, display an Edit button to tweak it. Beware that updating that app later will undo your changes.",
			AcceptedValues: []string{"No", "Yes"},
			DefaultValue:   "No",
		},
		{
			Name:           "Shuffle App list",
			Description:    "Tired of Apps being sorted alphabetically? Randomizing the order will keep things fresh.",
			AcceptedValues: []string{"No", "Yes"},
			DefaultValue:   "No",
		},
	}
)

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// runSettingsAction launches an api-go subcommand with the App List Style theme environment.
func runSettingsAction(directory, action, appListTheme string) {
	var cmd *exec.Cmd
	apiPath := filepath.Join(directory, "api-go")

	switch action {
	case "category_editor":
		cmd = exec.Command(apiPath, "categoryedit")
	case "log_viewer":
		cmd = exec.Command(apiPath, "logviewer")
	case "multi_install":
		cmd = exec.Command(apiPath, "multi_install_gui")
	case "create_app":
		cmd = exec.Command(apiPath, "createapp")
	case "import_app":
		cmd = exec.Command(apiPath, "importapp")
	case "multi_uninstall":
		cmd = exec.Command(apiPath, "multi_uninstall_gui")
	default:
		fmt.Println(Tf("Unknown action: %s", action))
		return
	}

	cmd.Env = GetThemeEnvironmentForLaunch(appListTheme)

	go func() {
		if err := cmd.Start(); err != nil {
			fmt.Println(Tf("Failed to start %s: %v", action, err))
		}
	}()
}

// NewSettingsWindow creates and initializes a new settings window
func NewSettingsWindow() (*SettingsWindow, error) {
	return nil, nil
}

// Show displays the settings window
func (sw *SettingsWindow) Show() {
	return
}

// Run starts the TUI main loop
func (sw *SettingsWindow) Run() {
	_ = RunSettingsTUI()
	return
}
