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

// Module: preload.go
// Description: Provides functions for generating GTK3-friendly app lists for the GUI.
// This replaces the bash preload script functionality for the Go rewrite.

package gui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// AppListItem represents a single item in the app list
type AppListItem struct {
	Type        string // "app", "category", "back"
	Name        string
	Path        string // For navigation (category paths)
	Description string
	IconPath    string
	Status      string // "installed", "uninstalled", "corrupted", "disabled", ""
	IsUpdates   bool   // Special Updates category
}

// PreloadedList contains the preloaded app list data
type PreloadedList struct {
	Items     []AppListItem
	Prefix    string
	Generated time.Time
}

// AppListConfig holds configuration for app list generation
type AppListConfig struct {
	Directory string
	Prefix    string
	Format    string // "gtk" (GTK3 native instead of yad/xlunch)
}

// DirectoryInfo holds information about directories to check for changes
type DirectoryInfo struct {
	Path       string
	ModTime    int64
	LatestFile string
}

// TimeStampChecker manages timestamp checking for change detection
type TimeStampChecker struct {
	Directory   string
	CheckedDirs []DirectoryInfo
}

// NewTimeStampChecker creates a new timestamp checker
func NewTimeStampChecker(directory string) *TimeStampChecker {
	return &TimeStampChecker{
		Directory: directory,
		CheckedDirs: []DirectoryInfo{
			{Path: filepath.Join(directory, "apps")},
			{Path: filepath.Join(directory, "data", "settings")},
			{Path: filepath.Join(directory, "data", "status")},
			{Path: filepath.Join(directory, "etc")},
			{Path: filepath.Join(directory, "icons", "categories")},
			{Path: filepath.Join(directory, "preload")},
			{Path: filepath.Join(directory, "api")},
			{Path: filepath.Join(directory, "data", "category-overrides")},
		},
	}
}

