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

// Module: gui.go
// Description: Main GUI implementation for Pi-Apps Go.
// This replaces the bash gui script functionality with native Go and GTK3.

package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/charmbracelet/log"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/kbinani/screenshot"
)

// GUI represents the main Pi-Apps GUI application
type GUI struct {
	directory        string
	guiMode          string
	screenWidth      int
	screenHeight     int
	window           *gtk.Window
	contentContainer *gtk.Box // Main content container for switching views
	appList          *gtk.TreeView
	appStore         *gtk.ListStore
	detailsWindow    *gtk.Window
	currentPrefix    string
	daemon           *PreloadDaemon
	ctx              context.Context
	cancel           context.CancelFunc
	currentApps      []AppListItem // Store current apps by index for reliable access
	widgetCount      int           // Track number of widgets created for memory management
}

// GUIConfig holds configuration for the GUI
type GUIConfig struct {
	Directory string
	GuiMode   string
}

// WindowGeometry holds window position and size information
type WindowGeometry struct {
	Width   int
	Height  int
	XOffset int
	YOffset int
}

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportCaller:    true,
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
})

// NewGUI creates a new GUI instance
func NewGUI(config GUIConfig) (*GUI, error) {
	if config.Directory == "" {
		config.Directory = os.Getenv("PI_APPS_DIR")
		if config.Directory == "" {
			return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
		}
	}

	if config.GuiMode == "" {
		// Read GUI mode from settings
		modeBytes, err := os.ReadFile(filepath.Join(config.Directory, "data", "settings", "App List Style"))
		if err != nil {
			config.GuiMode = "default"
		} else {
			config.GuiMode = strings.TrimSpace(string(modeBytes))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	gui := &GUI{
		directory:     config.Directory,
		guiMode:       config.GuiMode,
		currentPrefix: "",
		ctx:           ctx,
		cancel:        cancel,
	}

	return gui, nil
}

// Initialize sets up the GUI environment and dependencies
func (g *GUI) Initialize() error {
	// Check if running as root
	if os.Getuid() == 0 {
		return fmt.Errorf("Pi-Apps is not designed to be run as root! Please try again as a regular user")
	}

	// Set GUI format version
	os.Setenv("GUI_FORMAT_VERSION", "2")
	os.Setenv("PI_APPS_DIR", g.directory)

	// Initialize app name
	glib.SetPrgname("Pi-Apps")

	// Initialize GTK
	gtk.Init(nil)

	// Get screen dimensions
	if err := g.getScreenDimensions(); err != nil {
		return fmt.Errorf("failed to get screen dimensions: %w", err)
	}

	// Create necessary directories
	if err := g.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Start background tasks
	go g.startBackgroundTasks()

	// Start preload daemon
	daemon, err := StartPreloadDaemon(g.directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start preload daemon: %v\n", err)
	} else {
		g.daemon = daemon
	}

	return nil
}

// Run starts the main GUI application
func (g *GUI) Run() error {
	logger.Info(fmt.Sprintf("GUI Run() called with mode: %s", g.guiMode))

	switch {
	case strings.HasPrefix(g.guiMode, "yad"):
		logger.Info("Using YAD mode")
		return g.runYADMode()
	case strings.HasPrefix(g.guiMode, "xlunch"):
		logger.Info("Using Xlunch mode")
		return g.runXlunchMode()
	case g.guiMode == "gtk" || g.guiMode == "native":
		logger.Info("Using native GTK mode")
		return g.runNativeMode()
	case g.guiMode == "default" || g.guiMode == "":
		// Default mode - prefer native GTK if available, otherwise fall back
		if canUseGTK() {
			logger.Info("Default mode: using native GTK")
			return g.runNativeMode()
		} else {
			logger.Warn("Default mode: GTK not available, falling back to bash GUI")
			return g.runYADMode()
		}
	default:
		logger.Error("Unrecognized app list style '%s'", g.guiMode)
		return fmt.Errorf("unrecognized app list style '%s'", g.guiMode)
	}
}

// Cleanup performs cleanup operations
func (g *GUI) Cleanup() {
	if g.cancel != nil {
		g.cancel()
	}
	if g.daemon != nil {
		g.daemon.Stop()
	}
	if g.window != nil {
		g.window.Destroy()
	}
	if g.detailsWindow != nil {
		g.detailsWindow.Destroy()
	}
}

// getScreenDimensions gets the current screen dimensions using screenshot library with fallbacks
func (g *GUI) getScreenDimensions() error {
	// Try to get screen dimensions using screenshot library first
	bounds := screenshot.GetDisplayBounds(0)
	if bounds.Dx() > 0 && bounds.Dy() > 0 {
		g.screenWidth = bounds.Dx()
		g.screenHeight = bounds.Dy()
		return nil
	}

	// Fallback: Try to get screen dimensions using xrandr (same as bash version)
	// Note: this method isn't cross-platform, but this can be removed if we want to use screenshot library only and GTK fallbacks
	// TODO: remove this fallback on non-linux systems if we want to use screenshot library only and GTK fallbacks
	cmd := exec.Command("xrandr", "--nograb", "--current")
	output, err := cmd.Output()
	if err == nil {
		// Parse xrandr output to get screen dimensions
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "connected") && strings.Contains(line, "primary") {
				// Look for pattern like "1920x1080+0+0"
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.Contains(part, "x") && strings.Contains(part, "+") {
						coords := strings.Split(part, "+")[0]
						dimensions := strings.Split(coords, "x")
						if len(dimensions) == 2 {
							if width, err := strconv.Atoi(dimensions[0]); err == nil {
								g.screenWidth = width
							}
							if height, err := strconv.Atoi(dimensions[1]); err == nil {
								g.screenHeight = height
							}
							return nil
						}
					}
				}
			}
		}

		// Fallback: look for any connected display if primary not found
		for _, line := range lines {
			if strings.Contains(line, "connected") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.Contains(part, "x") && strings.Contains(part, "+") {
						coords := strings.Split(part, "+")[0]
						dimensions := strings.Split(coords, "x")
						if len(dimensions) == 2 {
							if width, err := strconv.Atoi(dimensions[0]); err == nil {
								g.screenWidth = width
							}
							if height, err := strconv.Atoi(dimensions[1]); err == nil {
								g.screenHeight = height
							}
							return nil
						}
					}
				}
			}
		}
	}

	// Final fallback to GTK method if both screenshot and xrandr fail
	display, err := gdk.DisplayGetDefault()
	if err != nil {
		logger.Error("failed to get default display: %w", err)
		return fmt.Errorf("failed to get default display: %w", err)
	}

	monitor, err := display.GetPrimaryMonitor()
	if err != nil {
		logger.Error("failed to get primary monitor: %w", err)
		return fmt.Errorf("failed to get primary monitor: %w", err)
	}

	geometry := monitor.GetGeometry()
	g.screenWidth = geometry.GetWidth()
	g.screenHeight = geometry.GetHeight()

	return nil
}

// createDirectories creates necessary directories
func (g *GUI) createDirectories() error {
	dirs := []string{
		"data/status",
		"data/update-status",
		"data/preload",
		"data/settings",
		"data/categories",
	}

	for _, dir := range dirs {
		path := filepath.Join(g.directory, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			logger.Error("failed to create directory %s: %w", path, err)
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	return nil
}

// startBackgroundTasks starts background operations
func (g *GUI) startBackgroundTasks() {
	// Start updater status check
	go func() {
		cmd := exec.Command(filepath.Join(g.directory, "updater"), "set-status")
		cmd.Run() // Ignore errors, this is background
	}()

	// Usage tracking
	go func() {
		// Equivalent to shlink_link usage active
		// This would need to be implemented based on the actual shlink implementation
		// TODO: implement this once our shlink server is ready
	}()
}

// runNativeMode runs the GUI in native GTK3 mode
func (g *GUI) runNativeMode() error {
	logger.Info("runNativeMode: Starting GTK3 interface")

	// Create main window
	window, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		logger.Fatal("failed to create window: %w", err)
		return fmt.Errorf("failed to create window: %w", err)
	}
	g.window = window
	logger.Info("runNativeMode: Window created successfully")

	window.SetTitle("Pi-Apps")

	// Set window size based on screen resolution (matching bash logic)
	var windowWidth, windowHeight int
	if g.screenWidth <= 1000 || g.screenHeight <= 600 {
		// Small screen settings - more compact
		windowHeight = 600
		windowWidth = 500
		logger.Info(fmt.Sprintf("Small screen detected (%dx%d), using window size %dx%d\n",
			g.screenWidth, g.screenHeight, windowWidth, windowHeight))
	} else {
		// Large screen settings - wider but still compact
		windowHeight = 700
		windowWidth = 600
		logger.Info(fmt.Sprintf("Large screen detected (%dx%d), using window size %dx%d\n",
			g.screenWidth, g.screenHeight, windowWidth, windowHeight))
	}

	window.SetDefaultSize(windowWidth, windowHeight)
	window.SetPosition(gtk.WIN_POS_CENTER)
	window.SetResizable(true)
	logger.Info(fmt.Sprintf("runNativeMode: Window size set to %dx%d\n", windowWidth, windowHeight))

	// Set window icon
	iconPath := filepath.Join(g.directory, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		if err := window.SetIconFromFile(iconPath); err != nil {
			logger.Warn(fmt.Sprintf("failed to set window icon: %v\n", err))
		}
	}

	// Create main layout - no margins for compact look
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to create main box: %w", err))
		return fmt.Errorf("failed to create main box: %w", err)
	}
	logger.Info("runNativeMode: Main layout created")

	// Create app info header (like the CloudBuddy/WiFi Hotspot area)
	if err := g.createAppInfoHeader(vbox); err != nil {
		logger.Fatal(fmt.Errorf("failed to create app info header: %w", err))
		return fmt.Errorf("failed to create app info header: %w", err)
	}
	logger.Info("runNativeMode: App info header created")

	// Create content container for switching between views
	contentContainer, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		logger.Fatal(fmt.Errorf("failed to create content container: %w", err))
		return fmt.Errorf("failed to create content container: %w", err)
	}
	g.contentContainer = contentContainer
	vbox.PackStart(contentContainer, true, true, 0)
	logger.Info("runNativeMode: Content container created")

	// Create initial category list view
	if err := g.showCategoryListView(); err != nil {
		logger.Fatal(fmt.Errorf("failed to create category list: %w", err))
		return fmt.Errorf("failed to create category list: %w", err)
	}
	logger.Info("runNativeMode: Category list created")

	// Create bottom buttons
	if err := g.createBottomButtons(vbox); err != nil {
		logger.Fatal(fmt.Errorf("failed to create bottom buttons: %w", err))
		return fmt.Errorf("failed to create bottom buttons: %w", err)
	}
	logger.Info("runNativeMode: Bottom buttons created")

	window.Add(vbox)

	// Connect signals
	window.Connect("destroy", func() {
		logger.Info("runNativeMode: Window destroy signal received")
		g.Cleanup()
		gtk.MainQuit()
	})

	// Show window
	logger.Info("runNativeMode: Showing window...")
	window.ShowAll()

	// Start GTK main loop
	logger.Info("runNativeMode: Starting GTK main loop")
	gtk.Main()

	logger.Info("runNativeMode: GTK main loop exited")
	return nil
}

