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

// Module: app_search.go
// Description: Provides functions for searching for apps in the app list.

package api

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gtk"
)

// WillReinstall returns true if the given app will be reinstalled during an update, false otherwise
//
//	false - app will not be reinstalled
//	true - app will be reinstalled
//	error - error if app is not specified
func WillReinstall(app string) (bool, error) {
	// Get environment variables
	directory := GetPiAppsDir()
	if directory == "" {
		return false, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Exit immediately if app is not installed
	status, err := GetAppStatus(app)
	if err != nil {
		return false, fmt.Errorf("error checking app status: %w", err)
	}
	if status != "installed" {
		return false, nil
	}

	// Detect which installation script exists for local install
	localScriptName, err := ScriptNameCPU(app)
	if err != nil {
		return false, fmt.Errorf("error getting local script name: %w", err)
	}

	// Store original directory to restore it later
	originalDir := GetPiAppsDir()

	// Set directory to update location to check the update script
	os.Setenv("PI_APPS_DIR", filepath.Join(directory, "update", "pi-apps"))

	// Get script name from update directory
	newScriptName, err := ScriptNameCPU(app)

	// Restore original directory
	os.Setenv("PI_APPS_DIR", originalDir)

	if err != nil {
		// If there's an error getting the script name, it might be that the app
		// no longer exists in the update directory. We'll assume no reinstall is needed.
		return false, nil
	}

	// Migration from package-app to script-app
	if newScriptName != "" && localScriptName == "packages" && newScriptName != "packages" {
		return true, nil
	}

	// Migration from script-app to package-app
	if localScriptName != "packages" && newScriptName == "packages" {
		return true, nil
	}

	// Update to package-app: compare required packages
	if newScriptName == "packages" {
		// Get required packages from local and update directories
		localPkgs, err := PkgAppPackagesRequired(app)
		if err != nil {
			return false, fmt.Errorf("error getting local packages: %w", err)
		}

		// Set directory to update location to check the update packages
		os.Setenv("PI_APPS_DIR", filepath.Join(directory, "update", "pi-apps"))
		updatePkgs, err := PkgAppPackagesRequired(app)
		os.Setenv("PI_APPS_DIR", originalDir) // Restore original directory

		if err != nil {
			return false, fmt.Errorf("error getting update packages: %w", err)
		}

		// If the required packages have changed, reinstall
		if localPkgs != updatePkgs {
			return true, nil
		}

		return false, nil
	}

	// For script apps, compare the script files
	if newScriptName != "" {
		localScriptPath := filepath.Join(directory, "apps", app, localScriptName)
		updateScriptPath := filepath.Join(directory, "update", "pi-apps", "apps", app, newScriptName)

		// If the files don't match, reinstall
		match, err := filesMatch(localScriptPath, updateScriptPath)
		if err != nil {
			return false, fmt.Errorf("error comparing script files: %w", err)
		}

		if !match {
			return true, nil
		}
	}

	return false, nil
}

// filesMatch returns true if the contents of the two files match, false otherwise
//
//	false - files do not match
//	true - files match
//	error - error if files do not exist
func filesMatch(file1, file2 string) (bool, error) {
	// Check if files exist
	if !FileExists(file1) || !FileExists(file2) {
		return false, nil
	}

	// Read both files
	data1, err := os.ReadFile(file1)
	if err != nil {
		return false, fmt.Errorf("error reading file %s: %w", file1, err)
	}

	data2, err := os.ReadFile(file2)
	if err != nil {
		return false, fmt.Errorf("error reading file %s: %w", file2, err)
	}

	// Compare the contents
	return string(data1) == string(data2), nil
}

// AppSearch searches all apps for the given query in the specified files
//
//	[]string - list of apps
//	error - error if query is not specified
func AppSearch(query string, searchFiles ...string) ([]string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// If no search files specified, use default ones
	if len(searchFiles) == 0 {
		searchFiles = []string{"description", "website", "credits"}
	}

	// Search for the query in the specified files
	results := make(map[string]bool) // Use a map to avoid duplicates

	for _, appDir := range listAppDirs(directory) {
		appName := filepath.Base(appDir)

		// Search in each specified file
		for _, fileName := range searchFiles {
			filePath := filepath.Join(appDir, fileName)

			if FileExists(filePath) {
				// Read and check the file for the query
				found, err := fileContainsText(filePath, query)
				if err != nil {
					DebugTf("Error searching in %s: %v", filePath, err)
					continue
				}

				if found {
					results[appName] = true
					break // Once found in one file, no need to check other files for this app
				}
			}
		}

		// Also check app name for matches
		appNameLower := strings.ToLower(appName)
		queryLower := strings.ToLower(query)

		if strings.HasPrefix(appNameLower, queryLower) || strings.Contains(appNameLower, queryLower) {
			results[appName] = true
		}
	}

	// Convert map keys to slice
	var appList []string
	for app := range results {
		appList = append(appList, app)
	}

	// Get list of CPU installable and hidden apps
	cpuInstallable, err := ListApps("cpu_installable")
	if err != nil {
		return nil, fmt.Errorf("error getting cpu_installable apps: %w", err)
	}

	hidden, err := ListApps("hidden")
	if err != nil {
		return nil, fmt.Errorf("error getting hidden apps: %w", err)
	}

	// Filter out incompatible and hidden apps
	var filteredResults []string
	for _, app := range appList {
		if stringInSlice(app, cpuInstallable) && !stringInSlice(app, hidden) {
			filteredResults = append(filteredResults, app)
		}
	}

	// Sort results: first prioritize apps starting with query, then containing query
	var startsWith, contains, others []string
	queryLower := strings.ToLower(query)

	for _, app := range filteredResults {
		appLower := strings.ToLower(app)
		if strings.HasPrefix(appLower, queryLower) {
			startsWith = append(startsWith, app)
		} else if strings.Contains(appLower, queryLower) {
			contains = append(contains, app)
		} else {
			others = append(others, app)
		}
	}

	// Sort each category alphabetically
	sort.Strings(startsWith)
	sort.Strings(contains)
	sort.Strings(others)

	// Combine the lists
	filteredResults = append(startsWith, contains...)
	filteredResults = append(filteredResults, others...)

	return filteredResults, nil
}

// fileContainsText returns true if the file contains the given text (case-insensitive)
//
//	false - file does not contain text
//	true - file contains text
//	error - error if file does not exist
func fileContainsText(filePath, text string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	textLower := strings.ToLower(text)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), textLower) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// stringInSlice returns true if the string is in the slice
//
//	false - string is not in slice
//	true - string is in slice
func stringInSlice(s string, slice []string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// listAppDirs returns a list of app directories
//
//	[]string - list of app directories
func listAppDirs(directory string) []string {
	appsDir := filepath.Join(directory, "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		Debug(fmt.Sprintf("Error reading apps directory: %v", err))
		return nil
	}

	var appDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			appDirs = append(appDirs, filepath.Join(appsDir, entry.Name()))
		}
	}

	return appDirs
}

