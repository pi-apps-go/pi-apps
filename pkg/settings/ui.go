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

// Module: ui.go
// Description: UI components and tab creation for the settings window

package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/gotk3/gotk3/gtk"
)

// createSettingsTab creates the main settings tab with configuration options
func (sw *SettingsWindow) createSettingsTab() error {
	// Create scrolled window for settings
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create scrolled window: %w", err)
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)

	// Create main container to center content
	mainContainer, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return fmt.Errorf("failed to create main container: %w", err)
	}
	mainContainer.SetHAlign(gtk.ALIGN_CENTER)

	// Create main box for settings with better width management
	settingsBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 15)
	if err != nil {
		return fmt.Errorf("failed to create settings box: %w", err)
	}
	settingsBox.SetMarginTop(25)
	settingsBox.SetMarginBottom(25)
	settingsBox.SetMarginStart(25)
	settingsBox.SetMarginEnd(25)

	// Set a minimum width for the settings box to better utilize space
	settingsBox.SetSizeRequest(450, -1)

	// Sort settings by name for consistent display
	var settingNames []string
	for name := range sw.settings {
		settingNames = append(settingNames, name)
	}
	sort.Strings(settingNames)

	// Create UI for each setting
	for _, settingName := range settingNames {
		setting := sw.settings[settingName]

		// Skip if no values available
		if len(setting.Values) == 0 {
			continue
		}

		// Create horizontal box for this setting
		hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 15)
		if err != nil {
			return fmt.Errorf("failed to create horizontal box: %w", err)
		}
		hbox.SetHAlign(gtk.ALIGN_FILL)

		// Create label with translated setting name
		translatedName := TranslateSettingName(settingName)
		label, err := gtk.LabelNew(translatedName)
		if err != nil {
			return fmt.Errorf("failed to create label: %w", err)
		}
		label.SetHAlign(gtk.ALIGN_START)
		label.SetVAlign(gtk.ALIGN_CENTER)
		label.SetSizeRequest(180, -1)

		// Enable text wrapping for long setting names
		label.SetLineWrap(true)
		label.SetLineWrapMode(2) // PANGO_WRAP_WORD = 2
		label.SetMaxWidthChars(25)

		// Set tooltip if available (translate it)
		if setting.Tooltip != "" {
			translatedTooltip := TranslateTooltip(setting.Tooltip)
			label.SetTooltipText(translatedTooltip)
		}

		// Create combo box
		combo, err := gtk.ComboBoxTextNew()
		if err != nil {
			return fmt.Errorf("failed to create combo box: %w", err)
		}
		combo.SetSizeRequest(240, -1)

		// Populate combo box with translated values
		activeIndex := 0
		for i, value := range setting.Values {
			translatedValue := TranslateSettingValue(value)
			combo.AppendText(translatedValue)
			if value == setting.Current {
				activeIndex = i
			}
		}
		combo.SetActive(activeIndex)

		// Store reference for saving later
		sw.comboBoxes[settingName] = combo

		// Set tooltip for combo box too (translate it)
		if setting.Tooltip != "" {
			translatedTooltip := TranslateTooltip(setting.Tooltip)
			combo.SetTooltipText(translatedTooltip)
		}

		// Special handling for App List Style to apply theme changes immediately
		if settingName == "App List Style" {
			combo.Connect("changed", func() {
				activeText := combo.GetActiveText()
				if activeText != "" {
					sw.applyThemeToCurrentWindow(activeText)
				}
			})
		}

		// Pack into horizontal box
		hbox.PackStart(label, false, false, 0)
		hbox.PackEnd(combo, false, false, 0)

		// Add to settings box
		settingsBox.PackStart(hbox, false, false, 0)
	}

	// Pack settings box into main container
	mainContainer.PackStart(settingsBox, true, false, 0)

	// Add main container to scrolled window
	scrolled.Add(mainContainer)

	// Create tab label
	tabLabel, err := gtk.LabelNew(T("Settings"))
	if err != nil {
		return fmt.Errorf("failed to create tab label: %w", err)
	}

	sw.notebook.AppendPage(scrolled, tabLabel)

	return nil
}