// createAppInfoHeader creates the top section showing app info (like CloudBuddy/WiFi Hotspot)
func (g *GUI) createAppInfoHeader(parent *gtk.Box) error {
	// Create frame for the app info section
	frame, err := gtk.FrameNew("")
	if err != nil {
		return err
	}
	frame.SetShadowType(gtk.SHADOW_IN)

	// Create horizontal box for icon and text
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return err
	}
	hbox.SetMarginTop(8)
	hbox.SetMarginBottom(8)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)

	// Add app icon (placeholder - this would be updated based on selected app)
	iconPath := filepath.Join(g.directory, "icons", "logo-24.png")
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		iconPath = filepath.Join(g.directory, "icons", "logo.png")
	}

	// Load and scale the icon properly
	pixbuf, err := gdk.PixbufNewFromFile(iconPath)
	if err == nil {
		// Scale the pixbuf to exactly 64x64 pixels
		scaledPixbuf, err := pixbuf.ScaleSimple(64, 64, gdk.INTERP_BILINEAR)
		if err == nil {
			image, err := gtk.ImageNewFromPixbuf(scaledPixbuf)
			if err == nil {
				hbox.PackStart(image, false, false, 0)
			}
		}
	}

	// Add app description text
	motd := g.GetMessageOfTheDay()
	label, err := gtk.LabelNew("")
	if err == nil {
		label.SetMarkup(motd) // Use SetMarkup to enable HTML-like tags including <b>
		label.SetLineWrap(true)
		label.SetLines(2)     // Limit to 2 lines like original
		label.SetEllipsize(3) // PANGO_ELLIPSIZE_END
		label.SetJustify(gtk.JUSTIFY_LEFT)
		label.SetHAlign(gtk.ALIGN_START)
		label.SetVAlign(gtk.ALIGN_CENTER)
		hbox.PackStart(label, true, true, 0)
	}

	frame.Add(hbox)
	parent.PackStart(frame, false, false, 0)
	return nil
}

// showCategoryListView displays the main category list in the content container
func (g *GUI) showCategoryListView() error {
	// Clear existing content
	g.clearContentContainer()

	// Create scrolled window for the list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC) // No horizontal scroll
	scrolled.SetShadowType(gtk.SHADOW_IN)

	// Create list box for categories
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		return err
	}
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)

	// Build categories list dynamically
	var categories []struct {
		name        string
		icon        string
		description string
	}

	// Check if updates are available (matching bash logic)
	updatableFilesPath := filepath.Join(g.directory, "data", "update-status", "updatable-files")
	updatableAppsPath := filepath.Join(g.directory, "data", "update-status", "updatable-apps")

	updatesAvailable := false
	if stat, err := os.Stat(updatableFilesPath); err == nil && stat.Size() > 0 {
		updatesAvailable = true
	}
	if stat, err := os.Stat(updatableAppsPath); err == nil && stat.Size() > 0 {
		updatesAvailable = true
	}

	// Add Updates category only if updates are available
	if updatesAvailable {
		categories = append(categories, struct {
			name        string
			icon        string
			description string
		}{"Updates", "Updates.png", "Pi-Apps updates are available."})
	}

	// Add standard categories in the correct order
	standardCategories := []struct {
		name        string
		icon        string
		description string
	}{
		{"All Apps", "All Apps.png", "All Pi-Apps Applications in one long list."},
		{"Appearance", "Appearance.png", "Applications and Themes which modify the look and feel of your OS."},
		{"Creative Arts", "Creative Arts.png", "Drawing, Painting, and Photo and Movie Editors"},
		{"Engineering", "Engineering.png", "3D Printing slicers, CAD/modeling, and general design software"},
		{"Games", "Games.png", "Games and Emulators"},
		{"Installed", "Installed.png", "All Pi-Apps Apps that you have installed."},
		{"Internet", "Internet.png", "Browsers, Chat Clients, Email Clients, and so much more."},
		{"Multimedia", "Multimedia.png", "Video playback and creation, audio playback and creation, and streaming alternatives."},
		{"Office", "Office.png", "Office suites (document and slideshow editors), and other office tools."},
		{"Packages", "Packages.png", "Simple Apps that install directly from APT repos."},
		{"Programming", "Programming.png", "Code editors, IDEs, and other applications to help you write and make other programs."},
		{"System Management", "System Management.png", "Apps that help you keep track of system resources and general system management."},
		{"Terminals", "Terminals.png", "Alternative terminal programs built for the modern age as well as to replicate your old vintage computer."},
		{"Tools", "Tools.png", "An assortment of helpful programs that don't already fit into another category."},
	}

	categories = append(categories, standardCategories...)

	for _, category := range categories {
		row, err := g.createCategoryRow(category.name, category.icon, category.description)
		if err != nil {
			continue // Skip on error
		}
		listBox.Add(row)
	}

	// Connect selection handler
	listBox.Connect("row-activated", func(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
		index := row.GetIndex()
		if index >= 0 && index < len(categories) {
			category := categories[index]
			g.onCategorySelected(category.name)
		}
	})

	scrolled.Add(listBox)
	g.contentContainer.PackStart(scrolled, true, true, 0)

	// Show the new content
	g.contentContainer.ShowAll()

	return nil
}

// clearContentContainer safely clears all children from the content container
func (g *GUI) clearContentContainer() {
	if g.contentContainer == nil {
		return
	}

	// Get all children and remove them properly
	children := g.contentContainer.GetChildren()
	children.Foreach(func(item interface{}) {
		if widget, ok := item.(*gtk.Widget); ok {
			// First remove from parent
			g.contentContainer.Remove(widget)
			// Then explicitly destroy the widget to free all resources
			widget.Destroy()
		}
	})

	// Clear our app row data map since rows are being destroyed
	if g.currentApps != nil {
		g.currentApps = []AppListItem{}
	}
}

// createCategoryRow creates a single category row with icon and text
func (g *GUI) createCategoryRow(name, iconFile, description string) (*gtk.ListBoxRow, error) {
	row, err := gtk.ListBoxRowNew()
	if err != nil {
		return nil, err
	}

	// Create horizontal box for row content
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return nil, err
	}
	hbox.SetMarginTop(4)
	hbox.SetMarginBottom(4)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)

	// Add category icon
	iconPath := filepath.Join(g.directory, "icons", "categories", iconFile)
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		iconPath = filepath.Join(g.directory, "icons", "categories", "default.png")
	}

	image, err := gtk.ImageNewFromFile(iconPath)
	if err == nil {
		image.SetPixelSize(24)
		hbox.PackStart(image, false, false, 0)
	}

	// Add category name
	label, err := gtk.LabelNew(name)
	if err == nil {
		label.SetHAlign(gtk.ALIGN_START)
		hbox.PackStart(label, true, true, 0)
	}

	row.Add(hbox)
	return row, nil
}

