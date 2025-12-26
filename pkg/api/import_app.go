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

// Module: import_app.go
// Description: Provides functions for importing apps from various sources.

package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// ImportAppGUI provides a graphical interface for importing apps in Pi-Apps Go
func ImportAppGUI() error {

	// Set program name
	glib.SetPrgname("Import App Wizard")

	// Initialize GTK
	gtk.Init(nil)

	// Get PI_APPS_DIR environment variable
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Create the dialog window
	window, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("error creating window: %w", err)
	}
	window.SetTitle("App Importer")
	window.SetDefaultSize(500, 300)
	window.SetPosition(gtk.WIN_POS_CENTER)

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
	if FileExists(iconPath) {
		if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
			window.SetIcon(pixbuf)
		}
	}

	// Create a vertical box to hold the widgets
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		return fmt.Errorf("error creating vbox: %w", err)
	}
	vbox.SetMarginStart(10)
	vbox.SetMarginEnd(10)
	vbox.SetMarginTop(10)
	vbox.SetMarginBottom(10)
	window.Add(vbox)

	// Create a label with instructions
	label, err := gtk.LabelNew("")
	if err != nil {
		return fmt.Errorf("error creating label: %w", err)
	}
	account, repo := GetGitUrl()
	if account == "" || repo == "" {
		label.SetMarkup(fmt.Sprintf("Import an app from somewhere else.\nApps are saved in <b>%s/apps</b>.\nPut something in the blank below.\nExamples:\n\n    <b>https://github.com/pi-apps-go/pi-apps-go/pull/1068</b>\n    <b>1068</b>\n    <b>https://link/to/app.zip</b>\n    <b>$HOME/my-app.zip</b>", piAppsDir))
	} else {
		label.SetMarkup(fmt.Sprintf("Import an app from somewhere else.\nApps are saved in <b>%s/apps</b>.\nPut something in the blank below.\nExamples:\n\n    <b>https://github.com/%s/%s/pull/1068</b>\n    <b>1068</b>\n    <b>https://link/to/app.zip</b>\n    <b>$HOME/my-app.zip</b>", piAppsDir, account, repo))
	}
	label.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(label, false, false, 5)

	// Create an entry for the import source
	entry, err := gtk.EntryNew()
	if err != nil {
		return fmt.Errorf("error creating entry: %w", err)
	}
	vbox.PackStart(entry, false, false, 5)

	// Create a button box
	buttonBox, err := gtk.ButtonBoxNew(gtk.ORIENTATION_HORIZONTAL)
	if err != nil {
		return fmt.Errorf("error creating button box: %w", err)
	}
	buttonBox.SetLayout(gtk.BUTTONBOX_END)
	buttonBox.SetSpacing(8)
	vbox.PackEnd(buttonBox, false, false, 0)

	// Add Import button
	importButton, err := gtk.ButtonNewWithLabel("Import")
	if err != nil {
		return fmt.Errorf("error creating import button: %w", err)
	}
	buttonBox.Add(importButton)

	// Add Cancel button
	cancelButton, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return fmt.Errorf("error creating cancel button: %w", err)
	}
	buttonBox.Add(cancelButton)

	// Connect signals
	importButton.Connect("clicked", func() {
		importSource, err := entry.GetText()
		if err != nil {
			DialogError("Error getting import source: " + err.Error())
			return
		}

		if importSource == "" {
			DialogError("Please enter an import source")
			return
		}

		// Handle the import
		importedApps, err := handleImport(importSource, piAppsDir)
		if err != nil {
			DialogError("Error importing app: " + err.Error())
			return
		}

		if len(importedApps) == 0 {
			DialogError("No apps were imported")
			return
		}

		// Show success dialog with imported apps
		showImportSuccessDialog(importedApps, piAppsDir)
		window.Close()
	})

	cancelButton.Connect("clicked", func() {
		window.Close()
	})

	window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	window.ShowAll()
	gtk.Main()

	return nil
}

