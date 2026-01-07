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

// Module: runonce-entries.go
// Description: Embedded runonce entries used by the updater.
// SPDX-License-Identifier: GPL-3.0-or-later

// This file contains various dirty fixes to keep Pi-Apps Go running smoothly as it matures.
// It repairs mistakes that some apps have made, as well as fixing other system issues. Some apps have been removed or renamed.

// These functions are executed by the updater binary as well as the main Pi-Apps Go installation script.

// The updater binary calls 'ExecuteRunonceEntries()' which uses the 'api.RunonceFunc()' function - it avoids running any of these fixes more than once.
// If a runonce entry is modified (by changing its version identifier), then it will be run once more.

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// addUserDirs creates necessary user and system directories.
func addUserDirs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get user home dir: %v", err)
	}

	desktopPath := filepath.Join(homeDir, "Desktop")
	// If $HOME/Desktop exists as a file, remove it and create directory
	info, err := os.Lstat(desktopPath)
	if err == nil && !info.IsDir() {
		// It's a file
		if err := os.Remove(desktopPath); err != nil {
			return fmt.Errorf("failed to remove $HOME/Desktop file: %v", err)
		}
	}
	// If $HOME/Desktop doesn't exist or isn't a directory, create it
	if err != nil || !info.IsDir() {
		if mkerr := os.MkdirAll(desktopPath, 0755); mkerr != nil {
			return fmt.Errorf("failed to create $HOME/Desktop: %v", mkerr)
		}
	}

	// Check and create /opt
	if stat, err := os.Stat("/opt"); os.IsNotExist(err) || !stat.IsDir() {
		if err := api.SudoPopup("mkdir", "/opt"); err != nil {
			return fmt.Errorf("failed to create /opt: %v", err)
		}
	}

	// Check and create /usr/local/bin
	if stat, err := os.Stat("/usr/local/bin"); os.IsNotExist(err) || !stat.IsDir() {
		if err := api.SudoPopup("mkdir", "-p", "/usr/local/bin"); err != nil {
			return fmt.Errorf("failed to create /usr/local/bin: %v", err)
		}
	}

	// Check and create /usr/local/share/applications
	if stat, err := os.Stat("/usr/local/share/applications"); os.IsNotExist(err) || !stat.IsDir() {
		if err := api.SudoPopup("mkdir", "-p", "/usr/local/share/applications"); err != nil {
			return fmt.Errorf("failed to create /usr/local/share/applications: %v", err)
		}
	}

	// Bind-mount logic based on XDG_DATA_DIRS and temporary dir
	xdgDataDirs := os.Getenv("XDG_DATA_DIRS")
	const localShare = "/usr/local/share"
	const mountFrom = "/usr/local/share/applications"
	const mountTo = "/usr/share/applications/usr-local-temporary"

	// Only bind-mount if /usr/local/share isn't in $XDG_DATA_DIRS and not already bound
	if !strings.Contains(xdgDataDirs, localShare) {
		// Check if mount target dir is empty or missing
		needMount := false
		if entries, err := os.ReadDir(mountTo); os.IsNotExist(err) || len(entries) == 0 {
			needMount = true
		}
		if needMount {
			_ = api.SudoPopup("mkdir", "-p", mountTo) // Ignore error if already exists
			if err := api.SudoPopup("mount", "--bind", mountFrom, mountTo); err != nil {
				return fmt.Errorf("failed to bind-mount %s to %s: %v", mountFrom, mountTo, err)
			}
		}
	}

	return nil
}

// Generate a settings file within the data directory
func generateSettingsEntry() error {
	directory := api.GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}
	settingsDir := filepath.Join(directory, "data", "settings")
	entries, err := os.ReadDir(settingsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read settings directory: %v", err)
	}
	// Only refresh if there are 2 or fewer entries (similar to original bash logic)
	if err == nil && len(entries) <= 2 {
		err = exec.Command(filepath.Join(directory, "settings"), "refresh").Run()
		if err != nil {
			return fmt.Errorf("failed to generate settings entry: %v", err)
		}
	}
	return nil
}

