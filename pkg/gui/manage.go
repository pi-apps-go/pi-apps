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

// Module: manage.go
// Description: Provides functions for managing apps on Pi-Apps Go via the command line.

package gui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/toqueteos/webbrowser"
)

// QueueItem represents an item in the installation/uninstallation queue
type QueueItem struct {
	Action         string // install, uninstall, update, refresh
	AppName        string
	Status         string // waiting, in-progress, success, failure
	IconPath       string
	ErrorMessage   string // Error message if the operation failed
	ForceReinstall bool
}

// StatusIconMapping maps status to icon paths
var StatusIconMapping = map[string]string{
	"waiting":         "icons/wait.png",
	"in-progress":     "icons/prompt.png",
	"success":         "icons/success.png",
	"failure":         "icons/failure.png",
	"diagnosed":       "icons/failure.png", // Use failure icon for diagnosed items
	"daemon-complete": "icons/success.png", // Use success icon for daemon completion
}

// ActionIconMapping maps actions to icon paths
var ActionIconMapping = map[string]string{
	"install":     "icons/install.png",
	"uninstall":   "icons/uninstall.png",
	"update":      "icons/update.png",
	"refresh":     "icons/refresh.png",
	"update-file": "icons/update.png",
	"daemon":      "icons/none-24.png", // Special daemon completion marker
}

// GTK initialization state
var (
	gtkInitialized bool
	gtkInitMutex   sync.Mutex
	inMainLoop     bool // Tracks if we're inside a GTK main loop
)

// ensureGTKInitialized safely initializes GTK if it's not already initialized
func ensureGTKInitialized() bool {
	gtkInitMutex.Lock()
	defer gtkInitMutex.Unlock()

	if !gtkInitialized {
		// Check if we can use GTK
		if !canUseGTK() {
			return false
		}

		// Initialize application name
		glib.SetPrgname("Pi-Apps")
		glib.SetApplicationName("Pi-Apps (user dialog for managing apps)")
		// Initialize GTK
		gtk.Init(nil)
		gtkInitialized = true
	}
	return true
}

// runGtkDialog runs a GTK dialog and returns the response
// This handles the GTK main loop appropriately
func runGtkDialog(dialog *gtk.Dialog) (gtk.ResponseType, error) {
	if !ensureGTKInitialized() {
		return gtk.RESPONSE_CANCEL, fmt.Errorf("GTK not initialized")
	}

	// If we're already in a main loop, just run the dialog
	if inMainLoop {
		return dialog.Run(), nil
	}

	// Set a flag to indicate we're running a main loop
	inMainLoop = true
	defer func() { inMainLoop = false }()

	// Make the dialog modal but don't block
	responseChannel := make(chan gtk.ResponseType, 1)

	// Connect to the response signal
	dialog.Connect("response", func(_ *gtk.Dialog, responseId gtk.ResponseType) {
		responseChannel <- responseId
		// Hide the dialog but don't destroy it yet
		dialog.Hide()
		// Quit the main loop
		glib.IdleAdd(gtk.MainQuit)
	})

	// Show the dialog
	dialog.ShowAll()

	// Run the main loop
	gtk.Main()

	// Get the response
	response := <-responseChannel
	return response, nil
}

// ValidateAppsGUI validates a list of apps and shows appropriate dialogs for invalid apps
// or asks for confirmation for certain operations.
// Returns the validated queue of operations.
func ValidateAppsGUI(queue []QueueItem) ([]QueueItem, error) {
	// If we can't use GTK, return the queue as is with a message
	if !canUseGTK() {
		fmt.Println("Cannot use GUI for validation. Continuing with operations...")
		return queue, nil
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		fmt.Println("Cannot use GUI for validation. Continuing with operations...")
		return queue, nil
	}

	// Get Pi-Apps directory
	piAppsDir := getPiAppsDir()
	if piAppsDir == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	var validatedQueue []QueueItem

	// Validate actions and apps
	for _, item := range queue {
		// Check if action is valid
		if item.Action != "install" && item.Action != "uninstall" &&
			item.Action != "update" && item.Action != "refresh" &&
			item.Action != "update-file" {
			showErrorDialog(fmt.Sprintf("Invalid action: <b>%s</b>", item.Action))
			continue
		}

		// Check if app exists
		if item.Action == "update-file" {
			// Skip app validation for update-file action
			validatedQueue = append(validatedQueue, item)
			continue
		}

		var appDirPath string
		if item.Action == "update" || item.Action == "refresh" {
			// For updates, check in the update directory
			appDirPath = filepath.Join(piAppsDir, "update", "pi-apps", "apps", item.AppName)
		} else {
			// For install/uninstall, check in the apps directory
			appDirPath = filepath.Join(piAppsDir, "apps", item.AppName)
		}

		if _, err := os.Stat(appDirPath); os.IsNotExist(err) {
			showErrorDialog(fmt.Sprintf("Invalid app \"<b>%s</b>\". Cannot %s it.",
				item.AppName, item.Action))
			continue
		}

		// If app is already installed/uninstalled, ask for confirmation
		if isAppInstalled(item.AppName) && item.Action == "install" {
			// Add a pre-validation check to avoid trying to install an already installed app
			// which would result in an error later
			if !showConfirmDialog(fmt.Sprintf("<b>%s</b> is already installed. Are you sure you want to install it again?",
				item.AppName)) {
				// User chose not to reinstall, skip this app
				fmt.Printf("Skipping reinstallation of %s as requested by user.\n", item.AppName)
				continue
			} else {
				// User chose to reinstall, mark it with a special flag
				item.ForceReinstall = true
			}
		} else if !isAppInstalled(item.AppName) && item.Action == "uninstall" {
			if !showConfirmDialog(fmt.Sprintf("<b>%s</b> is already uninstalled. Are you sure you want to uninstall it again?",
				item.AppName)) {
				// User chose not to uninstall, skip this app
				fmt.Printf("Skipping uninstallation of %s as requested by user.\n", item.AppName)
				continue
			} else {
				// User chose to uninstall anyway, mark it with a special flag
				item.ForceReinstall = true
			}
		}

		// Check if update is available (for install action)
		if item.Action == "install" {
			scriptName := getInstallScriptName(item.AppName)
			if scriptName != "" {
				updateScriptPath := filepath.Join(piAppsDir, "update", "pi-apps", "apps", item.AppName, scriptName)
				currentScriptPath := filepath.Join(piAppsDir, "apps", item.AppName, scriptName)

				if fileExists(updateScriptPath) && !filesMatch(updateScriptPath, currentScriptPath) {
					// Ask if user wants to install the newest version
					if showUpdateConfirmDialog(item.AppName, scriptName) {
						// User wants to install newest version, add a refresh action
						refreshItem := QueueItem{
							Action:   "refresh",
							AppName:  item.AppName,
							Status:   "waiting",
							IconPath: getAppIconPath(item.AppName),
						}
						validatedQueue = append(validatedQueue, refreshItem)
						continue
					}
				}
			}
		}

		// If we reached here, the app is valid
		validatedQueue = append(validatedQueue, item)
	}

	return validatedQueue, nil
}

