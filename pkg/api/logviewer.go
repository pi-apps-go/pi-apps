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

// Module: logviewer.go
// Description: Provides functions for viewing and managing installation/uninstall log files.

package api

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// LogEntry represents a single log file entry
type LogEntry struct {
	Filepath   string
	App        string
	Action     string
	Result     string
	Date       string
	Caption    string
	AppIcon    string
	ActionIcon string
	ResultIcon string
	ModTime    time.Time
}

// CleanupOldLogFiles removes log files older than 6 days
func CleanupOldLogFiles() error {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	logsDir := filepath.Join(piAppsDir, "logs")
	if !DirExists(logsDir) {
		return nil // No logs directory, nothing to clean up
	}

	cutoffTime := time.Now().AddDate(0, 0, -6) // 6 days ago

	return filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Check if file is older than cutoff time
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				// Don't fail the entire cleanup if one file can't be removed
				fmt.Fprintf(os.Stderr, "Warning: Could not remove old log file %s: %v\n", path, err)
			}
		}

		return nil
	})
}

// GetLogFiles returns all log files sorted by modification time (newest first)
func GetLogFiles() ([]LogEntry, error) {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	logsDir := filepath.Join(piAppsDir, "logs")
	if !DirExists(logsDir) {
		return []LogEntry{}, nil // No logs directory, return empty slice
	}

	var logFiles []LogEntry

	err := filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Only process .log files
		if !strings.HasSuffix(d.Name(), ".log") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		logEntry, err := parseLogFilename(path, info.ModTime())
		if err != nil {
			// Skip files that don't match the expected format
			return nil
		}

		logFiles = append(logFiles, logEntry)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modification time, newest first
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime.After(logFiles[j].ModTime)
	})

	return logFiles, nil
}

// parseLogFilename parses a log filename to extract app, action, and result
// Expected format: {action}-{result}-{app}.log
// Examples: install-success-Firefox.log, uninstall-fail-Chrome.log
func parseLogFilename(filePath string, modTime time.Time) (LogEntry, error) {
	filename := filepath.Base(filePath)
	basename := strings.TrimSuffix(strings.ToLower(filename), ".log")

	// Use regex to parse the filename components
	// Pattern matches: {action}-{result}-{app}
	// where action can be 'install' or 'uninstall'
	// result can be 'success', 'fail', or 'incomplete'
	pattern := regexp.MustCompile(`^(install|uninstall)-(success|fail|incomplete)-(.+)$`)
	matches := pattern.FindStringSubmatch(basename)

	if len(matches) != 4 {
		return LogEntry{}, fmt.Errorf("filename does not match expected pattern: %s", basename)
	}

	action := matches[1]
	result := matches[2]
	app := matches[3]

	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		piAppsDir = "."
	}

	// Generate formatted date string
	dateStr := formatLogDate(modTime)

	// Generate caption
	caption := generateCaption(action, result, app)

	// Get icon paths
	appIcon := getAppIcon(app, piAppsDir)
	actionIcon := getActionIcon(action, piAppsDir)
	resultIcon := getResultIcon(result, piAppsDir)

	return LogEntry{
		Filepath:   filePath,
		App:        app,
		Action:     action,
		Result:     result,
		Date:       dateStr,
		Caption:    caption,
		AppIcon:    appIcon,
		ActionIcon: actionIcon,
		ResultIcon: resultIcon,
		ModTime:    modTime,
	}, nil
}

// formatLogDate formats the log date in a human-readable format
func formatLogDate(modTime time.Time) string {
	now := time.Now()
	today := now.Format("Monday")
	yesterday := now.AddDate(0, 0, -1).Format("Monday")
	logDay := modTime.Format("Monday")

	// Check if we use AM/PM format by checking if time.Now() includes AM/PM
	timeStr := now.Format("3:04 PM")
	useAMPM := strings.Contains(timeStr, "AM") || strings.Contains(timeStr, "PM")

	var dateFormat string
	if useAMPM {
		dateFormat = "Monday 3:04 PM"
	} else {
		dateFormat = "Monday 15:04"
	}

	dateStr := modTime.Format(dateFormat)

	// Replace day names with Today/Yesterday if applicable
	if logDay == today {
		dateStr = strings.Replace(dateStr, logDay, "Today", 1)
	} else if logDay == yesterday {
		dateStr = strings.Replace(dateStr, logDay, "Yesterday", 1)
	}

	return dateStr
}