// onCategorySelected handles category selection
func (g *GUI) onCategorySelected(category string) {
	logger.Info(fmt.Sprintf("Selected category: %s\n", category))

	// Prevent rapid category switching - add a small delay to ensure previous operations complete
	if g.currentPrefix == category {
		return // Already showing this category
	}

	// Handle special categories
	switch category {
	case "Updates":
		g.showUpdatesWindow()
		return
	case "Search":
		g.onSearchClicked()
		return
	}

	// Update the current prefix
	g.currentPrefix = category

	// Show the category apps view with better error handling
	if err := g.showCategoryAppsView(category); err != nil {
		logger.Error(fmt.Sprintf("Error showing category apps: %v\n", err))

		// Try to recover by going back to category list
		g.currentPrefix = ""
		if recoverErr := g.showCategoryListView(); recoverErr != nil {
			logger.Error(fmt.Sprintf("Error recovering to category list: %v\n", recoverErr))

			// Last resort: show error dialog
			dialog := gtk.MessageDialogNew(
				g.window,
				gtk.DIALOG_MODAL,
				gtk.MESSAGE_ERROR,
				gtk.BUTTONS_OK,
				"Error loading apps for category '%s': %v\n\nReturning to main category list.",
				category, err,
			)
			defer dialog.Destroy()
			dialog.Run()
		}
	}
}

// showCategoryAppsView displays apps for a specific category
func (g *GUI) showCategoryAppsView(category string) error {
	// Clear existing content first
	g.clearContentContainer()

	// Force garbage collection
	g.widgetCount++
	if g.widgetCount%10 == 0 {
		logger.Info(fmt.Sprintf("Triggering garbage collection after %d widget operations\n", g.widgetCount))
		runtime.GC()
		runtime.GC()
	}

	// Check if this category has subcategories
	subcategories := g.getSubcategories(category)

	// Create header with back button and category name
	headerBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return err
	}
	headerBox.SetMarginTop(8)
	headerBox.SetMarginBottom(8)
	headerBox.SetMarginStart(8)
	headerBox.SetMarginEnd(8)

	// Back button
	backBtn, err := gtk.ButtonNewWithLabel("← Back")
	if err == nil {
		backBtn.Connect("clicked", func() {
			g.currentPrefix = ""
			g.showCategoryListView() // Return to main category list
		})
		headerBox.PackStart(backBtn, false, false, 0)
	}

	// Category title
	categoryLabel, err := gtk.LabelNew("")
	if err == nil {
		categoryLabel.SetMarkup(fmt.Sprintf("<b>%s</b>", category))
		categoryLabel.SetHAlign(gtk.ALIGN_START)
		headerBox.PackStart(categoryLabel, true, true, 0)
	}

	g.contentContainer.PackStart(headerBox, false, false, 0)

	// Add separator
	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err == nil {
		g.contentContainer.PackStart(separator, false, false, 0)
	}

	// Create scrolled window for the list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)

	// Create list box
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		return err
	}
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)

	if len(subcategories) > 0 {
		// Show subcategories
		g.populateSubcategories(listBox, category, subcategories)

		// Connect subcategory selection handler
		listBox.Connect("row-activated", func(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
			logger.Info(fmt.Sprintf("Subcategory row activated in category: %s\n", category))

			// Get the row index instead of using pointer mapping
			rowIndex := row.GetIndex()
			logger.Info(fmt.Sprintf("Selected row index: %d\n", rowIndex))

			if rowIndex >= 0 && rowIndex < len(subcategories) {
				subcategoryName := subcategories[rowIndex]
				logger.Info(fmt.Sprintf("Selected subcategory: %s in category: %s\n", subcategoryName, category))
				g.showSubcategoryAppsView(category, subcategoryName)
			} else {
				logger.Error(fmt.Sprintf("Invalid row index: %d (available subcategories: %d)\n", rowIndex, len(subcategories)))
			}
		})
	} else {
		// Show apps directly in this category
		g.populateAppsInCategory(listBox, category)

		// Connect app selection handler
		listBox.Connect("row-activated", func(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
			logger.Info(fmt.Sprintf("App row activated in category: %s\n", category))
			rowIndex := row.GetIndex()
			logger.Info(fmt.Sprintf("Selected row index: %d\n", rowIndex))

			if appName := g.getAppNameFromRow(row); appName != "" {
				appPath := appName
				if category != "" && category != "All Apps" {
					appPath = fmt.Sprintf("%s/%s", category, appName)
				}
				logger.Info(fmt.Sprintf("Opening app details for: %s\n", appPath))
				g.showAppDetails(appPath)
			} else {
				logger.Error(fmt.Sprintf("Could not get app name from row index %d (total apps: %d)\n", rowIndex, len(g.currentApps)))
			}
		})
	}

	scrolled.Add(listBox)
	g.contentContainer.PackStart(scrolled, true, true, 0)

	// Show the new content
	g.contentContainer.ShowAll()

	return nil
}

// onAppSelectionChanged handles app selection changes
func (g *GUI) onAppSelectionChanged() {
	// This could be used to show app info in a side panel
}

// onAppDoubleClicked handles app double-click events
func (g *GUI) onAppDoubleClicked(treeView *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
	appPath, err := GetSelectedAppPath(treeView)
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting selected app: %v\n", err))
		return
	}

	if appPath == "" {
		return
	}

	// Handle different types of selections
	if strings.HasSuffix(appPath, "/") {
		// Category selected - refresh list
		category := strings.TrimSuffix(appPath, "/")
		if category == "Updates" {
			g.showUpdatesWindow()
		} else {
			g.onCategorySelected(category)
		}
	} else {
		// App selected - show details
		g.showAppDetails(appPath)
	}
}

// onSearchClicked handles search button clicks
func (g *GUI) onSearchClicked() {
	logger.Info("Search button clicked, creating custom search dialog")

	// Load the last search query if available
	lastSearchFile := filepath.Join(g.directory, "data", "last-search")
	lastSearch := ""
	if data, err := os.ReadFile(lastSearchFile); err == nil {
		lastSearch = strings.TrimSpace(string(data))
	}

	// Create search dialog with advanced options (matching the API search interface)
	dialog, err := gtk.DialogNew()
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating search dialog: %v", err))
		return
	}
	defer dialog.Destroy()

	// Set dialog properties
	dialog.SetTitle("Search")
	dialog.SetTransientFor(g.window)
	dialog.SetModal(true)
	dialog.SetDefaultSize(310, 200)

	// Add buttons manually to ensure both appear
	dialog.AddButton("_Cancel", gtk.RESPONSE_CANCEL)
	dialog.AddButton("_Search", gtk.RESPONSE_OK)
	dialog.SetDefaultResponse(gtk.RESPONSE_OK) // Make Search the default button

	// Create main content box
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return
	}

	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return
	}
	mainBox.SetMarginTop(10)
	mainBox.SetMarginBottom(10)
	mainBox.SetMarginStart(10)
	mainBox.SetMarginEnd(10)

	// Header text
	headerLabel, err := gtk.LabelNew("Search for apps.\nNot case-sensitive.")
	if err == nil {
		mainBox.PackStart(headerLabel, false, false, 0)
	}

	// Search entry
	searchEntry, err := gtk.EntryNew()
	if err != nil {
		return
	}
	searchEntry.SetText(lastSearch)
	mainBox.PackStart(searchEntry, false, false, 0)

	// Search options checkboxes (matching API dialog)
	descCheck, err := gtk.CheckButtonNewWithLabel("Search description")
	if err == nil {
		descCheck.SetActive(true)
		mainBox.PackStart(descCheck, false, false, 0)
	}

	websiteCheck, err := gtk.CheckButtonNewWithLabel("Search website")
	if err == nil {
		websiteCheck.SetActive(true)
		mainBox.PackStart(websiteCheck, false, false, 0)
	}

	creditsCheck, err := gtk.CheckButtonNewWithLabel("Search credits")
	if err == nil {
		creditsCheck.SetActive(false)
		mainBox.PackStart(creditsCheck, false, false, 0)
	}

	scriptsCheck, err := gtk.CheckButtonNewWithLabel("Search scripts")
	if err == nil {
		scriptsCheck.SetActive(false)
		mainBox.PackStart(scriptsCheck, false, false, 0)
	}

	contentArea.PackStart(mainBox, true, true, 0)
	dialog.ShowAll()

	// Handle Enter key in search entry
	searchEntry.Connect("activate", func() {
		dialog.Response(gtk.RESPONSE_OK)
	})

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		searchText, err := searchEntry.GetText()
		if err == nil && searchText != "" {
			// Build search files list based on checkboxes
			var searchFiles []string
			if descCheck != nil && descCheck.GetActive() {
				searchFiles = append(searchFiles, "description")
			}
			if websiteCheck != nil && websiteCheck.GetActive() {
				searchFiles = append(searchFiles, "website")
			}
			if creditsCheck != nil && creditsCheck.GetActive() {
				searchFiles = append(searchFiles, "credits")
			}
			if scriptsCheck != nil && scriptsCheck.GetActive() {
				searchFiles = append(searchFiles, "install", "install-32", "install-64", "uninstall")
			}

			// Perform the search
			g.performAdvancedSearch(searchText, searchFiles)
		}
	}
}