// ProgressMonitor shows a dialog with the current progress of operations
func ProgressMonitor(queue []QueueItem) error {
	return ProgressMonitorWithOptions(queue, false)
}

// ProgressMonitorDaemon shows a progress dialog that doesn't auto-close (for daemon mode)
func ProgressMonitorDaemon(queue []QueueItem) error {
	return ProgressMonitorWithOptions(queue, true)
}

// ProgressMonitorWithOptions shows a dialog with the current progress of operations
func ProgressMonitorWithOptions(queue []QueueItem, daemonMode bool) error {
	// If we can't use GTK, use a simple CLI progress reporter
	if !canUseGTK() {
		return progressMonitorCLI(queue)
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		return progressMonitorCLI(queue)
	}

	// Set a flag to indicate we're running a main loop if we're not already
	if !inMainLoop {
		inMainLoop = true
		defer func() { inMainLoop = false }()
	}

	// Create a new window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	win.SetTitle("Monitor Progress")
	win.SetDefaultSize(480, 400)
	win.SetPosition(gtk.WIN_POS_CENTER)
	win.SetBorderWidth(5) // Reduced border width

	// Set window icon
	icon, err := gdk.PixbufNewFromFile(filepath.Join(getPiAppsDir(), "icons", "logo.png"))
	if err == nil {
		win.SetIcon(icon)
	}

	// Create a box to hold the content
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2) // Even less spacing
	if err != nil {
		return err
	}
	win.Add(box)

	// Create a list store for the queue
	listStore, err := gtk.ListStoreNew(
		glib.TYPE_OBJECT, // Status icon pixbuf
		glib.TYPE_OBJECT, // Action icon pixbuf
		glib.TYPE_STRING, // Action text
		glib.TYPE_OBJECT, // App icon pixbuf
		glib.TYPE_STRING, // App name
	)
	if err != nil {
		return err
	}

	// Create the tree view
	treeView, err := gtk.TreeViewNew()
	if err != nil {
		return err
	}
	treeView.SetModel(listStore)
	treeView.SetHeadersVisible(false)

	// Set row spacing to be compact
	treeView.SetProperty("margin", 0)

	// Add columns for icons and text - make them more compact
	statusRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	statusRenderer.SetProperty("xpad", 1)
	statusRenderer.SetProperty("ypad", 1)

	column, err := gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(0)
	column.PackStart(statusRenderer, false)
	column.AddAttribute(statusRenderer, "pixbuf", 0) // Use pixbuf attribute for pixbufs
	treeView.AppendColumn(column)

	actionRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	actionRenderer.SetProperty("xpad", 1)
	actionRenderer.SetProperty("ypad", 1)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(0)
	column.PackStart(actionRenderer, false)
	column.AddAttribute(actionRenderer, "pixbuf", 1) // Use pixbuf attribute for pixbufs
	treeView.AppendColumn(column)

	textRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}
	textRenderer.SetProperty("xpad", 1)
	textRenderer.SetProperty("ypad", 0)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(0)
	column.PackStart(textRenderer, false)
	column.AddAttribute(textRenderer, "markup", 2) // Use markup attribute for rich text
	treeView.AppendColumn(column)

	appIconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	appIconRenderer.SetProperty("xpad", 1)
	appIconRenderer.SetProperty("ypad", 1)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(0)
	column.PackStart(appIconRenderer, false)
	column.AddAttribute(appIconRenderer, "pixbuf", 3) // Use pixbuf attribute for pixbufs
	treeView.AppendColumn(column)

	appNameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}
	appNameRenderer.SetProperty("xpad", 1)
	appNameRenderer.SetProperty("ypad", 0)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(0)
	column.PackStart(appNameRenderer, true)
	column.AddAttribute(appNameRenderer, "markup", 4) // Use markup attribute for rich text
	treeView.AppendColumn(column)

	// Create a scrolled window for the tree view
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	scrolledWindow.SetHExpand(true)
	scrolledWindow.SetVExpand(true)
	scrolledWindow.Add(treeView)
	scrolledWindow.SetShadowType(gtk.SHADOW_ETCHED_IN) // Add a subtle border
	box.PackStart(scrolledWindow, true, true, 0)

	// Update the list store with queue items
	for _, item := range queue {
		addQueueItemToPixbufListStore(listStore, item)
	}

	// Show all widgets
	win.ShowAll()

	// Variable to track if we should close the window
	shouldClose := false

	// Update function for the tree view
	updateFunc := func() bool {
		// Check if we should close the window
		if shouldClose {
			win.Destroy()
			return false // Stop the timer
		}

		// In daemon mode, try to read updated queue from status file
		currentQueue := queue // Default to original queue
		if daemonMode {
			// Try to read from a well-known status file location
			piAppsDir := getPiAppsDir()
			statusFile := filepath.Join(piAppsDir, "data", "manage-daemon", "status")
			if updatedQueue, err := readQueueFromStatusFile(statusFile); err == nil && len(updatedQueue) > 0 {
				currentQueue = updatedQueue
			}
		}

		// Update list store with current status
		listStore.Clear()
		for _, item := range currentQueue {
			addQueueItemToPixbufListStore(listStore, item)
		}

		// Check if all operations are complete (success or failure)
		allComplete := true
		daemonShouldClose := false
		for _, item := range currentQueue {
			if item.Status == "daemon-complete" {
				daemonShouldClose = true
			}
			if item.Status != "success" && item.Status != "failure" && item.Status != "daemon-complete" && item.Status != "diagnosed" {
				allComplete = false
			}
		}

		// If all operations are complete, close after a short delay (unless in daemon mode)
		// In daemon mode, only close when explicitly signaled
		if (allComplete && !daemonMode) || (daemonMode && daemonShouldClose) {
			// Wait 1 second so user can see the status, then close
			time.Sleep(1 * time.Second)

			// Use glib.IdleAdd to schedule the window destruction on the main thread
			glib.IdleAdd(func() bool {
				win.Destroy()
				return false
			})
			return false // Stop the timer
		}

		return true // Continue the timer
	}

	// Add a timer to update the UI periodically
	_ = glib.TimeoutAdd(1000, updateFunc)

	// Connect signals
	win.Connect("destroy", func() {
		shouldClose = true
		gtk.MainQuit()
	})

	// Start the GTK main loop
	gtk.Main()

	return nil
}