// generateCaption creates a human-readable caption for the log entry
func generateCaption(action, result, app string) string {
	var caption string

	if action == "uninstall" {
		caption = "Uninstalling " + app
	} else if action == "install" {
		caption = "Installing " + app
	}

	if result == "success" {
		caption += " succeeded."
	} else if result == "fail" {
		caption += " failed."
	} else { // incomplete
		caption += " was interrupted."
	}

	return caption
}

// getAppIcon returns the path to the app's icon or a default icon
func getAppIcon(app, piAppsDir string) string {
	appIconPath := filepath.Join(piAppsDir, "apps", app, "icon-24.png")
	if FileExists(appIconPath) {
		return appIconPath
	}
	return filepath.Join(piAppsDir, "icons", "none-24.png")
}

// getActionIcon returns the path to the action icon
func getActionIcon(action, piAppsDir string) string {
	if action == "uninstall" {
		return filepath.Join(piAppsDir, "icons", "uninstall.png")
	} else if action == "install" {
		return filepath.Join(piAppsDir, "icons", "install.png")
	}
	return filepath.Join(piAppsDir, "icons", "none-24.png")
}

// getResultIcon returns the path to the result icon
func getResultIcon(result, piAppsDir string) string {
	if result == "success" {
		return filepath.Join(piAppsDir, "icons", "success.png")
	} else if result == "fail" {
		return filepath.Join(piAppsDir, "icons", "failure.png")
	} else { // incomplete
		return filepath.Join(piAppsDir, "icons", "interrupted.png")
	}
}

// DeleteAllLogFiles removes all log files from the logs directory
func DeleteAllLogFiles() error {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	logsDir := filepath.Join(piAppsDir, "logs")
	if !DirExists(logsDir) {
		return nil // No logs directory, nothing to delete
	}

	// Remove all files in the logs directory
	err := os.RemoveAll(logsDir)
	if err != nil {
		return fmt.Errorf("failed to remove logs directory: %w", err)
	}

	// Recreate the logs directory
	err = os.MkdirAll(logsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to recreate logs directory: %w", err)
	}

	return nil
}

// ShowLogViewer displays the log viewer GUI
func ShowLogViewer() error {
	// Clean up old log files first
	if err := CleanupOldLogFiles(); err != nil {
		Warning("Failed to clean up old log files: " + err.Error())
	}

	// Get all log files
	logEntries, err := GetLogFiles()
	if err != nil {
		return fmt.Errorf("failed to get log files: %w", err)
	}

	// Show GUI
	return showLogViewerGUI(logEntries)
}

