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

// Module: categoryedit.go
// Description: Provides functions for editing and managing app categories.
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// Embedded default category data - structured Go-native configuration
var (
	// embeddedGlobalCategories contains the default categories
	embeddedGlobalCategories = []CategoryAssignment{
		{AppName: "AbiWord", Category: "Office"},
		{AppName: "Alacritty Terminal", Category: "Terminals"},
		{AppName: "All Is Well", Category: "System Management"},
		{AppName: "Amiberry", Category: "Games"},
		{AppName: "AndroidBuddy", Category: "Tools"},
		{AppName: "Angry IP scanner", Category: "Internet"},
		{AppName: "AntiMicroX", Category: "Tools"},
		{AppName: "Arduino", Category: "Programming"},
		{AppName: "AstroMenace", Category: "Games"},
		{AppName: "Audacious", Category: "Multimedia"},
		{AppName: "Audacity", Category: "Multimedia"},
		{AppName: "Autostar", Category: "System Management"},
		{AppName: "BalenaEtcher", Category: "Tools"},
		{AppName: "Bambu Studio", Category: "Engineering"},
		{AppName: "Better Chromium", Category: "Internet/Browsers"},
		{AppName: "BleachBit", Category: "System Management"},
		{AppName: "BlockBench", Category: "Creative Arts"},
		{AppName: "BlueJ Java IDE", Category: "Programming"},
		{AppName: "Bongo Cam", Category: "Multimedia"},
		{AppName: "Botspot Screen Recorder", Category: "Multimedia"},
		{AppName: "Botspot Virtual Machine", Category: "Tools/Emulation"},
		{AppName: "Box64", Category: "Tools/Emulation"},
		{AppName: "Box86", Category: "Tools/Emulation"},
		{AppName: "Boxy SVG", Category: "Creative Arts"},
		{AppName: "Brave", Category: "Internet/Browsers"},
		{AppName: "Browsh", Category: "Internet/Browsers"},
		{AppName: "btop++", Category: "System Management"},
		{AppName: "Caprine", Category: "Internet/Communication"},
		{AppName: "Caskaydia Cove NF", Category: "Appearance"},
		{AppName: "Celeste64", Category: "Games"},
		{AppName: "Celeste Classic", Category: "Games"},
		{AppName: "Chromium", Category: "Internet/Browsers"},
		{AppName: "ckb-next", Category: "Tools"},
		{AppName: "Clam Antivirus", Category: "System Management"},
		{AppName: "CloudBuddy", Category: "Internet"},
		{AppName: "Codex", Category: "Programming"},
		{AppName: "Colored Man Pages", Category: "Appearance"},
		{AppName: "Color Emoji font", Category: "Appearance"},
		{AppName: "CommanderPi", Category: "System Management"},
		{AppName: "Conky", Category: "Appearance"},
		{AppName: "Conky Rings", Category: "Appearance"},
		{AppName: "Cool Retro Term", Category: "Terminals"},
		{AppName: "Cura", Category: "Engineering"},
		{AppName: "DDNet", Category: "Games"},
		{AppName: "Deluge", Category: "Internet"},
		{AppName: "Descent 1", Category: "Games"},
		{AppName: "Descent 2", Category: "Games"},
		{AppName: "Deskreen", Category: "Internet"},
		{AppName: "Disk Usage Analyzer", Category: "System Management"},
		{AppName: "Doom 3", Category: "Games"},
		{AppName: "Dot Matrix", Category: "Creative Arts"},
		{AppName: "Downgrade Chromium", Category: "Internet/Browsers"},
		{AppName: "Drawing", Category: "Creative Arts"},
		{AppName: "Ducopanel", Category: "Tools/Crypto"},
		{AppName: "Eagle CAD", Category: "Engineering"},
		{AppName: "Easy Effects", Category: "Multimedia"},
		{AppName: "Electron Fiddle", Category: "Programming"},
		{AppName: "Epiphany", Category: "Internet/Browsers"},
		{AppName: "Fastfetch", Category: "System Management"},
		{AppName: "Feather Wallet", Category: "Tools/Crypto"},
		{AppName: "FF Multi Converter", Category: "Tools"},
		{AppName: "Filezilla", Category: "Internet"},
		{AppName: "Firefox Rapid Release", Category: "Internet/Browsers"},
		{AppName: "Flameshot", Category: "Tools"},
		{AppName: "Floorp", Category: "Internet/Browsers"},
		{AppName: "Flow", Category: "Internet/Browsers"},
		{AppName: "FreeTube", Category: "Multimedia"},
		{AppName: "Friday Night Funkin' Rewritten", Category: "Games"},
		{AppName: "Fritzing", Category: "Engineering"},
		{AppName: "Geany Dark Mode", Category: "Appearance"},
		{AppName: "Geekbench 5", Category: "Tools"},
		{AppName: "Geekbench 6", Category: "Tools"},
		{AppName: "GIMP", Category: "Creative Arts"},
		{AppName: "Github-CLI", Category: "Programming"},
		{AppName: "Github Desktop", Category: "Programming"},
		{AppName: "Gnome Builder IDE", Category: "Programming"},
		{AppName: "Gnome Maps", Category: "Tools"},
		{AppName: "Gnome Software", Category: "hidden"},
		{AppName: "Gnumeric", Category: "Office"},
		{AppName: "Godot", Category: "Games"},
		{AppName: "GParted", Category: "System Management"},
		{AppName: "Guake Terminal", Category: "Terminals"},
		{AppName: "Hangover", Category: "Tools/Emulation"},
		{AppName: "Heroes 2", Category: "Games"},
		{AppName: "Https File Server", Category: "Tools"},
		{AppName: "HTTrack Website Copier", Category: "Internet"},
		{AppName: "Hyper", Category: "hidden"},
		{AppName: "Imager", Category: "Tools"},
		{AppName: "INAV Configurator", Category: "Engineering"},
		{AppName: "Inkscape", Category: "Creative Arts"},
		{AppName: "Intellij IDEA", Category: "Programming"},
		{AppName: "jGRASP IDE", Category: "Programming"},
		{AppName: "Kdenlive", Category: "Multimedia"},
		{AppName: "KeePassXC", Category: "Tools"},
		{AppName: "KiCad", Category: "Engineering"},
		{AppName: "Kodi", Category: "Multimedia"},
		{AppName: "Kolourpaint", Category: "Creative Arts"},
		{AppName: "Krita", Category: "Creative Arts"},
		{AppName: "Legcord", Category: "Internet/Communication"},
		{AppName: "Lego Digital Designer", Category: "Creative Arts"},
		{AppName: "LibreCAD", Category: "Engineering"},
		{AppName: "Libreoffice MS theme", Category: "Office"},
		{AppName: "LibreOffice", Category: "Office"},
		{AppName: "LibrePCB", Category: "Engineering"},
		{AppName: "LibreWolf", Category: "Internet/Browsers"},
		{AppName: "Lightpad", Category: "Appearance"},
		{AppName: "LineRider", Category: "Games"},
		{AppName: "Linux Wifi Hotspot", Category: "Tools"},
		{AppName: "LMMS", Category: "Multimedia"},
		{AppName: "Marathon", Category: "Games"},
		{AppName: "MatterControl", Category: "Engineering"},
		{AppName: "Microsoft PowerShell", Category: "Terminals"},
		{AppName: "Microsoft Teams", Category: "Internet/Communication"},
		{AppName: "Minecraft Bedrock", Category: "Games"},
		{AppName: "Minecraft Java GDLauncher", Category: "Games"},
		{AppName: "Minecraft Java Prism Launcher", Category: "Games"},
		{AppName: "Minecraft Java Server", Category: "Games"},
		{AppName: "Minecraft Pi (Modded)", Category: "Games"},
		{AppName: "Min", Category: "Internet/Browsers"},
		{AppName: "Mission Planner", Category: "Engineering"},
		{AppName: "Monero GUI", Category: "Tools/Crypto"},
		{AppName: "More RAM", Category: "Tools"},
		{AppName: "Mullvad", Category: "Internet/Browsers"},
		{AppName: "Mu", Category: "Programming"},
		{AppName: "MuseScore", Category: "Multimedia"},
		{AppName: "Nautilus", Category: "Tools"},
		{AppName: "Nemo", Category: "Tools"},
		{AppName: "Neofetch", Category: "System Management"},
		{AppName: "NixNote2", Category: "Office"},
		{AppName: "Node.js", Category: "Tools"},
		{AppName: "Notejot", Category: "Office"},
		{AppName: "Notepad ++", Category: "Programming"},
		{AppName: "Obsidian", Category: "Office"},
		{AppName: "OBS Studio", Category: "Multimedia"},
		{AppName: "Oh My Posh", Category: "Appearance"},
		{AppName: "Ollama GUI", Category: "Tools"},
		{AppName: "OnionShare", Category: "Tools"},
		{AppName: "Oomox Theme Designer", Category: "Appearance"},
		{AppName: "OpenSCAD", Category: "Engineering"},
		{AppName: "Open-Typer", Category: "Office"},
		{AppName: "Organic Maps", Category: "Tools"},
		{AppName: "Pac-Man", Category: "Games"},
		{AppName: "PeaZip", Category: "Tools"},
		{AppName: "Persepolis Download Manager", Category: "Internet"},
		{AppName: "Pi-Apps Terminal Plugin (bash)", Category: "Tools"},
		{AppName: "PiGro", Category: "Tools"},
		{AppName: "Pika Backup", Category: "System Management"},
		{AppName: "Pinta", Category: "Creative Arts"},
		{AppName: "Pi Power Tools", Category: "System Management"},
		{AppName: "PiSafe", Category: "Tools"},
		{AppName: "Pixelorama", Category: "Creative Arts"},
		{AppName: "Powerline-Shell", Category: "Appearance"},
		{AppName: "PPSSPP (PSP emulator)", Category: "Games"},
		{AppName: "Processing IDE", Category: "Programming"},
		{AppName: "ProjectLibre", Category: "Office"},
		{AppName: "Project OutFox", Category: "Games"},
		{AppName: "PrusaSlicer", Category: "Engineering"},
		{AppName: "Puffin", Category: "Internet/Browsers"},
		{AppName: "Pycharm CE", Category: "Programming"},
		{AppName: "PyChess", Category: "Games"},
		{AppName: "QEMU", Category: "Tools/Emulation"},
		{AppName: "QR Code Reader", Category: "Tools"},
		{AppName: "Quartz", Category: "Internet/Browsers"},
		{AppName: "Reaper", Category: "Multimedia"},
		{AppName: "Remarkable", Category: "Programming"},
		{AppName: "Renoise (Demo)", Category: "Multimedia"},
		{AppName: "RiiTag-RPC", Category: "Internet"},
		{AppName: "RustDesk", Category: "Internet"},
		{AppName: "Scratch 2", Category: "Programming"},
		{AppName: "Scratch 3", Category: "Programming"},
		{AppName: "Scrcpy", Category: "Tools"},
		{AppName: "Screenshot", Category: "Tools"},
		{AppName: "Shattered Pixel Dungeon", Category: "Games"},
		{AppName: "Shotwell", Category: "Creative Arts"},
		{AppName: "Signal", Category: "Internet/Communication"},
		{AppName: "SimpleScreenRecorder", Category: "Multimedia"},
		{AppName: "Snapdrop", Category: "Tools"},
		{AppName: "Snap Store", Category: "Tools"},
		{AppName: "Sonic Pi", Category: "Multimedia"},
		{AppName: "Sound Recorder", Category: "Multimedia"},
		{AppName: "SpeedTest-CLI", Category: "Internet"},
		{AppName: "Sphero SDK", Category: "Programming"},
		{AppName: "StackEdit", Category: "Programming"},
		{AppName: "Steam", Category: "Games"},
		{AppName: "Steam Link", Category: "Games"},
		{AppName: "StepMania", Category: "Games"},
		{AppName: "Stunt Rally", Category: "hidden"},
		{AppName: "Sublime Merge", Category: "Programming"},
		{AppName: "Sublime Text", Category: "Programming"},
		{AppName: "Synaptic", Category: "System Management"},
		{AppName: "Syncthing", Category: "System Management"},
		{AppName: "SysMonTask", Category: "System Management"},
		{AppName: "Systemd Pilot", Category: "System Management"},
		{AppName: "System Monitoring Center", Category: "System Management"},
		{AppName: "Tabby", Category: "Terminals"},
		{AppName: "TeamViewer", Category: "Internet"},
		{AppName: "Telegram", Category: "Internet/Communication"},
		{AppName: "template", Category: "hidden"},
		{AppName: "Tetris CLI", Category: "Games"},
		{AppName: "Thonny", Category: "Programming"},
		{AppName: "Thunderbird", Category: "Internet/Communication"},
		{AppName: "TiLP", Category: "Tools"},
		{AppName: "Timeshift", Category: "System Management"},
		{AppName: "tldr", Category: "Tools"},
		{AppName: "Tor", Category: "Internet/Browsers"},
		{AppName: "Transmission", Category: "Internet"},
		{AppName: "Turbowarp", Category: "Programming"},
		{AppName: "Ulauncher", Category: "Appearance"},
		{AppName: "Unciv", Category: "Games"},
		{AppName: "Update Buddy", Category: "System Management"},
		{AppName: "USBImager", Category: "Tools"},
		{AppName: "VARA HF", Category: "Engineering"},
		{AppName: "VeraCrypt", Category: "Tools"},
		{AppName: "Visual Studio Code", Category: "Programming"},
		{AppName: "Vivaldi", Category: "Internet/Browsers"},
		{AppName: "VMware Horizon Client", Category: "Tools"},
		{AppName: "VSCodium", Category: "Programming"},
		{AppName: "WACUP (new WinAmp)", Category: "Multimedia"},
		{AppName: "Waveform", Category: "Multimedia"},
		{AppName: "Web Apps", Category: "Internet"},
		{AppName: "Webcord", Category: "Internet/Communication"},
		{AppName: "Wechat", Category: "Internet/Communication"},
		{AppName: "WhatsApp", Category: "Internet/Communication"},
		{AppName: "Windows Flasher", Category: "Tools"},
		{AppName: "Windows Screensavers", Category: "Appearance"},
		{AppName: "Wine (x64)", Category: "Tools/Emulation"},
		{AppName: "Wine (x86)", Category: "Tools/Emulation"},
		{AppName: "WorldPainter", Category: "Games"},
		{AppName: "WPS Office", Category: "Office"},
		{AppName: "Xfburn", Category: "Tools"},
		{AppName: "XMRig", Category: "Tools/Crypto"},
		{AppName: "XSnow", Category: "Appearance"},
		{AppName: "Xtreme Download Manager", Category: "Internet"},
		{AppName: "YouTubuddy", Category: "Multimedia"},
		{AppName: "Zen", Category: "Internet/Browsers"},
		{AppName: "Zoom", Category: "Internet/Communication"},
		{AppName: "Zoom PWA", Category: "Internet/Communication"},
	}

	// embeddedCategoryOverridesNonRaspberry contains overrides for non-Raspberry Pi devices
	embeddedCategoryOverridesNonRaspberry = []CategoryAssignment{
		{AppName: "CommanderPi", Category: "hidden"},
		{AppName: "Downgrade Chromium", Category: "hidden"},
		{AppName: "Flow", Category: "hidden"},
		{AppName: "Gnome Software", Category: "Tools"},
		{AppName: "Lightpad", Category: "hidden"},
		{AppName: "PiGro", Category: "hidden"},
		{AppName: "Pi Power Tools", Category: "hidden"},
		{AppName: "Windows Flasher", Category: "hidden"},
		{AppName: "Windows Screensavers", Category: "hidden"},
	}

	// embeddedCategoryOverridesJetsonGeneric contains overrides for Jetson devices (generic)
	embeddedCategoryOverridesJetsonGeneric = []CategoryAssignment{
		{AppName: "Autostar", Category: "hidden"},
		{AppName: "Better Chromium", Category: "hidden"},
		{AppName: "Box86", Category: "hidden"},
		{AppName: "CommanderPi", Category: "hidden"},
		{AppName: "Downgrade Chromium", Category: "hidden"},
		{AppName: "Floorp", Category: "hidden"},
		{AppName: "Flow", Category: "hidden"},
		{AppName: "FreeTube", Category: "hidden"},
		{AppName: "Gnome Software", Category: "Tools"},
		{AppName: "Godot", Category: "hidden"},
		{AppName: "Hangover", Category: "hidden"},
		{AppName: "Kodi", Category: "hidden"},
		{AppName: "Lightpad", Category: "hidden"},
		{AppName: "Minecraft Pi (Modded)", Category: "hidden"},
		{AppName: "More RAM", Category: "hidden"},
		{AppName: "Oomox Theme Designer", Category: "hidden"},
		{AppName: "PiGro", Category: "hidden"},
		{AppName: "Pi Power Tools", Category: "hidden"},
		{AppName: "QEMU", Category: "hidden"},
		{AppName: "Snap Store", Category: "hidden"},
		{AppName: "Steam", Category: "hidden"},
		{AppName: "Stunt Rally", Category: "hidden"},
		{AppName: "Windows Flasher", Category: "hidden"},
		{AppName: "Windows Screensavers", Category: "hidden"},
	}
)

