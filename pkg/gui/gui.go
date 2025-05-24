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

// Module: gui.go
// Description: Provides functions for app deprecation dialogs.

package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// InitGUI initializes the GUI components
func InitGUI() {
	// Only initialize if we can use GTK
	if canUseGTK() {
		// Initialize application name
		glib.SetPrgname("Pi-Apps")
		glib.SetApplicationName("Pi-Apps (deprecated application dialog)")
		// Ensure GTK is initialized
		gtk.Init(nil)
	}
}

// ShowMessageDialog shows a simple message dialog
func ShowMessageDialog(title, message string, dialogType gtk.MessageType) {
	// If we can't use GTK, fall back to CLI
	if !canUseGTK() {
		fmt.Printf("\n[%s] %s\n", title, message)
		fmt.Println("Press Enter to continue...")
		fmt.Scanln()
		return
	}

	// Initialize GTK if not already initialized
	gtk.Init(nil)

	// Create dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dialog: %v\n", err)
		return
	}
	dialog.SetTitle(title)
	dialog.AddButton("OK", gtk.RESPONSE_OK)

	// Set dialog properties
	dialog.SetDefaultSize(400, 150)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set dialog icon
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir != "" {
		icon, err := gdk.PixbufNewFromFile(filepath.Join(piAppsDir, "icons", "logo.png"))
		if err == nil {
			dialog.SetIcon(icon)
		}
	}

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return
	}
	contentArea.SetSpacing(6)

	// Add message label
	label, err := gtk.LabelNew(message)
	if err != nil {
		dialog.Destroy()
		return
	}
	contentArea.Add(label)

	// Show all widgets
	dialog.ShowAll()

	// Run dialog
	dialog.Run()
	dialog.Destroy()
}

// canUseGTK checks if GTK can be used (display available)
func canUseGTK() bool {
	// Check for --cli flag to force CLI mode
	for _, arg := range os.Args {
		if arg == "--cli" {
			fmt.Fprintf(os.Stderr, "GTK disabled by --cli flag\n")
			return false
		}
	}

	// Check essential environment variables for GUI
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintf(os.Stderr, "GTK not available: No display environment variable set\n")
		return false
	}

	// Check if we're in an SSH session without X forwarding
	if os.Getenv("SSH_CONNECTION") != "" && os.Getenv("DISPLAY") == "" {
		fmt.Fprintf(os.Stderr, "GTK not available: SSH connection without X forwarding\n")
		return false
	}

	// If we reached here, attempt to use GTK
	return true
}