// ShowSummaryDialog shows a summary of completed actions with donation reminders
func ShowSummaryDialog(completedQueue []QueueItem) error {
	// If we can't use GTK, use a simple CLI summary
	if !canUseGTK() {
		return showSummaryDialogCLI(completedQueue)
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		return showSummaryDialogCLI(completedQueue)
	}

	// Set a flag to indicate we're running a main loop if we're not already
	if !inMainLoop {
		inMainLoop = true
		defer func() { inMainLoop = false }()
	}

	// Create a new window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	win.SetTitle("Actions Complete")
	win.SetDefaultSize(520, 350) // Increased height and width to accommodate larger icons
	win.SetPosition(gtk.WIN_POS_CENTER)
	win.SetBorderWidth(10) // Increased border width
	win.SetResizable(true)

	// Set window icon
	icon, err := gdk.PixbufNewFromFile(filepath.Join(getPiAppsDir(), "icons", "logo.png"))
	if err == nil {
		win.SetIcon(icon)
	}

	// Create a box to hold the content
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5) // Increased spacing
	if err != nil {
		return err
	}
	win.Add(box)

	// Add a label at the top
	label, err := gtk.LabelNew("")
	if err != nil {
		return err
	}
	label.SetMarkup("<span size='large'><b>Thank you for using Pi-Apps!</b></span> The following apps completed:")
	label.SetHAlign(gtk.ALIGN_START)
	box.PackStart(label, false, false, 5) // Increased padding

	// Create a list store for the summary
	listStore, err := gtk.ListStoreNew(
		glib.TYPE_OBJECT, // Status icon pixbuf
		glib.TYPE_OBJECT, // Action icon pixbuf
		glib.TYPE_STRING, // Action text
		glib.TYPE_OBJECT, // App icon pixbuf
		glib.TYPE_STRING, // App name
	)
	if err != nil {
		return err
	}

	// Create the tree view
	treeView, err := gtk.TreeViewNew()
	if err != nil {
		return err
	}
	treeView.SetModel(listStore)
	treeView.SetHeadersVisible(false)

	// Set up column sizing
	treeView.SetHExpand(true)
	treeView.SetVExpand(true)
	treeView.SetProperty("margin", 0)

	// Add columns for icons and text (with improved spacing for larger icons)
	// Status icon column
	renderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	renderer.SetProperty("xpad", 2)
	renderer.SetProperty("ypad", 2)

	column, err := gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(2)
	column.PackStart(renderer, false)
	column.AddAttribute(renderer, "pixbuf", 0) // Use pixbuf for icons
	treeView.AppendColumn(column)

	// Action icon column
	renderer, err = gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	renderer.SetProperty("xpad", 2)
	renderer.SetProperty("ypad", 2)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(2)
	column.PackStart(renderer, false)
	column.AddAttribute(renderer, "pixbuf", 1) // Use pixbuf for icons
	treeView.AppendColumn(column)

	// Action text column
	textRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}
	textRenderer.SetProperty("xpad", 5)     // Increased horizontal padding
	textRenderer.SetProperty("ypad", 10)    // Increased vertical padding for rows with larger icons
	textRenderer.SetProperty("xalign", 0.0) // Left-align text

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(5) // Increased spacing
	column.PackStart(textRenderer, false)
	column.AddAttribute(textRenderer, "markup", 2) // Use markup for rich text
	treeView.AppendColumn(column)

	// App icon column (will contain large 64x64 icons for completed items)
	renderer, err = gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}
	renderer.SetProperty("xpad", 5)  // Increased horizontal padding
	renderer.SetProperty("ypad", 10) // Increased vertical padding for larger icons

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(5) // Increased spacing
	column.PackStart(renderer, false)
	column.AddAttribute(renderer, "pixbuf", 3) // Use pixbuf for icons
	column.SetFixedWidth(70)                   // Set a fixed width to accommodate larger icons
	treeView.AppendColumn(column)

	// App name column
	textRenderer, err = gtk.CellRendererTextNew()
	if err != nil {
		return err
	}
	textRenderer.SetProperty("xpad", 5)     // Increased horizontal padding
	textRenderer.SetProperty("ypad", 10)    // Increased vertical padding for rows with larger icons
	textRenderer.SetProperty("xalign", 0.0) // Left-align text
	// Enable word wrapping for sponsor messages
	textRenderer.SetProperty("wrap-mode", 2) // 2 = PANGO_WRAP_WORD_CHAR
	textRenderer.SetProperty("wrap-width", 300)

	column, err = gtk.TreeViewColumnNew()
	if err != nil {
		return err
	}
	column.SetSpacing(5) // Increased spacing
	column.PackStart(textRenderer, true)
	column.AddAttribute(textRenderer, "markup", 4) // Use markup for rich text
	treeView.AppendColumn(column)

	// Add a scrolled window for the tree view
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	scrolledWindow.SetHExpand(true)
	scrolledWindow.SetVExpand(true)
	scrolledWindow.Add(treeView)
	scrolledWindow.SetShadowType(gtk.SHADOW_ETCHED_IN) // Add a subtle border
	box.PackStart(scrolledWindow, true, true, 0)

	// Update the list store with completed queue items
	for _, item := range completedQueue {
		addQueueItemToPixbufListStore(listStore, item)
	}

	// Add donation reminders
	addDonationItemsToPixbufListStore(listStore)

	// Add a close button
	closeButton, err := gtk.ButtonNewWithLabel("Close")
	if err != nil {
		return err
	}
	closeButton.Connect("clicked", func() {
		win.Destroy()
	})
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		return err
	}
	buttonBox.PackEnd(closeButton, false, false, 0)
	box.PackEnd(buttonBox, false, false, 5)

	// Handle double-click on donation links
	treeView.Connect("row-activated", func(tv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		iter, err := listStore.GetIter(path)
		if err != nil {
			return
		}

		value, err := listStore.GetValue(iter, 4) // Get app name (or donation text)
		if err != nil {
			return
		}

		strVal, err := value.GetString()
		if err != nil {
			return
		}

		if strings.Contains(strVal, "Botspot") {
			openURL("https://github.com/sponsors/botspot")
		} else if strings.Contains(strVal, "theofficialgman") {
			openURL("https://github.com/sponsors/theofficialgman")
		}
	})

	// Show all widgets
	win.ShowAll()

	// Connect signals
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Start the GTK main loop
	gtk.Main()

	return nil
}