// GetTimestamps generates timestamp information for all monitored directories
func (tc *TimeStampChecker) GetTimestamps() string {
	var result strings.Builder

	for _, dirInfo := range tc.CheckedDirs {
		stat, err := os.Stat(dirInfo.Path)
		if err != nil {
			result.WriteString(fmt.Sprintf("%s 0 \n", dirInfo.Path))
			continue
		}

		result.WriteString(fmt.Sprintf("%s %d ", dirInfo.Path, stat.ModTime().Unix()))

		if stat.IsDir() {
			// Find the latest file in this directory
			latestTime := int64(0)
			latestPath := ""

			err := filepath.WalkDir(dirInfo.Path, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Continue on errors
				}
				if !d.IsDir() {
					info, err := d.Info()
					if err == nil && info.ModTime().Unix() > latestTime {
						latestTime = info.ModTime().Unix()
						latestPath = path
					}
				}
				return nil
			})

			if err == nil && latestPath != "" {
				result.WriteString(fmt.Sprintf("%d %s\n", latestTime, latestPath))
			} else {
				result.WriteString("\n")
			}
		} else {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// HasChanged checks if any monitored directory has changed since the last check
func (tc *TimeStampChecker) HasChanged(prefix string) (bool, error) {
	timestampFile := filepath.Join(tc.Directory, "data", "preload",
		fmt.Sprintf("timestamps-%s", sanitizePath(prefix)))

	currentTimestamps := tc.GetTimestamps()

	if _, err := os.Stat(timestampFile); os.IsNotExist(err) {
		return true, nil // No timestamp file, needs reload
	}

	savedTimestamps, err := os.ReadFile(timestampFile)
	if err != nil {
		return true, nil // Can't read timestamp file, needs reload
	}

	return currentTimestamps != string(savedTimestamps), nil
}

// SaveTimestamps saves current timestamps to file
func (tc *TimeStampChecker) SaveTimestamps(prefix string) error {
	preloadDir := filepath.Join(tc.Directory, "data", "preload")
	if err := os.MkdirAll(preloadDir, 0755); err != nil {
		logger.Error(fmt.Sprintf("failed to create preload directory: %v\n", err))
		return fmt.Errorf("failed to create preload directory: %w", err)
	}

	timestampFile := filepath.Join(preloadDir,
		fmt.Sprintf("timestamps-%s", sanitizePath(prefix)))

	timestamps := tc.GetTimestamps()
	return os.WriteFile(timestampFile, []byte(timestamps), 0644)
}

// PreloadAppList generates or loads a cached app list
func PreloadAppList(directory, prefix string) (*PreloadedList, error) {
	if directory == "" {
		directory = os.Getenv("PI_APPS_DIR")
		if directory == "" {
			logger.Error("PI_APPS_DIR environment variable not set")
			return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
		}
	}

	config := &AppListConfig{
		Directory: directory,
		Prefix:    prefix,
		Format:    "gtk",
	}

	tc := NewTimeStampChecker(directory)

	// Check if we need to reload the list
	needsReload, err := shouldReloadList(config, tc)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to check if reload needed: %v\n", err))
		return nil, fmt.Errorf("failed to check if reload needed: %w", err)
	}

	// Try to load cached list if no reload needed
	if !needsReload {
		cached, err := loadCachedList(config)
		if err == nil {
			logger.Info(fmt.Sprintf("Reading cached list for '%s'...\n", prefix))
			return cached, nil
		}
		// If loading cached fails, fall through to regenerate
		needsReload = true
	}

	// Generate new list
	logger.Info(fmt.Sprintf("Generating list for '%s'...\n", prefix))

	// Load API functions
	os.Setenv("PI_APPS_DIR", directory)

	list, err := generateAppList(config)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to generate app list: %v\n", err))
		return nil, fmt.Errorf("failed to generate app list: %w", err)
	}

	// Save the list and timestamps
	if err := saveCachedList(config, list); err != nil {
		logger.Warn(fmt.Sprintf("failed to save cached list: %v\n", err))
	}

	if err := tc.SaveTimestamps(prefix); err != nil {
		logger.Warn(fmt.Sprintf("failed to save timestamps: %v\n", err))
	}

	logger.Info(fmt.Sprintf("Finished preload for '%s'\n", prefix))
	return list, nil
}

// shouldReloadList determines if the app list needs to be regenerated
func shouldReloadList(config *AppListConfig, tc *TimeStampChecker) (bool, error) {
	// Check if timestamps have changed
	changed, err := tc.HasChanged(config.Prefix)
	if err != nil {
		return true, nil // If we can't check, assume we need to reload
	}

	if changed {
		logger.Info("Timestamps don't match")
		return true, nil
	}

	// Check if list file exists
	listFile := getListFilePath(config)
	if _, err := os.Stat(listFile); os.IsNotExist(err) {
		logger.Info(fmt.Sprintf("List file for %s does not exist.\n", config.Prefix))
		return true, nil
	}

	// Check if list file is empty
	stat, err := os.Stat(listFile)
	if err != nil || stat.Size() == 0 {
		logger.Info(fmt.Sprintf("List file for %s is empty.\n", config.Prefix))
		return true, nil
	}

	logger.Info("Timestamps match.")
	return false, nil
}