// onSettingsClicked handles settings button clicks
func (g *GUI) onSettingsClicked() {
	// Remember the current GUI mode before opening settings
	currentGuiMode := g.guiMode

	// Hide the main window while settings is open
	g.window.Hide()

	// Run settings script
	cmd := exec.Command(filepath.Join(g.directory, "settings"))
	err := cmd.Run()

	if err != nil {
		logger.Error(fmt.Sprintf("Error running settings: %v\n", err))
		// Show the window again if settings failed
		g.window.Show()
		return
	}

	// Settings completed - check if GUI mode changed
	newGuiMode := ""
	if modeBytes, err := os.ReadFile(filepath.Join(g.directory, "data", "settings", "App List Style")); err == nil {
		newGuiMode = strings.TrimSpace(string(modeBytes))
	}
	if newGuiMode == "" {
		newGuiMode = "default"
	}

	// Only restart if the GUI mode actually changed
	if newGuiMode != currentGuiMode {
		logger.Info(fmt.Sprintf("GUI mode changed from '%s' to '%s', restarting GUI\n", currentGuiMode, newGuiMode))

		// Settings changed the GUI mode - restart the GUI to apply new settings
		g.window.Destroy()
		gtk.MainQuit()

		// Start a new GUI instance in a goroutine to avoid blocking
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to ensure cleanup

			// Create new GUI with the updated mode
			config := GUIConfig{
				Directory: g.directory,
				GuiMode:   newGuiMode,
			}

			newGUI, err := NewGUI(config)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to create new GUI: %v\n", err))
				return
			}

			if err := newGUI.Initialize(); err != nil {
				logger.Error(fmt.Sprintf("Failed to initialize new GUI: %v\n", err))
				return
			}

			if err := newGUI.Run(); err != nil {
				logger.Error(fmt.Sprintf("Failed to run new GUI: %v\n", err))
			}
		}()
	} else {
		// No GUI mode change, just show the window again
		logger.Info("Settings closed, no GUI mode change detected")
		g.window.Show()
	}
}

// performSearch performs app search using the API's AppSearchGUI function
func (g *GUI) performSearch(query string) {
	logger.Info(fmt.Sprintf("Performing search for: %s", query))

	// Use the API's AppSearch function to get search results
	results, err := api.AppSearch(query)
	if err != nil {
		logger.Error(fmt.Sprintf("Error performing search: %v", err))
		dialog := gtk.MessageDialogNew(
			g.window,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_ERROR,
			gtk.BUTTONS_OK,
			"Error performing search: %v",
			err,
		)
		defer dialog.Destroy()
		dialog.Run()
		return
	}

	// Save the search query for next time (like the original bash version)
	lastSearchFile := filepath.Join(g.directory, "data", "last-search")
	os.MkdirAll(filepath.Dir(lastSearchFile), 0755)
	os.WriteFile(lastSearchFile, []byte(query), 0644)

	// Handle search results
	if len(results) == 0 {
		// No results found
		dialog := gtk.MessageDialogNew(
			g.window,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_INFO,
			gtk.BUTTONS_OK,
			"No results found for \"%s\".",
			query,
		)
		defer dialog.Destroy()
		dialog.Run()
		return
	}

	if len(results) == 1 {
		// Single result - show app details directly
		logger.Info(fmt.Sprintf("Single search result: %s", results[0]))
		g.showAppDetails(results[0])
		return
	}

	// Multiple results - show search results view
	logger.Info(fmt.Sprintf("Multiple search results: %d apps found", len(results)))
	g.showSearchResults(query, results)
}

// showUpdatesWindow shows the updates window
func (g *GUI) showUpdatesWindow() {
	// Run updater in GUI mode
	cmd := exec.Command(filepath.Join(g.directory, "updater"), "gui", "fast")
	if err := cmd.Run(); err != nil {
		logger.Error(fmt.Sprintf("Error running updater: %v\n", err))
	}

	// Updates window closed, user can now see updated category list
	logger.Info("Updates completed, category list will reflect changes")
}

// showAppDetails shows the app details window
func (g *GUI) showAppDetails(appPath string) {
	if g.detailsWindow != nil {
		g.detailsWindow.Destroy()
		g.detailsWindow = nil
	}

	// Parse app path - remove category prefix to get just the app name
	var appName string
	if strings.Contains(appPath, "/") {
		parts := strings.Split(appPath, "/")
		appName = parts[len(parts)-1]
	} else {
		appName = appPath
	}

	logger.Info(fmt.Sprintf("Showing details for app: %s\n", appName))

	// Create details window
	window, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating details window: %v\n", err))
		return
	}
	g.detailsWindow = window

	window.SetTitle(fmt.Sprintf("Details of %s", appName))
	window.SetDefaultSize(500, 400)
	window.SetTransientFor(g.window)
	window.SetPosition(gtk.WIN_POS_CENTER_ON_PARENT)
	window.SetResizable(true)

	// Set window icon
	iconPath := filepath.Join(g.directory, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		window.SetIconFromFile(iconPath)
	}

	// Create content
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		window.Destroy()
		return
	}
	vbox.SetMarginTop(15)
	vbox.SetMarginBottom(15)
	vbox.SetMarginStart(15)
	vbox.SetMarginEnd(15)

	// App icon and header info
	headerBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 15)
	if err == nil {
		// Add app icon (64px like in original)
		iconPath := filepath.Join(g.directory, "apps", appName, "icon-64.png")
		if _, err := os.Stat(iconPath); err == nil {
			if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
				if image, err := gtk.ImageNewFromPixbuf(pixbuf); err == nil {
					image.SetVAlign(gtk.ALIGN_START)
					headerBox.PackStart(image, false, false, 0)
				}
			}
		}

		// App info (name, status, website, user count)
		infoBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
		if err == nil {
			// App name and status (matching original format)
			status := g.getAppStatus(appName)
			nameLabel, err := gtk.LabelNew("")
			if err == nil {
				statusText := ""
				switch status {
				case "installed":
					statusText = "(installed)"
				case "corrupted":
					statusText = "(corrupted - installation failed)"
				case "disabled":
					statusText = "(disabled - installation is prevented on your system)"
				case "uninstalled":
					statusText = "(uninstalled)"
				default:
					statusText = "(uninstalled)"
				}

				nameLabel.SetMarkup(fmt.Sprintf("<b>%s</b> %s", appName, statusText))
				nameLabel.SetHAlign(gtk.ALIGN_START)
				nameLabel.SetLineWrap(true)
				infoBox.PackStart(nameLabel, false, false, 0)
			}

			// Package info if it's a package app
			if g.isPackageApp(appName) {
				packageLabel, err := gtk.LabelNew("- This app installs system packages")
				if err == nil {
					packageLabel.SetHAlign(gtk.ALIGN_START)
					infoBox.PackStart(packageLabel, false, false, 0)
				}
			}

			// Website link
			if website := g.getAppWebsite(appName); website != "" {
				websiteLabel, err := gtk.LabelNew("")
				if err == nil {
					websiteLabel.SetMarkup(fmt.Sprintf("- Website: <a href='%s'>%s</a>", website, website))
					websiteLabel.SetHAlign(gtk.ALIGN_START)
					infoBox.PackStart(websiteLabel, false, false, 0)
				}
			}

			// User count (placeholder - would need actual implementation)
			userCountLabel, err := gtk.LabelNew("- User count not available in Go version")
			if err == nil {
				userCountLabel.SetHAlign(gtk.ALIGN_START)
				infoBox.PackStart(userCountLabel, false, false, 0)
			}

			headerBox.PackStart(infoBox, true, true, 0)
		}

		vbox.PackStart(headerBox, false, false, 0)
	}

	// App description in scrolled text view
	desc := g.getAppDescription(appName)
	if desc != "" {
		scrolled, err := gtk.ScrolledWindowNew(nil, nil)
		if err == nil {
			scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
			scrolled.SetShadowType(gtk.SHADOW_IN)

			textView, err := gtk.TextViewNew()
			if err == nil {
				textView.SetEditable(false)
				textView.SetWrapMode(gtk.WRAP_WORD)
				textView.SetMarginTop(5)
				textView.SetMarginBottom(5)
				textView.SetMarginStart(5)
				textView.SetMarginEnd(5)

				buffer, err := textView.GetBuffer()
				if err == nil {
					buffer.SetText(desc)
				}

				scrolled.Add(textView)
				vbox.PackStart(scrolled, true, true, 0)
			}
		}
	}

	// Button box at bottom - different buttons based on status
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err == nil {
		buttonBox.SetHAlign(gtk.ALIGN_END)

		status := g.getAppStatus(appName)

		// Scripts button (if install script exists)
		if g.hasInstallScript(appName) {
			scriptsBtn, err := gtk.ButtonNewWithLabel("Scripts")
			if err == nil {
				scriptsBtn.Connect("clicked", func() {
					g.openAppScripts(appName)
				})
				buttonBox.PackStart(scriptsBtn, false, false, 0)
			}
		}

		// Buttons based on status (matching original logic)
		if status == "installed" {
			// Only uninstall button for installed apps
			uninstallBtn, err := gtk.ButtonNewWithLabel("Uninstall")
			if err == nil {
				uninstallBtn.Connect("clicked", func() {
					g.performAppAction(appName, "uninstall")
					window.Destroy()
					g.detailsWindow = nil
				})
				buttonBox.PackStart(uninstallBtn, false, false, 0)
			}
		} else if status == "uninstalled" {
			// Only install button for uninstalled apps
			installBtn, err := gtk.ButtonNewWithLabel("Install")
			if err == nil {
				installBtn.Connect("clicked", func() {
					g.performAppAction(appName, "install")
					window.Destroy()
					g.detailsWindow = nil
				})
				buttonBox.PackStart(installBtn, false, false, 0)
			}
		} else if status == "disabled" {
			// Only enable button for disabled apps
			enableBtn, err := gtk.ButtonNewWithLabel("Enable")
			if err == nil {
				enableBtn.Connect("clicked", func() {
					g.enableApp(appName)
					window.Destroy()
					g.detailsWindow = nil
				})
				buttonBox.PackStart(enableBtn, false, false, 0)
			}
		} else {
			// For corrupted or unknown status, show both buttons
			// Plus errors button for corrupted apps
			if status == "corrupted" {
				errorsBtn, err := gtk.ButtonNewWithLabel("Errors")
				if err == nil {
					errorsBtn.Connect("clicked", func() {
						g.viewAppErrors(appName)
					})
					buttonBox.PackStart(errorsBtn, false, false, 0)
				}
			}

			// Uninstall button
			uninstallBtn, err := gtk.ButtonNewWithLabel("Uninstall")
			if err == nil {
				uninstallBtn.Connect("clicked", func() {
					g.performAppAction(appName, "uninstall")
					window.Destroy()
					g.detailsWindow = nil
				})
				buttonBox.PackStart(uninstallBtn, false, false, 0)
			}

			// Install button
			installBtn, err := gtk.ButtonNewWithLabel("Install")
			if err == nil {
				installBtn.Connect("clicked", func() {
					g.performAppAction(appName, "install")
					window.Destroy()
					g.detailsWindow = nil
				})
				buttonBox.PackStart(installBtn, false, false, 0)
			}
		}

		vbox.PackStart(buttonBox, false, false, 0)
	}

	window.Add(vbox)
	window.ShowAll()
}

