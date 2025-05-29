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

// Module: categoryedit.go
// Description: Provides functions for editing and managing app categories.

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

// CategoryData represents the category assignment data
type CategoryData struct {
	GlobalCategories map[string]string // app -> category mapping from global file
	LocalCategories  map[string]string // app -> category mapping from overrides file
}

// ReadCategoryData reads both global and local category files
func ReadCategoryData() (*CategoryData, error) {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	data := &CategoryData{
		GlobalCategories: make(map[string]string),
		LocalCategories:  make(map[string]string),
	}

	// Read global categories file
	globalFile := filepath.Join(piAppsDir, "etc", "categories")
	if FileExists(globalFile) {
		if err := readCategoryFile(globalFile, data.GlobalCategories); err != nil {
			return nil, fmt.Errorf("failed to read global categories: %w", err)
		}
	}

	// Read local category overrides file
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
	piAppsDir := os.Getenv("PI_APPS_DIR")
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
	piAppsDir := os.Getenv("PI_APPS_DIR")
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
	piAppsDir := os.Getenv("PI_APPS_DIR")
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
	piAppsDir := os.Getenv("PI_APPS_DIR")
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
	piAppsDir := os.Getenv("PI_APPS_DIR")
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
		app, ok := appInterface.(string)
		if !ok {
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
		category, ok := categoryInterface.(string)
		if !ok {
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