// generateDesktopEntries creates .desktop menu buttons, settings buttons, autostart entry, and copies icons
func generateDesktopEntries() error {
	directory := api.GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	applicationsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
	desktopPath := filepath.Join(os.Getenv("HOME"), "Desktop")
	iconsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "icons")
	autostartDir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")

	// Ensure directories exist
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create applications dir: %w", err)
	}
	if err := os.MkdirAll(desktopPath, 0o755); err != nil {
		return fmt.Errorf("failed to create Desktop dir: %w", err)
	}
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create icons dir: %w", err)
	}
	if err := os.MkdirAll(autostartDir, 0o755); err != nil {
		return fmt.Errorf("failed to create autostart dir: %w", err)
	}

	// Choose the correct categories field based on menu file
	extraCategories := ""
	if _, err := os.Stat("/etc/xdg/menus/lxde-pi-applications.menu"); os.IsNotExist(err) {
		extraCategories = "System;PackageManager;"
	}

	// Write menu button .desktop file
	menuDesktop := `[Desktop Entry]
Name=Pi-Apps Go
Comment=Raspberry Pi App Store for open source projects
Exec=` + filepath.Join(directory, "gui") + `
Icon=` + filepath.Join(directory, "icons", "logo.png") + `
Terminal=false
StartupWMClass=Pi-Apps Go
Type=Application
Categories=Utility;` + extraCategories + `
StartupNotify=true
`
	menuDesktopPath := filepath.Join(applicationsDir, "pi-apps-go.desktop")
	if err := os.WriteFile(menuDesktopPath, []byte(menuDesktop), 0o755); err != nil {
		return fmt.Errorf("failed to write menu pi-apps-go.desktop: %w", err)
	}

	// Set trusted metadata if available
	_ = exec.Command("gio", "set", menuDesktopPath, "metadata::trusted", "yes").Run()

	// Copy to Desktop and fix permissions, set trusted
	desktopCopy := filepath.Join(desktopPath, "pi-apps-go.desktop")
	if err := api.CopyFile(menuDesktopPath, desktopCopy); err != nil {
		return fmt.Errorf("failed to copy menu .desktop to Desktop: %w", err)
	}
	if err := os.Chmod(desktopCopy, 0o755); err != nil {
		return fmt.Errorf("failed to chmod Desktop .desktop file: %w", err)
	}
	_ = exec.Command("gio", "set", desktopCopy, "metadata::trusted", "yes").Run()

	// Copy icons to ~/.local/share/icons
	if err := api.CopyFile(filepath.Join(directory, "icons", "logo.png"),
		filepath.Join(iconsDir, "pi-apps-go.png")); err != nil {
		return fmt.Errorf("failed to copy logo.png: %w", err)
	}
	if err := api.CopyFile(filepath.Join(directory, "icons", "settings.png"),
		filepath.Join(iconsDir, "pi-apps-go-settings.png")); err != nil {
		return fmt.Errorf("failed to copy settings.png: %w", err)
	}

	// Write Settings menu button .desktop file
	settingsDesktop := `[Desktop Entry]
Name=Pi-Apps Go Settings
Comment=Configure Pi-Apps Go or create an App
Exec=` + filepath.Join(directory, "settings") + `
Icon=` + filepath.Join(directory, "icons", "settings.png") + `
Terminal=false
StartupWMClass=Pi-Apps-Go-Settings
Type=Application
Categories=Settings;
StartupNotify=true
`
	settingsDesktopPath := filepath.Join(applicationsDir, "pi-apps-go-settings.desktop")
	if err := os.WriteFile(settingsDesktopPath, []byte(settingsDesktop), 0o755); err != nil {
		return fmt.Errorf("failed to write settings .desktop file: %w", err)
	}

	// Write autostart updater .desktop entry
	updaterDesktop := `[Desktop Entry]
Name=Pi-Apps Go Updater
Exec=` + filepath.Join(directory, "updater") + ` onboot
Icon=` + filepath.Join(directory, "icons", "logo.png") + `
Terminal=false
StartupWMClass=Pi-Apps Go
Type=Application
X-GNOME-Autostart-enabled=true
Hidden=false
NoDisplay=false
`
	updaterDesktopPath := filepath.Join(autostartDir, "pi-apps-go-updater.desktop")
	if err := os.WriteFile(updaterDesktopPath, []byte(updaterDesktop), 0o644); err != nil {
		return fmt.Errorf("failed to write autostart updater .desktop: %w", err)
	}

	return nil
}

// fixGnuPG fixes the ownership of the ~/.gnupg directory
func fixGnuPG() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get user home dir: %v", err)
	}
	gnupgDir := filepath.Join(homeDir, ".gnupg")
	if _, err := os.Stat(gnupgDir); os.IsNotExist(err) {
		return nil
	}
	user := os.Getenv("USER")
	if user == "" {
		return fmt.Errorf("USER environment variable not set")
	}
	if err := api.SudoPopup("chown", "-R", user+":"+user, gnupgDir); err != nil {
		return fmt.Errorf("failed to fix ownership of ~/.gnupg: %w", err)
	}
	return nil
}

// debianFrontendEnv sets the DEBIAN_FRONTEND environment variable to "noninteractive"
func debianFrontendEnv() error {
	content := "Defaults      env_keep += DEBIAN_FRONTEND\n"
	if err := api.SudoPopup("sh", "-c", fmt.Sprintf("echo '%s' > /etc/sudoers.d/debian_frontend", content)); err != nil {
		return fmt.Errorf("failed to set DEBIAN_FRONTEND environment variable: %w", err)
	}
	return nil
}

func deprecatedApps() error {
	// currently this function does nothing as no deprecated apps have been added yet
	// to deprecate an app, call this function: api.RemoveDeprecatedApp("app name", "architecture", "reason")
	return nil
}

// ExecuteRunonceEntries executes all runonce entries in the updater binary.
// Uses version-coded Go functions instead of bash scripts.
func ExecuteRunonceEntries() error {
	err := api.RunonceFunc("addUserDirs-v1", addUserDirs)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the addUserDirs runonce entry: %v", err))
		return err
	}
	err = api.RunonceFunc("generateSettingsEntry-v1", generateSettingsEntry)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the generateSettingsEntry runonce entry: %v", err))
		return err
	}
	err = api.RunonceFunc("generateDesktopEntries-v1", generateDesktopEntries)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the generateDesktopEntries runonce entry: %v", err))
		return err
	}
	err = api.RunonceFunc("fixGnuPG-v1", fixGnuPG)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the fixGnuPG runonce entry: %v", err))
		return err
	}
	err = api.RunonceFunc("debianFrontendEnv-v1", debianFrontendEnv)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the debianFrontendEnv runonce entry: %v", err))
		return err
	}
	err = api.RunonceFunc("deprecatedApps-v1", deprecatedApps)
	if err != nil {
		api.ErrorNoExit(fmt.Sprintf("An error occurred while running the deprecatedApps runonce entry: %v", err))
		return err
	}
	return nil
}
