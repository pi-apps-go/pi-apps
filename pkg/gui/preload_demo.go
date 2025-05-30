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

// Module: preload_demo.go
// Description: Demonstrates how to use the preload functionality to create a simple app browser
// Note: This example might or might not get removed (if not then other ones will be added for other functionfs of the rewrite)

package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// AppBrowserDemo demonstrates a simple app browser using the preload functionality
type AppBrowserDemo struct {
	window      *gtk.Window
	treeView    *gtk.TreeView
	listStore   *gtk.ListStore
	currentList *PreloadedList
	directory   string
	currentPath string
}

// NewAppBrowserDemo creates a new app browser demo window
func NewAppBrowserDemo() (*AppBrowserDemo, error) {
	// Initialize GTK
	gtk.Init(nil)

	// Set application name
	glib.SetPrgname("Pi-Apps Browser Demo")
	glib.SetApplicationName("Pi-Apps Browser Demo")

	// Get Pi-Apps directory
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	demo := &AppBrowserDemo{
		directory:   directory,
		currentPath: "", // Start at root
	}

	// Create the main window
	if err := demo.createWindow(); err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	// Load initial app list
	if err := demo.loadAppList(demo.currentPath); err != nil {
		return nil, fmt.Errorf("failed to load initial app list: %w", err)
	}

	return demo, nil
}

// createWindow creates and configures the main window
func (demo *AppBrowserDemo) createWindow() error {
	var err error

	// Create main window
	demo.window, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("failed to create window: %w", err)
	}

	demo.window.SetTitle("Pi-Apps Browser Demo")
	demo.window.SetDefaultSize(800, 600)
	demo.window.SetPosition(gtk.WIN_POS_CENTER)

	// Set window icon
	iconPath := filepath.Join(demo.directory, "icons", "logo.png")
	if appListFileExists(iconPath) {
		pixbuf, err := gdk.PixbufNewFromFile(iconPath)
		if err == nil {
			demo.window.SetIcon(pixbuf)
		}
	}

	// Create main container
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 6)
	if err != nil {
		return fmt.Errorf("failed to create main vbox: %w", err)
	}
	vbox.SetMarginTop(12)
	vbox.SetMarginBottom(12)
	vbox.SetMarginStart(12)
	vbox.SetMarginEnd(12)
	demo.window.Add(vbox)

	// Create header with navigation info
	headerLabel, err := gtk.LabelNew("Pi-Apps Browser - Select an app or category")
	if err != nil {
		return fmt.Errorf("failed to create header label: %w", err)
	}
	headerLabel.SetHAlign(gtk.ALIGN_START)
	headerLabel.SetUseMarkup(true)
	vbox.PackStart(headerLabel, false, false, 0)

	// Create scrolled window for the app list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create scrolled window: %w", err)
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)
	vbox.PackStart(scrolled, true, true, 0)

	// Create tree view for app list
	demo.treeView, demo.listStore, err = CreateAppListTreeView()
	if err != nil {
		return fmt.Errorf("failed to create tree view: %w", err)
	}
	scrolled.Add(demo.treeView)

	// Connect selection handler
	selection, err := demo.treeView.GetSelection()
	if err != nil {
		return fmt.Errorf("failed to get tree selection: %w", err)
	}

	selection.Connect("changed", demo.onSelectionChanged)

	// Connect double-click handler
	demo.treeView.Connect("row-activated", demo.onRowActivated)

	// Connect window close handler
	demo.window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	return nil
}

// loadAppList loads and displays the app list for the given path
func (demo *AppBrowserDemo) loadAppList(path string) error {
	// Preload the app list
	list, err := PreloadAppList(demo.directory, path)
	if err != nil {
		return fmt.Errorf("failed to preload app list: %w", err)
	}

	demo.currentList = list
	demo.currentPath = path

	// Populate the tree view
	if err := PopulateGTKTreeView(demo.treeView, list); err != nil {
		return fmt.Errorf("failed to populate tree view: %w", err)
	}

	// Update window title with current location
	title := "Pi-Apps Browser Demo"
	if path != "" {
		title += " - " + path
	}
	demo.window.SetTitle(title)

	fmt.Printf("Loaded %d items for path '%s'\n", len(list.Items), path)
	return nil
}

// onSelectionChanged handles tree view selection changes
func (demo *AppBrowserDemo) onSelectionChanged() {
	// For now, just print the selected item
	path, err := GetSelectedAppPath(demo.treeView)
	if err != nil {
		return // No selection
	}

	fmt.Printf("Selected: %s\n", path)
}

// onRowActivated handles double-click or Enter on tree view rows
func (demo *AppBrowserDemo) onRowActivated(treeView *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
	// Get the selected app path
	appPath, err := GetSelectedAppPath(demo.treeView)
	if err != nil {
		fmt.Printf("Error getting selected path: %v\n", err)
		return
	}

	fmt.Printf("Activated: %s\n", appPath)

	// Check if this is a navigation item
	if appPath == "" || appPath == demo.currentPath {
		// Navigate to parent or stay in current location
		demo.navigateToParent()
	} else if appPath[len(appPath)-1] == '/' {
		// This is a category, navigate into it
		categoryPath := appPath[:len(appPath)-1] // Remove trailing slash
		if err := demo.loadAppList(categoryPath); err != nil {
			fmt.Printf("Error loading category '%s': %v\n", categoryPath, err)
		}
	} else {
		// This is an app, show app details (placeholder)
		demo.showAppDetails(appPath)
	}
}

// navigateToParent navigates to the parent directory
func (demo *AppBrowserDemo) navigateToParent() {
	if demo.currentPath == "" {
		return // Already at root
	}

	// Calculate parent path
	parentPath := ""
	if lastSlash := filepath.Dir(demo.currentPath); lastSlash != "." {
		parentPath = lastSlash
	}

	if err := demo.loadAppList(parentPath); err != nil {
		fmt.Printf("Error navigating to parent: %v\n", err)
	}
}

// showAppDetails shows details for the selected app (placeholder implementation)
func (demo *AppBrowserDemo) showAppDetails(appPath string) {
	fmt.Printf("Would show details for app: %s\n", appPath)

	// Create a simple dialog showing app info
	dialog, err := gtk.DialogNew()
	if err != nil {
		return
	}
	defer dialog.Destroy()

	dialog.SetTitle("App Details")
	dialog.SetDefaultSize(400, 300)
	dialog.AddButton("Close", gtk.RESPONSE_CLOSE)

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return
	}

	// Add app info
	info := fmt.Sprintf("App: %s\nThis is a placeholder for app details.\n\nIn a real implementation, this would show:\n- App description\n- Installation status\n- Install/uninstall buttons\n- Dependencies\n- etc.", appPath)

	label, err := gtk.LabelNew(info)
	if err != nil {
		return
	}
	label.SetHAlign(gtk.ALIGN_START)
	label.SetVAlign(gtk.ALIGN_START)
	label.SetLineWrap(true)
	label.SetSelectable(true)
	contentArea.Add(label)

	dialog.ShowAll()
	dialog.Run()
}

// Show displays the demo window
func (demo *AppBrowserDemo) Show() {
	demo.window.ShowAll()
}

// Run starts the GTK main loop
func (demo *AppBrowserDemo) Run() {
	gtk.Main()
}

// RunAppBrowserDemo creates and runs the app browser demo
// This is a convenience function for easy testing
func RunAppBrowserDemo() error {
	demo, err := NewAppBrowserDemo()
	if err != nil {
		return err
	}

	demo.Show()
	demo.Run()

	return nil
}
