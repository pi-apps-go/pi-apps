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

// Module: createapp.go
// Description: Provides functions for creating new apps.

package api

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

// commandExists checks if a command is available in the system PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// CreateApp provides a graphical interface for creating new apps in Pi-Apps Go
//
//	appName - the name of the app to edit, or empty to create a new app
func CreateApp(appName string) error {

	// Initialize application name
	glib.SetPrgname("Pi-Apps-Settings")
	glib.SetApplicationName("Pi-Apps Settings (app creation wizard)")

	// Initialize GTK
	gtk.Init(nil)

	// Check if we can use GTK
	if !canUseGTK() {
		return fmt.Errorf("createapp requires a GUI environment")
	}

	// Get the Pi-Apps directory
	piAppsDir := GetPiAppsDir()
	if piAppsDir == "" {
		return fmt.Errorf("failed to get Pi-Apps directory")
	}
	// For debugging, print out the directory
	fmt.Println("Pi-Apps directory:", piAppsDir)

	// Check if template directory exists
	templateDir := filepath.Join(piAppsDir, "apps", "template")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return fmt.Errorf("template directory not found at %s", templateDir)
	}

	// State variables for the wizard
	var (
		step       int         = 0
		appType    string      = ""
		appDetails *AppDetails = nil
	)

	// If an app name was provided, start at step 2 and set editing mode
	if appName != "" {
		step = 2
		// We're in editing mode

		// Determine app type
		dir := filepath.Join(piAppsDir, "apps", appName)
		pkgFile := filepath.Join(dir, "packages")
		instFile := filepath.Join(dir, "install")
		inst32File := filepath.Join(dir, "install-32")
		inst64File := filepath.Join(dir, "install-64")

		if _, err := os.Stat(pkgFile); err == nil {
			appType = "package"
		} else if _, err := os.Stat(instFile); err == nil {
			appType = "standard"
		} else {
			// Check for install-32 and install-64 files
			_, err1 := os.Stat(inst32File)
			_, err2 := os.Stat(inst64File)

			if err1 == nil || err2 == nil {
				appType = "standard"
			}
		}
	}

	// Main wizard loop
	for {
		// Log the current state
		fmt.Printf("\nName: %s\nAppType: %s\nStep: %d\n", appName, appType, step)

		// Process the current step
		switch step {
		case 0:
			// Introduction step
			result, err := showIntroDialog()
			if err != nil {
				return err
			}

			if result == "Next" {
				step++
			} else {
				return nil // User cancelled
			}

		case 1:
			// Choose app name and type
			result, name, appt, err := showBasicsDialog(appName, appType)
			if err != nil {
				return err
			}

			if result == "Next" {
				appName = name
				appType = appt

				// Make sure we have a valid app type
				if appType == "" {
					appType = "standard" // Default to standard if not specified
				}

				// Create app directory if it doesn't exist
				appDir := filepath.Join(piAppsDir, "apps", appName)
				if _, err := os.Stat(appDir); os.IsNotExist(err) {
					if err := os.MkdirAll(appDir, 0755); err != nil {
						return fmt.Errorf("failed to create app directory: %v", err)
					}
				}

				step++
			} else if result == "Previous" {
				step--
			} else {
				return nil // User cancelled
			}

		case 2:
			// App information step (icon, website, description, etc.)
			result, details, err := showAppDetailsDialog(appName, appType)
			if err != nil {
				return err
			}

			appDetails = details // Store details for use in later steps

			if result == "Next" {
				// Process the entered details
				if appDetails.Icon != "" {
					if err := GenerateAppIcons(appDetails.Icon, appName); err != nil {
						Warning(fmt.Sprintf("Failed to generate icons: %v\n", err))
					}
				}

				// Save website if provided
				if appDetails.Website != "" {
					websiteFile := filepath.Join(piAppsDir, "apps", appName, "website")
					if err := os.WriteFile(websiteFile, []byte(appDetails.Website), 0644); err != nil {
						Warning(fmt.Sprintf("Failed to save website: %v\n", err))
					}
				}

				// Save description if provided
				if appDetails.Description != "" {
					descFile := filepath.Join(piAppsDir, "apps", appName, "description")
					if err := os.WriteFile(descFile, []byte(appDetails.Description), 0644); err != nil {
						Warning(fmt.Sprintf("Failed to save description: %v\n", err))
					}
				}

				// Save credits if provided
				if appDetails.Credits != "" {
					creditsFile := filepath.Join(piAppsDir, "apps", appName, "credits")
					if err := os.WriteFile(creditsFile, []byte(appDetails.Credits), 0644); err != nil {
						Warning(fmt.Sprintf("Failed to save credits: %v\n", err))
					}
				}

				// For package apps, save packages
				if appType == "package" && appDetails.Packages != "" {
					pkgFile := filepath.Join(piAppsDir, "apps", appName, "packages")
					if err := os.WriteFile(pkgFile, []byte(appDetails.Packages), 0644); err != nil {
						Warning(fmt.Sprintf("Failed to save packages: %v\n", err))
					}

					// For package apps, we're done - skip to the final step
					step4Dialog := createAppPreviewDialog(appName, piAppsDir)
					previewResponse := step4Dialog.Run()
					step4Dialog.Destroy()

					if previewResponse == gtk.RESPONSE_OK {
						// Show final success dialog
						showSuccessDialog(appName, piAppsDir)
					} else if previewResponse == gtk.RESPONSE_CANCEL {
						// Go back to test dialog
						continue
					} else {
						// If user closes dialog with X button, exit
						return nil
					}
				}

				step++
			} else if result == "Previous" {
				step--
			} else if result == "Save" {
				// If Save was clicked, exit after saving changes
				return nil
			} else {
				return nil // User cancelled
			}

		case 3:
			// This case follows case 2, so appDetails from case 2 is accessible here
			// For standard apps, handle the compatibility selection
			var scriptType string

			// Use the compatibility from the appDetails struct
			if appDetails != nil && appDetails.Compatibility == "32bit only" {
				scriptType = "install-32"
			} else if appDetails != nil && appDetails.Compatibility == "64bit only" {
				scriptType = "install-64"
			} else {
				// Check if they want 1 combined or 2 separate scripts
				scriptDialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_NONE,
					"Do you want two install scripts, one for 32bit and the other for 64bit?\nOr do you want one combined install script?")

				prevButton, _ := scriptDialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
				twoScriptsButton, _ := scriptDialog.AddButton("2 scripts", gtk.RESPONSE_REJECT)
				oneScriptButton, _ := scriptDialog.AddButton("1 script", gtk.RESPONSE_ACCEPT)

				// Add icons to the buttons
				backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
				if backIcon != nil {
					prevButton.SetImage(backIcon)
					prevButton.SetAlwaysShowImage(true)
					prevButton.SetImagePosition(gtk.POS_LEFT)
				}

				// Use appropriate icons for script choice buttons
				scriptIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "shellscript.png"))
				scriptIconMulti, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "shellscript-multi.png"))
				if scriptIcon != nil {
					twoScriptsButton.SetImage(scriptIconMulti)
					twoScriptsButton.SetAlwaysShowImage(true)
					twoScriptsButton.SetImagePosition(gtk.POS_LEFT)

					oneScriptButton.SetImage(scriptIcon)
					oneScriptButton.SetAlwaysShowImage(true)
					oneScriptButton.SetImagePosition(gtk.POS_LEFT)
				}

				scriptDialog.SetName("Pi-Apps Settings")

				// Set window icon
				iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
				if _, err := os.Stat(iconPath); err == nil {
					scriptDialog.SetIconFromFile(iconPath)
				}

				scriptDialog.Connect("delete-event", func() bool {
					scriptDialog.Response(gtk.RESPONSE_DELETE_EVENT)
					return true
				})

				dialogResponse := scriptDialog.Run()
				scriptDialog.Destroy()

				if dialogResponse == gtk.RESPONSE_CANCEL {
					fmt.Println("Debug: Going back from step 3 to step 2")
					step--
					continue
				} else if dialogResponse == gtk.RESPONSE_REJECT {
					scriptType = "install-32-and-64"
				} else if dialogResponse == gtk.RESPONSE_ACCEPT {
					scriptType = "install"
				} else {
					// If user closes dialog by clicking X, exit program
					return nil
				}
			}

			// Create app directory if it doesn't exist
			appDir := filepath.Join(piAppsDir, "apps", appName)
			if _, err := os.Stat(appDir); os.IsNotExist(err) {
				if err := os.MkdirAll(appDir, 0755); err != nil {
					return fmt.Errorf("failed to create app directory: %v", err)
				}
			}

			// Create empty install script template
			emptyInstallScript := "#!/bin/bash\n\n\n# Be sure to use the \"error\" function - it will display a message if a command fails to run. Example below:\n\ngit_clone https://example.com | error \"Failed to clone repository!\"\n\n# Install some packages that are necessary to run this app - no need for \"error\", as the install_packages function already handles errors.\ninstall_packages package1 package2 package3 || exit 1\n"

			// Create script files based on selected type
			switch scriptType {
			case "install":
				// Create single install script
				installPath := filepath.Join(appDir, "install")
				if err := os.WriteFile(installPath, []byte(emptyInstallScript), 0755); err != nil {
					return fmt.Errorf("failed to create install script: %v", err)
				}

				// Open the script in preferred text editor
				OpenFile(installPath)

			case "install-32-and-64":
				// Create install-32 script
				installPath32 := filepath.Join(appDir, "install-32")
				if err := os.WriteFile(installPath32, []byte(emptyInstallScript), 0755); err != nil {
					return fmt.Errorf("failed to create install-32 script: %v", err)
				}

				// Create install-64 script
				installPath64 := filepath.Join(appDir, "install-64")
				if err := os.WriteFile(installPath64, []byte(emptyInstallScript), 0755); err != nil {
					return fmt.Errorf("failed to create install-64 script: %v", err)
				}

				// Open the scripts in preferred text editor - make sure both open, with delay between them
				OpenFile(installPath32)

				// Small delay to prevent potential race conditions
				time.Sleep(500 * time.Millisecond)
				OpenFile(installPath64)

			case "install-32":
				// Create install-32 script only
				installPath32 := filepath.Join(appDir, "install-32")
				if err := os.WriteFile(installPath32, []byte(emptyInstallScript), 0755); err != nil {
					return fmt.Errorf("failed to create install-32 script: %v", err)
				}

				// Open the script in preferred text editor
				OpenFile(installPath32)

			case "install-64":
				// Create install-64 script only
				installPath64 := filepath.Join(appDir, "install-64")
				if err := os.WriteFile(installPath64, []byte(emptyInstallScript), 0755); err != nil {
					return fmt.Errorf("failed to create install-64 script: %v", err)
				}

				// Open the script in preferred text editor
				OpenFile(installPath64)
			}

			// Create uninstall script if it doesn't exist
			uninstallPath := filepath.Join(piAppsDir, "apps", appName, "uninstall")
			if _, err := os.Stat(uninstallPath); os.IsNotExist(err) {
				emptyUninstallScript := "#!/bin/bash\n\n# Allow packages required by this app to be uninstalled\npurge_packages || exit 1\n"
				if err := os.WriteFile(uninstallPath, []byte(emptyUninstallScript), 0755); err != nil {
					Warning(fmt.Sprintf("Failed to create uninstall script: %v\n", err))
				}
			}

			// Open the script in preferred text editor
			OpenFile(uninstallPath)

			// Main testing dialog loop - continue this loop until the user proceeds forward or exits
			for {
				// Create a dialog for testing scripts similar to the original bash script
				testDialog, err := gtk.DialogNew()
				if err != nil {
					return fmt.Errorf("failed to create test dialog: %v", err)
				}
				// Don't use defer here, we'll destroy it manually when needed

				// Set dialog properties
				testDialog.SetTitle("Test Install Scripts")
				testDialog.SetDefaultSize(500, 400)
				testDialog.SetPosition(gtk.WIN_POS_CENTER)

				// Set the dialog class to match original script
				testDialog.SetName("Pi-Apps Settings")

				// Set window icon
				iconPath := filepath.Join(piAppsDir, "icons", "settings.png")
				if _, err := os.Stat(iconPath); err == nil {
					testDialog.SetIconFromFile(iconPath)
				}

				// Get the content area
				contentArea, err := testDialog.GetContentArea()
				if err != nil {
					testDialog.Destroy() // Clean up if there's an error
					return fmt.Errorf("failed to get content area: %v", err)
				}
				contentArea.SetMarginTop(15)
				contentArea.SetMarginBottom(15)
				contentArea.SetMarginStart(15)
				contentArea.SetMarginEnd(15)
				contentArea.SetSpacing(10)

				// Add title
				titleLabel, _ := gtk.LabelNew("")
				titleLabel.SetMarkup("<span font_size='large'>Testing Install Scripts</span>")
				titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
				contentArea.Add(titleLabel)

				// Add description
				descLabel, _ := gtk.LabelNew("")
				descLabel.SetMarkup("Now it's time to test your install scripts. These will be executed when somebody clicks your app's Install button.\n\n" +
					"A text editor should have opened and you can create your install script.\n" +
					"Need help? <a href=\"https://pi-apps.io/wiki/development/Creating-an-app/\">Read the tutorial!</a>\n" +
					"Still need help? Open an issue on the GitHub repository.")
				descLabel.SetLineWrap(true)
				descLabel.SetJustify(gtk.JUSTIFY_CENTER)
				contentArea.Add(descLabel)

				// Create a grid for the buttons
				grid, _ := gtk.GridNew()
				grid.SetColumnSpacing(10)
				grid.SetRowSpacing(10)
				contentArea.Add(grid)

				var row int

				// Add buttons based on which script types were created
				appDir = filepath.Join(piAppsDir, "apps", appName)
				installPath := filepath.Join(appDir, "install")
				installPath32 := filepath.Join(appDir, "install-32")
				installPath64 := filepath.Join(appDir, "install-64")
				// Don't redeclare uninstallPath, reuse the one from above

				// Define icon paths - using an existing icon file for run buttons
				runIconPath := filepath.Join(piAppsDir, "icons", "install.png")
				uninstallIconPath := filepath.Join(piAppsDir, "icons", "uninstall.png")
				checkIconPath := filepath.Join(piAppsDir, "icons", "check.png")

				// Check if icons exist and log path for debugging
				_, runIconExists := os.Stat(runIconPath)
				_, checkIconExists := os.Stat(checkIconPath)
				fmt.Printf("Run icon path: %s (exists: %v)\n", runIconPath, runIconExists == nil)
				fmt.Printf("Check icon path: %s (exists: %v)\n", checkIconPath, checkIconExists == nil)

				// These will be reused for all buttons
				var runIcon, checkIcon *gtk.Image
				var iconErr error

				// Check which scripts exist and add corresponding buttons
				if _, err := os.Stat(installPath); err == nil {
					// For single install script
					runInstallBtn, _ := gtk.ButtonNewWithLabel("Run install script")
					runInstallBtn.Connect("clicked", func() {
						runScript(installPath, appName)
					})

					// Create and set button icon - include error handling
					runIcon, iconErr = gtk.ImageNewFromFile(runIconPath)
					if iconErr == nil {
						runIcon.Show()
						runInstallBtn.SetImage(runIcon)
						runInstallBtn.SetAlwaysShowImage(true)
						runInstallBtn.SetImagePosition(gtk.POS_LEFT)
					} else {
						fmt.Printf("Error loading run icon: %v\n", iconErr)
					}

					grid.Attach(runInstallBtn, 0, row, 1, 1)

					shellcheckInstallBtn, _ := gtk.ButtonNewWithLabel("Shellcheck install")
					shellcheckInstallBtn.Connect("clicked", func() {
						runShellcheck(installPath)
					})

					// Create and set button icon - include error handling
					checkIcon, iconErr = gtk.ImageNewFromFile(checkIconPath)
					if iconErr == nil {
						checkIcon.Show()
						shellcheckInstallBtn.SetImage(checkIcon)
						shellcheckInstallBtn.SetAlwaysShowImage(true)
						shellcheckInstallBtn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(shellcheckInstallBtn, 1, row, 1, 1)
					row++
				}

				// Check for install-32 script
				if _, err := os.Stat(installPath32); err == nil {
					var runInstall32Btn *gtk.Button
					runInstall32Btn, _ = gtk.ButtonNewWithLabel("Run install-32 script")
					runInstall32Btn.Connect("clicked", func() {
						runScript(installPath32, appName)
					})

					// Create and set button icon - include error handling
					runIcon, iconErr = gtk.ImageNewFromFile(runIconPath)
					if iconErr == nil {
						runIcon.Show()
						runInstall32Btn.SetImage(runIcon)
						runInstall32Btn.SetAlwaysShowImage(true)
						runInstall32Btn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(runInstall32Btn, 0, row, 1, 1)

					shellcheckInstall32Btn, _ := gtk.ButtonNewWithLabel("Shellcheck install-32")
					shellcheckInstall32Btn.Connect("clicked", func() {
						runShellcheck(installPath32)
					})

					// Create and set button icon - include error handling
					checkIcon, iconErr = gtk.ImageNewFromFile(checkIconPath)
					if iconErr == nil {
						checkIcon.Show()
						shellcheckInstall32Btn.SetImage(checkIcon)
						shellcheckInstall32Btn.SetAlwaysShowImage(true)
						shellcheckInstall32Btn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(shellcheckInstall32Btn, 1, row, 1, 1)
					row++
				}

				// Check for install-64 script
				if _, err := os.Stat(installPath64); err == nil {
					runInstall64Btn, _ := gtk.ButtonNewWithLabel("Run install-64 script")
					runInstall64Btn.Connect("clicked", func() {
						runScript(installPath64, appName)
					})

					// Create and set button icon - include error handling
					runIcon, iconErr = gtk.ImageNewFromFile(runIconPath)
					if iconErr == nil {
						runIcon.Show()
						runInstall64Btn.SetImage(runIcon)
						runInstall64Btn.SetAlwaysShowImage(true)
						runInstall64Btn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(runInstall64Btn, 0, row, 1, 1)

					shellcheckInstall64Btn, _ := gtk.ButtonNewWithLabel("Shellcheck install-64")
					shellcheckInstall64Btn.Connect("clicked", func() {
						runShellcheck(installPath64)
					})

					// Create and set button icon - include error handling
					checkIcon, iconErr = gtk.ImageNewFromFile(checkIconPath)
					if iconErr == nil {
						checkIcon.Show()
						shellcheckInstall64Btn.SetImage(checkIcon)
						shellcheckInstall64Btn.SetAlwaysShowImage(true)
						shellcheckInstall64Btn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(shellcheckInstall64Btn, 1, row, 1, 1)
					row++
				}

				// Add uninstall script buttons
				if _, err := os.Stat(uninstallPath); err == nil {
					runUninstallBtn, _ := gtk.ButtonNewWithLabel("Run uninstall script")
					runUninstallBtn.Connect("clicked", func() {
						runScript(uninstallPath, appName)
					})

					// Create and set button icon - include error handling
					runIcon, iconErr = gtk.ImageNewFromFile(uninstallIconPath)
					if iconErr == nil {
						runIcon.Show()
						runUninstallBtn.SetImage(runIcon)
						runUninstallBtn.SetAlwaysShowImage(true)
						runUninstallBtn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(runUninstallBtn, 0, row, 1, 1)

					shellcheckUninstallBtn, _ := gtk.ButtonNewWithLabel("Shellcheck uninstall")
					shellcheckUninstallBtn.Connect("clicked", func() {
						runShellcheck(uninstallPath)
					})

					// Create and set button icon - include error handling
					checkIcon, iconErr = gtk.ImageNewFromFile(checkIconPath)
					if iconErr == nil {
						checkIcon.Show()
						shellcheckUninstallBtn.SetImage(checkIcon)
						shellcheckUninstallBtn.SetAlwaysShowImage(true)
						shellcheckUninstallBtn.SetImagePosition(gtk.POS_LEFT)
					}

					grid.Attach(shellcheckUninstallBtn, 1, row, 1, 1)
				}

				previousBtn, _ := testDialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
				backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
				if backIcon != nil {
					previousBtn.SetImage(backIcon)
					previousBtn.SetAlwaysShowImage(true)
					previousBtn.SetImagePosition(gtk.POS_LEFT)
				}

				nextBtn, _ := testDialog.AddButton("Next", gtk.RESPONSE_OK)
				forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
				if forwardIcon != nil {
					nextBtn.SetImage(forwardIcon)
					nextBtn.SetAlwaysShowImage(true)
					nextBtn.SetImagePosition(gtk.POS_LEFT)
				}

				// Connect the delete-event signal to handle window closing
				testDialog.Connect("delete-event", func() bool {
					testDialog.Response(gtk.RESPONSE_DELETE_EVENT)
					return true
				})

				// Show all widgets
				testDialog.ShowAll()

				// Run the dialog and wait for response
				testResponse := testDialog.Run()

				if testResponse == gtk.RESPONSE_OK {
					// Next - go to app list preview step (Step 5 in bash script)
					testDialog.Destroy()

					// App list preview dialog (Step 5)
					step4Dialog := createAppPreviewDialog(appName, piAppsDir)
					previewResponse := step4Dialog.Run()
					step4Dialog.Destroy()

					if previewResponse == gtk.RESPONSE_OK {
						// If Next is clicked, go to details preview (Step 6 in bash script)
						detailsDialog := createDetailsPreviewDialog(appName, piAppsDir)
						detailsResponse := detailsDialog.Run()
						detailsDialog.Destroy()

						if detailsResponse == gtk.RESPONSE_OK {
							// If Next is clicked, show success dialog (Step 7 in bash script)
							showSuccessDialog(appName, piAppsDir)
							return nil // Exit after completed wizard
						} else if detailsResponse == gtk.RESPONSE_CANCEL {
							// If Previous is clicked on details dialog, go back to app list preview
							// Continue the loop to recreate the app list preview
							continue
						} else {
							// If X is clicked, exit
							return nil
						}
					} else if previewResponse == gtk.RESPONSE_CANCEL {
						// If Previous is clicked on app list preview, continue the loop to recreate test dialog
						continue
					} else {
						// If dialog is closed with X button, exit
						return nil
					}
				} else if testResponse == gtk.RESPONSE_CANCEL {
					// Previous button clicked - go back to step 2
					testDialog.Destroy()
					step--
					break // Exit the loop and go back to switch statement
				} else {
					// If dialog is closed without selecting (X button), exit the app
					testDialog.Destroy()
					return nil
				}
			}
		}
	}
}

// showIntroDialog displays the welcome screen for the create app wizard
func showIntroDialog() (string, error) {
	// Initialize GTK
	gtk.Init(nil)

	piAppsDir := GetPiAppsDir()

	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", fmt.Errorf("failed to create dialog: %v", err)
	}
	defer dialog.Destroy()

	// Set dialog properties
	dialog.SetTitle("Create App Wizard")
	dialog.SetDefaultSize(400, 300)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class to match original script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon - use in-progress.png from the icons directory
	iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, err := gtk.CssProviderNew()
	if err == nil {
		css := `
		window {
			background-color: #222;
			color: #fff;
		}
		label {
			color: #fff;
		}
		entry {
			background-color: #333;
			color: #fff;
			border: 1px solid #555;
		}
		textview {
			background-color: #333;
			color: #fff;
		}
		button {
			background-color: #444;
			color: #fff;
			border: 1px solid #666;
		}
		button:hover {
			background-color: #555;
		}
		`
		err = provider.LoadFromData(css)
		if err == nil {
			screen, _ := gdk.ScreenGetDefault()
			gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
		}
	}

	// Create content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return "", fmt.Errorf("failed to get content area: %v", err)
	}
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add logo at top if available
	logoPath := filepath.Join(piAppsDir, "icons", "in-progress.png")
	if _, err := os.Stat(logoPath); err == nil {
		logoImage, err := gtk.ImageNewFromFile(logoPath)
		if err == nil {
			contentArea.Add(logoImage)
		}
	}

	// Add welcome message
	welcomeLabel, err := gtk.LabelNew("")
	if err != nil {
		return "", fmt.Errorf("failed to create label: %v", err)
	}
	welcomeLabel.SetMarkup("<span font_size='large'>Welcome to the Create App wizard!</span>")
	welcomeLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(welcomeLabel)

	// Add description
	descLabel, err := gtk.LabelNew("")
	if err != nil {
		return "", fmt.Errorf("failed to create description label: %v", err)
	}
	descLabel.SetMarkup("With a few simple steps, your project can take advantage of Pi-Apps' features and be displayed in the app-list.\nThis wizard will save your work as you go.")
	descLabel.SetLineWrap(true)
	descLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(descLabel)

	// Add tutorial link
	linkLabel, err := gtk.LabelNew("")
	if err != nil {
		return "", fmt.Errorf("failed to create link label: %v", err)
	}
	linkLabel.SetMarkup("<a href=\"https://pi-apps.io/wiki/development/Creating-an-app/\">READ THIS TUTORIAL FIRST!!</a>")
	linkLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(linkLabel)

	// Add buttons with icons
	cancelButton, _ := dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	exitIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "exit.png"))
	if exitIcon != nil {
		cancelButton.SetImage(exitIcon)
		cancelButton.SetAlwaysShowImage(true)
		cancelButton.SetImagePosition(gtk.POS_LEFT)
	}

	nextButton, _ := dialog.AddButton("Next", gtk.RESPONSE_OK)
	forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
	if forwardIcon != nil {
		nextButton.SetImage(forwardIcon)
		nextButton.SetAlwaysShowImage(true)
		nextButton.SetImagePosition(gtk.POS_LEFT)
	}

	// Show all widgets
	dialog.ShowAll()

	// Run the dialog and wait for response
	response := dialog.Run()

	switch response {
	case gtk.RESPONSE_OK:
		return "Next", nil
	case gtk.RESPONSE_CANCEL:
		return "Cancel", nil
	default:
		return "Cancel", nil
	}
}

