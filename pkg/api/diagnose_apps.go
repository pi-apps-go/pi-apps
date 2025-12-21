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

// Module: diagnose_apps.go
// Description: Provides functions for diagnosing app failures.

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// AppError represents an application action that failed
type AppError struct {
	Action  string // "install", "uninstall", "update"
	AppName string // The name of the app
	LogFile string // Path to the log file
}

// DiagnoseResult contains the user's choice after diagnosis
type DiagnoseResult struct {
	Action    string // "send", "retry", "next", "close"
	AppName   string // The name of the app that was diagnosed
	ActionStr string // The original action string (e.g., "install;appname")
}

// GetLogfile returns the path to the log file for an app
func GetLogfile(appName string) string {
	if appName == "" {
		fmt.Println("Error: GetLogfile(): no app specified!")
		return ""
	}

	// Default location where Pi-Apps stores logs
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		piAppsDir = "."
	}

	logsDir := filepath.Join(piAppsDir, "logs")

	// Read the logs directory
	files, err := os.ReadDir(logsDir)
	if err != nil {
		fmt.Printf("Error reading logs directory: %v\n", err)
		return filepath.Join(piAppsDir, "logs", appName)
	}

	// Create a slice to store file info with modification times
	type FileInfo struct {
		path    string
		modTime time.Time
	}
	var fileInfos []FileInfo

	// Filter files and get their modification times
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()

		// Skip success logs (matching the bash version's grep -v 'success')
		if strings.Contains(fileName, "success") {
			continue
		}

		// Look for files matching log file patterns:
		// - "action-fail-appname.log" (e.g., "install-fail-Eagle CAD.log")
		// - "action-incomplete-appname.log" (e.g., "install-incomplete-Eagle CAD.log")
		// - Files containing "-appname-" and ending with ".log"
		// - Files ending with "-appname.log"
		matches := false
		if strings.HasSuffix(fileName, ".log") {
			// Check for patterns: action-fail-appname.log or action-incomplete-appname.log
			if strings.HasSuffix(fileName, fmt.Sprintf("-%s.log", appName)) {
				matches = true
			} else if strings.Contains(fileName, fmt.Sprintf("-fail-%s.log", appName)) {
				matches = true
			} else if strings.Contains(fileName, fmt.Sprintf("-incomplete-%s.log", appName)) {
				matches = true
			} else if strings.Contains(fileName, fmt.Sprintf("-%s-", appName)) {
				// Pattern with dashes on both sides
				matches = true
			}
		}

		if matches {
			filePath := filepath.Join(logsDir, fileName)
			fileInfo, err := os.Stat(filePath)
			if err == nil {
				fileInfos = append(fileInfos, FileInfo{
					path:    filePath,
					modTime: fileInfo.ModTime(),
				})
			}
		}
	}

	// Sort files by modification time (newest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	// Return the most recent matching log file, or default if none found
	if len(fileInfos) > 0 {
		return fileInfos[0].path
	}

	// Return the default path if no matching logs found
	return filepath.Join(piAppsDir, "logs", appName)
}

// CapitalizeFirst capitalizes the first letter of a string
func CapitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// CheckCanSendErrorReport determines if we can send an error report for this app
func CheckCanSendErrorReport(app, action, errorType string) (bool, string) {
	// Setup variables similar to how the bash script does
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return false, "PI_APPS_DIR environment variable is not set"
	}

	// Check 1: Check if app is a package app
	appType, err := AppType(app)
	if err != nil {
		return false, fmt.Sprintf("Error checking app type: %v", err)
	}
	if appType == "package" {
		return false, PackageAppNoErrorReporting
	}

	// Check 2: Check error type - cannot send reports for system, internet, or package errors
	if errorType == "system" || errorType == "internet" || errorType == "package" {
		return false, "Error report cannot be sent because this is not an issue with Pi-Apps."
	}

	// Check 3: Check if app is in the official repository
	apps, err := ListApps("online")
	if err != nil {
		return false, fmt.Sprintf("Error checking official repository: %v", err)
	}
	found := false
	for _, officialApp := range apps {
		if officialApp == app {
			found = true
			break
		}
	}
	if !found {
		return false, "Error report cannot be sent because this app is not in the official repository."
	}

	// Check 4 & 5: Check if app script matches the official version
	switch action {
	case "install":
		scriptName, err := ScriptNameCPU(app)
		if err != nil {
			return false, "Error report cannot be sent because the script name couldn't be determined."
		}

		// Check if files match
		match, err := FilesMatch(
			filepath.Join(directory, "update", "pi-apps", "apps", app, scriptName),
			filepath.Join(directory, "apps", app, scriptName),
		)
		if err != nil {
			return false, fmt.Sprintf("Error checking file match: %v", err)
		}
		if !match {
			return false, "Error report cannot be sent because this app is not the official version."
		}
	case "uninstall":
		// Check if uninstall script matches
		match, err := FilesMatch(
			filepath.Join(directory, "update", "pi-apps", "apps", app, "uninstall"),
			filepath.Join(directory, "apps", app, "uninstall"),
		)
		if err != nil {
			return false, fmt.Sprintf("Error checking file match: %v", err)
		}
		if !match {
			return false, "Error report cannot be sent because this app is not the official version."
		}
	}

	// Check 6: Check if system is supported
	supportStatus, err := IsSystemSupported()
	if err != nil {
		return false, fmt.Sprintf("Error checking system support: %v", err)
	}
	if !supportStatus.IsSupported {
		return false, "Error report cannot be sent because your system is unsupported: " + supportStatus.Message
	}

	return true, "" // Can send error report
}