// getAppStatus gets the installation status of an app
func (g *GUI) getAppStatus(appName string) string {
	statusFile := filepath.Join(g.directory, "data", "status", appName)
	if data, err := os.ReadFile(statusFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "uninstalled"
}

// getAppDescription gets the description of an app
func (g *GUI) getAppDescription(appName string) string {
	descFile := filepath.Join(g.directory, "apps", appName, "description")
	if data, err := os.ReadFile(descFile); err == nil {
		return string(data)
	}
	return "Description unavailable"
}

// createAppInfoLabel creates additional app info labels
func (g *GUI) createAppInfoLabel(appName string) *gtk.Label {
	var info []string

	// Check for website
	websiteFile := filepath.Join(g.directory, "apps", appName, "website")
	if data, err := os.ReadFile(websiteFile); err == nil {
		website := strings.TrimSpace(string(data))
		info = append(info, fmt.Sprintf("Website: %s", website))
	}

	// Check if it's a package app
	packagesFile := filepath.Join(g.directory, "apps", appName, "packages")
	if _, err := os.Stat(packagesFile); err == nil {
		info = append(info, "This app installs system packages")
	}

	if len(info) > 0 {
		label, err := gtk.LabelNew(strings.Join(info, "\n"))
		if err == nil {
			label.SetHAlign(gtk.ALIGN_START)
			label.SetLineWrap(true)
			return label
		}
	}

	return nil
}

// performAppAction performs install/uninstall actions using terminal_manage
func (g *GUI) performAppAction(appName, action string) {
	fmt.Printf("Performing %s action for app: %s\n", action, appName)

	// Call the API's terminal_manage function like the original bash GUI does
	// This is equivalent to: "${DIRECTORY}/api" terminal_manage "$action" "$app"
	apiScript := filepath.Join(g.directory, "api")
	// Debug logging
	fmt.Println(apiScript, "terminal_manage", action, fmt.Sprintf("'%s'", appName))
	cmd := exec.Command(apiScript, "terminal_manage", action, fmt.Sprintf("'%s'", appName))

	// Set environment variables that might be needed
	cmd.Env = append(os.Environ(),
		"DIRECTORY="+g.directory,
		"GUI_FORMAT_VERSION=2",
	)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting %s for %s: %v\n", action, appName, err)

		// Show error dialog
		dialog := gtk.MessageDialogNew(
			g.window,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_ERROR,
			gtk.BUTTONS_OK,
			"Failed to %s %s: %v",
			action, appName, err,
		)
		defer dialog.Destroy()
		dialog.Run()
	} else {
		fmt.Printf("Started %s process for %s using API terminal_manage\n", action, appName)
	}
}

// GetMessageOfTheDay gets the current message of the day
func (g *GUI) GetMessageOfTheDay() string {
	announcementsFile := filepath.Join(g.directory, "data", "announcements")

	// Check if file exists and is recent
	if stat, err := os.Stat(announcementsFile); err != nil || time.Since(stat.ModTime()) > 24*time.Hour {
		// Download new announcements in background
		go g.downloadAnnouncements()
	}

	// Read existing announcements
	if data, err := os.ReadFile(announcementsFile); err == nil {
		lines := strings.Split(string(data), "\n")

		// Collect non-empty lines
		var validLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				validLines = append(validLines, line)
			}
		}

		// Pick a random line (matching bash `shuf -n 1` behavior)
		if len(validLines) > 0 {
			// Use current time as seed for randomness
			randomIndex := int(time.Now().UnixNano()) % len(validLines)
			return validLines[randomIndex]
		}
	}

	return "Welcome to Pi-Apps!"
}

// downloadAnnouncements downloads the latest announcements
func (g *GUI) downloadAnnouncements() {
	cmd := exec.Command("wget", "-qO-", "https://raw.githubusercontent.com/pi-apps-go/pi-apps-announcements/main/message")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	announcementsFile := filepath.Join(g.directory, "data", "announcements")
	os.WriteFile(announcementsFile, output, 0644)
}

// runYADMode runs the GUI in YAD compatibility mode
func (g *GUI) runYADMode() error {
	// For YAD mode, we can either:
	// 1. Call the original YAD-based GUI
	// 2. Implement YAD command generation in Go
	// For now, let's fall back to calling the bash script

	fmt.Println("YAD mode not yet fully implemented in Go, falling back to bash")
	cmd := exec.Command(filepath.Join(g.directory, "gui"))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runXlunchMode runs the GUI in Xlunch compatibility mode
func (g *GUI) runXlunchMode() error {
	// Similar to YAD mode, fall back for now
	fmt.Println("Xlunch mode not yet fully implemented in Go, falling back to bash")
	cmd := exec.Command(filepath.Join(g.directory, "gui"))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

// ShowMessageDialog shows a simple message dialog
func ShowMessageDialog(title, message string, dialogType int) {
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
	var msgType gtk.MessageType
	switch dialogType {
	case 1:
		msgType = gtk.MESSAGE_INFO
	case 2:
		msgType = gtk.MESSAGE_WARNING
	case 3:
		msgType = gtk.MESSAGE_ERROR
	default:
		msgType = gtk.MESSAGE_INFO
	}

	dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, msgType, gtk.BUTTONS_OK, message)
	if dialog == nil {
		fmt.Fprintf(os.Stderr, "Error creating dialog\n")
		return
	}
	dialog.SetTitle(title)

	// Set dialog properties
	dialog.SetDefaultSize(400, 150)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set dialog icon
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir != "" {
		iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
		if err := dialog.SetIconFromFile(iconPath); err == nil {
			// Icon set successfully
		}
	}

	// Show and run dialog
	dialog.ShowAll()
	dialog.Run()
	dialog.Destroy()
}

// createBottomButtons creates the bottom button bar with search and settings
func (g *GUI) createBottomButtons(parent *gtk.Box) error {
	// Create a horizontal box for buttons at the bottom with separators
	buttonArea, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return err
	}

	// Add top border line
	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err == nil {
		parent.PackStart(separator, false, false, 0)
	}

	// Search button - simpler style like original
	searchBtn, err := gtk.ButtonNewWithLabel("Search")
	if err != nil {
		return err
	}
	searchBtn.SetHExpand(true)
	searchBtn.SetSizeRequest(-1, 35) // Only set height, width will expand

	// Add search icon to button
	searchIcon := filepath.Join(g.directory, "icons", "search.png")
	if img, err := gtk.ImageNewFromFile(searchIcon); err == nil {
		img.SetPixelSize(16)
		searchBtn.SetImage(img)
		searchBtn.SetAlwaysShowImage(true)
	}

	searchBtn.Connect("clicked", g.onSearchClicked)

	// Add vertical separator between buttons
	vertSep, err := gtk.SeparatorNew(gtk.ORIENTATION_VERTICAL)
	if err != nil {
		return err
	}

	// Settings button - simpler style like original
	settingsBtn, err := gtk.ButtonNewWithLabel("Settings")
	if err != nil {
		return err
	}
	settingsBtn.SetHExpand(true)
	settingsBtn.SetSizeRequest(-1, 35) // Only set height, width will expand

	// Add settings icon to button
	settingsIcon := filepath.Join(g.directory, "icons", "options.png")
	if img, err := gtk.ImageNewFromFile(settingsIcon); err == nil {
		img.SetPixelSize(16)
		settingsBtn.SetImage(img)
		settingsBtn.SetAlwaysShowImage(true)
	}

	settingsBtn.Connect("clicked", g.onSettingsClicked)

	// Pack buttons with separator
	buttonArea.PackStart(searchBtn, true, true, 0)
	buttonArea.PackStart(vertSep, false, false, 0)
	buttonArea.PackStart(settingsBtn, true, true, 0)

	// Add button area to parent
	parent.PackStart(buttonArea, false, false, 0)

	return nil
}