// showBasicsDialog handles step 1 of the wizard - getting app name and type
func showBasicsDialog(currentName, currentType string) (string, string, string, error) {
	// Initialize GTK
	gtk.Init(nil)

	piAppsDir := GetPiAppsDir()

	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create dialog: %v", err)
	}
	defer dialog.Destroy()

	// Set dialog properties
	dialog.SetTitle("Create App: Step 1")
	dialog.SetDefaultSize(400, 300)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class to match original script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, err := gtk.CssProviderNew()
	if err == nil {
		css := `
		window {
			background-color: #222;
			color: #fff;
		}
		label {
			color: #fff;
		}
		entry {
			background-color: #333;
			color: #fff;
			border: 1px solid #555;
		}
		textview {
			background-color: #333;
			color: #fff;
		}
		button {
			background-color: #444;
			color: #fff;
			border: 1px solid #666;
		}
		button:hover {
			background-color: #555;
		}
		combobox button {
			background-color: #333;
			color: #fff;
		}
		`
		err = provider.LoadFromData(css)
		if err == nil {
			screen, _ := gdk.ScreenGetDefault()
			gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
		}
	}

	// Create content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get content area: %v", err)
	}
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add step title
	titleLabel, err := gtk.LabelNew("Step 1: The basics")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create title label: %v", err)
	}
	titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(titleLabel)

	// Create grid for form
	grid, err := gtk.GridNew()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create grid: %v", err)
	}
	grid.SetColumnSpacing(10)
	grid.SetRowSpacing(10)
	contentArea.Add(grid)

	// Create app name label and entry
	nameLabel, err := gtk.LabelNew("Name of app:")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create name label: %v", err)
	}
	nameLabel.SetHAlign(gtk.ALIGN_START)
	grid.Attach(nameLabel, 0, 0, 1, 1)

	var nameEntry *gtk.Entry

	// If name is already set, show it as readonly
	if currentName != "" {
		nameLabel.SetMarkup("<b>Name of app:</b> " + currentName)
		nameEntry, err = gtk.EntryNew()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create name entry: %v", err)
		}
		nameEntry.SetText(currentName)
		nameEntry.SetSensitive(false) // Make it read-only
	} else {
		nameEntry, err = gtk.EntryNew()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create name entry: %v", err)
		}
		grid.Attach(nameEntry, 1, 0, 1, 1)
	}

	// Create app type label and combobox
	typeLabel, err := gtk.LabelNew("App type:")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create type label: %v", err)
	}
	typeLabel.SetHAlign(gtk.ALIGN_START)
	grid.Attach(typeLabel, 0, 1, 1, 1)

	var appType string

	// Handle app type selection differently based on whether it's already set
	if currentType != "" {
		if currentType == "standard" {
			typeLabel.SetMarkup("<b>App type:</b> standard")
			appType = "standard"
		} else if currentType == "package" {
			typeLabel.SetMarkup("<b>App type:</b> package")
			appType = "package"
		}
	} else {
		// Create combobox for app type
		typeCombo, err := gtk.ComboBoxTextNew()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create combo box: %v", err)
		}
		typeCombo.AppendText("standard - Use scripts to install the app")
		typeCombo.AppendText("package - Will install apt package(s)")
		typeCombo.SetActive(0) // Default to standard
		grid.Attach(typeCombo, 1, 1, 1, 1)

		// Connect signal to update appType when selection changes
		typeCombo.Connect("changed", func() {
			text := typeCombo.GetActiveText()
			if strings.HasPrefix(text, "standard") {
				appType = "standard"
			} else if strings.HasPrefix(text, "package") {
				appType = "package"
			}
		})
		// Set initial value
		appType = "standard"
	}

	// Add buttons
	prevButton, _ := dialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
	backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
	if backIcon != nil {
		prevButton.SetImage(backIcon)
		prevButton.SetAlwaysShowImage(true)
		prevButton.SetImagePosition(gtk.POS_LEFT)
	}

	if currentName != "" {
		saveButton, _ := dialog.AddButton("Save", gtk.RESPONSE_APPLY)
		saveIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "save.png"))
		if saveIcon != nil {
			saveButton.SetImage(saveIcon)
			saveButton.SetAlwaysShowImage(true)
			saveButton.SetImagePosition(gtk.POS_LEFT)
		}
	}

	nextButton, _ := dialog.AddButton("Next", gtk.RESPONSE_OK)
	forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
	if forwardIcon != nil {
		nextButton.SetImage(forwardIcon)
		nextButton.SetAlwaysShowImage(true)
		nextButton.SetImagePosition(gtk.POS_LEFT)
	}

	// Show all widgets
	dialog.ShowAll()

	// Run the dialog in a loop until valid input or cancellation
	for {
		response := dialog.Run()

		var name string
		if nameEntry != nil {
			name, err = nameEntry.GetText()
			if err != nil {
				return "", "", "", fmt.Errorf("failed to get name text: %v", err)
			}
		} else {
			name = currentName
		}

		// Determine which button was clicked
		switch response {
		case gtk.RESPONSE_OK:
			// If it's a new app and name is empty, show error and continue the loop
			if currentName == "" && name == "" {
				errorDialog := gtk.MessageDialogNew(dialog.ToWindow(), gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, "Name of app may not be left blank!")
				if errorDialog != nil {
					errorDialog.Run()
					errorDialog.Destroy()
				}
				// Continue the loop to show the basics dialog again
				continue
			}
			return "Next", name, appType, nil
		case gtk.RESPONSE_CANCEL:
			return "Previous", name, appType, nil
		default:
			return "Cancel", "", "", nil
		}
	}
}