// CategoryAssignment represents a single app-to-category mapping
type CategoryAssignment struct {
	AppName  string // Name of the app
	Category string // Category name (empty string means unlisted)
}

// CategoryData represents the category assignment data
type CategoryData struct {
	GlobalCategories map[string]string // app -> category mapping from global file
	LocalCategories  map[string]string // app -> category mapping from overrides file
}

// parseCategoryAssignments converts a slice of CategoryAssignment to a map
func parseCategoryAssignments(assignments []CategoryAssignment, categories map[string]string) {
	for _, assignment := range assignments {
		if assignment.AppName != "" {
			categories[assignment.AppName] = assignment.Category
		}
	}
}

// getDeviceCategoryOverrides returns the appropriate device-specific category overrides
// based on the device model and OS version
func getDeviceCategoryOverrides() []CategoryAssignment {
	model, socID := GetDeviceModel()

	// Check if it's a non-Raspberry Pi device
	if !strings.Contains(model, "Raspberry Pi") {
		return embeddedCategoryOverridesNonRaspberry
	}

	// Check if it's a Jetson device (Tegra-based)
	jetsonModel := ""
	if strings.Contains(socID, "tegra") || strings.Contains(socID, "xavier") || strings.Contains(socID, "orin") {
		jetsonModel = socID
	}

	if jetsonModel != "" {
		return embeddedCategoryOverridesJetsonGeneric
	}

	return nil // No device-specific overrides
}