// populateAppsInCategory populates the app list for a specific category
func (g *GUI) populateAppsInCategory(listBox *gtk.ListBox, category string) {
	// Use the preload system to get apps for this category
	appList, err := PreloadAppList(g.directory, category)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to preload apps for category %s: %v\n", category, err))
		g.addPlaceholderRow(listBox, fmt.Sprintf("Failed to load apps: %v", err))
		return
	}

	// Filter out categories and back buttons - we only want actual apps
	var apps []AppListItem
	for _, app := range appList.Items {
		if app.Type == "app" {
			apps = append(apps, app)
		}
	}

	if len(apps) == 0 {
		g.addPlaceholderRow(listBox, fmt.Sprintf("No apps found in %s category", category))
		return
	}

	// Store the current apps for index-based access
	g.currentApps = apps
	logger.Info(fmt.Sprintf("Stored %d apps for category %s\n", len(g.currentApps), category))

	// Add each app as a row
	for _, app := range apps {
		row, err := g.createAppRow(app)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to create row for app %s: %v\n", app.Name, err))
			continue
		}

		listBox.Add(row)
	}
}

// addPlaceholderRow adds a placeholder message row
func (g *GUI) addPlaceholderRow(listBox *gtk.ListBox, message string) {
	row, err := gtk.ListBoxRowNew()
	if err == nil {
		label, err := gtk.LabelNew(message)
		if err == nil {
			label.SetJustify(gtk.JUSTIFY_CENTER)
			label.SetVAlign(gtk.ALIGN_CENTER)
			label.SetHAlign(gtk.ALIGN_CENTER)
			label.SetMarginTop(50)
			label.SetMarginBottom(50)
			row.Add(label)
			listBox.Add(row)
		}
	}
}

// createAppRow creates a row for an individual app
func (g *GUI) createAppRow(app AppListItem) (*gtk.ListBoxRow, error) {
	row, err := gtk.ListBoxRowNew()
	if err != nil {
		return nil, err
	}

	// Create horizontal box for row content
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return nil, err
	}
	hbox.SetMarginTop(6)
	hbox.SetMarginBottom(6)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)

	// Add app icon
	iconPath := app.IconPath
	if iconPath == "" || iconPath == "none-24.png" {
		iconPath = filepath.Join(g.directory, "icons", "none-24.png")
	}

	// Load and scale the app icon
	if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
		if scaledPixbuf, err := pixbuf.ScaleSimple(24, 24, gdk.INTERP_BILINEAR); err == nil {
			if image, err := gtk.ImageNewFromPixbuf(scaledPixbuf); err == nil {
				hbox.PackStart(image, false, false, 0)
			}
		}
	}

	// Create vertical box for app name and description
	vboxText, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err == nil {
		// App name with status color
		nameLabel, err := gtk.LabelNew("")
		if err == nil {
			// Set text color based on status
			var color string
			switch app.Status {
			case "installed":
				color = "#00AA00" // Green
			case "uninstalled":
				color = "#CC3333" // Red
			case "corrupted":
				color = "#888800" // Yellow
			case "disabled":
				color = "#FF0000" // Bright red
			default:
				color = "#FFFFFF" // Default white
			}

			nameText := app.Name
			if app.Status != "" && app.Status != "uninstalled" {
				nameText = fmt.Sprintf("%s (%s)", app.Name, app.Status)
			}

			nameLabel.SetMarkup(fmt.Sprintf("<span foreground='%s'>%s</span>", color, nameText))
			nameLabel.SetHAlign(gtk.ALIGN_START)
			vboxText.PackStart(nameLabel, false, false, 0)

			// App description (if available)
			if app.Description != "" {
				descLabel, err := gtk.LabelNew(app.Description)
				if err == nil {
					descLabel.SetHAlign(gtk.ALIGN_START)
					descLabel.SetEllipsize(3) // PANGO_ELLIPSIZE_END
					descLabel.SetMaxWidthChars(50)

					// Make description text smaller and gray - NO status prefix
					descLabel.SetMarkup(fmt.Sprintf("<span size='small' foreground='#AAAAAA'>%s</span>", app.Description))
					vboxText.PackStart(descLabel, false, false, 0)
				}
			}

			hbox.PackStart(vboxText, true, true, 0)
		}
	}

	row.Add(hbox)
	return row, nil
}

// getAppNameFromRow retrieves the app name from a row using index
func (g *GUI) getAppNameFromRow(row *gtk.ListBoxRow) string {
	if g.currentApps != nil {
		rowIndex := row.GetIndex()
		if rowIndex >= 0 && rowIndex < len(g.currentApps) {
			return g.currentApps[rowIndex].Name
		}
	}
	return ""
}

// getAppWebsite gets the website URL for an app
func (g *GUI) getAppWebsite(appName string) string {
	websiteFile := filepath.Join(g.directory, "apps", appName, "website")
	if data, err := os.ReadFile(websiteFile); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// hasInstallScript checks if an app has install scripts
func (g *GUI) hasInstallScript(appName string) bool {
	installScript := filepath.Join(g.directory, "apps", appName, "install")
	install32Script := filepath.Join(g.directory, "apps", appName, "install-32")
	install64Script := filepath.Join(g.directory, "apps", appName, "install-64")
	uninstallScript := filepath.Join(g.directory, "apps", appName, "uninstall")
	packagesFile := filepath.Join(g.directory, "apps", appName, "packages")

	_, installExists := os.Stat(installScript)
	_, install32Exists := os.Stat(install32Script)
	_, install64Exists := os.Stat(install64Script)
	_, uninstallExists := os.Stat(uninstallScript)
	_, packagesExists := os.Stat(packagesFile)

	return installExists == nil || install32Exists == nil || install64Exists == nil || uninstallExists == nil || packagesExists == nil
}

// getSubcategories retrieves the subcategories for a given category
func (g *GUI) getSubcategories(category string) []string {
	// Map of categories to their subcategories
	subcategoryMap := map[string][]string{
		"Internet": {"Browsers", "Communication"},
		"Tools":    {"Crypto", "Emulation"},
	}

	if subcategories, exists := subcategoryMap[category]; exists {
		return subcategories
	}

	return []string{}
}

// populateSubcategories populates the subcategory list
func (g *GUI) populateSubcategories(listBox *gtk.ListBox, category string, subcategories []string) {
	logger.Info(fmt.Sprintf("Populating subcategories for %s: %v\n", category, subcategories))

	// Descriptions for subcategories
	subcategoryDescriptions := map[string]string{
		"Browsers":      "Web browsers for browsing the internet",
		"Communication": "Chat clients, email clients, and communication tools",
		"Crypto":        "Cryptocurrency tools and blockchain applications",
		"Emulation":     "Emulators for running non-native software",
	}

	for _, subcategory := range subcategories {
		row, err := g.createSubcategoryRow(subcategory, subcategoryDescriptions[subcategory])
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to create row for subcategory %s: %v\n", subcategory, err))
			continue
		}

		listBox.Add(row)
	}

	if len(subcategories) == 0 {
		g.addPlaceholderRow(listBox, fmt.Sprintf("No subcategories found in %s", category))
	}

	logger.Info(fmt.Sprintf("Total subcategory rows in map: %d\n", len(g.currentApps)))
}

// showSubcategoryAppsView displays apps for a specific subcategory
func (g *GUI) showSubcategoryAppsView(category, subcategory string) {
	logger.Info(fmt.Sprintf("Showing subcategory: %s → %s\n", category, subcategory))

	// Clear existing content first
	g.clearContentContainer()

	// Force garbage collection
	g.widgetCount++
	if g.widgetCount%10 == 0 {
		logger.Info(fmt.Sprintf("Triggering garbage collection after %d widget operations\n", g.widgetCount))
		runtime.GC()
		runtime.GC()
	}

	// Create header with back button and subcategory name
	headerBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating header box: %v\n", err))
		return
	}
	headerBox.SetMarginTop(8)
	headerBox.SetMarginBottom(8)
	headerBox.SetMarginStart(8)
	headerBox.SetMarginEnd(8)

	// Back button - goes back to the parent category
	backBtn, err := gtk.ButtonNewWithLabel("← Back")
	if err == nil {
		backBtn.Connect("clicked", func() {
			// Go back to the parent category (which will show subcategories)
			logger.Info(fmt.Sprintf("Going back from %s to %s\n", subcategory, category))
			g.showCategoryAppsView(category)
		})
		headerBox.PackStart(backBtn, false, false, 0)
	}

	// Subcategory title
	subcategoryLabel, err := gtk.LabelNew("")
	if err == nil {
		subcategoryLabel.SetMarkup(fmt.Sprintf("<b>%s → %s</b>", category, subcategory))
		subcategoryLabel.SetHAlign(gtk.ALIGN_START)
		headerBox.PackStart(subcategoryLabel, true, true, 0)
	}

	g.contentContainer.PackStart(headerBox, false, false, 0)

	// Add separator
	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err == nil {
		g.contentContainer.PackStart(separator, false, false, 0)
	}

	// Create scrolled window for app list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating scrolled window: %v\n", err))
		return
	}
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)

	// Create list box for apps
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating list box: %v\n", err))
		return
	}
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)

	// Load apps from the subcategory using preload system
	subcategoryPath := fmt.Sprintf("%s/%s", category, subcategory)
	logger.Info(fmt.Sprintf("Loading apps for subcategory path: %s\n", subcategoryPath))

	// Try to populate apps in subcategory
	g.populateAppsInCategory(listBox, subcategoryPath)

	// Check if any apps were added
	children := listBox.GetChildren()
	numApps := children.Length()
	logger.Info(fmt.Sprintf("Found %d items in subcategory %s\n", numApps, subcategoryPath))

	if numApps == 0 {
		// Add a message if no apps found
		g.addPlaceholderRow(listBox, fmt.Sprintf("No apps found in %s → %s", category, subcategory))
		logger.Info(fmt.Sprintf("No apps found in subcategory %s, added placeholder\n", subcategoryPath))
	}

	// Connect app selection handler
	listBox.Connect("row-activated", func(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
		logger.Info(fmt.Sprintf("App row activated in subcategory: %s/%s\n", category, subcategory))
		rowIndex := row.GetIndex()
		logger.Info(fmt.Sprintf("Selected row index: %d\n", rowIndex))

		if appName := g.getAppNameFromRow(row); appName != "" {
			appPath := fmt.Sprintf("%s/%s", subcategoryPath, appName)
			logger.Info(fmt.Sprintf("Opening app details for: %s\n", appPath))
			g.showAppDetails(appPath)
		} else {
			logger.Error(fmt.Sprintf("Could not get app name from row index %d (total apps: %d)\n", rowIndex, len(g.currentApps)))
		}
	})

	scrolled.Add(listBox)
	g.contentContainer.PackStart(scrolled, true, true, 0)

	// Show the new content
	g.contentContainer.ShowAll()
}