// ShowBrokenPackagesDialog shows a dialog to enter sudo password for repairing broken package repositories
func ShowBrokenPackagesDialog() (string, error) {
	// If we can't use GTK, use CLI to ask for password
	if !canUseGTK() {
		return showBrokenPackagesDialogCLI()
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		return showBrokenPackagesDialogCLI()
	}

	// Create dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", err
	}
	dialog.SetTitle("Broken Local Packages Repo Detected")
	dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dialog.AddButton("Repair", gtk.RESPONSE_OK)

	// Set dialog properties
	dialog.SetDefaultSize(400, 150)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set dialog icon
	icon, err := gdk.PixbufNewFromFile(filepath.Join(getPiAppsDir(), "icons", "logo.png"))
	if err == nil {
		dialog.SetIcon(icon)
	}

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return "", err
	}
	contentArea.SetSpacing(6)

	// Add message label with markup
	label, err := gtk.LabelNew("")
	if err != nil {
		return "", err
	}
	label.SetMarkup("Please enter your <b>user password</b>\nso pi-apps can attempt a repair:")
	contentArea.Add(label)

	// Add password entry
	passwordEntry, err := gtk.EntryNew()
	if err != nil {
		return "", err
	}
	passwordEntry.SetVisibility(false)
	passwordEntry.SetHExpand(true)
	contentArea.Add(passwordEntry)

	// Use our custom dialog runner
	response, err := runGtkDialog(dialog)
	password, _ := passwordEntry.GetText()
	dialog.Destroy()

	if err != nil {
		return "", err
	}

	if response == gtk.RESPONSE_OK {
		return password, nil
	}
	return "", fmt.Errorf("canceled by user")
}

// Helper functions

// getIconPath ensures icon paths are correctly resolved
func getIconPath(iconName string) string {
	piAppsDir := getPiAppsDir()

	// If iconName is empty, return default icon immediately
	if iconName == "" {
		return filepath.Join(piAppsDir, "icons", "none-24.png")
	}

	// If iconName is already an absolute path, verify it exists and is a file
	if filepath.IsAbs(iconName) {
		if info, err := os.Stat(iconName); err == nil && !info.IsDir() {
			return iconName
		}
		// If absolute path doesn't exist or is a directory, fall back to default icon
		return filepath.Join(piAppsDir, "icons", "none-24.png")
	}

	// Otherwise, construct the path relative to PI_APPS_DIR
	iconPath := filepath.Join(piAppsDir, iconName)

	// Verify the icon exists and is a file (not a directory)
	if info, err := os.Stat(iconPath); err != nil || info.IsDir() {
		// Fall back to a default icon if the requested icon doesn't exist or is a directory
		return filepath.Join(piAppsDir, "icons", "none-24.png")
	}

	return iconPath
}