// createActionsTab creates the tab with action buttons (categories, logs, etc.)
func (sw *SettingsWindow) createActionsTab() error {
	// Create scrolled window for actions
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create scrolled window: %w", err)
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)

	// Create main container box to center content
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		return fmt.Errorf("failed to create main box: %w", err)
	}
	mainBox.SetHAlign(gtk.ALIGN_CENTER)
	mainBox.SetVAlign(gtk.ALIGN_CENTER)

	// Create grid for actions
	grid, err := gtk.GridNew()
	if err != nil {
		return fmt.Errorf("failed to create grid: %w", err)
	}
	grid.SetRowSpacing(15)
	grid.SetColumnSpacing(15)
	grid.SetHAlign(gtk.ALIGN_CENTER)
	grid.SetVAlign(gtk.ALIGN_CENTER)

	// Define action buttons with their properties
	actions := []struct {
		name    string
		icon    string
		tooltip string
		action  string
	}{
		{
			name:    T("Categories"),
			icon:    "categories.png",
			tooltip: T("Does an App belong in Editors instead of Tools? This lets you move it."),
			action:  "category_editor",
		},
		{
			name:    T("Log files"),
			icon:    "log-file.png",
			tooltip: T("View past installation logs. Useful for debugging, or to see what you installed yesterday."),
			action:  "log_viewer",
		},
		{
			name:    T("Multi-Install"),
			icon:    "multi-select.png",
			tooltip: T("Install multiple apps at the same time."),
			action:  "multi_install",
		},
		{
			name:    T("New App"),
			icon:    "create.png",
			tooltip: T("Make your own app! It's pretty easy if you follow the instructions."),
			action:  "create_app",
		},
		{
			name:    T("Import App"),
			icon:    "categories/Imported.png",
			tooltip: T("Did someone else make an app but it's not on Pi-Apps yet? Import it here."),
			action:  "import_app",
		},
		{
			name:    T("Multi-Uninstall"),
			icon:    "multi-select.png",
			tooltip: T("Uninstall multiple apps at the same time."),
			action:  "multi_uninstall",
		},
	}

	// Create buttons for actions in a 3x2 grid
	for i, action := range actions {
		button, err := gtk.ButtonNew()
		if err != nil {
			return fmt.Errorf("failed to create button: %w", err)
		}

		// Create button content box
		buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
		if err != nil {
			return fmt.Errorf("failed to create button box: %w", err)
		}
		buttonBox.SetHAlign(gtk.ALIGN_CENTER)
		buttonBox.SetVAlign(gtk.ALIGN_CENTER)

		// Add icon if it exists
		iconPath := filepath.Join(sw.directory, "icons", action.icon)
		if fileExists(iconPath) {
			image, err := gtk.ImageNewFromFile(iconPath)
			if err == nil {
				// Scale icon to consistent size
				if pixbuf := image.GetPixbuf(); pixbuf != nil {
					scaledPixbuf, err := pixbuf.ScaleSimple(32, 32, 2) // GDK_INTERP_BILINEAR = 2
					if err == nil {
						image.SetFromPixbuf(scaledPixbuf)
					}
				}
				buttonBox.PackStart(image, false, false, 0)
			}
		}

		// Add label
		label, err := gtk.LabelNew(action.name)
		if err != nil {
			return fmt.Errorf("failed to create button label: %w", err)
		}
		label.SetHAlign(gtk.ALIGN_CENTER)
		// Enable text wrapping for longer names
		label.SetLineWrap(true)
		label.SetLineWrapMode(2) // PANGO_WRAP_WORD = 2
		label.SetMaxWidthChars(12)
		buttonBox.PackStart(label, false, false, 0)

		button.Add(buttonBox)
		button.SetTooltipText(action.tooltip)
		button.SetSizeRequest(140, 100)

		// Connect button click
		script := action.action
		button.Connect("clicked", func() {
			sw.runAction(script)
		})

		// Calculate grid position (3 columns)
		row := i / 3
		col := i % 3

		// Add to grid
		grid.Attach(button, col, row, 1, 1)
	}

	// Pack grid into main box with some padding
	mainBox.SetMarginTop(30)
	mainBox.SetMarginBottom(30)
	mainBox.SetMarginStart(30)
	mainBox.SetMarginEnd(30)
	mainBox.PackStart(grid, true, true, 0)

	scrolled.Add(mainBox)

	// Create tab label
	tabLabel, err := gtk.LabelNew(T("Actions"))
	if err != nil {
		return fmt.Errorf("failed to create tab label: %w", err)
	}

	sw.notebook.AppendPage(scrolled, tabLabel)

	return nil
}