// createSubcategoryRow creates a row for a subcategory
func (g *GUI) createSubcategoryRow(subcategory, description string) (*gtk.ListBoxRow, error) {
	row, err := gtk.ListBoxRowNew()
	if err != nil {
		return nil, err
	}

	// Create horizontal box for row content
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return nil, err
	}
	hbox.SetMarginTop(4)
	hbox.SetMarginBottom(4)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)

	// Add subcategory icon - use specific icons for each subcategory
	var iconFile string
	switch subcategory {
	case "Browsers":
		iconFile = "Browsers.png"
	case "Communication":
		iconFile = "Communication.png"
	case "Crypto":
		iconFile = "Crypto.png"
	case "Emulation":
		iconFile = "Emulation.png"
	default:
		iconFile = "default.png"
	}

	iconPath := filepath.Join(g.directory, "icons", "categories", iconFile)
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		// Fallback to a generic folder icon or none icon
		iconPath = filepath.Join(g.directory, "icons", "none-24.png")
	}

	image, err := gtk.ImageNewFromFile(iconPath)
	if err == nil {
		image.SetPixelSize(24)
		hbox.PackStart(image, false, false, 0)
	}

	// Create vertical box for name and description
	vboxText, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err == nil {
		// Add subcategory name
		label, err := gtk.LabelNew(subcategory)
		if err == nil {
			label.SetHAlign(gtk.ALIGN_START)
			vboxText.PackStart(label, false, false, 0)
		}

		// Add description
		if description != "" {
			descLabel, err := gtk.LabelNew("")
			if err == nil {
				descLabel.SetMarkup(fmt.Sprintf("<span size='small' foreground='#AAAAAA'>%s</span>", description))
				descLabel.SetHAlign(gtk.ALIGN_START)
				descLabel.SetEllipsize(3) // PANGO_ELLIPSIZE_END
				descLabel.SetMaxWidthChars(50)
				vboxText.PackStart(descLabel, false, false, 0)
			}
		}

		hbox.PackStart(vboxText, true, true, 0)
	}

	row.Add(hbox)
	return row, nil
}

// isPackageApp checks if an app is a package app
func (g *GUI) isPackageApp(appName string) bool {
	packagesFile := filepath.Join(g.directory, "apps", appName, "packages")
	_, err := os.Stat(packagesFile)
	return err == nil
}

// openAppScripts opens the app scripts in a text editor
func (g *GUI) openAppScripts(appName string) {
	// Get all possible script paths
	installScript := filepath.Join(g.directory, "apps", appName, "install")
	install32Script := filepath.Join(g.directory, "apps", appName, "install-32")
	install64Script := filepath.Join(g.directory, "apps", appName, "install-64")
	uninstallScript := filepath.Join(g.directory, "apps", appName, "uninstall")

	// Collect all scripts that exist, in order of importance
	var scriptsToOpen []string

	// Always try to open uninstall script first if it exists
	if _, err := os.Stat(uninstallScript); err == nil {
		scriptsToOpen = append(scriptsToOpen, uninstallScript)
	}

	// Then open install scripts in order: install-32, install-64, install
	if _, err := os.Stat(install32Script); err == nil {
		scriptsToOpen = append(scriptsToOpen, install32Script)
	}
	if _, err := os.Stat(install64Script); err == nil {
		scriptsToOpen = append(scriptsToOpen, install64Script)
	}
	if _, err := os.Stat(installScript); err == nil {
		scriptsToOpen = append(scriptsToOpen, installScript)
	}

	if len(scriptsToOpen) == 0 {
		// Show message if no scripts found
		dialog := gtk.MessageDialogNew(
			g.detailsWindow,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_INFO,
			gtk.BUTTONS_OK,
			"No install or uninstall scripts found for %s",
			appName,
		)
		defer dialog.Destroy()
		dialog.Run()
		return
	}

	// Open scripts in preferred text editor (like original bash GUI)
	go func() {
		for i, scriptPath := range scriptsToOpen {
			// GTK is weird, we need to force garbage collection before opening the file to avoid an immediate segfault because of goroutines
			runtime.GC()
			runtime.GC()

			if i > 0 {
				// Add delay between opening multiple scripts like original
				time.Sleep(100 * time.Millisecond)
			}

			api.OpenFile(scriptPath)
		}
	}()

	// Log which scripts are being opened
	logger.Info(fmt.Sprintf("Opening %d script(s) for %s: %v", len(scriptsToOpen), appName, scriptsToOpen))
}

// enableApp enables a disabled app by removing its status file
func (g *GUI) enableApp(appName string) {
	statusFile := filepath.Join(g.directory, "data", "status", appName)
	if err := os.Remove(statusFile); err != nil {
		logger.Error(fmt.Sprintf("Failed to enable app %s: %v\n", appName, err))
	} else {
		logger.Info(fmt.Sprintf("Enabled app: %s\n", appName))
	}
}

// viewAppErrors shows the error log for a failed app
func (g *GUI) viewAppErrors(appName string) {
	// Find the most recent error log for this app (matching original bash logic)
	logsDir := filepath.Join(g.directory, "logs")
	if entries, err := os.ReadDir(logsDir); err == nil {
		var latestLog string
		var latestTime time.Time

		for _, entry := range entries {
			if strings.Contains(entry.Name(), "fail-"+appName) {
				if info, err := entry.Info(); err == nil {
					if info.ModTime().After(latestTime) {
						latestTime = info.ModTime()
						latestLog = filepath.Join(logsDir, entry.Name())
					}
				}
			}
		}

		if latestLog != "" {
			// Open log file with logviewer command like original
			cmd := exec.Command(filepath.Join(g.directory, "api-go"), "logviewer", latestLog)
			if err := cmd.Start(); err != nil {
				logger.Error(fmt.Sprintf("Failed to open log viewer: %v\n", err))
				// Fallback: open in text editor
				fallbackCmd := exec.Command("xdg-open", latestLog)
				fallbackCmd.Start()
			}
			logger.Info(fmt.Sprintf("Viewing error log for %s: %s\n", appName, latestLog))
		} else {
			// Show message if no log found
			dialog := gtk.MessageDialogNew(
				g.detailsWindow,
				gtk.DIALOG_MODAL,
				gtk.MESSAGE_INFO,
				gtk.BUTTONS_OK,
				"No error log found for %s",
				appName,
			)
			defer dialog.Destroy()
			dialog.Run()
		}
	}
}

// showSearchResults displays search results in the content container
func (g *GUI) showSearchResults(query string, results []string) {
	// Clear existing content first
	g.clearContentContainer()

	// Force garbage collection
	g.widgetCount++
	if g.widgetCount%10 == 0 {
		logger.Info(fmt.Sprintf("Triggering garbage collection after %d widget operations", g.widgetCount))
		runtime.GC()
		runtime.GC()
	}

	// Create header with back button and search title
	headerBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating header box: %v", err))
		return
	}
	headerBox.SetMarginTop(8)
	headerBox.SetMarginBottom(8)
	headerBox.SetMarginStart(8)
	headerBox.SetMarginEnd(8)

	// Back button
	backBtn, err := gtk.ButtonNewWithLabel("← Back")
	if err == nil {
		backBtn.Connect("clicked", func() {
			g.currentPrefix = ""
			g.showCategoryListView() // Return to main category list
		})
		headerBox.PackStart(backBtn, false, false, 0)
	}

	// Search results title
	searchLabel, err := gtk.LabelNew("")
	if err == nil {
		searchLabel.SetMarkup(fmt.Sprintf("<b>Search Results for \"%s\"</b> (%d apps found)", query, len(results)))
		searchLabel.SetHAlign(gtk.ALIGN_START)
		headerBox.PackStart(searchLabel, true, true, 0)
	}

	g.contentContainer.PackStart(headerBox, false, false, 0)

	// Add separator
	separator, err := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	if err == nil {
		g.contentContainer.PackStart(separator, false, false, 0)
	}

	// Create scrolled window for the search results
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating scrolled window: %v", err))
		return
	}
	scrolled.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)

	// Create list box for results
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating list box: %v", err))
		return
	}
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)

	// Convert search results to AppListItem format and populate the list
	g.populateSearchResults(listBox, results)

	// Connect app selection handler
	listBox.Connect("row-activated", func(listBox *gtk.ListBox, row *gtk.ListBoxRow) {
		logger.Info(fmt.Sprintf("Search result app row activated"))
		rowIndex := row.GetIndex()
		logger.Info(fmt.Sprintf("Selected row index: %d", rowIndex))

		if appName := g.getAppNameFromRow(row); appName != "" {
			logger.Info(fmt.Sprintf("Opening app details for search result: %s", appName))
			g.showAppDetails(appName)
		} else {
			logger.Error(fmt.Sprintf("Could not get app name from row index %d (total apps: %d)", rowIndex, len(g.currentApps)))
		}
	})

	scrolled.Add(listBox)
	g.contentContainer.PackStart(scrolled, true, true, 0)

	// Show the new content
	g.contentContainer.ShowAll()
}