// generateAppList creates a new app list for the specified configuration
func generateAppList(config *AppListConfig) (*PreloadedList, error) {
	list := &PreloadedList{
		Prefix:    config.Prefix,
		Generated: time.Now(),
	}

	// Check for Updates category first (if on main page)
	if config.Prefix == "" && hasUpdatesAvailable(config.Directory) {
		updatesItem := AppListItem{
			Type:        "category",
			Name:        "Updates",
			Path:        "Updates/",
			Description: "Pi-Apps Go updates are available. Click here to update your apps.",
			IconPath:    filepath.Join(config.Directory, "icons", "categories", "Updates.png"),
			IsUpdates:   true,
		}
		list.Items = append(list.Items, updatesItem)
	}

	// Get virtual file system with apps/categories
	vfiles, err := getVirtualFileSystem(config)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get virtual file system: %v\n", err))
		return nil, fmt.Errorf("failed to get virtual file system: %w", err)
	}

	// Separate apps and directories
	apps, dirs := separateAppsAndDirs(vfiles, config.Directory)

	// Shuffle if enabled
	if shouldShuffleList(config.Directory) {
		apps = shuffleSlice(apps)
		dirs = shuffleSlice(dirs)
	}

	// Add back button if within a prefix
	if config.Prefix != "" {
		backItem := AppListItem{
			Type:        "back",
			Name:        "Back",
			Path:        getParentPath(config.Prefix),
			Description: "Return to the previous location",
			IconPath:    filepath.Join(config.Directory, "icons", "back.png"),
		}
		list.Items = append(list.Items, backItem)
	}

	// Add directory items
	for _, dir := range dirs {
		dirItem, err := createDirectoryItem(dir, config)
		if err != nil {
			logger.Warn(fmt.Sprintf("failed to create directory item for %s: %v\n", dir, err))
			continue
		}
		list.Items = append(list.Items, dirItem)
	}

	// Add app items
	for _, app := range apps {
		appItem, err := createAppItem(app, config)
		if err != nil {
			logger.Warn(fmt.Sprintf("failed to create app item for %s: %v\n", app, err))
			continue
		}
		list.Items = append(list.Items, appItem)
	}

	return list, nil
}

// getVirtualFileSystem creates a virtual file system representation
func getVirtualFileSystem(config *AppListConfig) ([]string, error) {
	if config.Prefix != "" {
		// Show apps within specific prefix
		logger.Info(fmt.Sprintf("Showing apps within %s/\n", config.Prefix))
		vfiles, err := api.AppPrefixCategory(config.Directory, config.Prefix)
		if err != nil {
			return nil, err
		}

		// Remove the prefix from each item
		var result []string
		prefixWithSlash := config.Prefix + "/"
		for _, vfile := range vfiles {
			if strings.HasPrefix(vfile, prefixWithSlash) {
				result = append(result, strings.TrimPrefix(vfile, prefixWithSlash))
			}
		}
		return result, nil
	} else {
		// Show all categories (main page)
		vfiles, err := api.AppPrefixCategory(config.Directory, "")
		if err != nil {
			return nil, err
		}

		// Filter out hidden category
		var result []string
		for _, vfile := range vfiles {
			if !strings.HasPrefix(vfile, "hidden/") {
				result = append(result, vfile)
			}
		}
		return result, nil
	}
}

// separateAppsAndDirs separates apps from directories
func separateAppsAndDirs(vfiles []string, directory string) (apps []string, dirs []string) {
	// Remove apps within categories - show this layer only
	var processed []string
	for _, vfile := range vfiles {
		if strings.Contains(vfile, "/") {
			// This is a category/app combination, extract just the category
			parts := strings.Split(vfile, "/")
			category := parts[0] + "/"
			processed = append(processed, category)
		} else {
			// This is a direct app
			processed = append(processed, vfile)
		}
	}

	// Remove duplicates
	processed = removeDuplicates(processed)

	// Get CPU installable apps for filtering - this ensures architecture compatibility
	cpuInstallableApps, err := api.ListApps("cpu_installable")
	if err != nil {
		logger.Warn(fmt.Sprintf("failed to get CPU installable apps: %v\n", err))
		cpuInstallableApps = []string{}
	}

	// Separate apps and directories
	for _, item := range processed {
		if strings.HasSuffix(item, "/") {
			// It's a directory
			dirs = append(dirs, strings.TrimSuffix(item, "/"))
		} else {
			// It's an app, check if it's CPU installable (compatible with current architecture)
			if contains(cpuInstallableApps, item) {
				apps = append(apps, item)
			}
			// If not in cpuInstallableApps, the app is not compatible with current architecture and will be hidden
		}
	}

	return apps, dirs
}