// createButtons creates the main action buttons (Save, Cancel, Reset)
func (sw *SettingsWindow) createButtons(buttonBox *gtk.Box) error {
	// Reset button
	resetButton, err := gtk.ButtonNewWithLabel(T("Reset"))
	if err != nil {
		return fmt.Errorf("failed to create reset button: %w", err)
	}
	resetButton.SetTooltipText(T("Reset all settings to their defaults"))
	resetButton.SetSizeRequest(80, 35)
	resetButton.Connect("clicked", func() {
		sw.resetSettings()
	})

	// Cancel button
	cancelButton, err := gtk.ButtonNewWithLabel(T("Cancel"))
	if err != nil {
		return fmt.Errorf("failed to create cancel button: %w", err)
	}
	cancelButton.SetSizeRequest(80, 35)
	cancelButton.Connect("clicked", func() {
		sw.window.Close()
	})

	// Save button
	saveButton, err := gtk.ButtonNewWithLabel(T("Save"))
	if err != nil {
		return fmt.Errorf("failed to create save button: %w", err)
	}
	saveButton.SetSizeRequest(80, 35)
	saveButton.Connect("clicked", func() {
		sw.saveSettings()
		sw.window.Close()
	})

	// Pack buttons with consistent spacing
	buttonBox.PackStart(resetButton, false, false, 5)
	buttonBox.PackStart(cancelButton, false, false, 5)
	buttonBox.PackStart(saveButton, false, false, 5)

	return nil
}

// runAction executes an action using the api-go binary via shell command
// This avoids GTK threading issues and memory corruption
func (sw *SettingsWindow) runAction(action string) {
	var cmd *exec.Cmd
	apiPath := filepath.Join(sw.directory, "api-go")

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

	// Apply current theme environment for launched applications
	var currentTheme string
	if appListSetting, exists := sw.settings["App List Style"]; exists {
		currentTheme = appListSetting.Current
	}
	cmd.Env = GetThemeEnvironmentForLaunch(currentTheme)

	// Run in background
	go func() {
		if err := cmd.Start(); err != nil {
			fmt.Println(Tf("Failed to start %s: %v", action, err))
		}
	}()
}

// resetSettings resets all settings to their default values
func (sw *SettingsWindow) resetSettings() {
	// Show confirmation dialog
	dialog := gtk.MessageDialogNew(sw.window, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION,
		gtk.BUTTONS_YES_NO, T("Are you sure you want to reset all settings to their defaults?"))
	defer dialog.Destroy()

	response := dialog.Run()
	if response != gtk.RESPONSE_YES {
		return
	}

	// Reset each setting
	for settingName, setting := range sw.settings {
		if len(setting.Values) > 0 {
			// Set to first value (default)
			defaultValue := setting.Values[0]
			setting.Current = defaultValue

			// Update combo box
			if combo, exists := sw.comboBoxes[settingName]; exists {
				combo.SetActive(0)
			}

			// Save to file
			settingPath := filepath.Join(sw.directory, "data", "settings", settingName)
			if err := os.WriteFile(settingPath, []byte(defaultValue), 0644); err != nil {
				fmt.Println(Tf("Failed to reset setting %s: %v", settingName, err))
			}
		}
	}
}

// saveSettings saves current settings to files
func (sw *SettingsWindow) saveSettings() {
	for settingName, combo := range sw.comboBoxes {
		activeText := combo.GetActiveText()
		if activeText == "" {
			continue
		}

		// Update internal state
		if setting, exists := sw.settings[settingName]; exists {
			setting.Current = activeText
		}

		// Save to file
		settingPath := filepath.Join(sw.directory, "data", "settings", settingName)
		if err := os.WriteFile(settingPath, []byte(activeText), 0644); err != nil {
			fmt.Println(Tf("Failed to save setting %s: %v", settingName, err))
		}
	}
}