// showLogViewerGUI displays the log viewer using GTK
func showLogViewerGUI(logEntries []LogEntry) error {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Initialize GTK
	glib.SetPrgname("Log file viewer")
	gtk.Init(nil)

	// Create main window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("unable to create window: %w", err)
	}

	win.SetTitle("Log file viewer")
	win.SetDefaultSize(500, 400)
	win.SetPosition(gtk.WIN_POS_CENTER)
	win.SetKeepAbove(true)

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
	if FileExists(iconPath) {
		pixbuf, err := gdk.PixbufNewFromFile(iconPath)
		if err == nil {
			win.SetIcon(pixbuf)
		}
	}

	// Create main vbox
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return fmt.Errorf("unable to create main vbox: %w", err)
	}
	vbox.SetMarginTop(12)
	vbox.SetMarginBottom(12)
	vbox.SetMarginStart(12)
	vbox.SetMarginEnd(12)
	win.Add(vbox)

	// Create description label
	description := "Review the errors from installing or uninstalling apps.\nClick a line to open its log. Week-old log files will be deleted."
	descLabel, err := gtk.LabelNew(description)
	if err != nil {
		return fmt.Errorf("unable to create description label: %w", err)
	}
	descLabel.SetHAlign(gtk.ALIGN_START)
	descLabel.SetJustify(gtk.JUSTIFY_LEFT)
	vbox.PackStart(descLabel, false, false, 0)

	// Create scrolled window for the list
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("unable to create scrolled window: %w", err)
	}
	scrolledWindow.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWindow.SetShadowType(gtk.SHADOW_IN)
	vbox.PackStart(scrolledWindow, true, true, 0)

	// Create tree view and model
	treeView, listStore, err := createLogTreeView()
	if err != nil {
		return fmt.Errorf("unable to create tree view: %w", err)
	}
	scrolledWindow.Add(treeView)

	// Populate the list store with log entries
	populateLogList(listStore, logEntries)

	// Handle row activation (double-click or Enter)
	treeView.Connect("row-activated", func(tv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		handleLogSelection(tv, path)
	})

	// Create button box
	buttonBox, err := gtk.ButtonBoxNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return fmt.Errorf("unable to create button box: %w", err)
	}
	buttonBox.SetLayout(gtk.BUTTONBOX_END)
	buttonBox.SetSpacing(8)
	vbox.PackStart(buttonBox, false, false, 0)

	// Create Delete All button
	deleteAllBtn, err := gtk.ButtonNewWithLabel("Delete all")
	if err != nil {
		return fmt.Errorf("unable to create delete all button: %w", err)
	}

	trashIconPath := filepath.Join(piAppsDir, "icons", "trash.png")
	if FileExists(trashIconPath) {
		trashIcon, err := gtk.ImageNewFromFile(trashIconPath)
		if err == nil {
			deleteAllBtn.SetImage(trashIcon)
		}
	}

	deleteAllBtn.SetTooltipText("Delete all log files from " + filepath.Join(piAppsDir, "logs"))
	buttonBox.Add(deleteAllBtn)

	// Create Close button
	closeBtn, err := gtk.ButtonNewWithLabel("Close")
	if err != nil {
		return fmt.Errorf("unable to create close button: %w", err)
	}

	exitIconPath := filepath.Join(piAppsDir, "icons", "exit.png")
	if FileExists(exitIconPath) {
		exitIcon, err := gtk.ImageNewFromFile(exitIconPath)
		if err == nil {
			closeBtn.SetImage(exitIcon)
		}
	}

	buttonBox.Add(closeBtn)

	// Connect button signals
	deleteAllBtn.Connect("clicked", func() {
		if confirmDeleteAll() {
			if err := DeleteAllLogFiles(); err != nil {
				showErrorDialog("Failed to delete log files: " + err.Error())
			} else {
				// Clear the list and show success message
				listStore.Clear()
				Status("Deleted everything inside of " + filepath.Join(piAppsDir, "logs"))
			}
		}
	})

	closeBtn.Connect("clicked", func() {
		win.Close()
	})

	// Connect window destroy signal
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Show all widgets and start main loop
	win.ShowAll()
	gtk.Main()

	return nil
}