// addQueueItemToPixbufListStore adds a queue item to the list store using pixbufs instead of file paths
func addQueueItemToPixbufListStore(listStore *gtk.ListStore, item QueueItem) {
	// Target heights for icons
	const targetStatusActionHeight = 22
	const targetAppHeight = 20
	const largeAppIconHeight = 64 // Larger icon size for completed installations/uninstalls

	// --- Status Icon ---
	statusIconName, exists := StatusIconMapping[item.Status]
	if !exists {
		// If status is unknown, default to waiting icon
		statusIconName = StatusIconMapping["waiting"]
		fmt.Printf("Warning: unknown status '%s' for app %s, using waiting icon\n", item.Status, item.AppName)
	}
	statusIconPath := getIconPath(statusIconName)
	statusPixbuf, err := gdk.PixbufNewFromFile(statusIconPath)
	if err != nil {
		fmt.Printf("Error loading status icon %s: %v\n", statusIconPath, err)
		statusPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetStatusActionHeight, targetStatusActionHeight)
	} else if statusPixbuf != nil {
		// Scale based on height, preserving aspect ratio
		origWidth := statusPixbuf.GetWidth()
		origHeight := statusPixbuf.GetHeight()
		if origHeight != targetStatusActionHeight {
			newWidth := int(float64(targetStatusActionHeight) * float64(origWidth) / float64(origHeight))
			if newWidth == 0 {
				newWidth = 1
			} // Prevent zero width

			scaledPixbuf, scaleErr := statusPixbuf.ScaleSimple(newWidth, targetStatusActionHeight, gdk.INTERP_BILINEAR)
			if scaleErr != nil {
				fmt.Printf("Error scaling status icon: %v\n", scaleErr)
				statusPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetStatusActionHeight, targetStatusActionHeight)
			} else {
				statusPixbuf = scaledPixbuf
			}
		}
	}

	// --- Action Icon ---
	actionIconName, exists := ActionIconMapping[item.Action]
	if !exists {
		// If action is unknown, default to install icon
		actionIconName = ActionIconMapping["install"]
		fmt.Printf("Warning: unknown action '%s' for app %s, using install icon\n", item.Action, item.AppName)
	}
	actionIconPath := getIconPath(actionIconName)
	actionPixbuf, err := gdk.PixbufNewFromFile(actionIconPath)
	if err != nil {
		fmt.Printf("Error loading action icon %s: %v\n", actionIconPath, err)
		actionPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetStatusActionHeight, targetStatusActionHeight)
	} else if actionPixbuf != nil {
		// Scale based on height, preserving aspect ratio
		origWidth := actionPixbuf.GetWidth()
		origHeight := actionPixbuf.GetHeight()
		if origHeight != targetStatusActionHeight {
			newWidth := int(float64(targetStatusActionHeight) * float64(origWidth) / float64(origHeight))
			if newWidth == 0 {
				newWidth = 1
			} // Prevent zero width

			scaledPixbuf, scaleErr := actionPixbuf.ScaleSimple(newWidth, targetStatusActionHeight, gdk.INTERP_BILINEAR)
			if scaleErr != nil {
				fmt.Printf("Error scaling action icon: %v\n", scaleErr)
				actionPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetStatusActionHeight, targetStatusActionHeight)
			} else {
				actionPixbuf = scaledPixbuf
			}
		}
	}

	// --- App Icon ---
	appIconPath := item.IconPath
	if appIconPath == "" {
		appIconPath = getIconPath("icons/none-64.png")
	} else if !filepath.IsAbs(appIconPath) {
		appIconPath = getIconPath(appIconPath)
	} else if _, err := os.Stat(appIconPath); os.IsNotExist(err) {
		appIconPath = getIconPath("icons/none-64.png")
	}

	// Determine if this is a completed installation or uninstallation
	isCompletedInstallOrUninstall := item.Status == "success" &&
		(item.Action == "install" || item.Action == "uninstall")

	// Define the target app icon height based on whether this is a completed installation/uninstall
	appIconTargetHeight := targetAppHeight
	if isCompletedInstallOrUninstall {
		appIconTargetHeight = largeAppIconHeight
	}

	appPixbuf, err := gdk.PixbufNewFromFile(appIconPath)
	if err != nil {
		fmt.Printf("Error loading app icon %s: %v\n", appIconPath, err)
		appPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, appIconTargetHeight, appIconTargetHeight)
	} else if appPixbuf != nil {
		// Scale based on height, preserving aspect ratio
		origWidth := appPixbuf.GetWidth()
		origHeight := appPixbuf.GetHeight()
		if origHeight != appIconTargetHeight {
			newWidth := int(float64(appIconTargetHeight) * float64(origWidth) / float64(origHeight))
			if newWidth == 0 {
				newWidth = 1
			} // Prevent zero width

			scaledPixbuf, scaleErr := appPixbuf.ScaleSimple(newWidth, appIconTargetHeight, gdk.INTERP_BILINEAR)
			if scaleErr != nil {
				fmt.Printf("Error scaling app icon: %v\n", scaleErr)
				appPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, appIconTargetHeight, appIconTargetHeight)
			} else {
				appPixbuf = scaledPixbuf
			}
		}
	}

	var actionText string
	switch item.Status {
	case "waiting":
		actionText = fmt.Sprintf("Will %s", item.Action)
	case "in-progress":
		actionText = fmt.Sprintf("%sing...", capitalize(item.Action))
	case "success":
		actionText = fmt.Sprintf("%sed", capitalize(item.Action))
	case "failure":
		// For failures, show the action that failed
		actionText = fmt.Sprintf("<span foreground='red'>%s failed</span>", capitalize(item.Action))
	case "diagnosed":
		// For diagnosed items, show that they were diagnosed
		actionText = fmt.Sprintf("<span foreground='orange'>%s failed (diagnosed)</span>", capitalize(item.Action))
	case "daemon-complete":
		// For daemon completion, don't add this item to the display
		return
	default:
		// Fallback for unknown statuses
		actionText = fmt.Sprintf("%s (%s)", capitalize(item.Action), item.Status)
	}

	// Fix "updateed" text
	actionText = strings.Replace(actionText, "Updateed", "Updated", 1)
	actionText = strings.Replace(actionText, "Updateing", "Updating", 1)

	// Prepare the app name display
	appNameDisplay := item.AppName

	// Apply bold formatting to app names for completed installations/uninstalls
	if isCompletedInstallOrUninstall {
		appNameDisplay = fmt.Sprintf("<span size='large'><b>%s</b></span>", item.AppName)
	}

	iter := listStore.Append()
	listStore.Set(iter,
		[]int{0, 1, 2, 3, 4},
		[]interface{}{statusPixbuf, actionPixbuf, actionText, appPixbuf, appNameDisplay},
	)
}