// AppSearchGUI provides a graphical interface for searching apps using GTK3
//
//	"" - no app selected
//	error - error if GTK is not initialized
//
// Deprecated: This function has moved internally to the gui package for a better GUI main loop integration.
func AppSearchGUI() (string, error) {
	directory := GetPiAppsDir()
	if directory == "" {
		return "", fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Initialize GTK
	gtk.Init(nil)

	// Create a window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return "", fmt.Errorf("unable to create window: %w", err)
	}
	win.SetTitle("Search")
	win.SetDefaultSize(310, 200)
	win.SetResizable(false)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Load the last search query if available
	lastSearchFile := filepath.Join(directory, "data", "last-search")
	lastSearch := ""
	if FileExists(lastSearchFile) {
		data, err := os.ReadFile(lastSearchFile)
		if err == nil {
			lastSearch = strings.TrimSpace(string(data))
		}
	}

	// Create main box
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return "", fmt.Errorf("unable to create main box: %w", err)
	}
	mainBox.SetMarginTop(10)
	mainBox.SetMarginBottom(10)
	mainBox.SetMarginStart(10)
	mainBox.SetMarginEnd(10)

	// Create header label
	headerLabel, err := gtk.LabelNew("Search for apps.\nNot case-sensitive.")
	if err != nil {
		return "", fmt.Errorf("unable to create header label: %w", err)
	}
	mainBox.PackStart(headerLabel, false, false, 0)

	// Create search entry
	searchEntry, err := gtk.EntryNew()
	if err != nil {
		return "", fmt.Errorf("unable to create search entry: %w", err)
	}
	searchEntry.SetText(lastSearch)
	mainBox.PackStart(searchEntry, false, false, 0)

	// Create checkboxes
	checkBox1, err := gtk.CheckButtonNewWithLabel("Search description")
	if err != nil {
		return "", fmt.Errorf("unable to create checkbox: %w", err)
	}
	checkBox1.SetActive(true)
	mainBox.PackStart(checkBox1, false, false, 0)

	checkBox2, err := gtk.CheckButtonNewWithLabel("Search website")
	if err != nil {
		return "", fmt.Errorf("unable to create checkbox: %w", err)
	}
	checkBox2.SetActive(true)
	mainBox.PackStart(checkBox2, false, false, 0)

	checkBox3, err := gtk.CheckButtonNewWithLabel("Search credits")
	if err != nil {
		return "", fmt.Errorf("unable to create checkbox: %w", err)
	}
	checkBox3.SetActive(false)
	mainBox.PackStart(checkBox3, false, false, 0)

	checkBox4, err := gtk.CheckButtonNewWithLabel("Search scripts")
	if err != nil {
		return "", fmt.Errorf("unable to create checkbox: %w", err)
	}
	checkBox4.SetActive(false)
	mainBox.PackStart(checkBox4, false, false, 0)

	// Create button box
	buttonBox, err := gtk.ButtonBoxNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return "", fmt.Errorf("unable to create button box: %w", err)
	}
	buttonBox.SetLayout(gtk.BUTTONBOX_END)

	// Create search button
	searchButton, err := gtk.ButtonNewWithLabel("Search")
	if err != nil {
		return "", fmt.Errorf("unable to create search button: %w", err)
	}
	buttonBox.Add(searchButton)

	// Create cancel button
	cancelButton, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return "", fmt.Errorf("unable to create cancel button: %w", err)
	}
	buttonBox.Add(cancelButton)

	mainBox.PackEnd(buttonBox, false, false, 0)
	win.Add(mainBox)
	win.ShowAll()

	selectedApp := ""
	searchDone := false

	// Connect signals
	cancelButton.Connect("clicked", func() {
		searchDone = true
		win.Close()
	})

	searchButton.Connect("clicked", func() {
		query, err := searchEntry.GetText()
		if err != nil {
			DialogError("Error getting search query: " + err.Error())
			return
		}

		if query == "" {
			searchDone = true
			win.Close()
			return
		}

		// Save query for next time
		err = os.MkdirAll(filepath.Join(directory, "data"), 0755)
		if err == nil {
			os.WriteFile(lastSearchFile, []byte(query), 0644)
		}

		// Build search file list
		var searchFiles []string
		if checkBox1.GetActive() {
			searchFiles = append(searchFiles, "description")
		}
		if checkBox2.GetActive() {
			searchFiles = append(searchFiles, "website")
		}
		if checkBox3.GetActive() {
			searchFiles = append(searchFiles, "credits")
		}
		if checkBox4.GetActive() {
			searchFiles = append(searchFiles, "install", "install-32", "install-64", "uninstall")
		}

		// Skip file-based search if user searched for exact app name
		appList, err := ListApps("cpu_installable")
		if err != nil {
			DialogError("Error listing apps: " + err.Error())
			return
		}

		hiddenApps, err := ListApps("hidden")
		if err != nil {
			DialogError("Error listing hidden apps: " + err.Error())
			return
		}

		// Filter out hidden apps
		var filteredAppList []string
		for _, app := range appList {
			if !stringInSlice(app, hiddenApps) {
				filteredAppList = append(filteredAppList, app)
			}
		}

		// Check for exact match
		for _, app := range filteredAppList {
			if strings.EqualFold(app, query) {
				selectedApp = app
				searchDone = true
				win.Close()
				return
			}
		}

		// Do the search
		results, err := AppSearch(query, searchFiles...)
		if err != nil {
			DialogError("Error searching: " + err.Error())
			return
		}

		if len(results) == 0 {
			// No results found
			dialog := gtk.MessageDialogNew(win, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_OK,
				"No results found for \"%s\".", query)
			dialog.Run()
			dialog.Destroy()
			return
		}

		if len(results) == 1 {
			// Single result, return it directly
			selectedApp = results[0]
			searchDone = true
			win.Close()
			return
		}

		// Multiple results, show a list for selection
		searchDone = true
		win.Close()

		// Show results in a new window
		selectedApp = showSearchResults(directory, results, query)
	})

	win.Connect("destroy", func() {
		if !searchDone {
			selectedApp = ""
		}
		gtk.MainQuit()
	})

	// Run the GTK main loop
	gtk.Main()

	return selectedApp, nil
}