// createDirectoryItem creates an AppListItem for a directory/category
func createDirectoryItem(dir string, config *AppListConfig) (AppListItem, error) {
	var iconPath string
	categoryIconPath := filepath.Join(config.Directory, "icons", "categories", dir+".png")
	if appListFileExists(categoryIconPath) {
		iconPath = categoryIconPath
	} else {
		iconPath = filepath.Join(config.Directory, "icons", "categories", "default.png")
	}

	var path string
	if config.Prefix != "" {
		path = config.Prefix + "/" + dir + "/"
	} else {
		path = dir + "/"
	}

	description := getCategoryDescription(dir)

	return AppListItem{
		Type:        "category",
		Name:        dir,
		Path:        path,
		Description: description,
		IconPath:    iconPath,
	}, nil
}

// createAppItem creates an AppListItem for an app
func createAppItem(app string, config *AppListConfig) (AppListItem, error) {
	// Get app status
	status, err := api.GetAppStatus(app)
	if err != nil {
		status = ""
	}

	// Get app description (first line only, like the original bash script)
	descFile := filepath.Join(config.Directory, "apps", app, "description")
	description := "Description unavailable"
	if descData, err := os.ReadFile(descFile); err == nil {
		// Split into lines and take only the first line (matching bash read -r behavior)
		lines := strings.Split(string(descData), "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			description = strings.TrimSpace(lines[0])
		}
	}

	// Get app icon
	iconPath := filepath.Join(config.Directory, "apps", app, "icon-24.png")
	if !appListFileExists(iconPath) {
		iconPath = filepath.Join(config.Directory, "icons", "none-24.png")
	}

	var path string
	if config.Prefix != "" {
		path = config.Prefix + "/" + app
	} else {
		path = app
	}

	return AppListItem{
		Type:        "app",
		Name:        app,
		Path:        path,
		Description: description,
		IconPath:    iconPath,
		Status:      status,
	}, nil
}

// Helper functions

func sanitizePath(path string) string {
	return strings.ReplaceAll(path, "/", "")
}

func getListFilePath(config *AppListConfig) string {
	return filepath.Join(config.Directory, "data", "preload",
		fmt.Sprintf("LIST-%s", sanitizePath(config.Prefix)))
}

func hasUpdatesAvailable(directory string) bool {
	updatableFiles := filepath.Join(directory, "data", "update-status", "updatable-files")
	updatableApps := filepath.Join(directory, "data", "update-status", "updatable-apps")

	return (appListFileExists(updatableFiles) && fileSize(updatableFiles) > 0) ||
		(appListFileExists(updatableApps) && fileSize(updatableApps) > 0)
}

func shouldShuffleList(directory string) bool {
	shuffleFile := filepath.Join(directory, "data", "settings", "Shuffle App list")
	if data, err := os.ReadFile(shuffleFile); err == nil {
		return strings.TrimSpace(string(data)) == "Yes"
	}
	return false
}

func getParentPath(path string) string {
	if path == "" {
		return ""
	}
	parent := filepath.Dir(path)
	if parent == "." {
		return ""
	}
	return parent + "/"
}

func getCategoryDescription(category string) string {
	descriptions := map[string]string{
		"Browsers":          "Internet browsers.",
		"All Apps":          "All Pi-Apps Applications in one long list.",
		"Appearance":        "Applications and Themes which modify the look and feel of your OS.",
		"System Management": "Apps that help you keep track of system resources and general system management.",
		"Games":             "Games and Emulators",
		"Installed":         "All Pi-Apps Apps that you have installed.",
		"Internet":          "Browsers, Chat Clients, Email Clients, and so much more.",
		"Multimedia":        "Video playback and creation, audio playback and creation, and streaming alternatives.",
		"Packages":          "Simple Apps that install directly from APT repos.",
		"Tools":             "An assortment of helpful programs that don't already fit into another category.",
		"Terminals":         "Alternative terminal programs built for the modern age as well as to replicate your old vintage computer.",
		"Programming":       "Code editors, IDEs, and other applications to help you write and make other programs.",
		"Creative Arts":     "Drawing, Painting, and Photo and Movie Editors",
		"Engineering":       "3D Printing slicers, CAD/modeling, and general design software",
		"Office":            "Office suites (document and slideshow editors), and other office tools.",
		"Emulation":         "Applications that help you run non-ARM or non-Linux software.",
		"Communication":     "Internet messaging, calling, video chatting, and email clients.",
	}

	if desc, ok := descriptions[category]; ok {
		return desc
	}
	return ""
}