// ReadCategoryData reads both global and local category files
// Uses embedded default categories instead of reading from files
func ReadCategoryData() (*CategoryData, error) {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	data := &CategoryData{
		GlobalCategories: make(map[string]string),
		LocalCategories:  make(map[string]string),
	}

	// Load embedded global categories from structured data
	parseCategoryAssignments(embeddedGlobalCategories, data.GlobalCategories)

	// Read local category overrides file (user overrides)
	localFile := filepath.Join(piAppsDir, "data", "category-overrides")
	if FileExists(localFile) {
		if err := readCategoryFile(localFile, data.LocalCategories); err != nil {
			return nil, fmt.Errorf("failed to read local categories: %w", err)
		}
	}

	return data, nil
}

// readCategoryFile reads a category file into a map
func readCategoryFile(filename string, categories map[string]string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			app := strings.TrimSpace(parts[0])
			category := strings.TrimSpace(parts[1])
			categories[app] = category
		}
	}

	return scanner.Err()
}

// GetAppCategory returns the effective category for an app
func (cd *CategoryData) GetAppCategory(app string) string {
	// Local overrides take precedence over global categories
	if category, exists := cd.LocalCategories[app]; exists {
		return category
	}
	if category, exists := cd.GlobalCategories[app]; exists {
		return category
	}
	return "" // No category assigned
}