// handleImport processes the import source and returns a list of imported app names
func handleImport(source, piAppsDir string) ([]string, error) {
	var importedApps []string

	// Expand environment variables in the source string
	expandedSource := os.ExpandEnv(source)

	// Handle different types of import sources
	switch {
	case strings.HasPrefix(expandedSource, "http") && strings.HasSuffix(expandedSource, ".zip"):
		// Download and extract zip file
		appName, err := importFromZipURL(expandedSource, piAppsDir)
		if err != nil {
			return nil, err
		}
		importedApps = append(importedApps, appName)
	case strings.HasPrefix(expandedSource, "/"):
		// Local file or directory
		switch {
		case strings.HasSuffix(expandedSource, ".zip"):
			appName, err := importFromLocalZip(expandedSource, piAppsDir)
			if err != nil {
				return nil, err
			}
			importedApps = append(importedApps, appName)
		case isDir(expandedSource):
			appName, err := importFromDirectory(expandedSource, piAppsDir)
			if err != nil {
				return nil, err
			}
			importedApps = append(importedApps, appName)
		default:
			return nil, fmt.Errorf("unsupported local file type")
		}
	case strings.Contains(expandedSource, "github.com") && strings.Contains(expandedSource, "/pull/"):
		// GitHub pull request
		apps, err := importFromPullRequest(expandedSource, piAppsDir)
		if err != nil {
			return nil, err
		}
		importedApps = append(importedApps, apps...)
	case isNumeric(expandedSource):
		// PR number
		account, repo := GetGitUrl()
		prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%s", account, repo, expandedSource)
		apps, err := importFromPullRequest(prURL, piAppsDir)
		if err != nil {
			return nil, err
		}
		importedApps = append(importedApps, apps...)
	default:
		return nil, fmt.Errorf("unsupported import source")
	}

	return importedApps, nil
}

// showImportSuccessDialog displays a dialog showing the successfully imported apps
func showImportSuccessDialog(apps []string, piAppsDir string) {
	// Create dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		DialogError("Error creating dialog: " + err.Error())
		return
	}
	defer dialog.Destroy()

	dialog.SetTitle("App Importer")
	dialog.SetDefaultSize(310, 200)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set icon
	iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
	if FileExists(iconPath) {
		if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
			dialog.SetIcon(pixbuf)
		}
	}

	// Create content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		DialogError("Error getting content area: " + err.Error())
		return
	}

	// Add text summary
	summaryText := fmt.Sprintf("These apps have been imported:\n%s", strings.Join(apps, "\n"))
	summaryLabel, err := gtk.LabelNew(summaryText)
	if err != nil {
		DialogError("Error creating summary label: " + err.Error())
		return
	}
	summaryLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.Add(summaryLabel)

	// Create list store for apps
	listStore, err := gtk.ListStoreNew(gdk.PixbufGetType(), glib.TYPE_STRING)
	if err != nil {
		DialogError("Error creating list store: " + err.Error())
		return
	}

	// Create tree view
	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		DialogError("Error creating tree view: " + err.Error())
		return
	}
	treeView.SetHeadersVisible(false)

	// Add columns
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		DialogError("Error creating icon renderer: " + err.Error())
		return
	}
	iconColumn, err := gtk.TreeViewColumnNew()
	if err != nil {
		DialogError("Error creating icon column: " + err.Error())
		return
	}
	iconColumn.PackStart(iconRenderer, false)
	iconColumn.AddAttribute(iconRenderer, "pixbuf", 0)
	treeView.AppendColumn(iconColumn)

	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		DialogError("Error creating name renderer: " + err.Error())
		return
	}
	nameColumn, err := gtk.TreeViewColumnNew()
	if err != nil {
		DialogError("Error creating name column: " + err.Error())
		return
	}
	nameColumn.PackStart(nameRenderer, true)
	nameColumn.AddAttribute(nameRenderer, "text", 1)
	treeView.AppendColumn(nameColumn)

	// Add apps to list store
	for _, app := range apps {
		iter := listStore.Append()
		var icon *gdk.Pixbuf
		iconPath := filepath.Join(piAppsDir, "apps", app, "icon-24.png")
		if FileExists(iconPath) {
			icon, _ = gdk.PixbufNewFromFile(iconPath)
		} else {
			icon, _ = gdk.PixbufNewFromFile(filepath.Join(piAppsDir, "icons", "none.png"))
		}
		listStore.SetValue(iter, 0, icon)
		listStore.SetValue(iter, 1, app)

		// Add to Imported category if not already categorized
		categoriesFile := filepath.Join(piAppsDir, "etc", "categories")
		overridesFile := filepath.Join(piAppsDir, "data", "category-overrides")

		// Check if app is already in categories
		inCategories := false
		if FileExists(categoriesFile) {
			content, _ := os.ReadFile(categoriesFile)
			inCategories = strings.Contains(string(content), app)
		}

		// Check if app is already in overrides
		inOverrides := false
		if FileExists(overridesFile) {
			content, _ := os.ReadFile(overridesFile)
			inOverrides = strings.Contains(string(content), app)
		}

		// Add to overrides if not in either file
		if !inCategories && !inOverrides {
			f, err := os.OpenFile(overridesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(fmt.Sprintf("%s|Imported\n", app))
				f.Close()
			}
		}
	}

	// Create scrolled window
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		DialogError("Error creating scrolled window: " + err.Error())
		return
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolled.Add(treeView)
	contentArea.Add(scrolled)

	// Add close button
	dialog.AddButton("Close", gtk.RESPONSE_CLOSE)

	dialog.ShowAll()
	dialog.Run()
}