// loadImage loads an image from a file path and returns a new GtkImage
func loadImage(path string) (*gtk.Image, error) {
	pixbuf, err := gdk.PixbufNewFromFile(path)
	if err != nil {
		return nil, err
	}

	// Scale the image if it's too large
	width := pixbuf.GetWidth()
	height := pixbuf.GetHeight()

	if width > 64 || height > 64 {
		scale := 64.0 / float64(width)
		if height > width {
			scale = 64.0 / float64(height)
		}

		newWidth := int(float64(width) * scale)
		newHeight := int(float64(height) * scale)

		pixbuf, err = pixbuf.ScaleSimple(newWidth, newHeight, gdk.INTERP_BILINEAR)
		if err != nil {
			return nil, err
		}
	}

	image, err := gtk.ImageNewFromPixbuf(pixbuf)
	if err != nil {
		return nil, err
	}

	return image, nil
}

// DiagnoseApps presents GTK3-based error diagnosis dialogs for a list of failed actions
// failureList format: "action;app" entries separated by newlines
func DiagnoseApps(failureList string) []DiagnoseResult {
	// Debug output
	fmt.Printf("Diagnosing app failures: %s\n", failureList)

	// Set program name
	glib.SetPrgname("Pi-Apps")

	// Initialize GTK
	gtk.Init(nil)

	// Split the failure list into lines
	failures := strings.Split(strings.TrimSpace(failureList), "\n")
	numFailures := len(failures)
	fmt.Printf("Found %d failures to diagnose\n", numFailures)

	var results []DiagnoseResult

	// Process each failure
	for i, failure := range failures {
		if failure == "" {
			continue
		}

		// Parse action and app name
		parts := strings.SplitN(failure, ";", 2)
		if len(parts) != 2 {
			WarningT("Invalid failure format: %s (expected 'action;app')\n", failure)
			continue
		}
		action := parts[0]
		appName := parts[1]

		fmt.Printf("Diagnosing %s action for app: %s\n", action, appName)

		// Get log file path
		logFile := GetLogfile(appName)
		fmt.Printf("Using logfile: %s\n", logFile)

		if !FileExists(logFile) {
			WarningT("Log file does not exist: %s\n", logFile)
			// Attempt to create a blank log file for diagnosis
			os.WriteFile(logFile, []byte("No log file found for this app."), 0644)
		}

		// Diagnose the error
		diagnosis, err := LogDiagnose(logFile, true)
		if err != nil {
			fmt.Printf("Error diagnosing log: %v\n", err)
			continue // Skip if diagnosis fails
		}

		errorType := diagnosis.ErrorType
		errorCaption := strings.Join(diagnosis.Captions, "\n")
		fmt.Printf("Diagnosis found error type: %s\n", errorType)

		// Create the dialog window
		dialog, err := gtk.DialogNew()
		if err != nil {
			fmt.Printf("Error creating dialog: %v\n", err)
			continue // Skip if dialog creation fails
		}
		dialog.SetTitle(fmt.Sprintf("Error occurred when %sing %s (%d/%d)",
			strings.Replace(action, "update", "updat", 1), appName, i+1, numFailures))
		dialog.SetModal(true)
		dialog.SetDefaultSize(700, 400)

		// Set dialog class - for proper styling
		dialog.SetName("Pi-Apps")

		// Get the content area
		contentArea, err := dialog.GetContentArea()
		if err != nil {
			dialog.Destroy()
			continue
		}
		contentArea.SetSpacing(12)
		contentArea.SetMarginStart(12)
		contentArea.SetMarginEnd(12)
		contentArea.SetMarginTop(12)
		contentArea.SetMarginBottom(12)

		// Create header box
		headerBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
		if err != nil {
			dialog.Destroy()
			continue
		}

		// Add error icon
		iconPath := filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "error.png")
		if icon, err := loadImage(iconPath); err == nil {
			headerBox.PackStart(icon, false, false, 0)
		}

		// Prepare header text
		var headerText string
		if errorType == "unknown" {
			headerText = fmt.Sprintf("<b>%s</b> failed to %s for an <b>unknown</b> reason.",
				CapitalizeFirst(appName), action)
		} else {
			article := "a"
			if strings.Contains("aeiou", string(errorType[0])) {
				article = "an"
			}
			headerText = fmt.Sprintf("<b>%s</b> failed to %s because Pi-Apps encountered %s <b>%s</b> error.",
				CapitalizeFirst(appName), action, article, errorType)
		}

		// Check if we can send an error report
		canSend, reason := CheckCanSendErrorReport(appName, action, errorType)
		if !canSend {
			headerText += "\n" + reason
		}

		// Add support links
		appType, _ := AppType(appName)

		// TODO: Change this below message depending on the package manager being used.
		if appType == "package" {
			headerText += "\n" + PackageAppErrorMessage
		} else {
			account, repo := GetGitUrl()
			if account == "" || repo == "" {
				headerText += "\nSupport is available on <a href=\"https://discord.gg/RXSTvaUvuu\">Discord</a> and <a href=\"https://github.com/pi-apps-go/pi-apps-go/issues/new/choose\">Github</a>."
			} else {
				headerText += fmt.Sprintf("\nSupport is available on <a href=\"https://discord.gg/RXSTvaUvuu\">Discord</a> and <a href=\"https://github.com/%s/%s/issues/new/choose\">Github</a>.", account, repo)
			}
		}

		// If we have no error caption, tell user to view the log
		if errorCaption == "" {
			headerText += "\nYou can view the terminal output below. (scroll down)"
			// Get content of log file
			content, err := os.ReadFile(logFile)
			if err == nil {
				errorCaption = string(content)
			}
		} else {
			headerText += "\nBelow, Pi-Apps explains what went wrong and how you can fix it."
		}

		// Create the header label with rich text
		headerLabel, err := gtk.LabelNew("")
		if err != nil {
			dialog.Destroy()
			continue
		}
		headerLabel.SetMarkup(headerText)
		headerLabel.SetLineWrap(true)
		headerLabel.SetMaxWidthChars(80)
		headerLabel.SetJustify(gtk.JUSTIFY_LEFT)
		headerLabel.SetHAlign(gtk.ALIGN_START)
		headerLabel.SetVAlign(gtk.ALIGN_START)
		headerBox.PackStart(headerLabel, true, true, 0)

		contentArea.PackStart(headerBox, false, false, 0)

		// Create scrolled window for log text
		scrollWin, err := gtk.ScrolledWindowNew(nil, nil)
		if err != nil {
			dialog.Destroy()
			continue
		}
		scrollWin.SetHExpand(true)
		scrollWin.SetVExpand(true)
		scrollWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)

		// Create text view for error caption
		textView, err := gtk.TextViewNew()
		if err != nil {
			dialog.Destroy()
			continue
		}
		textView.SetEditable(false)
		textView.SetWrapMode(gtk.WRAP_WORD_CHAR)

		buffer, err := textView.GetBuffer()
		if err != nil {
			dialog.Destroy()
			continue
		}
		buffer.SetText(errorCaption)

		scrollWin.Add(textView)
		contentArea.PackStart(scrollWin, true, true, 0)

		// Add View Log button as action widget with custom response ID
		viewLogButton, err := gtk.ButtonNewWithLabel("View Log")
		if err != nil {
			dialog.Destroy()
			continue
		}
		// Try to add icon
		iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "log-file.png")
		if icon, err := gtk.ImageNewFromFile(iconPath); err == nil {
			viewLogButton.SetImage(icon)
			viewLogButton.SetAlwaysShowImage(true)
		}
		// Add to dialog with custom response ID (we'll handle this specially)
		dialog.AddActionWidget(viewLogButton, 100) // Custom response ID for View Log

		// Send Report button (if applicable)
		if canSend {
			sendReportButton, err := gtk.ButtonNewWithLabel("Send Report")
			if err != nil {
				dialog.Destroy()
				continue
			}
			// Try to add icon
			iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "send-error-report.png")
			if icon, err := gtk.ImageNewFromFile(iconPath); err == nil {
				sendReportButton.SetImage(icon)
				sendReportButton.SetAlwaysShowImage(true)
			}
			dialog.AddActionWidget(sendReportButton, gtk.RESPONSE_APPLY) // Custom response for Send Report
		}

		// Retry button
		retryButton, err := gtk.ButtonNewWithLabel("Retry")
		if err != nil {
			dialog.Destroy()
			continue
		}
		// Try to add icon
		iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "refresh.png")
		if icon, err := gtk.ImageNewFromFile(iconPath); err == nil {
			retryButton.SetImage(icon)
			retryButton.SetAlwaysShowImage(true)
		}
		dialog.AddActionWidget(retryButton, gtk.RESPONSE_OK) // RESPONSE_OK for Retry

		// Close/Next button
		var closeButton *gtk.Button
		if i < numFailures-1 {
			closeButton, err = gtk.ButtonNewWithLabel("Next Error")
			if err != nil {
				dialog.Destroy()
				continue
			}
			// Try to add icon
			iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "forward.png")
			if icon, err := gtk.ImageNewFromFile(iconPath); err == nil {
				closeButton.SetImage(icon)
				closeButton.SetAlwaysShowImage(true)
			}
		} else {
			closeButton, err = gtk.ButtonNewWithLabel("Close")
			if err != nil {
				dialog.Destroy()
				continue
			}
			// Try to add icon
			iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "exit.png")
			if icon, err := gtk.ImageNewFromFile(iconPath); err == nil {
				closeButton.SetImage(icon)
				closeButton.SetAlwaysShowImage(true)
			}
		}
		dialog.AddActionWidget(closeButton, gtk.RESPONSE_CANCEL) // RESPONSE_CANCEL for Close/Next

		// Show all widgets
		dialog.ShowAll()

		// Run the dialog and process the response in a loop to handle View Log
		for {
			response := dialog.Run()

			// Process response
			switch response {
			case 100: // View Log - handle without closing dialog
				// Get the directory where the binary is running from
				exePath, err := os.Executable()
				if err != nil {
					fmt.Printf("Error getting executable path: %v\n", err)
					continue // Stay in loop, don't close dialog
				}

				// Launch a separate process for viewing the log file
				// to avoid conflicts with the current GTK main loop
				cmd := exec.Command(exePath, "view_file", logFile)
				cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"))
				err = cmd.Start()
				if err != nil {
					fmt.Printf("Error opening log viewer: %v\n", err)
				}
				// Continue the loop to keep dialog open
				continue
			case gtk.RESPONSE_OK: // Retry
				results = append(results, DiagnoseResult{
					Action:    "retry",
					AppName:   appName,
					ActionStr: failure,
				})
			case gtk.RESPONSE_APPLY: // Send Report
				results = append(results, DiagnoseResult{
					Action:    "send",
					AppName:   appName,
					ActionStr: failure,
				})
			case gtk.RESPONSE_CANCEL: // Close/Next
				results = append(results, DiagnoseResult{
					Action:    "next",
					AppName:   appName,
					ActionStr: failure,
				})
			default: // Any other response (e.g., window closed)
				results = append(results, DiagnoseResult{
					Action:    "close",
					AppName:   appName,
					ActionStr: failure,
				})
			}
			// Exit the loop to close dialog
			break
		}

		// Destroy the dialog
		dialog.Destroy()
	}

	// Process GTK events to ensure proper cleanup
	for gtk.EventsPending() {
		gtk.MainIteration()
	}

	return results
}

// ProcessSendErrorReport handles sending an error report from the UI
func ProcessSendErrorReport(logfilePath string) (string, error) {
	// This leverages the existing SendErrorReport function that's already implemented
	return SendErrorReport(logfilePath)
}