// showSearchResults shows the search results in a list and returns the selected app
func showSearchResults(directory string, results []string, query string) string {
	// Initialize GTK
	gtk.Init(nil)

	// Create a window
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		DialogError("Unable to create window: " + err.Error())
		return ""
	}
	win.SetTitle("Results for \"" + query + "\"")
	win.SetDefaultSize(310, 250)
	win.SetPosition(gtk.WIN_POS_CENTER)

	// Create main box
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		DialogError("Unable to create main box: " + err.Error())
		return ""
	}

	// Create scrolled window
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		DialogError("Unable to create scrolled window: " + err.Error())
		return ""
	}
	scrolledWindow.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_AUTOMATIC)

	// Create list box for results
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		DialogError("Unable to create list box: " + err.Error())
		return ""
	}
	listBox.SetSelectionMode(gtk.SELECTION_SINGLE)

	// Read category files
	categoryEntries, err := ReadCategoryFiles(directory)
	if err != nil {
		DialogError("Error reading category files: " + err.Error())
		return ""
	}

	// Create a map to store the app name for each row index
	rowToAppMap := make(map[int]string)
	rowIndex := 0

	// Add results to the list box
	for _, app := range results {
		// Find category for the app
		category := "Other"
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			if len(parts) >= 2 && parts[0] == app {
				category = parts[1]
				break
			}
		}

		// Create row box
		rowBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
		if err != nil {
			DebugTf("Error creating row box: %v", err)
			continue
		}
		rowBox.SetMarginStart(10)
		rowBox.SetMarginEnd(10)
		rowBox.SetMarginTop(5)
		rowBox.SetMarginBottom(5)

		// Add app icon
		appIcon, err := gtk.ImageNewFromFile(filepath.Join(directory, "apps", app, "icon-24.png"))
		if err != nil || !FileExists(filepath.Join(directory, "apps", app, "icon-24.png")) {
			// Use default icon if app icon doesn't exist
			appIcon, _ = gtk.ImageNewFromIconName("applications-other", gtk.ICON_SIZE_LARGE_TOOLBAR)
		}
		rowBox.PackStart(appIcon, false, false, 0)

		// Add app name
		appLabel, err := gtk.LabelNew(app)
		if err != nil {
			DebugTf("Error creating app label: %v", err)
			continue
		}
		rowBox.PackStart(appLabel, false, false, 0)

		// Add spacer
		spacer, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
		if err != nil {
			DebugTf("Error creating spacer: %v", err)
			continue
		}
		rowBox.PackStart(spacer, true, true, 0)

		// Add "in" label
		inLabel, err := gtk.LabelNew("in")
		if err != nil {
			DebugTf("Error creating 'in' label: %v", err)
			continue
		}
		inLabel.SetSizeRequest(20, -1)
		rowBox.PackStart(inLabel, false, false, 0)

		// Add category icon
		categoryIcon, err := gtk.ImageNewFromFile(filepath.Join(directory, "icons", "categories", category+".png"))
		if err != nil || !FileExists(filepath.Join(directory, "icons", "categories", category+".png")) {
			// Use default icon if category icon doesn't exist
			categoryIcon, _ = gtk.ImageNewFromIconName("folder", gtk.ICON_SIZE_LARGE_TOOLBAR)
		}
		rowBox.PackStart(categoryIcon, false, false, 0)

		// Add category name
		categoryLabel, err := gtk.LabelNew(category)
		if err != nil {
			DebugTf("Error creating category label: %v", err)
			continue
		}
		rowBox.PackStart(categoryLabel, false, false, 0)

		// Create list box row and add the box to it
		row, err := gtk.ListBoxRowNew()
		if err != nil {
			DebugTf("Error creating list box row: %v", err)
			continue
		}
		row.Add(rowBox)
		row.SetTooltipText(getAppDescription(directory, app))

		// Store app name in the map with this row's index
		rowToAppMap[rowIndex] = app
		rowIndex++

		listBox.Add(row)
	}

	scrolledWindow.Add(listBox)
	mainBox.PackStart(scrolledWindow, true, true, 0)
	win.Add(mainBox)
	win.ShowAll()

	selectedApp := ""

	// Connect signals
	listBox.Connect("row-activated", func(box *gtk.ListBox, row *gtk.ListBoxRow) {
		// Get the app name from the map using the row's index
		index := row.GetIndex()
		appName, ok := rowToAppMap[index]
		if ok && appName != "" {
			selectedApp = appName
			win.Close()
		}
	})

	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Run the GTK main loop
	gtk.Main()

	return selectedApp
}

// getAppDescription returns the first line of the app's description
//
//	"" - description unavailable
//	description - description
func getAppDescription(directory, app string) string {
	descFile := filepath.Join(directory, "apps", app, "description")
	if !FileExists(descFile) {
		return T("Description unavailable")
	}

	file, err := os.Open(descFile)
	if err != nil {
		return T("Description unavailable")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text()
	}

	return T("Description unavailable")
}

// DialogError displays an error dialog with the given message
func DialogError(message string) {
	// Initialize GTK
	gtk.Init(nil)

	dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, message)
	dialog.Run()
	dialog.Destroy()
}