// addDonationItemsToPixbufListStore adds donation items to the list store using pixbufs
func addDonationItemsToPixbufListStore(listStore *gtk.ListStore) {
	const targetAppHeight = 64 // Define target height for donation icons (was 24, now matches large app icon)

	botspotMessage := "to Botspot (Pi-Apps founder, college student, abandoned by family recently, needs help)"
	gmanMessage := "to theofficialgman (notable Pi-Apps contributor)"

	// Create empty pixbufs for blank columns
	emptyPixbuf, _ := gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, 1, 1)

	// --- Botspot Icon ---
	iter := listStore.Append()
	botspotIconPath := getIconPath("icons/botspot.png")
	botspotPixbuf, err := gdk.PixbufNewFromFile(botspotIconPath)
	if err != nil {
		fmt.Printf("Error loading Botspot icon: %v\n", err)
		botspotPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetAppHeight, targetAppHeight)
	} else if botspotPixbuf != nil {
		// Scale based on height, preserving aspect ratio
		origWidth := botspotPixbuf.GetWidth()
		origHeight := botspotPixbuf.GetHeight()
		if origHeight != targetAppHeight {
			newWidth := int(float64(targetAppHeight) * float64(origWidth) / float64(origHeight))
			if newWidth == 0 {
				newWidth = 1
			} // Prevent zero width

			scaledPixbuf, scaleErr := botspotPixbuf.ScaleSimple(newWidth, targetAppHeight, gdk.INTERP_BILINEAR)
			if scaleErr != nil {
				fmt.Printf("Error scaling Botspot icon: %v\n", scaleErr)
				botspotPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetAppHeight, targetAppHeight)
			} else {
				botspotPixbuf = scaledPixbuf
			}
		}
	}

	listStore.Set(iter,
		[]int{0, 1, 2, 3, 4},
		[]interface{}{
			emptyPixbuf,
			emptyPixbuf,
			"<u><b>Donate</b></u>",
			botspotPixbuf,
			botspotMessage,
		},
	)

	// --- theofficialgman Icon ---
	iter = listStore.Append()
	gmanIconPath := getIconPath("icons/theofficialgman.png")
	gmanPixbuf, err := gdk.PixbufNewFromFile(gmanIconPath)
	if err != nil {
		fmt.Printf("Error loading theofficialgman icon: %v\n", err)
		gmanPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetAppHeight, targetAppHeight)
	} else if gmanPixbuf != nil {
		// Scale based on height, preserving aspect ratio
		origWidth := gmanPixbuf.GetWidth()
		origHeight := gmanPixbuf.GetHeight()
		if origHeight != targetAppHeight {
			newWidth := int(float64(targetAppHeight) * float64(origWidth) / float64(origHeight))
			if newWidth == 0 {
				newWidth = 1
			} // Prevent zero width

			scaledPixbuf, scaleErr := gmanPixbuf.ScaleSimple(newWidth, targetAppHeight, gdk.INTERP_BILINEAR)
			if scaleErr != nil {
				fmt.Printf("Error scaling theofficialgman icon: %v\n", scaleErr)
				gmanPixbuf, _ = gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, targetAppHeight, targetAppHeight)
			} else {
				gmanPixbuf = scaledPixbuf
			}
		}
	}

	listStore.Set(iter,
		[]int{0, 1, 2, 3, 4},
		[]interface{}{
			emptyPixbuf,
			emptyPixbuf,
			"<u><b>Donate</b></u>",
			gmanPixbuf,
			gmanMessage,
		},
	)
}

// showErrorDialog shows an error dialog
func showErrorDialog(message string) {
	// If we can't use GTK, print error to console
	if !canUseGTK() {
		fmt.Printf("\nERROR: %s\n", message)
		return
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		fmt.Printf("\nERROR: %s\n", message)
		return
	}

	dialog, err := gtk.DialogNew()
	if err != nil {
		return
	}
	dialog.SetTitle("Error")

	// Add OK button
	dialog.AddButton("OK", gtk.RESPONSE_OK)

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return
	}

	// Add message with markup support
	label, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		return
	}
	label.SetMarkup(message) // Use SetMarkup for rich text formatting
	contentArea.Add(label)

	// Use our custom dialog runner
	_, _ = runGtkDialog(dialog)
	dialog.Destroy()
}