// Helper functions for import handling

func importFromZipURL(url, piAppsDir string) (string, error) {
	// Download the zip file
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("error downloading zip file: %w", err)
	}
	defer resp.Body.Close()

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "pi-apps-import-*.zip")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Save the zip file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("error saving zip file: %w", err)
	}
	tmpFile.Close()

	// Import from the temporary zip file
	return importFromLocalZip(tmpFile.Name(), piAppsDir)
}

func importFromLocalZip(zipPath, piAppsDir string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("error opening zip file: %w", err)
	}
	defer reader.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "pi-apps-import-*")
	if err != nil {
		return "", fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract zip contents
	for _, file := range reader.File {
		// Normalize and validate the file path
		cleanPath := filepath.Join(tmpDir, filepath.Clean(file.Name))
		if !strings.HasPrefix(cleanPath, tmpDir) {
			return "", fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(cleanPath, 0755)
			continue
		}

		// Create parent directories
		os.MkdirAll(filepath.Dir(cleanPath), 0755)

		// Extract file
		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("error opening zip file entry: %w", err)
		}

		out, err := os.Create(cleanPath)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("error creating output file: %w", err)
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("error extracting file: %w", err)
		}
	}

	// Determine app name and validate structure
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error reading temporary directory: %w", err)
	}

	var appDir string
	var appName string

	if len(entries) == 1 && entries[0].IsDir() {
		// Single directory, use its name
		appName = entries[0].Name()
		appDir = filepath.Join(tmpDir, appName)
	} else {
		// Multiple files or no directory, use zip filename
		appName = strings.TrimSuffix(filepath.Base(zipPath), ".zip")
		appDir = tmpDir
	}

	// Validate app structure
	if err := validateAppStructure(appDir); err != nil {
		return "", fmt.Errorf("invalid app structure: %w", err)
	}

	// Move to apps directory
	targetDir := filepath.Join(piAppsDir, "apps", appName)
	os.RemoveAll(targetDir)
	err = os.Rename(appDir, targetDir)
	if err != nil {
		return "", fmt.Errorf("error moving app directory: %w", err)
	}

	return appName, nil
}

// validateAppStructure checks if the app directory has the required files
func validateAppStructure(appDir string) error {
	var missingFiles []string

	// Check for icon files
	hasIcon := false
	iconPatterns := []string{"icon-24.png", "icon-64.png"}
	for _, pattern := range iconPatterns {
		if FileExists(filepath.Join(appDir, pattern)) {
			hasIcon = true
			break
		}
	}
	if !hasIcon {
		missingFiles = append(missingFiles, "icon-*.png (24x24, 64x64)")
	}

	// Check for install files
	hasInstall := false
	installFiles := []string{"install", "install-32", "install-64", "packages"}
	for _, file := range installFiles {
		if FileExists(filepath.Join(appDir, file)) {
			hasInstall = true
			break
		}
	}
	if !hasInstall {
		missingFiles = append(missingFiles, "install/install-32/64 or packages")
	}

	// Check for description file
	if !FileExists(filepath.Join(appDir, "description")) {
		missingFiles = append(missingFiles, "description")
	}

	// If any required files are missing, return error
	if len(missingFiles) > 0 {
		return fmt.Errorf("missing required files: %s", strings.Join(missingFiles, ", "))
	}

	return nil
}

func importFromDirectory(dirPath, piAppsDir string) (string, error) {
	appName := filepath.Base(dirPath)
	appDir := filepath.Join(piAppsDir, "apps", appName)
	os.RemoveAll(appDir)
	err := os.Rename(dirPath, appDir)
	if err != nil {
		return "", fmt.Errorf("error moving app directory: %w", err)
	}
	return appName, nil
}