// SetAppCategory sets the category for an app (modifies local overrides)
func (cd *CategoryData) SetAppCategory(app, category string) {
	globalCategory := cd.GlobalCategories[app]

	if globalCategory == category {
		// If setting to the same category as global, remove from overrides
		delete(cd.LocalCategories, app)
	} else {
		// Otherwise, add/update in local overrides
		cd.LocalCategories[app] = category
	}
}

// SaveLocalCategories saves the local category overrides to file
func (cd *CategoryData) SaveLocalCategories() error {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	localFile := filepath.Join(piAppsDir, "data", "category-overrides")

	// Ensure the data directory exists
	if err := EnsureDir(filepath.Dir(localFile)); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	file, err := os.Create(localFile)
	if err != nil {
		return fmt.Errorf("failed to create local categories file: %w", err)
	}
	defer file.Close()

	// Sort apps for consistent output
	var apps []string
	for app := range cd.LocalCategories {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	// Write sorted entries
	for _, app := range apps {
		category := cd.LocalCategories[app]
		if _, err := fmt.Fprintf(file, "%s|%s\n", app, category); err != nil {
			return fmt.Errorf("failed to write category entry: %w", err)
		}
	}

	return nil
}

// ResetToGlobalCategories removes all local overrides
func (cd *CategoryData) ResetToGlobalCategories() {
	cd.LocalCategories = make(map[string]string)
}

// ClearAllCategories removes categories from all apps (except hidden ones)
func (cd *CategoryData) ClearAllCategories() {
	// Keep hidden apps but clear others
	newLocal := make(map[string]string)

	// Preserve hidden apps in local overrides
	for app, category := range cd.LocalCategories {
		if category == "hidden" {
			newLocal[app] = category
		}
	}

	// Add entries to clear categories for non-hidden global apps
	for app, category := range cd.GlobalCategories {
		if category != "hidden" {
			newLocal[app] = ""
		}
	}

	cd.LocalCategories = newLocal
}

// ShowCategoryEditor displays the category editor GUI
func ShowCategoryEditor() error {
	return showCategoryEditorGUI()
}

// EditAppCategory edits a specific app's category (command line interface)
func EditAppCategory(app, category string) error {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get list of apps
	apps, err := ListApps("local")
	if err != nil {
		return fmt.Errorf("failed to get app list: %w", err)
	}

	// Check if app exists
	appExists := false
	for _, existingApp := range apps {
		if existingApp == app {
			appExists = true
			break
		}
	}
	if !appExists {
		return fmt.Errorf("the '%s' app does not exist", app)
	}

	// Read category data
	data, err := ReadCategoryData()
	if err != nil {
		return fmt.Errorf("failed to read category data: %w", err)
	}

	// Set the category
	data.SetAppCategory(app, category)

	// Save changes
	if err := data.SaveLocalCategories(); err != nil {
		return fmt.Errorf("failed to save category changes: %w", err)
	}

	return nil
}

// showCategoryEditorGUI displays the category editor using GTK
func showCategoryEditorGUI() error {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Initialize GTK
	glib.SetPrgname("Category editor")
	gtk.Init(nil)

	for {
		// Read current category data
		data, err := ReadCategoryData()
		if err != nil {
			return fmt.Errorf("failed to read category data: %w", err)
		}

		// Get list of apps
		apps, err := ListApps("local")
		if err != nil {
			return fmt.Errorf("failed to get app list: %w", err)
		}

		// Show the dialog
		action, newData, err := showCategoryDialog(data, apps)
		if err != nil {
			return fmt.Errorf("failed to show category dialog: %w", err)
		}

		switch action {
		case "save":
			// Apply changes from the dialog
			*data = *newData
			if err := data.SaveLocalCategories(); err != nil {
				showErrorDialog("Failed to save category changes: " + err.Error())
				continue
			}

			// Refresh app list in background
			go func() {
				_ = RefreshAppList()
			}()

			return nil

		case "reset":
			// Reset to global categories
			data.ResetToGlobalCategories()
			continue // Show dialog again with reset data

		case "clear":
			// Clear all categories
			data.ClearAllCategories()
			continue // Show dialog again with cleared data

		case "cancel":
			return nil // Exit without saving

		default:
			return nil // Exit
		}
	}
}

// showCategoryDialog shows the main category editing dialog
func showCategoryDialog(data *CategoryData, apps []string) (string, *CategoryData, error) {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return "", nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Create main dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create dialog: %w", err)
	}
	defer dialog.Destroy()

	dialog.SetTitle("Category editor")
	dialog.SetDefaultSize(600, 400)
	dialog.SetPosition(gtk.WIN_POS_CENTER)
	dialog.SetModal(true)

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
	if FileExists(iconPath) {
		pixbuf, err := gdk.PixbufNewFromFile(iconPath)
		if err == nil {
			dialog.SetIcon(pixbuf)
		}
	}

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get content area: %w", err)
	}

	// Create header label
	headerText := "Changes saved to: " + strings.Replace(filepath.Join(piAppsDir, "data", "category-overrides"), os.Getenv("HOME"), "~", 1)
	headerLabel, err := gtk.LabelNew(headerText)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create header label: %w", err)
	}
	headerLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.PackStart(headerLabel, false, false, 8)

	// Create scrolled window
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create scrolled window: %w", err)
	}
	scrolledWindow.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWindow.SetShadowType(gtk.SHADOW_IN)
	contentArea.PackStart(scrolledWindow, true, true, 0)

	// Create tree view and model
	treeView, listStore, err := createCategoryTreeView()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create tree view: %w", err)
	}
	scrolledWindow.Add(treeView)

	// Populate the list with apps and their categories
	populateCategoryList(listStore, data, apps)

	// Create buttons manually so we have direct access to them
	resetBtn, err := gtk.ButtonNewWithLabel("Reset")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create reset button: %w", err)
	}
	setButtonIcon(resetBtn, filepath.Join(piAppsDir, "icons", "backup.png"))
	resetBtn.SetTooltipText("Removes all category overrides.")
	dialog.AddActionWidget(resetBtn, 4)

	allBtn, err := gtk.ButtonNewWithLabel("All")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create all button: %w", err)
	}
	setButtonIcon(allBtn, filepath.Join(piAppsDir, "icons", "trash.png"))
	allBtn.SetTooltipText("Clears categories so all apps are in one list.")
	dialog.AddActionWidget(allBtn, 2)

	cancelBtn, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create cancel button: %w", err)
	}
	setButtonIcon(cancelBtn, filepath.Join(piAppsDir, "icons", "exit.png"))
	cancelBtn.SetTooltipText("Don't save any changes.")
	dialog.AddActionWidget(cancelBtn, gtk.RESPONSE_CANCEL)

	saveBtn, err := gtk.ButtonNewWithLabel("Save")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create save button: %w", err)
	}
	setButtonIcon(saveBtn, filepath.Join(piAppsDir, "icons", "check.png"))
	dialog.AddActionWidget(saveBtn, gtk.RESPONSE_OK)

	// Show all widgets
	dialog.ShowAll()

	// Run dialog
	response := dialog.Run()

	// Extract modified data from the tree view
	newData := extractCategoryData(listStore, data)

	switch response {
	case gtk.RESPONSE_OK:
		return "save", newData, nil
	case 2:
		return "clear", newData, nil
	case 4:
		return "reset", newData, nil
	default:
		return "cancel", newData, nil
	}
}