// AppDetails holds the information collected in the app details dialog
type AppDetails struct {
	Icon          string
	Website       string
	Packages      string
	Description   string
	Credits       string
	Compatibility string
}

// showAppDetailsDialog displays the dialog for step 2 - collecting app details
func showAppDetailsDialog(appName, appType string) (string, *AppDetails, error) {
	// Initialize GTK
	gtk.Init(nil)

	piAppsDir := GetPiAppsDir()

	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create dialog: %v", err)
	}
	defer dialog.Destroy()

	// Set dialog properties
	dialog.SetTitle(fmt.Sprintf("Create App: Step 2 - %s", appName))
	dialog.SetDefaultSize(500, 600)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class to match original script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, err := gtk.CssProviderNew()
	if err == nil {
		css := `
		window {
			background-color: #222;
			color: #fff;
		}
		label {
			color: #fff;
		}
		entry {
			background-color: #333;
			color: #fff;
			border: 1px solid #555;
		}
		textview {
			background-color: #333;
			color: #fff;
		}
		button {
			background-color: #444;
			color: #fff;
			border: 1px solid #666;
		}
		button:hover {
			background-color: #555;
		}
		`
		err = provider.LoadFromData(css)
		if err == nil {
			screen, _ := gdk.ScreenGetDefault()
			gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
		}
	}

	// Create content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get content area: %v", err)
	}
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add step title
	titleLabel, err := gtk.LabelNew("")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create title label: %v", err)
	}
	titleLabel.SetMarkup(fmt.Sprintf("Step 2: Fill in information about <b>%s</b>.", appName))
	titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(titleLabel)

	// Create grid for form
	grid, err := gtk.GridNew()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create grid: %v", err)
	}
	grid.SetColumnSpacing(10)
	grid.SetRowSpacing(10)
	contentArea.Add(grid)

	// Create a struct to hold the form values
	details := &AppDetails{}

	// Current row index
	row := 0

	// Different fields based on app type
	if appType == "standard" {
		// Icon field for standard apps
		iconLabel, err := gtk.LabelNew("Icon:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create icon label: %v", err)
		}
		iconLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(iconLabel, 0, row, 1, 1)

		// Create file chooser for icon
		iconChooser, err := gtk.FileChooserButtonNew("Select Icon", gtk.FILE_CHOOSER_ACTION_OPEN)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create file chooser: %v", err)
		}

		// Set file filter for images
		filter, err := gtk.FileFilterNew()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create file filter: %v", err)
		}
		filter.SetName("Image files")
		filter.AddPattern("*.png")
		filter.AddPattern("*.jpg")
		filter.AddPattern("*.jpeg")
		filter.AddPattern("*.svg")
		iconChooser.AddFilter(filter)

		grid.Attach(iconChooser, 1, row, 1, 1)

		// Connect to the file-set signal
		iconChooser.Connect("file-set", func() {
			details.Icon = iconChooser.GetFilename()
		})

		row++

		// Website field
		websiteLabel, err := gtk.LabelNew("Website:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create website label: %v", err)
		}
		websiteLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(websiteLabel, 0, row, 1, 1)

		// Create entry for website
		websiteEntry, err := gtk.EntryNew()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create website entry: %v", err)
		}

		// Read existing website if available
		websiteFile := filepath.Join(piAppsDir, "apps", appName, "website")
		if _, err := os.Stat(websiteFile); err == nil {
			websiteContent, err := os.ReadFile(websiteFile)
			if err == nil {
				websiteEntry.SetText(string(websiteContent))
				details.Website = string(websiteContent)
			}
		}

		grid.Attach(websiteEntry, 1, row, 1, 1)

		// Connect to the changed signal
		websiteEntry.Connect("changed", func() {
			text, _ := websiteEntry.GetText()
			details.Website = text
		})

		row++

		// Add Compatibility field
		compatLabel, err := gtk.LabelNew("Compatibility:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create compatibility label: %v", err)
		}
		compatLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(compatLabel, 0, row, 1, 1)

		// Check if scripts already exist to determine if compatibility should be read-only
		appDir := filepath.Join(piAppsDir, "apps", appName)
		installPath := filepath.Join(appDir, "install")
		installPath32 := filepath.Join(appDir, "install-32")
		installPath64 := filepath.Join(appDir, "install-64")

		hasInstall := false
		hasInstall32 := false
		hasInstall64 := false

		if _, err := os.Stat(installPath); err == nil {
			hasInstall = true
		}
		if _, err := os.Stat(installPath32); err == nil {
			hasInstall32 = true
		}
		if _, err := os.Stat(installPath64); err == nil {
			hasInstall64 = true
		}

		// Create or setup the compatibility selection based on existing scripts
		if hasInstall || (hasInstall32 && hasInstall64) {
			// App has combined script or both 32/64 scripts - show as read-only
			compatLabel.SetMarkup("<b>Compatibility:</b> 64bit and 32bit")
			details.Compatibility = "64bit and 32bit"
		} else if hasInstall32 && !hasInstall64 {
			// App has only 32-bit script - show as read-only
			compatLabel.SetMarkup("<b>Compatibility:</b> 32bit only")
			details.Compatibility = "32bit only"
		} else if !hasInstall32 && hasInstall64 {
			// App has only 64-bit script - show as read-only
			compatLabel.SetMarkup("<b>Compatibility:</b> 64bit only")
			details.Compatibility = "64bit only"
		} else {
			// No scripts found - let user choose
			compatCombo, err := gtk.ComboBoxTextNew()
			if err != nil {
				return "", nil, fmt.Errorf("failed to create compatibility combo box: %v", err)
			}
			compatCombo.AppendText("64bit and 32bit")
			compatCombo.AppendText("32bit only")
			compatCombo.AppendText("64bit only")
			compatCombo.SetActive(0) // Default to "64bit and 32bit"
			grid.Attach(compatCombo, 1, row, 1, 1)

			// Connect to the changed signal
			compatCombo.Connect("changed", func() {
				text := compatCombo.GetActiveText()
				details.Compatibility = text
			})
			details.Compatibility = "64bit and 32bit" // Default value
		}
		row++
	} else if appType == "package" {
		// Packages field for package apps
		packagesLabel, err := gtk.LabelNew("Package(s) to install:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create packages label: %v", err)
		}
		packagesLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(packagesLabel, 0, row, 1, 1)

		// Create entry for packages
		packagesEntry, err := gtk.EntryNew()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create packages entry: %v", err)
		}

		// Read existing packages if available
		packagesFile := filepath.Join(piAppsDir, "apps", appName, "packages")
		if _, err := os.Stat(packagesFile); err == nil {
			packagesContent, err := os.ReadFile(packagesFile)
			if err == nil {
				packagesEntry.SetText(string(packagesContent))
				details.Packages = string(packagesContent)
			}
		}

		grid.Attach(packagesEntry, 1, row, 1, 1)

		// Connect to the changed signal
		packagesEntry.Connect("changed", func() {
			text, _ := packagesEntry.GetText()
			details.Packages = text
		})

		row++

		// Add icon selection for package apps
		iconLabel, err := gtk.LabelNew("Icon:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create icon label: %v", err)
		}
		iconLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(iconLabel, 0, row, 1, 1)

		// Create icon file chooser or auto-find button
		iconBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)

		// Create file chooser for icon
		iconChooser, err := gtk.FileChooserButtonNew("Select Icon", gtk.FILE_CHOOSER_ACTION_OPEN)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create file chooser: %v", err)
		}

		// Set file filter for images
		filter, err := gtk.FileFilterNew()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create file filter: %v", err)
		}
		filter.SetName("Image files")
		filter.AddPattern("*.png")
		filter.AddPattern("*.jpg")
		filter.AddPattern("*.jpeg")
		filter.AddPattern("*.svg")
		iconChooser.AddFilter(filter)
		iconBox.PackStart(iconChooser, true, true, 0)

		// Add auto-find icon button
		autoFindBtn, _ := gtk.ButtonNewWithLabel("Auto-find")
		iconBox.PackStart(autoFindBtn, false, false, 0)

		// Connect to auto-find button click
		autoFindBtn.Connect("clicked", func() {
			// Get the package name
			pkgText, _ := packagesEntry.GetText()
			if pkgText == "" {
				// If package field is empty, show an error
				dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK,
					"Please enter a package name first")
				dialog.Run()
				dialog.Destroy()
				return
			}

			// Get the first package from the list
			pkgName := strings.Split(pkgText, " ")[0]

			// Try to find the icon from the package
			iconPath := getIconFromPackage(pkgName, piAppsDir)
			if iconPath != "" {
				details.Icon = iconPath

				// Show a success message
				dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_OK,
					"Found icon for package: %s", pkgName)
				dialog.Run()
				dialog.Destroy()
			} else {
				// If icon not found, prompt to select one
				dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_WARNING, gtk.BUTTONS_OK,
					"No icon found for package: %s\nPlease select one manually.", pkgName)
				dialog.Run()
				dialog.Destroy()
			}
		})

		grid.Attach(iconBox, 1, row, 1, 1)

		// Connect to the file-set signal for the icon chooser
		iconChooser.Connect("file-set", func() {
			details.Icon = iconChooser.GetFilename()
		})

		row++

		// Website field for package apps too
		websiteLabel, err := gtk.LabelNew("Website:")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create website label: %v", err)
		}
		websiteLabel.SetHAlign(gtk.ALIGN_START)
		grid.Attach(websiteLabel, 0, row, 1, 1)

		// Create entry for website
		websiteEntry, err := gtk.EntryNew()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create website entry: %v", err)
		}

		// Read existing website if available
		websiteFile := filepath.Join(piAppsDir, "apps", appName, "website")
		if _, err := os.Stat(websiteFile); err == nil {
			websiteContent, err := os.ReadFile(websiteFile)
			if err == nil {
				websiteEntry.SetText(string(websiteContent))
				details.Website = string(websiteContent)
			}
		}

		grid.Attach(websiteEntry, 1, row, 1, 1)

		// Connect to the changed signal
		websiteEntry.Connect("changed", func() {
			text, _ := websiteEntry.GetText()
			details.Website = text
		})

		row++
	}

	// Add description field (common to both app types)
	descLabel, err := gtk.LabelNew("Description:")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create description label: %v", err)
	}
	descLabel.SetHAlign(gtk.ALIGN_START)
	grid.Attach(descLabel, 0, row, 2, 1)
	row++

	// Create text view for description
	descScrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create scrolled window: %v", err)
	}
	descScrolled.SetHExpand(true)
	descScrolled.SetVExpand(true)
	descScrolled.SetShadowType(gtk.SHADOW_IN)
	grid.Attach(descScrolled, 0, row, 2, 1)

	descTextView, err := gtk.TextViewNew()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create text view: %v", err)
	}
	descTextView.SetWrapMode(gtk.WRAP_WORD)
	descScrolled.Add(descTextView)

	// Read existing description if available
	descriptionFile := filepath.Join(piAppsDir, "apps", appName, "description")
	if _, err := os.Stat(descriptionFile); err == nil {
		descriptionContent, err := os.ReadFile(descriptionFile)
		if err == nil {
			descBuffer, _ := descTextView.GetBuffer()
			descBuffer.SetText(string(descriptionContent))
			details.Description = string(descriptionContent)
		}
	} else {
		// Use template if available
		templateDescFile := filepath.Join(piAppsDir, "apps", "template", "description")
		if _, err := os.Stat(templateDescFile); err == nil {
			templateContent, err := os.ReadFile(templateDescFile)
			if err == nil {
				descBuffer, _ := descTextView.GetBuffer()
				descBuffer.SetText(string(templateContent))
				details.Description = string(templateContent)
			}
		}
	}

	// Connect to the changed signal
	descBuffer, _ := descTextView.GetBuffer()
	descBuffer.Connect("changed", func() {
		start := descBuffer.GetStartIter()
		end := descBuffer.GetEndIter()
		text, _ := descBuffer.GetText(start, end, true)
		details.Description = text
	})

	row++

	// Add credits field (common to both app types)
	creditsLabel, err := gtk.LabelNew("Credits:")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create credits label: %v", err)
	}
	creditsLabel.SetHAlign(gtk.ALIGN_START)
	grid.Attach(creditsLabel, 0, row, 2, 1)
	row++

	// Create text view for credits
	creditsScrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create scrolled window: %v", err)
	}
	creditsScrolled.SetHExpand(true)
	creditsScrolled.SetVExpand(true)
	creditsScrolled.SetShadowType(gtk.SHADOW_IN)
	grid.Attach(creditsScrolled, 0, row, 2, 1)

	creditsTextView, err := gtk.TextViewNew()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create text view: %v", err)
	}
	creditsTextView.SetWrapMode(gtk.WRAP_WORD)
	creditsScrolled.Add(creditsTextView)

	// Read existing credits if available
	creditsFile := filepath.Join(piAppsDir, "apps", appName, "credits")
	if _, err := os.Stat(creditsFile); err == nil {
		creditsContent, err := os.ReadFile(creditsFile)
		if err == nil {
			creditsBuffer, _ := creditsTextView.GetBuffer()
			creditsBuffer.SetText(string(creditsContent))
			details.Credits = string(creditsContent)
		}
	} else {
		// Use template if available
		templateCreditsFile := filepath.Join(piAppsDir, "apps", "template", "credits")
		if _, err := os.Stat(templateCreditsFile); err == nil {
			templateContent, err := os.ReadFile(templateCreditsFile)
			if err == nil {
				creditsBuffer, _ := creditsTextView.GetBuffer()
				creditsBuffer.SetText(string(templateContent))
				details.Credits = string(templateContent)
			}
		}
	}

	// Connect to the changed signal
	creditsBuffer, _ := creditsTextView.GetBuffer()
	creditsBuffer.Connect("changed", func() {
		start := creditsBuffer.GetStartIter()
		end := creditsBuffer.GetEndIter()
		text, _ := creditsBuffer.GetText(start, end, true)
		details.Credits = text
	})

	// Add buttons
	prevButton, _ := dialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
	backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
	if backIcon != nil {
		prevButton.SetImage(backIcon)
		prevButton.SetAlwaysShowImage(true)
		prevButton.SetImagePosition(gtk.POS_LEFT)
	}

	if appName != "" {
		saveButton, _ := dialog.AddButton("Save", gtk.RESPONSE_APPLY)
		saveIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "save.png"))
		if saveIcon != nil {
			saveButton.SetImage(saveIcon)
			saveButton.SetAlwaysShowImage(true)
			saveButton.SetImagePosition(gtk.POS_LEFT)
		}
	}

	nextButton, _ := dialog.AddButton("Next", gtk.RESPONSE_OK)
	forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
	if forwardIcon != nil {
		nextButton.SetImage(forwardIcon)
		nextButton.SetAlwaysShowImage(true)
		nextButton.SetImagePosition(gtk.POS_LEFT)
	}

	// Show all widgets
	dialog.ShowAll()

	// Run the dialog and wait for response
	response := dialog.Run()

	// Return based on which button was clicked
	switch response {
	case gtk.RESPONSE_OK:
		return "Next", details, nil
	case gtk.RESPONSE_CANCEL:
		return "Previous", details, nil
	case gtk.RESPONSE_APPLY:
		return "Save", details, nil
	default:
		return "Cancel", details, nil
	}
}