func importFromPullRequest(prURL, piAppsDir string) ([]string, error) {
	// Parse GitHub PR URL to extract owner, repo, and PR number
	parts := strings.Split(prURL, "/")
	if len(parts) < 7 || !strings.Contains(prURL, "github.com") || !strings.Contains(prURL, "/pull/") {
		return nil, fmt.Errorf("invalid GitHub PR URL format")
	}

	owner := parts[3]
	repo := parts[4]
	prNumber := parts[6]

	// Fetch PR information from GitHub API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%s", owner, repo, prNumber)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching PR information: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	// Parse PR response
	var prInfo struct {
		Head struct {
			Ref  string `json:"ref"`
			Sha  string `json:"sha"`
			Repo struct {
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&prInfo); err != nil {
		return nil, fmt.Errorf("error parsing PR information: %w", err)
	}

	// Download the PR branch as zip
	zipURL := fmt.Sprintf("https://github.com/%s/archive/%s.zip", prInfo.Head.Repo.FullName, prInfo.Head.Sha)
	return importFromPRZip(zipURL, piAppsDir, prInfo.Head.Ref)
}

// importFromPRZip downloads and imports apps from a GitHub PR zip
func importFromPRZip(zipURL, piAppsDir, branchName string) ([]string, error) {
	// Download the zip file
	resp, err := http.Get(zipURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading PR zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error downloading PR zip: status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "pi-apps-pr-*.zip")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Save the zip file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error saving PR zip: %w", err)
	}
	tmpFile.Close()

	// Extract zip to temporary directory
	reader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error opening PR zip: %w", err)
	}
	defer reader.Close()

	tmpDir, err := os.MkdirTemp("", "pi-apps-pr-*")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract zip contents
	for _, file := range reader.File {
		// Construct the path and resolve its absolute form
		path := filepath.Join(tmpDir, file.Name)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("error resolving absolute path: %w", err)
		}

		// Validate that the resolved path is within the intended directory
		if !strings.HasPrefix(absPath, tmpDir) {
			return nil, fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(absPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(absPath), 0755)

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("error opening zip entry: %w", err)
		}

		out, err := os.Create(absPath)
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("error creating output file: %w", err)
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("error extracting file: %w", err)
		}
	}

	// Find the extracted repository directory
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary directory: %w", err)
	}

	var repoDir string
	for _, entry := range entries {
		if entry.IsDir() {
			repoDir = filepath.Join(tmpDir, entry.Name())
			break
		}
	}

	if repoDir == "" {
		return nil, fmt.Errorf("could not find repository directory in PR zip")
	}

	// Look for apps in the apps directory
	appsDir := filepath.Join(repoDir, "apps")
	if !isDir(appsDir) {
		return nil, fmt.Errorf("no apps directory found in PR")
	}

	// Get list of apps from PR
	prApps, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("error reading PR apps directory: %w", err)
	}

	// Get list of existing apps
	existingAppsDir := filepath.Join(piAppsDir, "apps")
	existingApps := make(map[string]bool)
	if isDir(existingAppsDir) {
		existing, err := os.ReadDir(existingAppsDir)
		if err == nil {
			for _, app := range existing {
				if app.IsDir() {
					existingApps[app.Name()] = true
				}
			}
		}
	}

	// Import new apps
	var importedApps []string
	for _, app := range prApps {
		if !app.IsDir() {
			continue
		}

		appName := app.Name()
		appSourceDir := filepath.Join(appsDir, appName)

		// Validate app structure
		if err := validateAppStructure(appSourceDir); err != nil {
			continue // Skip invalid apps
		}

		// TODO: Add more sophisticated comparison for existing apps
		// For now, we'll import all apps (both new and updates)
		// In the future, we could add more sophisticated comparison that looks like this:
		// Check if it's a new app or significantly different
		// isNew := !existingApps[appName]
		// if !isNew {
		//	insert code here to check if the app is significantly different
		// }

		// Copy app to pi-apps directory
		targetDir := filepath.Join(piAppsDir, "apps", appName)
		os.RemoveAll(targetDir) // Remove existing if present

		if err := copyDir(appSourceDir, targetDir); err != nil {
			continue // Skip apps that fail to copy
		}

		importedApps = append(importedApps, appName)
	}

	if len(importedApps) == 0 {
		return nil, fmt.Errorf("no valid apps found in PR")
	}

	return importedApps, nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return CopyFile(path, dstPath)
	})
}

// Helper functions

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Helper function to get the git URL from the git_url file
//
//	account - the account name
//	repo - the repository name
func GetGitUrl() (account, repo string) {
	piAppsDir := GetPiAppsDir()
	gitURLPath := filepath.Join(piAppsDir, "etc", "git_url")
	if FileExists(gitURLPath) {
		// Read git URL from file
		gitURLBytes, err := os.ReadFile(gitURLPath)
		if err == nil {
			gitURL := strings.TrimSpace(string(gitURLBytes))

			// Parse account and repository from URL
			parts := strings.Split(gitURL, "/")
			if len(parts) >= 2 {
				account := parts[len(parts)-2]
				repo := parts[len(parts)-1]
				return account, repo
			}
		}
	}
	return account, repo
}