// createCategoryTreeView creates and configures the tree view for displaying apps and categories
func createCategoryTreeView() (*gtk.TreeView, *gtk.ListStore, error) {
	// Create list store with columns: Icon(pixbuf), Name(string), Category(string)
	listStore, err := gtk.ListStoreNew(gdk.PixbufGetType(), glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create list store: %w", err)
	}

	// Create tree view
	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create tree view: %w", err)
	}

	// Create icon column
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create icon renderer: %w", err)
	}
	iconColumn, err := gtk.TreeViewColumnNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create icon column: %w", err)
	}
	iconColumn.PackStart(iconRenderer, false)
	iconColumn.AddAttribute(iconRenderer, "pixbuf", 0)
	iconColumn.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	iconColumn.SetFixedWidth(30)
	treeView.AppendColumn(iconColumn)

	// Create name column
	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create name renderer: %w", err)
	}
	nameColumn, err := gtk.TreeViewColumnNewWithAttribute("Name", nameRenderer, "text", 1)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create name column: %w", err)
	}
	treeView.AppendColumn(nameColumn)

	// Create category column (editable)
	categoryRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create category renderer: %w", err)
	}
	categoryRenderer.SetProperty("editable", true)
	categoryColumn, err := gtk.TreeViewColumnNewWithAttribute("Category", categoryRenderer, "text", 2)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create category column: %w", err)
	}
	treeView.AppendColumn(categoryColumn)

	// Handle category editing
	categoryRenderer.Connect("edited", func(renderer *gtk.CellRendererText, pathStr string, newText string) {
		path, err := gtk.TreePathNewFromString(pathStr)
		if err != nil {
			return
		}

		iter, err := listStore.GetIter(path)
		if err != nil {
			return
		}

		// Update the category in the model
		listStore.SetValue(iter, 2, newText)
	})

	return treeView, listStore, nil
}