// getIconFromPackage tries to find an icon for the given package
func getIconFromPackage(packageName, piAppsDir string) string {
	// ensure piAppsDir is set
	if piAppsDir == "" {
		piAppsDir = GetPiAppsDir()
		os.Setenv("PI_APPS_DIR", piAppsDir)
	}
	// Try running dpkg -L command to list files in the package
	cmd := exec.Command("dpkg", "-L", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package not installed, try apt-file
		if commandExists("apt-file") {
			cmd = exec.Command("apt-file", "list", packageName)
			output, err = cmd.Output()
			if err != nil {
				return ""
			}
		} else {
			return ""
		}
	}

	// Look for icon files in the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// For apt-file output, extract the filepath
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				line = parts[1]
			}
		}

		// Look for icon files in standard directories
		if (strings.Contains(line, "/icons/") || strings.Contains(line, "/pixmaps/")) &&
			(strings.HasSuffix(line, ".png") || strings.HasSuffix(line, ".svg") ||
				strings.HasSuffix(line, ".xpm") || strings.HasSuffix(line, ".jpg")) {
			// Check if the file exists
			if _, err := os.Stat(line); err == nil {
				return line
			}
		}
	}

	// If we couldn't find an icon, return empty string
	return ""
}

// createAppDirectory creates the directory structure for a new app
// TODO: this is not used anywhere and giving warnings in gopls, either remove or use it
func createAppDirectory(appName, appType string) error {
	if appName == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	piAppsDir := GetPiAppsDir()

	// Create app directory
	appDir := filepath.Join(piAppsDir, "apps", appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("failed to create app directory: %v", err)
	}

	// Copy template files based on app type
	templateDir := filepath.Join(piAppsDir, "apps", "template")

	// Copy template description file if it doesn't exist
	descFile := filepath.Join(appDir, "description")
	if _, err := os.Stat(descFile); os.IsNotExist(err) {
		if err := copyFile(filepath.Join(templateDir, "description"), descFile); err != nil {
			return fmt.Errorf("failed to copy description: %v", err)
		}
	}

	// Copy template credits file if it doesn't exist
	creditsFile := filepath.Join(appDir, "credits")
	if _, err := os.Stat(creditsFile); os.IsNotExist(err) {
		if err := copyFile(filepath.Join(templateDir, "credits"), creditsFile); err != nil {
			return fmt.Errorf("failed to copy credits: %v", err)
		}
	}

	// Create appropriate files based on app type
	if appType == "standard" {
		// Create install script
		installFile := filepath.Join(appDir, "install")
		if _, err := os.Stat(installFile); os.IsNotExist(err) {
			if err := copyFile(filepath.Join(templateDir, "install"), installFile); err != nil {
				return fmt.Errorf("failed to copy install script: %v", err)
			}
			// Make it executable
			if err := os.Chmod(installFile, 0755); err != nil {
				return fmt.Errorf("failed to make install script executable: %v", err)
			}
		}
	} else if appType == "package" {
		// Create empty packages file
		packagesFile := filepath.Join(appDir, "packages")
		if _, err := os.Stat(packagesFile); os.IsNotExist(err) {
			if err := os.WriteFile(packagesFile, []byte(""), 0644); err != nil {
				return fmt.Errorf("failed to create packages file: %v", err)
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// checkShellcheck verifies shellcheck is installed, prompts to install it if not
func checkShellcheck() error {
	// Check if shellcheck is installed
	if !commandExists("shellcheck") {
		// Ask if they want to install shellcheck
		dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO,
			"Shellcheck is not installed, but it's useful for finding errors in shell scripts. Install it now?")
		response := dialog.Run()
		dialog.Destroy()

		if response == gtk.RESPONSE_YES {
			cmd := exec.Command("sudo", "apt", "install", "-y", "shellcheck")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install shellcheck: %v", err)
			}
		}
	}
	return nil
}

// runShellcheck executes shellcheck on the given script file
func runShellcheck(scriptPath string) error {
	// Ensure shellcheck is available
	if err := checkShellcheck(); err != nil {
		return err
	}

	piAppsDir := GetPiAppsDir()
	terminalRunPath := filepath.Join(piAppsDir, "etc", "terminal-run")

	// Check if terminal-run exists
	if _, err := os.Stat(terminalRunPath); os.IsNotExist(err) {
		// Fallback to the previous method if terminal-run doesn't exist
		cmd := fmt.Sprintf(`shellcheck '%s'; echo 'Press Enter to exit.'; read dummy`, scriptPath)
		terminal := exec.Command("x-terminal-emulator", "-e", "bash", "-c", cmd)
		return terminal.Start()
	}

	// Use the Pi-Apps terminal-run script
	shellcheckCmd := fmt.Sprintf("shellcheck '%s'; echo 'Press Enter to exit.'; read dummy", scriptPath)
	terminal := exec.Command(terminalRunPath, shellcheckCmd, "Shellcheck")
	return terminal.Start()
}

// runScript executes the provided script in a terminal
func runScript(scriptPath, appName string) error {
	piAppsDir := GetPiAppsDir()
	terminalRunPath := filepath.Join(piAppsDir, "etc", "terminal-run")

	// Check if terminal-run exists
	if _, err := os.Stat(terminalRunPath); os.IsNotExist(err) {
		// Fallback to the previous method if terminal-run doesn't exist
		cmd := fmt.Sprintf(`cd %s; PI_APPS_DIR='%s'; export PI_APPS_DIR; set -a; source '%s/api'; app='%s'; export app; '%s' | cat; echo 'Press Enter to exit.'; read dummy`,
			piAppsDir, piAppsDir, piAppsDir, appName, scriptPath)
		terminal := exec.Command("x-terminal-emulator", "-e", "bash", "-c", cmd)
		return terminal.Start()
	}

	// Use the Pi-Apps terminal-run script which handles all the environment setup
	scriptCmd := fmt.Sprintf("cd %s; PI_APPS_DIR='%s'; set -a; source '%s/api'; app='%s'; '%s' | cat; echo 'Press Enter to exit.'; read dummy",
		piAppsDir, piAppsDir, piAppsDir, appName, scriptPath)
	scriptTitle := fmt.Sprintf("Running %s script of %s", filepath.Base(scriptPath), appName)
	terminal := exec.Command(terminalRunPath, scriptCmd, scriptTitle)
	return terminal.Start()
}

// createAppPreviewDialog creates a dialog showing how the app will appear in the app list
func createAppPreviewDialog(appName string, piAppsDir string) *gtk.Dialog {
	// Create dialog
	dialog, _ := gtk.DialogNew()
	dialog.SetTitle("List view")
	dialog.SetDefaultSize(400, 300)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class name to match the original bash script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, _ := gtk.CssProviderNew()
	css := `
	window {
		background-color: #222;
		color: #fff;
	}
	label {
		color: #fff;
	}
	button {
		background-color: #444;
		color: #fff;
		border: 1px solid #666;
	}
	button:hover {
		background-color: #555;
	}
	treeview {
		background-color: #333;
		color: #fff;
	}
	treeview:selected {
		background-color: #4a90d9;
	}
	`
	provider.LoadFromData(css)
	screen, _ := gdk.ScreenGetDefault()
	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// Create content area
	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add title and explanation
	titleLabel, _ := gtk.LabelNew("Make sure everything looks right.")
	titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(titleLabel)

	subtitleLabel, _ := gtk.LabelNew("Here's what it will look like in the app list:")
	subtitleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.Add(subtitleLabel)

	// Create a simple list box instead of a tree view to avoid column issues
	listBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	listBox.SetMarginTop(10)
	listBox.SetMarginBottom(10)
	listBox.SetMarginStart(10)
	listBox.SetMarginEnd(10)

	// Create a blue highlight box that mimics selection
	highlightBox, _ := gtk.EventBoxNew()
	highlightStyle, _ := gtk.CssProviderNew()
	highlightStyle.LoadFromData(`
	box {
		background-color: #0066cc;
		border-radius: 3px;
		padding: 5px;
	}
	`)
	highlightBoxContext, _ := highlightBox.GetStyleContext()
	highlightBoxContext.AddProvider(highlightStyle, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// Horizontal box for the list entry
	entryBox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	entryBox.SetMarginStart(5)
	entryBox.SetMarginEnd(5)
	entryBox.SetMarginTop(5)
	entryBox.SetMarginBottom(5)
	highlightBox.Add(entryBox)

	// Add app icon
	iconPath24 := filepath.Join(piAppsDir, "apps", appName, "icon-24.png")
	var iconImage *gtk.Image

	if _, err := os.Stat(iconPath24); err == nil {
		iconImage, _ = gtk.ImageNewFromFile(iconPath24)
	} else {
		// Use a placeholder icon if the app icon doesn't exist
		defaultIconPath := filepath.Join(piAppsDir, "icons", "logo-24.png")
		if _, err := os.Stat(defaultIconPath); err == nil {
			iconImage, _ = gtk.ImageNewFromFile(defaultIconPath)
		} else {
			// Create an empty image as last resort
			iconImage, _ = gtk.ImageNew()
			iconImage.SetSizeRequest(24, 24)
		}
	}

	entryBox.PackStart(iconImage, false, false, 0)

	// Add app name
	nameLabel, _ := gtk.LabelNew(appName)
	nameLabel.SetHAlign(gtk.ALIGN_START)
	nameLabel.SetMarginStart(5)
	nameStyleProvider, _ := gtk.CssProviderNew()
	nameStyleProvider.LoadFromData(`
	label {
		color: white;
		font-weight: bold;
	}
	`)
	nameLabelContext, _ := nameLabel.GetStyleContext()
	nameLabelContext.AddProvider(nameStyleProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	entryBox.PackStart(nameLabel, true, true, 0)

	// Add status
	statusLabel, _ := gtk.LabelNew("(uninstalled)")
	statusLabel.SetHAlign(gtk.ALIGN_END)
	statusStyleProvider, _ := gtk.CssProviderNew()
	statusStyleProvider.LoadFromData(`
	label {
		color: #aaaaaa;
		font-style: italic;
	}
	`)
	statusLabelContext, _ := statusLabel.GetStyleContext()
	statusLabelContext.AddProvider(statusStyleProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	entryBox.PackEnd(statusLabel, false, false, 10)

	// Add the entry to a container with padding
	containerBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	containerBox.PackStart(highlightBox, false, false, 10)

	// Get first line of description for tooltip
	descriptionPath := filepath.Join(piAppsDir, "apps", appName, "description")
	descriptionText := "Description unavailable"
	if _, err := os.Stat(descriptionPath); err == nil {
		descBytes, err := os.ReadFile(descriptionPath)
		if err == nil && len(descBytes) > 0 {
			descLines := strings.Split(string(descBytes), "\n")
			if len(descLines) > 0 {
				descriptionText = descLines[0]
			}
		}
	}

	// Add a tooltip to the entry
	highlightBox.SetTooltipText(descriptionText)

	// Add the container to the content area
	contentArea.Add(containerBox)

	// Add navigation buttons with icons
	previousBtn, _ := dialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
	backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
	if backIcon != nil {
		previousBtn.SetImage(backIcon)
		previousBtn.SetAlwaysShowImage(true)
		previousBtn.SetImagePosition(gtk.POS_LEFT)
	}

	nextBtn, _ := dialog.AddButton("Next", gtk.RESPONSE_OK)
	forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
	if forwardIcon != nil {
		nextBtn.SetImage(forwardIcon)
		nextBtn.SetAlwaysShowImage(true)
		nextBtn.SetImagePosition(gtk.POS_LEFT)
	}

	// Connect dialog-delete-event to exit instead of just closing
	dialog.Connect("delete-event", func() bool {
		dialog.Response(gtk.RESPONSE_DELETE_EVENT)
		return true
	})

	dialog.ShowAll()
	return dialog
}

// showSuccessDialog displays the final success message
func showSuccessDialog(appName string, piAppsDir string) {
	dialog, _ := gtk.DialogNew()
	dialog.SetTitle("Create App Wizard")
	dialog.SetDefaultSize(400, 300)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class to match original script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "in-progress.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, _ := gtk.CssProviderNew()
	css := `
	window {
		background-color: #222;
		color: #fff;
	}
	label {
		color: #fff;
	}
	button {
		background-color: #444;
		color: #fff;
		border: 1px solid #666;
	}
	button:hover {
		background-color: #555;
	}
	textview {
		background-color: #333;
		color: #fff;
	}
	`
	provider.LoadFromData(css)
	screen, _ := gdk.ScreenGetDefault()
	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// Create content area
	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add icon at the top
	if _, err := os.Stat(iconPath); err == nil {
		image, _ := gtk.ImageNewFromFile(iconPath)
		contentArea.PackStart(image, false, false, 0)
	}

	// Add title
	titleLabel, _ := gtk.LabelNew("Done!")
	titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	titleLabel.SetMarkup("<span font_size='large'>Done!</span>")
	contentArea.Add(titleLabel)

	// Create text view for message
	textScroll, _ := gtk.ScrolledWindowNew(nil, nil)
	textScroll.SetHExpand(true)
	textScroll.SetVExpand(true)

	textView, _ := gtk.TextViewNew()
	textView.SetWrapMode(gtk.WRAP_WORD)
	textView.SetEditable(false)

	// Show repository link as clickable
	textView.SetCursorVisible(false)
	textBuffer, _ := textView.GetBuffer()

	// Format message
	message := fmt.Sprintf(
		"Your app is located at %s/apps/%s\n\nTo add your app to the Pi-Apps official repository, put that folder in a .ZIP file and open an issue on the Pi-Apps repository: https://github.com/Botspot/pi-apps/issues/new/choose",
		piAppsDir, appName)

	textBuffer.SetText(message)

	// Add tag for URL
	tagTable, _ := textBuffer.GetTagTable()
	urlTag, _ := gtk.TextTagNew("url")
	urlTag.SetProperty("foreground", "#3584e4")
	urlTag.SetProperty("underline", pango.UNDERLINE_SINGLE)
	tagTable.Add(urlTag)

	// Find the URL position in text
	startIter := textBuffer.GetStartIter()
	endIter := textBuffer.GetEndIter()
	text, _ := textBuffer.GetText(startIter, endIter, false)
	urlPos := strings.Index(text, "https://")

	if urlPos >= 0 {
		start := textBuffer.GetIterAtOffset(urlPos)
		end := textBuffer.GetIterAtOffset(len(text))
		textBuffer.ApplyTag(urlTag, start, end)
	}

	textScroll.Add(textView)
	contentArea.Add(textScroll)

	// Add buttons with icons (the previous button is broken going back to the testing your scripts section and below, because of that this button is disabled)
	//previousBtn, _ := dialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
	//backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
	//	if backIcon != nil {
	//	previousBtn.SetImage(backIcon)
	//	previousBtn.SetAlwaysShowImage(true)
	//	previousBtn.SetImagePosition(gtk.POS_LEFT)
	//}

	closeBtn, _ := dialog.AddButton("Close", gtk.RESPONSE_CLOSE)
	exitIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "exit.png"))
	if exitIcon != nil {
		closeBtn.SetImage(exitIcon)
		closeBtn.SetAlwaysShowImage(true)
		closeBtn.SetImagePosition(gtk.POS_LEFT)
	}

	// Connect dialog-delete-event to exit instead of just closing
	dialog.Connect("delete-event", func() bool {
		dialog.Response(gtk.RESPONSE_DELETE_EVENT)
		return true
	})

	dialog.ShowAll()

	// Handle link clicking - simplified to just open the URL when clicked
	textView.Connect("button-release-event", func(tv *gtk.TextView, event *gdk.Event) {
		// Simply open the URL when clicked
		exec.Command("xdg-open", "https://github.com/Botspot/pi-apps/issues/new/choose").Start()
	})

	response := dialog.Run()
	dialog.Destroy()

	if response == gtk.RESPONSE_CANCEL {
		// Previous button - go back to details preview dialog (Step 6)
		detailsDialog := createDetailsPreviewDialog(appName, piAppsDir)
		detailsResponse := detailsDialog.Run()
		detailsDialog.Destroy()

		// Handle response from details dialog
		if detailsResponse == gtk.RESPONSE_OK {
			// If Next is clicked, show success dialog again
			showSuccessDialog(appName, piAppsDir)
		} else if detailsResponse == gtk.RESPONSE_CANCEL {
			// If Previous is clicked, show app preview dialog (Step 5)
			previewDialog := createAppPreviewDialog(appName, piAppsDir)
			previewResponse := previewDialog.Run()
			previewDialog.Destroy()

			if previewResponse == gtk.RESPONSE_OK {
				// If Next is clicked in app preview, go to details preview
				detailsDialog := createDetailsPreviewDialog(appName, piAppsDir)
				detailsResponse := detailsDialog.Run()
				detailsDialog.Destroy()

				if detailsResponse == gtk.RESPONSE_OK {
					// If Next is clicked in details preview, show success again
					showSuccessDialog(appName, piAppsDir)
				} else if detailsResponse != gtk.RESPONSE_CANCEL {
					// If dialog is closed with X button, exit
					return
				}
			} else if previewResponse != gtk.RESPONSE_CANCEL {
				// If dialog is closed with X button, exit
				return
			}
		} else {
			// If details dialog is closed with X button, exit
			return
		}
	} else if response != gtk.RESPONSE_CLOSE {
		// If dialog is closed with X button, exit
		return
	}
}

// createDetailsPreviewDialog creates a dialog showing the details of the app
func createDetailsPreviewDialog(appName string, piAppsDir string) *gtk.Dialog {
	// Create dialog
	dialog, _ := gtk.DialogNew()
	dialog.SetTitle("Details window")
	dialog.SetDefaultSize(500, 400)
	dialog.SetPosition(gtk.WIN_POS_CENTER)

	// Set the dialog class to match original script
	dialog.SetName("Pi-Apps Settings")

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons", "logo.png")
	if _, err := os.Stat(iconPath); err == nil {
		dialog.SetIconFromFile(iconPath)
	}

	// Add CSS for dark theme
	provider, _ := gtk.CssProviderNew()
	css := `
	window {
		background-color: #222;
		color: #fff;
	}
	label {
		color: #fff;
	}
	button {
		background-color: #444;
		color: #fff;
		border: 1px solid #666;
	}
	button:hover {
		background-color: #555;
	}
	textview {
		background-color: #333;
		color: #fff;
	}
	`
	provider.LoadFromData(css)
	screen, _ := gdk.ScreenGetDefault()
	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	// Create content area
	contentArea, _ := dialog.GetContentArea()
	contentArea.SetMarginTop(15)
	contentArea.SetMarginBottom(15)
	contentArea.SetMarginStart(15)
	contentArea.SetMarginEnd(15)
	contentArea.SetSpacing(10)

	// Add app icon at the top
	iconPath64 := filepath.Join(piAppsDir, "apps", appName, "icon-64.png")
	if _, err := os.Stat(iconPath64); err == nil {
		image, _ := gtk.ImageNewFromFile(iconPath64)
		contentArea.PackStart(image, false, false, 0)
	} else {
		// Use a placeholder icon if the app icon doesn't exist
		defaultIconPath := filepath.Join(piAppsDir, "icons", "logo-64.png")
		if _, err := os.Stat(defaultIconPath); err == nil {
			image, _ := gtk.ImageNewFromFile(defaultIconPath)
			contentArea.PackStart(image, false, false, 0)
		}
	}

	// Add title and info
	titleLabel, _ := gtk.LabelNew("")
	titleLabel.SetMarkup("<span font_size='large'>" + appName + "</span>")
	titleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.PackStart(titleLabel, false, false, 0)

	// Add subtitle
	subtitleLabel, _ := gtk.LabelNew("Make sure everything looks right.\nHere's a preview of the Details window:")
	subtitleLabel.SetJustify(gtk.JUSTIFY_CENTER)
	contentArea.PackStart(subtitleLabel, false, false, 10)

	// Create a box for the app details
	detailsBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	detailsBox.SetMarginStart(10)
	detailsBox.SetMarginEnd(10)
	contentArea.PackStart(detailsBox, false, false, 0)

	// Add status
	statusLabel, _ := gtk.LabelNew("- Current status: uninstalled")
	statusLabel.SetHAlign(gtk.ALIGN_START)
	detailsBox.PackStart(statusLabel, false, false, 0)

	// Add website link
	websiteFile := filepath.Join(piAppsDir, "apps", appName, "website")
	websiteURL := ""
	if _, err := os.Stat(websiteFile); err == nil {
		websiteBytes, err := os.ReadFile(websiteFile)
		if err == nil {
			websiteURL = strings.TrimSpace(string(websiteBytes))
		}
	}

	websiteLabel, _ := gtk.LabelNew("")
	if websiteURL != "" {
		websiteLabel.SetMarkup(fmt.Sprintf("- Website: <a href=\"%s\">%s</a>", websiteURL, websiteURL))
	} else {
		websiteLabel.SetText("- Website: Not specified")
	}
	websiteLabel.SetHAlign(gtk.ALIGN_START)
	detailsBox.PackStart(websiteLabel, false, false, 0)

	// Add a separator
	separator, _ := gtk.SeparatorNew(gtk.ORIENTATION_HORIZONTAL)
	contentArea.PackStart(separator, false, false, 10)

	// Add description text view
	descScrolled, _ := gtk.ScrolledWindowNew(nil, nil)
	descScrolled.SetHExpand(true)
	descScrolled.SetVExpand(true)
	descScrolled.SetShadowType(gtk.SHADOW_IN)

	descView, _ := gtk.TextViewNew()
	descView.SetWrapMode(gtk.WRAP_WORD)
	descView.SetEditable(false)
	descScrolled.Add(descView)

	// Read existing description
	descriptionFile := filepath.Join(piAppsDir, "apps", appName, "description")
	if _, err := os.Stat(descriptionFile); err == nil {
		content, err := os.ReadFile(descriptionFile)
		if err == nil {
			buffer, _ := descView.GetBuffer()
			buffer.SetText(string(content))
		}
	}

	contentArea.PackStart(descScrolled, true, true, 0)

	// Add navigation buttons with icons
	previousBtn, _ := dialog.AddButton("Previous", gtk.RESPONSE_CANCEL)
	backIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "back.png"))
	if backIcon != nil {
		previousBtn.SetImage(backIcon)
		previousBtn.SetAlwaysShowImage(true)
		previousBtn.SetImagePosition(gtk.POS_LEFT)
	}

	nextBtn, _ := dialog.AddButton("Next", gtk.RESPONSE_OK)
	forwardIcon, _ := gtk.ImageNewFromFile(filepath.Join(piAppsDir, "icons", "forward.png"))
	if forwardIcon != nil {
		nextBtn.SetImage(forwardIcon)
		nextBtn.SetAlwaysShowImage(true)
		nextBtn.SetImagePosition(gtk.POS_LEFT)
	}

	// Connect dialog-delete-event to exit instead of just closing
	dialog.Connect("delete-event", func() bool {
		dialog.Response(gtk.RESPONSE_DELETE_EVENT)
		return true
	})

	dialog.ShowAll()
	return dialog
}