// ShowErrorDialogWithRetry shows an error dialog with retry option
// Returns true if user chose to retry, false if they chose to skip
func ShowErrorDialogWithRetry(appName, action, message string) bool {
	// If we can't use GTK, print error to console and return false
	if !canUseGTK() {
		fmt.Printf("\nERROR: %s\n", message)
		return false
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		fmt.Printf("\nERROR: %s\n", message)
		return false
	}

	dialog, err := gtk.DialogNew()
	if err != nil {
		return false
	}
	dialog.SetTitle("Error")

	// Add buttons
	dialog.AddButton("Skip", gtk.RESPONSE_CANCEL)
	dialog.AddButton("Retry", gtk.RESPONSE_OK)

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return false
	}

	// Add message with markup support
	label, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		return false
	}

	// Format the error message with app name and action
	// Use glib.MarkupEscapeText to properly escape the message content
	escapedMessage := glib.MarkupEscapeText(message)
	formattedMessage := fmt.Sprintf("Failed to %s <b>%s</b>:\n%s", action, appName, escapedMessage)
	label.SetMarkup(formattedMessage) // Use SetMarkup for rich text formatting
	contentArea.Add(label)

	// Use our custom dialog runner
	response, err := runGtkDialog(dialog)
	dialog.Destroy()

	if err != nil {
		return false
	}

	return response == gtk.RESPONSE_OK
}

// showConfirmDialog shows a confirmation dialog and returns true if user confirms
func showConfirmDialog(message string) bool {
	// If we can't use GTK, ask for confirmation on command line
	if !canUseGTK() {
		fmt.Printf("\n%s (y/n): ", message)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(response)
		return response == "y" || response == "yes"
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		fmt.Printf("\n%s (y/n): ", message)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(response)
		return response == "y" || response == "yes"
	}

	dialog, err := gtk.DialogNew()
	if err != nil {
		return false
	}
	dialog.SetTitle("Quick question")

	// Add buttons
	dialog.AddButton("No", gtk.RESPONSE_NO)
	dialog.AddButton("Yes", gtk.RESPONSE_YES)

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return false
	}

	// Add message with markup support
	label, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		return false
	}
	label.SetMarkup(message) // Use SetMarkup instead of setting plain text
	contentArea.Add(label)

	// Use our custom dialog runner
	response, err := runGtkDialog(dialog)
	dialog.Destroy()

	if err != nil {
		return false
	}

	return response == gtk.RESPONSE_YES
}

// showUpdateConfirmDialog shows a dialog asking if user wants to install newest version
func showUpdateConfirmDialog(appName, scriptName string) bool {
	// If we can't use GTK, ask for confirmation on command line
	if !canUseGTK() {
		message := fmt.Sprintf(
			"\nHold up...\n%s's %s script does not match the online version. "+
				"Either you are about to install an outdated version, or you've made changes to the script yourself.\n\n"+
				"Would you like to install the newest official version of %s? (y/n): ",
			appName, scriptName, appName,
		)
		fmt.Print(message)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(response)
		return response == "y" || response == "yes"
	}

	// Make sure GTK is initialized
	if !ensureGTKInitialized() {
		return false
	}

	// Create message with markup for bold text
	message := fmt.Sprintf(
		"Hold up...\n<b>%s</b>'s %s script does not match the online version. "+
			"Either you are about to install an outdated version, or you've made changes to the script yourself.\n\n"+
			"<b>Would you like to install the newest official version of %s?</b>",
		appName, scriptName, appName,
	)

	dialog, err := gtk.DialogNew()
	if err != nil {
		return false
	}
	dialog.SetTitle("Quick question")

	// Add buttons
	dialog.AddButton("I know what I am doing, Install current version", gtk.RESPONSE_NO)
	dialog.AddButton("Yes, Install newest official version", gtk.RESPONSE_YES)

	// Get content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return false
	}

	// Add message with markup support
	label, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		return false
	}
	label.SetMarkup(message) // Use SetMarkup for rich text formatting
	contentArea.Add(label)

	// Use our custom dialog runner
	response, err := runGtkDialog(dialog)
	dialog.Destroy()

	if err != nil {
		return false
	}

	return response == gtk.RESPONSE_YES
}

// getAppIconPath returns the path to the app's icon
func getAppIconPath(appName string) string {
	piAppsDir := getPiAppsDir()
	icon64Path := filepath.Join(piAppsDir, "apps", appName, "icon-64.png")
	if fileExists(icon64Path) {
		return icon64Path
	}

	icon24Path := filepath.Join(piAppsDir, "apps", appName, "icon-24.png")
	if fileExists(icon24Path) {
		return icon24Path
	}

	// Return default icon if app-specific icon not found
	return filepath.Join(piAppsDir, "icons", "none-24.png")
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// getPiAppsDir returns the Pi-Apps directory from environment variable
func getPiAppsDir() string {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		// Default to a reasonable location if env var not set
		homeDir, err := os.UserHomeDir()
		if err == nil {
			piAppsDir = filepath.Join(homeDir, "pi-apps")
		} else {
			piAppsDir = "/home/pi/pi-apps"
		}
	}
	return piAppsDir
}

// isAppInstalled checks if an app is installed
func isAppInstalled(appName string) bool {
	statusFile := filepath.Join(getPiAppsDir(), "data", "status", appName)
	content, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(content))
	return status == "installed"
}

// fileExists checks if a file exists
func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// filesMatch checks if two files have the same content
func filesMatch(file1, file2 string) bool {
	content1, err := os.ReadFile(file1)
	if err != nil {
		return false
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		return false
	}

	return string(content1) == string(content2)
}

