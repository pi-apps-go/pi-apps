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

// Module: fileviewer.go
// Description: Provides functions for viewing files.

package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// ViewFile displays any text file in a GTK3 window
// This replicates the functionality of the original bash script's view_file function
func ViewFile(filePath string) error {

	// Set application name based on file type
	if isLogFile(filePath) {
		glib.SetPrgname("Log file viewer")
	} else {
		glib.SetPrgname("Text file viewer")
	}

	// Initialize GTK
	gtk.Init(nil)

	// Create a new window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("unable to create window: %v", err)
	}

	// Set window properties
	win.SetTitle("Pi-Apps File Viewer")
	win.SetDefaultSize(800, 600)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Try to get the Pi-Apps directory for icons
	piAppsDir := os.Getenv("DIRECTORY")
	if piAppsDir != "" {
		// Use different icons based on whether it's a log file or other file
		var iconPath string
		if isLogFile(filePath) {
			iconPath = filepath.Join(piAppsDir, "icons", "log-file.png")
		} else {
			iconPath = filepath.Join(piAppsDir, "icons", "logo.png")
		}

		if _, err := os.Stat(iconPath); err == nil {
			pixbuf, err := gdk.PixbufNewFromFile(iconPath)
			if err == nil {
				win.SetIcon(pixbuf)
			}
		}
	}

	// Create a vertical box for layout
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return fmt.Errorf("unable to create box: %v", err)
	}
	vbox.SetMarginTop(12)
	vbox.SetMarginBottom(12)
	vbox.SetMarginStart(12)
	vbox.SetMarginEnd(12)
	win.Add(vbox)

	// Create a header label
	headerLabel, err := gtk.LabelNew("")
	if err != nil {
		return fmt.Errorf("unable to create header label: %v", err)
	}

	// Set header text based on file type
	var headerText string
	if isLogFile(filePath) {
		headerText = fmt.Sprintf("<big><b>Log File: %s</b></big>", filepath.Base(filePath))
	} else {
		headerText = fmt.Sprintf("<big><b>File location: %s</b></big>", filePath)
	}

	headerLabel.SetMarkup(headerText)
	headerLabel.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(headerLabel, false, false, 0)

	// Create a scrolled window
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("unable to create scrolled window: %v", err)
	}
	scrolledWindow.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWindow.SetShadowType(gtk.SHADOW_IN)
	vbox.PackStart(scrolledWindow, true, true, 0)

	// Create a text view for displaying the log
	textView, err := gtk.TextViewNew()
	if err != nil {
		return fmt.Errorf("unable to create text view: %v", err)
	}
	textView.SetEditable(false)
	textView.SetWrapMode(gtk.WRAP_WORD_CHAR)
	scrolledWindow.Add(textView)

	// Get the buffer associated with the text view
	buffer, err := textView.GetBuffer()
	if err != nil {
		return fmt.Errorf("unable to get text buffer: %v", err)
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		buffer.SetText(fmt.Sprintf("Error reading file: %v", err))
	} else {
		buffer.SetText(string(content))
	}

	// Create a button box
	buttonBox, err := gtk.ButtonBoxNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return fmt.Errorf("unable to create button box: %v", err)
	}
	buttonBox.SetLayout(gtk.BUTTONBOX_END)
	buttonBox.SetSpacing(8)
	vbox.PackStart(buttonBox, false, false, 0)

	// Add close button
	closeButton, err := gtk.ButtonNewWithLabel("Close")
	if err != nil {
		return fmt.Errorf("unable to create close button: %v", err)
	}
	buttonBox.Add(closeButton)

	// Connect close button to quit
	closeButton.Connect("clicked", func() {
		win.Close()
	})

	// Connect window destroy signal to quit
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Show all widgets
	win.ShowAll()

	// Start GTK main loop
	gtk.Main()

	return nil
}

// ViewLog is an alias for ViewFile to maintain API compatibility
func ViewLog(logfilePath string) error {
	return ViewFile(logfilePath)
}

// isLogFile checks if a file is likely a log file based on its name
func isLogFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	return filepath.Ext(fileName) == ".log" ||
		filepath.Ext(fileName) == ".txt" ||
		filepath.Dir(filePath) == "logs" ||
		filepath.Base(filepath.Dir(filePath)) == "logs"
}