// createLogTreeView creates and configures the tree view for displaying log entries
func createLogTreeView() (*gtk.TreeView, *gtk.ListStore, error) {
	// Create list store with columns: Date(string), ActionIcon(pixbuf), AppIcon(pixbuf), ResultIcon(pixbuf), Description(string), FilePath(string)
	listStore, err := gtk.ListStoreNew(glib.TYPE_STRING, gdk.PixbufGetType(), gdk.PixbufGetType(), gdk.PixbufGetType(), glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create list store: %w", err)
	}

	// Create tree view
	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create tree view: %w", err)
	}

	// Create columns
	// Day column
	dayRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create day renderer: %w", err)
	}
	dayColumn, err := gtk.TreeViewColumnNewWithAttribute("Day", dayRenderer, "text", 0)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create day column: %w", err)
	}
	treeView.AppendColumn(dayColumn)

	// Action icon column
	actionRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create action renderer: %w", err)
	}
	actionColumn, err := gtk.TreeViewColumnNewWithAttribute("I", actionRenderer, "pixbuf", 1)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create action column: %w", err)
	}
	actionColumn.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	actionColumn.SetFixedWidth(30)
	treeView.AppendColumn(actionColumn)

	// App icon column
	appRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create app renderer: %w", err)
	}
	appColumn, err := gtk.TreeViewColumnNewWithAttribute("A", appRenderer, "pixbuf", 2)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create app column: %w", err)
	}
	appColumn.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	appColumn.SetFixedWidth(30)
	treeView.AppendColumn(appColumn)

	// Result icon column
	resultRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create result renderer: %w", err)
	}
	resultColumn, err := gtk.TreeViewColumnNewWithAttribute("R", resultRenderer, "pixbuf", 3)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create result column: %w", err)
	}
	resultColumn.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	resultColumn.SetFixedWidth(30)
	treeView.AppendColumn(resultColumn)

	// Description column
	descRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create description renderer: %w", err)
	}
	descColumn, err := gtk.TreeViewColumnNewWithAttribute("Description", descRenderer, "text", 4)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create description column: %w", err)
	}
	descColumn.SetExpand(true)
	treeView.AppendColumn(descColumn)

	return treeView, listStore, nil
}

// populateLogList adds log entries to the list store
func populateLogList(listStore *gtk.ListStore, logEntries []LogEntry) {
	for _, entry := range logEntries {
		iter := listStore.Append()

		// Load pixbufs for icons
		var actionPixbuf, appPixbuf, resultPixbuf *gdk.Pixbuf

		if FileExists(entry.ActionIcon) {
			actionPixbuf, _ = gdk.PixbufNewFromFile(entry.ActionIcon)
		}

		if FileExists(entry.AppIcon) {
			appPixbuf, _ = gdk.PixbufNewFromFile(entry.AppIcon)
		}

		if FileExists(entry.ResultIcon) {
			resultPixbuf, _ = gdk.PixbufNewFromFile(entry.ResultIcon)
		}

		listStore.SetValue(iter, 0, entry.Date)
		if actionPixbuf != nil {
			listStore.SetValue(iter, 1, actionPixbuf)
		}
		if appPixbuf != nil {
			listStore.SetValue(iter, 2, appPixbuf)
		}
		if resultPixbuf != nil {
			listStore.SetValue(iter, 3, resultPixbuf)
		}
		listStore.SetValue(iter, 4, entry.Caption)
		listStore.SetValue(iter, 5, entry.Filepath)
	}
}

// handleLogSelection handles when a user selects a log entry
func handleLogSelection(treeView *gtk.TreeView, path *gtk.TreePath) {
	model, _ := treeView.GetModel()
	listStore := model.(*gtk.ListStore)
	iter, err := listStore.GetIter(path)
	if err != nil {
		return
	}

	// Get the file path from column 5
	filepathVal, err := listStore.GetValue(iter, 5)
	if err != nil {
		return
	}

	filepathInterface, err := filepathVal.GoValue()
	if err != nil {
		return
	}

	filepath, ok := filepathInterface.(string)
	if !ok {
		return
	}

	// Open the log file for viewing
	if err := ViewFile(filepath); err != nil {
		showErrorDialog("Failed to view log file: " + err.Error())
	}
}

// confirmDeleteAll shows a confirmation dialog for deleting all log files
func confirmDeleteAll() bool {
	dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO, "Are you sure you want to delete all log files?")
	defer dialog.Destroy()

	dialog.SetTitle("Confirm Delete")
	response := dialog.Run()
	return response == gtk.RESPONSE_YES
}

// showErrorDialog displays an error message to the user
func showErrorDialog(message string) {
	dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, message)
	if dialog == nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		return
	}
	defer dialog.Destroy()

	dialog.SetTitle("Error")
	dialog.Run()
}