// getInstallScriptName determines which install script to use for an app
func getInstallScriptName(appName string) string {
	piAppsDir := getPiAppsDir()
	appDir := filepath.Join(piAppsDir, "apps", appName)

	// Check available scripts
	installPath := filepath.Join(appDir, "install")
	install32Path := filepath.Join(appDir, "install-32")
	install64Path := filepath.Join(appDir, "install-64")

	// Determine architecture (32-bit or 64-bit)
	is64bit := is64BitOS()

	// Choose the appropriate script based on architecture and available scripts
	if is64bit {
		if fileExists(install64Path) {
			return "install-64"
		} else if fileExists(installPath) {
			return "install"
		} else if fileExists(install32Path) {
			// Fall back to 32-bit script if that's all that's available
			return "install-32"
		}
	} else {
		// On 32-bit systems
		if fileExists(install32Path) {
			return "install-32"
		} else if fileExists(installPath) {
			return "install"
		}
		// Don't fall back to 64-bit script on 32-bit systems
	}

	return "" // No suitable script found
}

// is64BitOS checks if the OS is 64-bit
func is64BitOS() bool {
	return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" || runtime.GOARCH == "riscv64"
}

// openURL opens a URL in the default browser
func openURL(url string) error {
	return webbrowser.Open(url)
}

// CLI fallback functions

// progressMonitorCLI provides a simple CLI-based progress monitor
func progressMonitorCLI(queue []QueueItem) error {
	fmt.Println("\n=== Progress Monitor ===")
	fmt.Println("The following operations will be performed:")

	for _, item := range queue {
		fmt.Printf("%s %s: %s\n",
			strings.ToUpper(item.Action),
			item.AppName,
			strings.ToUpper(item.Status))
	}

	fmt.Println("\nPress Ctrl+C to cancel")
	return nil
}

// showSummaryDialogCLI shows a summary of completed actions in CLI
func showSummaryDialogCLI(completedQueue []QueueItem) error {
	fmt.Println("\n=== Operations Complete ===")
	fmt.Println("Thank you for using Pi-Apps! The following actions completed:")

	for _, item := range completedQueue {
		var actionText string
		switch item.Status {
		case "success":
			actionText = fmt.Sprintf("%sed successfully", capitalize(item.Action))
		case "failure":
			actionText = fmt.Sprintf("%s failed", capitalize(item.Action))
		default:
			actionText = fmt.Sprintf("%s status: %s", capitalize(item.Action), item.Status)
		}

		// Fix "updateed" text
		actionText = strings.Replace(actionText, "Updateed", "Updated", 1)

		fmt.Printf("%s: %s\n", item.AppName, actionText)
	}

	fmt.Println("\nDonations:")
	fmt.Println("- Botspot (Pi-Apps founder): https://github.com/sponsors/botspot")
	fmt.Println("- theofficialgman (Pi-Apps contributor): https://github.com/sponsors/theofficialgman")

	return nil
}

// showBrokenPackagesDialogCLI asks for sudo password in CLI
func showBrokenPackagesDialogCLI() (string, error) {
	fmt.Println("\n=== Broken Local Packages Repo Detected ===")
	fmt.Print("Please enter your user password to repair: ")

	// Note: This is not secure as the password will be visible,
	// but it's a simple fallback for testing
	var password string
	fmt.Scanln(&password)

	if password == "" {
		return "", fmt.Errorf("canceled by user")
	}

	return password, nil
}

// DisplayUnsupportedSystemWarning shows a formatted warning message for unsupported systems
func DisplayUnsupportedSystemWarning(message string, useGUI bool) {
	// Add ANSI color codes to match the original Bash implementation
	warningPrefix := "\033[93m\033[5m◢◣\033[25m\033[0m \033[93mWARNING:\033[0m \033[93mYOUR SYSTEM IS UNSUPPORTED:\033[0m\n"
	// Also format the message in yellow like in the original
	formattedMessage := "\033[93m" + message + "\033[0m\n"
	disabledMsg := "\033[103m\033[30mThe ability to send error reports has been disabled.\033[39m\033[49m\n"
	waitingMsg := "\033[103m\033[30mWaiting 10 seconds... (To cancel, press Ctrl+C or close this terminal)\033[39m\033[49m\n"

	// Write colored messages to stdout (terminal)
	fmt.Printf("%s%s%s%s", warningPrefix, formattedMessage, disabledMsg, waitingMsg)

	// Only show GUI dialog if explicitly requested
	if useGUI && canUseGTK() && ensureGTKInitialized() {
		// Create formatted message for GUI dialog
		dialogMessage := fmt.Sprintf("YOUR SYSTEM IS UNSUPPORTED:\n\n<b>%s</b>\n\nPi-Apps will disable the sending of any error reports until you have resolved the issue above.\nYour mileage may vary with using Pi-Apps in this state. Expect the majority of apps to be broken.", message)

		showErrorDialog(dialogMessage)
	}

	// Wait 10 seconds as in the original implementation
	time.Sleep(10 * time.Second)
}

// readQueueFromStatusFile reads queue status from a file (helper for progress monitor)
func readQueueFromStatusFile(statusFile string) ([]QueueItem, error) {
	if statusFile == "" {
		return nil, fmt.Errorf("no status file specified")
	}

	file, err := os.Open(statusFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var queue []QueueItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ";", 5)
		if len(parts) >= 4 {
			item := QueueItem{
				Action:   parts[0],
				AppName:  parts[1],
				Status:   parts[2],
				IconPath: parts[3],
			}
			if len(parts) >= 5 {
				item.ErrorMessage = parts[4]
			}
			queue = append(queue, item)
		}
	}

	return queue, scanner.Err()
}

// canUseGTK is already defined elsewhere in the package, so we've removed it

// max returns the maximum of two integers
//func max(a, b int) int {
//	if a > b {
//		return a
//	}
//	return b
//}