// populateSearchResults populates the search results list
func (g *GUI) populateSearchResults(listBox *gtk.ListBox, results []string) {
	// Get category data for showing which category each app belongs to
	categoryEntries, err := api.ReadCategoryFiles(g.directory)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to read category files: %v", err))
	}

	// Convert search results to AppListItem format
	var searchApps []AppListItem
	for _, appName := range results {
		// Filter out hidden apps - check if app belongs to "hidden" category
		isHidden := false
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			if len(parts) >= 2 && parts[0] == appName {
				if parts[1] == "hidden" {
					isHidden = true
					break
				}
				break
			}
		}

		// Skip hidden apps - they should not appear in search results
		if isHidden {
			logger.Info(fmt.Sprintf("Skipping hidden app from search results: %s", appName))
			continue
		}

		// Get app status
		status, err := api.GetAppStatus(appName)
		if err != nil {
			status = "uninstalled"
		}

		// Get app description (first line only, like the original)
		descFile := filepath.Join(g.directory, "apps", appName, "description")
		description := "Description unavailable"
		if descData, err := os.ReadFile(descFile); err == nil {
			lines := strings.Split(string(descData), "\n")
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				description = strings.TrimSpace(lines[0])
			}
		}

		// Get app icon
		iconPath := filepath.Join(g.directory, "apps", appName, "icon-24.png")
		if !appListFileExists(iconPath) {
			iconPath = filepath.Join(g.directory, "icons", "none-24.png")
		}

		// Create AppListItem
		appItem := AppListItem{
			Type:        "app",
			Name:        appName,
			Path:        appName, // No category prefix for search results
			Description: description,
			IconPath:    iconPath,
			Status:      status,
		}

		searchApps = append(searchApps, appItem)
	}

	// Store the search results for index-based access
	g.currentApps = searchApps
	logger.Info(fmt.Sprintf("Stored %d search result apps (hidden apps filtered out)", len(g.currentApps)))

	// Add each app as a row with category information
	for _, app := range searchApps {
		row, err := g.createSearchResultRow(app, app.Name, categoryEntries)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to create row for search result %s: %v", app.Name, err))
			continue
		}

		listBox.Add(row)
	}

	if len(searchApps) == 0 {
		g.addPlaceholderRow(listBox, "No compatible apps found for your search")
	}
}

// createSearchResultRow creates a row for search results with category information
func (g *GUI) createSearchResultRow(app AppListItem, appName string, categoryEntries []string) (*gtk.ListBoxRow, error) {
	row, err := gtk.ListBoxRowNew()
	if err != nil {
		return nil, err
	}

	// Create horizontal box for row content
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		return nil, err
	}
	hbox.SetMarginTop(6)
	hbox.SetMarginBottom(6)
	hbox.SetMarginStart(8)
	hbox.SetMarginEnd(8)

	// Add app icon
	iconPath := app.IconPath
	if iconPath == "" || iconPath == "none-24.png" {
		iconPath = filepath.Join(g.directory, "icons", "none-24.png")
	}

	// Load and scale the app icon
	if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
		if scaledPixbuf, err := pixbuf.ScaleSimple(24, 24, gdk.INTERP_BILINEAR); err == nil {
			if image, err := gtk.ImageNewFromPixbuf(scaledPixbuf); err == nil {
				hbox.PackStart(image, false, false, 0)
			}
		}
	}

	// Create vertical box for app name and description
	vboxText, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err == nil {
		// App name with status color
		nameLabel, err := gtk.LabelNew("")
		if err == nil {
			// Set text color based on status
			var color string
			switch app.Status {
			case "installed":
				color = "#00AA00" // Green
			case "uninstalled":
				color = "#CC3333" // Red
			case "corrupted":
				color = "#888800" // Yellow
			case "disabled":
				color = "#FF0000" // Bright red
			default:
				color = "#FFFFFF" // Default white
			}

			nameText := app.Name
			if app.Status != "" && app.Status != "uninstalled" {
				nameText = fmt.Sprintf("%s (%s)", app.Name, app.Status)
			}

			nameLabel.SetMarkup(fmt.Sprintf("<span foreground='%s'>%s</span>", color, nameText))
			nameLabel.SetHAlign(gtk.ALIGN_START)
			vboxText.PackStart(nameLabel, false, false, 0)

			// App description (if available) - show below the name like in category views
			if app.Description != "" && app.Description != "Description unavailable" {
				descLabel, err := gtk.LabelNew("")
				if err == nil {
					descLabel.SetMarkup(fmt.Sprintf("<span size='small' foreground='#AAAAAA'>%s</span>", app.Description))
					descLabel.SetHAlign(gtk.ALIGN_START)
					descLabel.SetEllipsize(3) // PANGO_ELLIPSIZE_END
					descLabel.SetMaxWidthChars(50)
					vboxText.PackStart(descLabel, false, false, 0)
				}
			}

			hbox.PackStart(vboxText, true, true, 0)
		}
	}

	// Add spacer
	spacer, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err == nil {
		hbox.PackStart(spacer, false, false, 0)
	}

	// Find and show category information (like the original search results)
	appCategory := "Uncategorised"
	for _, entry := range categoryEntries {
		parts := strings.Split(entry, "|")
		if len(parts) >= 2 && parts[0] == appName {
			// Check if the category is not empty
			if parts[1] != "" {
				appCategory = parts[1]
			}
			// If parts[1] is empty, keep the default "Uncategorised"
			break
		}
	}

	// Add "in" label
	inLabel, err := gtk.LabelNew("in")
	if err == nil {
		inLabel.SetSizeRequest(20, -1)
		hbox.PackStart(inLabel, false, false, 0)
	}

	// Add category icon
	categoryIconPath := filepath.Join(g.directory, "icons", "categories", appCategory+".png")
	if !appListFileExists(categoryIconPath) {
		if appCategory == "Uncategorised" {
			// For uncategorised apps, use a generic folder icon
			categoryIconPath = filepath.Join(g.directory, "icons", "categories", "default.png")
		} else {
			categoryIconPath = filepath.Join(g.directory, "icons", "categories", "default.png")
		}
	}

	if pixbuf, err := gdk.PixbufNewFromFile(categoryIconPath); err == nil {
		if scaledPixbuf, err := pixbuf.ScaleSimple(16, 16, gdk.INTERP_BILINEAR); err == nil {
			if categoryIcon, err := gtk.ImageNewFromPixbuf(scaledPixbuf); err == nil {
				hbox.PackStart(categoryIcon, false, false, 0)
			}
		}
	}

	// Add category name - ensure it's always visible
	categoryLabel, err := gtk.LabelNew(appCategory)
	if err == nil {
		categoryLabel.SetHAlign(gtk.ALIGN_START)
		// Make sure the category label is visible and not empty
		if appCategory == "" {
			categoryLabel.SetText("Uncategorised")
		}
		hbox.PackStart(categoryLabel, false, false, 0)
	}

	row.Add(hbox)
	return row, nil
}

// performAdvancedSearch performs app search with custom search files
func (g *GUI) performAdvancedSearch(query string, searchFiles []string) {
	logger.Info(fmt.Sprintf("Performing advanced search for: %s in files: %v", query, searchFiles))

	// Use the API's AppSearch function to get search results
	results, err := api.AppSearch(query, searchFiles...)
	if err != nil {
		logger.Error(fmt.Sprintf("Error performing search: %v", err))
		dialog := gtk.MessageDialogNew(
			g.window,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_ERROR,
			gtk.BUTTONS_OK,
			"Error performing search: %v",
			err,
		)
		defer dialog.Destroy()
		dialog.Run()
		return
	}

	// Save the search query for next time (like the original bash version)
	lastSearchFile := filepath.Join(g.directory, "data", "last-search")
	os.MkdirAll(filepath.Dir(lastSearchFile), 0755)
	os.WriteFile(lastSearchFile, []byte(query), 0644)

	// Handle search results
	if len(results) == 0 {
		// No results found
		dialog := gtk.MessageDialogNew(
			g.window,
			gtk.DIALOG_MODAL,
			gtk.MESSAGE_INFO,
			gtk.BUTTONS_OK,
			"No results found for \"%s\".",
			query,
		)
		defer dialog.Destroy()
		dialog.Run()
		return
	}

	if len(results) == 1 {
		// Single result - show app details directly
		logger.Info(fmt.Sprintf("Single search result: %s", results[0]))
		g.showAppDetails(results[0])
		return
	}

	// Multiple results - show search results view
	logger.Info(fmt.Sprintf("Multiple search results: %d apps found", len(results)))
	g.showSearchResults(query, results)
}