// populateCategoryList adds apps and their categories to the list store
func populateCategoryList(listStore *gtk.ListStore, data *CategoryData, apps []string) {
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return
	}

	for _, app := range apps {
		iter := listStore.Append()

		// Load app icon
		var appPixbuf *gdk.Pixbuf
		iconPath := filepath.Join(piAppsDir, "apps", app, "icon-24.png")
		if FileExists(iconPath) {
			appPixbuf, _ = gdk.PixbufNewFromFile(iconPath)
		}

		// Get current category
		category := data.GetAppCategory(app)

		// Set values
		if appPixbuf != nil {
			listStore.SetValue(iter, 0, appPixbuf)
		}
		listStore.SetValue(iter, 1, app)
		listStore.SetValue(iter, 2, category)
	}
}

// extractCategoryData extracts the modified category data from the tree view
func extractCategoryData(listStore *gtk.ListStore, originalData *CategoryData) *CategoryData {
	newData := &CategoryData{
		GlobalCategories: make(map[string]string),
		LocalCategories:  make(map[string]string),
	}

	// Copy global categories
	for app, category := range originalData.GlobalCategories {
		newData.GlobalCategories[app] = category
	}

	// Extract categories from the tree view
	iter, valid := listStore.GetIterFirst()
	for valid {
		// Get app name
		appVal, err := listStore.GetValue(iter, 1)
		if err != nil {
			valid = listStore.IterNext(iter)
			continue
		}
		appInterface, err := appVal.GoValue()
		if err != nil {
			valid = listStore.IterNext(iter)
			continue
		}

		var app string
		switch appValue := appInterface.(type) {
		case string:
			app = appValue
		default:
			valid = listStore.IterNext(iter)
			continue
		}

		// Get category
		categoryVal, err := listStore.GetValue(iter, 2)
		if err != nil {
			valid = listStore.IterNext(iter)
			continue
		}
		categoryInterface, err := categoryVal.GoValue()
		if err != nil {
			valid = listStore.IterNext(iter)
			continue
		}

		var category string
		switch categoryValue := categoryInterface.(type) {
		case string:
			category = categoryValue
		default:
			category = ""
		}

		// Set the category in new data
		newData.SetAppCategory(app, category)

		valid = listStore.IterNext(iter)
	}

	return newData
}

// setButtonIcon sets an icon for a button if the icon file exists
func setButtonIcon(button *gtk.Button, iconPath string) {
	if FileExists(iconPath) {
		icon, err := gtk.ImageNewFromFile(iconPath)
		if err == nil {
			button.SetImage(icon)
			button.SetAlwaysShowImage(true)
		}
	}
}
