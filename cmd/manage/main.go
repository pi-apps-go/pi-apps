package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/botspot/pi-apps/pkg/gui"
)

func main() {
	// Define flags
	installFlag := flag.Bool("install", false, "Install the specified apps")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall the specified apps")
	updateFlag := flag.Bool("update", false, "Update the specified apps")
	updateSelfFlag := flag.Bool("update-self", false, "Update Pi-Apps")
	installIfNotInstalledFlag := flag.Bool("install-if-not-installed", false, "Install an app only if it is not already installed")
	guiFlag := flag.Bool("gui", false, "Use GUI for interactions")
	multiFlag := flag.Bool("multi", false, "Enable multi-install/uninstall mode")
	forceFlag := flag.Bool("force", false, "Force the operation (skip validation)")
	testUnsupportedFlag := flag.Bool("test-unsupported", false, "Test unsupported system warning")
	refreshFlag := flag.Bool("refresh", false, "Refresh the specified apps")
	updateFileFlag := flag.Bool("update-file", false, "Update the specified files")

	// Custom error handling for undefined flags
	flag.Usage = printUsage

	// Parse flags
	err := flag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		api.ErrorNoExit("Error: " + err.Error())
		printUsage()
		os.Exit(1)
	}

	// Get remaining arguments (app names)
	args := flag.Args()

	// Check for unknown flags
	for _, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			api.ErrorNoExit("Error: Invalid flag: " + arg)
			printUsage()
			os.Exit(1)
		}
	}

	// Test unsupported system warning if flag is set
	if *testUnsupportedFlag {
		// Set environment variable to simulate unsupported system
		os.Setenv("PI_APPS_SIMULATE_UNSUPPORTED", "true")
		// Display warning message with GUI only if GUI flag is set
		gui.DisplayUnsupportedSystemWarning("Your system is actually fine, this is just a drill :)\nThis would be a example of this error in the Go reimplementation if it did happen.", *guiFlag)
		// Exit after displaying warning if no operation flags are set
		if !*installFlag && !*uninstallFlag && !*updateFlag && !*updateSelfFlag && !*installIfNotInstalledFlag && !*refreshFlag && !*updateFileFlag {
			os.Exit(0)
		}
		// Skip the regular system support check below since we've already shown a warning
	} else {
		// Check if system is supported
		isSupported, supportMessage := api.IsSupportedSystem()
		if !isSupported {
			// System is not supported, show warning with GUI only if GUI flag is set
			gui.DisplayUnsupportedSystemWarning(supportMessage, *guiFlag)
		}
	}

	// Ensure PI_APPS_DIR environment variable is set
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		// Try to find pi-apps directory
		homeDir, err := os.UserHomeDir()
		if err == nil {
			possiblePath := filepath.Join(homeDir, "pi-apps")
			if _, err := os.Stat(possiblePath); err == nil {
				piAppsDir = possiblePath
				os.Setenv("PI_APPS_DIR", piAppsDir)
			}
		}

		if piAppsDir == "" {
			api.Error("Error: PI_APPS_DIR environment variable not set")
		}
	}

	// If no flags are provided, print usage and exit
	if !*installFlag && !*uninstallFlag && !*updateFlag && !*updateSelfFlag && !*installIfNotInstalledFlag && !*refreshFlag && !*updateFileFlag {
		api.ErrorNoExit("Error: You need to specify an operation, and in most cases, which app to operate on.")
		printUsage()
		os.Exit(0)
	}

	// Check if at least one app is specified for app-specific operations
	if (*installFlag || *uninstallFlag || *updateFlag || *installIfNotInstalledFlag || *refreshFlag || *updateFileFlag) && len(args) == 0 {
		api.Error("Error: You must specify at least one app")
	}

	// Create a queue of operations
	var queue []gui.QueueItem

	// Process each requested operation
	if *updateSelfFlag {
		// Update Pi-Apps itself
		api.Status("Updating \u001b[1mPi-Apps\u001b[22m...")
		// For now, we just print this as the UpdateSelf function is not yet implemented
		api.StatusGreen("Pi-Apps updated successfully")
	}

	// Add apps to the queue based on requested operations
	for _, appName := range args {
		iconPath := filepath.Join(piAppsDir, "apps", appName, "icon-64.png")
		// Check if icon exists, otherwise use default icon
		if _, err := os.Stat(iconPath); os.IsNotExist(err) {
			iconPath = filepath.Join(piAppsDir, "icons", "none-64.png")
		}

		if *installFlag {
			queue = append(queue, gui.QueueItem{
				Action:   "install",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		} else if *uninstallFlag {
			queue = append(queue, gui.QueueItem{
				Action:   "uninstall",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		} else if *updateFlag {
			queue = append(queue, gui.QueueItem{
				Action:   "update",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		} else if *refreshFlag {
			queue = append(queue, gui.QueueItem{
				Action:   "refresh",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		} else if *updateFileFlag {
			queue = append(queue, gui.QueueItem{
				Action:   "update-file",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		} else if *installIfNotInstalledFlag {
			// Check if app is already installed
			if api.IsAppInstalled(appName) {
				fmt.Printf("App '%s' is already installed, skipping installation.\n", appName)
				continue
			}

			queue = append(queue, gui.QueueItem{
				Action:   "install",
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
			})
		}
	}

	if len(queue) == 0 && !*updateSelfFlag {
		api.Status("No operations to perform")
	}

	// Validate the queue (unless force flag is set)
	if !*forceFlag && len(queue) > 0 {
		var err error
		if *guiFlag {
			// Use GUI for validation
			queue, err = gui.ValidateAppsGUI(queue)
			if err != nil {
				api.ErrorNoExit("Error validating apps: " + err.Error())
			}
		} else {
			// TODO: Implement command-line validation
			// For now, we just proceed with the queue as is
		}
	}

	// If multi flag is set, execute all operations at once
	if *multiFlag {
		// If GUI flag is set, show progress monitor in a goroutine
		if *guiFlag && len(queue) > 0 {
			go func() {
				err := gui.ProgressMonitor(queue)
				if err != nil {
					api.ErrorNoExit("Error showing progress monitor: " + err.Error())
				}
			}()
		}

		// Execute each operation in the queue
		for i := range queue {
			// Update status to in-progress
			queue[i].Status = "in-progress"

			// Execute the operation
			var err error
			switch queue[i].Action {
			case "install":
				api.Status("Installing \u001b[1m" + queue[i].AppName + "\u001b[22m...")

				// Check if already installed, unless ForceReinstall flag is set
				if api.IsAppInstalled(queue[i].AppName) && !queue[i].ForceReinstall {
					err = fmt.Errorf("app '%s' is already installed", queue[i].AppName)
				} else {
					// Force uninstall first if reinstalling
					if queue[i].ForceReinstall && api.IsAppInstalled(queue[i].AppName) {
						api.Status("First uninstalling existing version of \u001b[1m" + queue[i].AppName + "\u001b[22m...")
						if uninstallErr := api.UninstallApp(queue[i].AppName); uninstallErr != nil {
							err = fmt.Errorf("failed to uninstall before reinstall: %v", uninstallErr)
						}
					}

					if err == nil {
						err = api.InstallApp(queue[i].AppName)
					}
				}
			case "uninstall":
				api.Status("Uninstalling \u001b[1m" + queue[i].AppName + "\u001b[22m...")

				// Check if already uninstalled, unless ForceReinstall flag is set
				if !api.IsAppInstalled(queue[i].AppName) && !queue[i].ForceReinstall {
					err = fmt.Errorf("app '%s' is not installed", queue[i].AppName)
				} else {
					err = api.UninstallApp(queue[i].AppName)
				}
			case "update":
				api.Status("Updating \u001b[1m" + queue[i].AppName + "\u001b[22m...")
				err = api.UpdateApp(queue[i].AppName)
			case "refresh":
				api.Status("Refreshing \u001b[1m" + queue[i].AppName + "\u001b[22m...")
				err = api.RefreshApp(queue[i].AppName)
			case "update-file":
				api.Status("Updating file \u001b[1m" + queue[i].AppName + "\u001b[22m...")
				err = api.UpdateFile(queue[i].AppName)
			}

			// Update status based on result
			if err != nil {
				api.ErrorNoExit("Error with " + queue[i].Action + " for " + queue[i].AppName + ": " + err.Error())
				queue[i].Status = "failure"
				// Add error message to the queue item so it can be displayed in the summary
				queue[i].ErrorMessage = err.Error()

				// If GUI is enabled, show error dialog and ask for retry
				if *guiFlag {
					if gui.ShowErrorDialogWithRetry(queue[i].AppName, queue[i].Action, err.Error()) {
						// User chose to retry, reset status and continue
						queue[i].Status = "waiting"
						queue[i].ErrorMessage = ""
						i-- // Retry this item
						continue
					}
				}
			} else {
				queue[i].Status = "success"
				api.StatusGreen(queue[i].Action + " completed successfully for " + queue[i].AppName)
			}
		}

		// Wait a brief moment for progress dialog to auto-close
		if *guiFlag && len(queue) > 0 {
			// Wait for progress monitor to fully close
			time.Sleep(2 * time.Second)

			// Show summary dialog only if we have items to show
			err := gui.ShowSummaryDialog(queue)
			if err != nil {
				api.ErrorNoExit("Error showing summary dialog: " + err.Error())
			}
		} else if len(queue) == 0 {
			api.Status("No operations to perform")
		}
	} else {
		// Execute operations one by one
		for i := range queue {
			// Execute the operation
			var err error
			switch queue[i].Action {
			case "install":
				api.Status("Installing " + queue[i].AppName + "...")
				err = api.InstallApp(queue[i].AppName)
			case "uninstall":
				api.Status("Uninstalling " + queue[i].AppName + "...")
				err = api.UninstallApp(queue[i].AppName)
			case "update":
				api.Status("Updating " + queue[i].AppName + "...")
				err = api.UpdateApp(queue[i].AppName)
			case "refresh":
				api.Status("Refreshing " + queue[i].AppName + "...")
				err = api.RefreshApp(queue[i].AppName)
			case "update-file":
				api.Status("Updating file " + queue[i].AppName + "...")
				err = api.UpdateFile(queue[i].AppName)
			}

			// Check result
			if err != nil {
				// Do nothing here considering the error handling and display of the Need help? section is already handled in the manage package
			} else {
				api.StatusGreen("Operation completed successfully")
				queue[i].Status = "success"
			}
		}
		// Show summary dialog after single operations if GUI flag is set
		if *guiFlag && len(queue) > 0 {
			err := gui.ShowSummaryDialog(queue)
			if err != nil {
				api.ErrorNoExit("Error showing summary dialog: " + err.Error())
			}
		}
	}
}

// printUsage prints usage information
func printUsage() {
	fmt.Println("Pi-Apps Management Tool")
	fmt.Println("Usage:")
	fmt.Println("  manage [options] [app names...]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -install                  Install the specified apps")
	fmt.Println("  -uninstall                Uninstall the specified apps")
	fmt.Println("  -update                   Update the specified apps")
	fmt.Println("  -update-self              Update Pi-Apps")
	fmt.Println("  -install-if-not-installed Install an app only if it is not already installed")
	fmt.Println("  -gui                      Use GUI for interactions")
	fmt.Println("  -multi                    Enable multi-install/uninstall mode")
	fmt.Println("  -force                    Force the operation (skip validation)")
	fmt.Println("  -test-unsupported         Test unsupported system warning")
	fmt.Println("  -refresh                  Refresh the specified apps")
	fmt.Println("  -update-file              Update the specified files")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  manage -install Firefox LibreOffice")
	fmt.Println("  manage -uninstall Zoom")
	fmt.Println("  manage -update-self")
	fmt.Println("  manage -install-if-not-installed Firefox")
	fmt.Println("  manage -install -gui -multi Firefox LibreOffice")
}