// loadCachedList loads a previously cached app list
func loadCachedList(config *AppListConfig) (*PreloadedList, error) {
	preloadDir := filepath.Join(config.Directory, "data", "preload")
	listFile := filepath.Join(preloadDir, fmt.Sprintf("LIST-%s", sanitizePath(config.Prefix)))

	// Check if the cached list file exists
	if !appListFileExists(listFile) {
		return nil, fmt.Errorf("cached list file does not exist: %s", listFile)
	}

	// Read the cached file
	data, err := os.ReadFile(listFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached list file: %w", err)
	}

	// Parse the pipe-delimited format: "Type|Name|Path|Description|IconPath|Status"
	lines := strings.Split(string(data), "\n")
	var items []AppListItem

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue // Skip malformed lines
		}

		item := AppListItem{
			Type:        parts[0],
			Name:        parts[1],
			Path:        parts[2],
			Description: parts[3],
			IconPath:    parts[4],
			Status:      parts[5],
		}

		// Handle Updates category
		if item.Name == "Updates" {
			item.IsUpdates = true
		}

		items = append(items, item)
	}

	return &PreloadedList{
		Items:     items,
		Prefix:    config.Prefix,
		Generated: time.Now(), // We don't store generation time in cache, use current time
	}, nil
}

func saveCachedList(config *AppListConfig, list *PreloadedList) error {
	listFile := getListFilePath(config)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(listFile), 0755); err != nil {
		return err
	}

	// Create temporary file
	tmpFile := listFile + "-tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write list items (this would be implementation-specific)
	// For GTK3, we might serialize differently than YAD format
	for _, item := range list.Items {
		line := fmt.Sprintf("%s|%s|%s|%s|%s|%s\n",
			item.Type, item.Name, item.Path, item.Description, item.IconPath, item.Status)
		if _, err := file.WriteString(line); err != nil {
			return err
		}
	}

	// Atomically move the temporary file to the final location
	return os.Rename(tmpFile, listFile)
}

// Utility functions

func fileSize(path string) int64 {
	if stat, err := os.Stat(path); err == nil {
		return stat.Size()
	}
	return 0
}

func appListFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

func shuffleSlice(slice []string) []string {
	result := make([]string, len(slice))
	copy(result, slice)

	// Fisher-Yates shuffle algorithm
	for i := len(result) - 1; i > 0; i-- {
		j := int(time.Now().UnixNano()) % (i + 1)
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// PopulateGTKTreeView populates a GTK TreeView with the preloaded app list
func PopulateGTKTreeView(treeView *gtk.TreeView, list *PreloadedList) error {
	// Get the model from the tree view
	model, err := treeView.GetModel()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get tree view model: %v\n", err))
		return fmt.Errorf("failed to get tree view model: %w", err)
	}

	listStore, ok := model.(*gtk.ListStore)
	if !ok {
		logger.Error("tree view model is not a ListStore")
		return fmt.Errorf("tree view model is not a ListStore")
	}

	// Clear existing items
	listStore.Clear()

	// Add items to the list store
	for _, item := range list.Items {
		iter := listStore.Append()

		// Load icon
		var pixbuf *gdk.Pixbuf
		if appListFileExists(item.IconPath) {
			pixbuf, _ = gdk.PixbufNewFromFile(item.IconPath)
		}

		// Set values (this assumes a specific column layout)
		// Column 0: Icon (Pixbuf)
		// Column 1: Name (String)
		// Column 2: Description (String)
		// Column 3: Path (String) - hidden column for navigation
		// Column 4: Status (String) - for coloring

		if pixbuf != nil {
			listStore.SetValue(iter, 0, pixbuf)
		}
		listStore.SetValue(iter, 1, item.Name)
		listStore.SetValue(iter, 2, item.Description)
		listStore.SetValue(iter, 3, item.Path)
		listStore.SetValue(iter, 4, item.Status)
	}

	return nil
}

// CreateAppListTreeView creates a GTK TreeView configured for displaying app lists
func CreateAppListTreeView() (*gtk.TreeView, *gtk.ListStore, error) {
	// Create list store with columns: Icon(pixbuf), Name(string), Description(string), Path(string), Status(string)
	listStore, err := gtk.ListStoreNew(
		gdk.PixbufGetType(), // 0: Icon
		glib.TYPE_STRING,    // 1: Name
		glib.TYPE_STRING,    // 2: Description
		glib.TYPE_STRING,    // 3: Path (hidden)
		glib.TYPE_STRING,    // 4: Status (for coloring)
	)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create list store: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create list store: %w", err)
	}

	// Create tree view
	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create tree view: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create tree view: %w", err)
	}

	// Create icon column
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create icon renderer: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create icon renderer: %w", err)
	}

	iconColumn, err := gtk.TreeViewColumnNewWithAttribute("", iconRenderer, "pixbuf", 0)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create icon column: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create icon column: %w", err)
	}
	iconColumn.SetSizing(gtk.TREE_VIEW_COLUMN_FIXED)
	iconColumn.SetFixedWidth(32)
	treeView.AppendColumn(iconColumn)

	// Create name column
	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create name renderer: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create name renderer: %w", err)
	}

	nameColumn, err := gtk.TreeViewColumnNewWithAttribute("Name", nameRenderer, "text", 1)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create name column: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create name column: %w", err)
	}
	nameColumn.SetResizable(true)
	nameColumn.SetSizing(gtk.TREE_VIEW_COLUMN_GROW_ONLY)
	treeView.AppendColumn(nameColumn)

	// Create description column
	descRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create description renderer: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create description renderer: %w", err)
	}

	descColumn, err := gtk.TreeViewColumnNewWithAttribute("Description", descRenderer, "text", 2)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create description column: %v\n", err))
		return nil, nil, fmt.Errorf("failed to create description column: %w", err)
	}
	descColumn.SetExpand(true)
	descColumn.SetResizable(true)
	treeView.AppendColumn(descColumn)

	// Set up selection mode
	selection, err := treeView.GetSelection()
	if err == nil {
		selection.SetMode(gtk.SELECTION_SINGLE)
	}

	return treeView, listStore, nil
}

// GetSelectedAppPath gets the path of the currently selected item
func GetSelectedAppPath(treeView *gtk.TreeView) (string, error) {
	selection, err := treeView.GetSelection()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get selection: %v\n", err))
		return "", fmt.Errorf("failed to get selection: %w", err)
	}

	_, iter, ok := selection.GetSelected()
	if !ok {
		logger.Error("no item selected")
		return "", fmt.Errorf("no item selected")
	}

	// Get the model directly from the tree view instead of from selection
	model, err := treeView.GetModel()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get tree view model: %v\n", err))
		return "", fmt.Errorf("failed to get tree view model: %w", err)
	}

	listStore, ok := model.(*gtk.ListStore)
	if !ok {
		logger.Error("tree view model is not a ListStore")
		return "", fmt.Errorf("tree view model is not a ListStore")
	}

	// Get the path from column 3
	value, err := listStore.GetValue(iter, 3)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get path value: %v\n", err))
		return "", fmt.Errorf("failed to get path value: %w", err)
	}

	pathInterface, err := value.GoValue()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to convert value to Go type: %v\n", err))
		return "", fmt.Errorf("failed to convert value to Go type: %w", err)
	}

	switch path := pathInterface.(type) {
	case string:
		return path, nil
	default:
		logger.Error("path value is not a string")
		return "", fmt.Errorf("path value is not a string")
	}
}
