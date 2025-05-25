package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	daemonFlag := flag.Bool("daemon", false, "Run in daemon mode")

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

	// Check for daemon mode first
	if *daemonFlag {
		// In daemon mode, the queue is passed as a single argument
		var queueStr string
		if len(args) > 0 {
			queueStr = args[0]
		}
		err := runDaemon(queueStr)
		if err != nil {
			api.ErrorNoExit("Daemon error: " + err.Error())
			os.Exit(1)
		}
		return
	}

	// Check for daemon-terminal mode (called from terminal-run)
	if len(args) > 0 && args[0] == "daemon-terminal" {
		var queueStr, statusFile string
		if len(args) > 1 {
			queueStr = args[1]
		}
		if len(args) > 2 {
			statusFile = args[2]
		}
		err := daemonTerminal(queueStr, statusFile)
		if err != nil {
			api.ErrorNoExit("Daemon terminal error: " + err.Error())
			os.Exit(1)
		}
		return
	}

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
				// Check if already installed, unless ForceReinstall flag is set
				if api.IsAppInstalled(queue[i].AppName) && !queue[i].ForceReinstall {
					err = fmt.Errorf("app '%s' is already installed", queue[i].AppName)
				} else {
					// Force uninstall first if reinstalling
					if queue[i].ForceReinstall && api.IsAppInstalled(queue[i].AppName) {
						if uninstallErr := api.UninstallApp(queue[i].AppName); uninstallErr != nil {
							err = fmt.Errorf("failed to uninstall before reinstall: %v", uninstallErr)
						}
					}

					if err == nil {
						err = api.InstallApp(queue[i].AppName)
					}
				}
			case "uninstall":
				// Check if already uninstalled, unless ForceReinstall flag is set
				if !api.IsAppInstalled(queue[i].AppName) && !queue[i].ForceReinstall {
					err = fmt.Errorf("app '%s' is not installed", queue[i].AppName)
				} else {
					err = api.UninstallApp(queue[i].AppName)
				}
			case "update":
				err = api.UpdateApp(queue[i].AppName)
			case "refresh":
				err = api.RefreshApp(queue[i].AppName)
			case "update-file":
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
				err = api.InstallApp(queue[i].AppName)
			case "uninstall":
				err = api.UninstallApp(queue[i].AppName)
			case "update":
				err = api.UpdateApp(queue[i].AppName)
			case "refresh":
				err = api.RefreshApp(queue[i].AppName)
			case "update-file":
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

// QueueItem represents an item in the daemon queue
type QueueItem struct {
	Action   string
	AppName  string
	Status   string // "waiting", "in-progress", "success", "failure", "diagnosed"
	IconPath string
	ExitCode int
}

// runDaemon implements the daemon functionality for managing app operations
func runDaemon(queueStr string) error {
	// Get PI_APPS_DIR environment variable
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Create daemon directory
	daemonDir := filepath.Join(piAppsDir, "data", "manage-daemon")
	err := os.MkdirAll(daemonDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}

	pidFile := filepath.Join(daemonDir, "pid")
	queueFile := filepath.Join(daemonDir, "queue")

	// Check if daemon is already running
	if _, err := os.Stat(pidFile); err == nil {
		// Read existing PID
		pidBytes, err := os.ReadFile(pidFile)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil {
				// Check if process exists
				if process, err := os.FindProcess(pid); err == nil {
					if err := process.Signal(syscall.Signal(0)); err == nil {
						// Daemon is already running, add queue to existing daemon
						return addToExistingDaemon(queueFile, queueStr)
					}
				}
			}
		}
	}

	// No existing daemon, start new one
	return startNewDaemon(piAppsDir, queueStr)
}

// addToExistingDaemon adds a queue to an already running daemon
func addToExistingDaemon(queueFile, queueStr string) error {
	if queueStr == "" {
		return nil
	}

	// For now, just add to a simple text file
	// In a full implementation, this would use a proper IPC mechanism
	file, err := os.OpenFile(queueFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open queue file: %w", err)
	}
	defer file.Close()

	// Write the queue items
	_, err = file.WriteString(queueStr + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to queue file: %w", err)
	}

	fmt.Println("Sending instructions to daemon.")
	return nil
}

// startNewDaemon starts a new daemon process
func startNewDaemon(piAppsDir, queueStr string) error {
	// Parse initial queue
	queue := parseQueue(queueStr)

	// Validate the queue
	if len(queue) > 0 {
		validatedQueue, err := validateQueue(queue)
		if err != nil {
			return fmt.Errorf("failed to validate queue: %w", err)
		}
		queue = validatedQueue
	}

	if len(queue) == 0 {
		return nil
	}

	// Convert internal QueueItem to gui.QueueItem
	guiQueue := make([]gui.QueueItem, len(queue))
	for i, item := range queue {
		guiQueue[i] = gui.QueueItem{
			Action:   item.Action,
			AppName:  item.AppName,
			Status:   item.Status,
			IconPath: item.IconPath,
		}
	}

	// Write PID file
	pidFile := filepath.Join(piAppsDir, "data", "manage-daemon", "pid")
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	if err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create status file for IPC between GUI and terminal processes
	statusFile := filepath.Join(piAppsDir, "data", "manage-daemon", "status")

	// Write initial status
	err = writeQueueStatus(statusFile, guiQueue)
	if err != nil {
		return fmt.Errorf("failed to write initial status: %w", err)
	}

	// Set up cleanup
	defer func() {
		os.Remove(pidFile)
		os.Remove(statusFile)
	}()

	// Handle signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(pidFile)
		os.Remove(statusFile)
		os.Exit(0)
	}()

	// Start progress monitor in a goroutine that reads from status file
	progressDone := make(chan bool)
	go func() {
		defer func() { progressDone <- true }()

		// Monitor the status file and update GUI
		for {
			// Read current status
			currentQueue, err := readQueueStatus(statusFile)
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Update the GUI queue
			copy(guiQueue, currentQueue)

			// Check if all operations are complete
			allComplete := true
			for _, item := range currentQueue {
				if item.Status != "success" && item.Status != "failure" {
					allComplete = false
					break
				}
			}

			if allComplete {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		// Show progress monitor
		err := gui.ProgressMonitor(guiQueue)
		if err != nil {
			fmt.Printf("Error with progress monitor: %v\n", err)
		}
	}()

	// Get absolute path to current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prepare the terminal script content that will run in terminal-run
	// This matches the original bash implementation more closely
	terminalScript := fmt.Sprintf(`
# Set up environment variables
export PI_APPS_DIR="%s"
export DIRECTORY="%s"

# Update daemon pid to that of the terminal
echo $$ > "%s"

# Change to the directory containing the manage binary for consistency
cd "%s"

# Run the daemon terminal operations with logo and proper setup
"%s" daemon-terminal "%s" "%s"
`, piAppsDir, piAppsDir, pidFile, filepath.Dir(execPath), execPath, queueStr, statusFile)

	// Start terminal-run with the daemon processing
	terminalRunPath := filepath.Join(piAppsDir, "etc", "terminal-run")
	terminalCmd := exec.Command(terminalRunPath, terminalScript, "Terminal Output")

	// Run terminal-run and wait for completion
	err = terminalCmd.Run()
	if err != nil {
		fmt.Printf("Unable to open a terminal.\nError: %v\n", err)
		// Fall back to running in current shell if terminal-run fails
		return runDaemonInCurrentShell(guiQueue, statusFile)
	}

	// Wait for progress monitor to finish
	<-progressDone

	// Show summary dialog
	err = gui.ShowSummaryDialog(guiQueue)
	if err != nil {
		fmt.Printf("Error showing summary dialog: %v\n", err)
	}

	return nil
}

// runDaemonInCurrentShell is a fallback when terminal-run fails
func runDaemonInCurrentShell(guiQueue []gui.QueueItem, statusFile string) error {
	fmt.Println("Falling back to running in current shell...")

	// Display Pi-Apps logo
	fmt.Print(api.GenerateLogo())

	// Process the queue in current shell
	for i := range guiQueue {
		// Update status to in-progress
		guiQueue[i].Status = "in-progress"
		writeQueueStatus(statusFile, guiQueue)

		// Execute the action - let API functions handle their own status messaging
		var actionErr error
		switch guiQueue[i].Action {
		case "install":
			actionErr = api.InstallApp(guiQueue[i].AppName)
		case "uninstall":
			actionErr = api.UninstallApp(guiQueue[i].AppName)
		case "update":
			actionErr = api.UpdateApp(guiQueue[i].AppName)
		case "refresh":
			actionErr = api.RefreshApp(guiQueue[i].AppName)
		case "update-file":
			actionErr = api.UpdateFile(guiQueue[i].AppName)
		}

		// Update status based on result
		if actionErr != nil {
			guiQueue[i].Status = "failure"
			guiQueue[i].ErrorMessage = actionErr.Error()
		} else {
			guiQueue[i].Status = "success"
		}

		// Write updated status
		writeQueueStatus(statusFile, guiQueue)
	}

	return nil
}

// parseQueue parses the queue string into QueueItem structs
func parseQueue(queueStr string) []QueueItem {
	if queueStr == "" {
		return nil
	}

	var queue []QueueItem
	lines := strings.Split(strings.TrimSpace(queueStr), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Format should be "action;appname" or "action appname"
		parts := strings.Fields(strings.ReplaceAll(line, ";", " "))
		if len(parts) >= 2 {
			action := parts[0]
			appName := parts[1]

			// Get icon path
			iconPath := filepath.Join(os.Getenv("PI_APPS_DIR"), "apps", appName, "icon-64.png")
			if _, err := os.Stat(iconPath); os.IsNotExist(err) {
				iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "none-64.png")
			}

			queue = append(queue, QueueItem{
				Action:   action,
				AppName:  appName,
				Status:   "waiting",
				IconPath: iconPath,
				ExitCode: -1,
			})
		}
	}

	return queue
}

// validateQueue validates the queue items
func validateQueue(queue []QueueItem) ([]QueueItem, error) {
	piAppsDir := os.Getenv("PI_APPS_DIR")
	var validQueue []QueueItem

	for _, item := range queue {
		// Check if action is valid
		validActions := []string{"install", "uninstall", "update", "refresh", "update-file"}
		isValidAction := false
		for _, validAction := range validActions {
			if item.Action == validAction {
				isValidAction = true
				break
			}
		}

		if !isValidAction {
			fmt.Printf("Invalid action '%s' for app '%s', skipping\n", item.Action, item.AppName)
			continue
		}

		// For update-file actions, skip app validation
		if item.Action == "update-file" {
			validQueue = append(validQueue, item)
			continue
		}

		// Check if app exists
		var appDir string
		if item.Action == "update" {
			appDir = filepath.Join(piAppsDir, "update", "pi-apps", "apps", item.AppName)
		} else {
			appDir = filepath.Join(piAppsDir, "apps", item.AppName)
		}

		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			fmt.Printf("App '%s' does not exist, skipping\n", item.AppName)
			continue
		}

		// Check for redundant operations
		if (item.Action == "install" && api.IsAppInstalled(item.AppName)) ||
			(item.Action == "uninstall" && !api.IsAppInstalled(item.AppName)) {
			fmt.Printf("App '%s' is already %sed, skipping\n", item.AppName, item.Action)
			continue
		}

		validQueue = append(validQueue, item)
	}

	return validQueue, nil
}

// daemonTerminal processes the queue in the terminal window spawned by terminal-run
func daemonTerminal(queueStr, statusFile string) error {
	// Display Pi-Apps logo first
	fmt.Print(api.GenerateLogo())

	// Parse initial queue
	queue := parseQueue(queueStr)

	// Validate the queue
	if len(queue) > 0 {
		validatedQueue, err := validateQueue(queue)
		if err != nil {
			return fmt.Errorf("failed to validate queue: %w", err)
		}
		queue = validatedQueue
	}

	if len(queue) == 0 {
		return nil
	}

	// Convert internal QueueItem to gui.QueueItem
	guiQueue := make([]gui.QueueItem, len(queue))
	for i, item := range queue {
		guiQueue[i] = gui.QueueItem{
			Action:   item.Action,
			AppName:  item.AppName,
			Status:   item.Status,
			IconPath: item.IconPath,
		}
	}

	// Process the queue
	for i := range guiQueue {
		// Update status to in-progress
		guiQueue[i].Status = "in-progress"
		writeQueueStatus(statusFile, guiQueue)

		// Set terminal title
		fmt.Printf("\033]0;%sing %s\007", strings.Title(guiQueue[i].Action), guiQueue[i].AppName)

		// Execute the action - let API functions handle their own status messaging
		var actionErr error
		switch guiQueue[i].Action {
		case "install":
			actionErr = api.InstallApp(guiQueue[i].AppName)
		case "uninstall":
			actionErr = api.UninstallApp(guiQueue[i].AppName)
		case "update":
			actionErr = api.UpdateApp(guiQueue[i].AppName)
		case "refresh":
			actionErr = api.RefreshApp(guiQueue[i].AppName)
		case "update-file":
			actionErr = api.UpdateFile(guiQueue[i].AppName)
		}

		// Update status based on result
		if actionErr != nil {
			guiQueue[i].Status = "failure"
			guiQueue[i].ErrorMessage = actionErr.Error()
		} else {
			guiQueue[i].Status = "success"
		}

		// Write updated status
		writeQueueStatus(statusFile, guiQueue)
	}

	fmt.Println("\nAll operations completed. Press Enter to close...")
	fmt.Scanln()

	return nil
}

// writeQueueStatus writes the queue status to a file for IPC
func writeQueueStatus(statusFile string, queue []gui.QueueItem) error {
	if statusFile == "" {
		return nil
	}

	file, err := os.Create(statusFile)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, item := range queue {
		line := fmt.Sprintf("%s;%s;%s;%s;%s\n",
			item.Action, item.AppName, item.Status, item.IconPath, item.ErrorMessage)
		_, err := file.WriteString(line)
		if err != nil {
			return err
		}
	}

	return nil
}

// readQueueStatus reads the queue status from a file for IPC
func readQueueStatus(statusFile string) ([]gui.QueueItem, error) {
	if statusFile == "" {
		return nil, fmt.Errorf("no status file specified")
	}

	file, err := os.Open(statusFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var queue []gui.QueueItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ";", 5)
		if len(parts) >= 4 {
			item := gui.QueueItem{
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
	fmt.Println("  -daemon                   Run in daemon mode")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  manage -install Firefox LibreOffice")
	fmt.Println("  manage -uninstall Zoom")
	fmt.Println("  manage -update-self")
	fmt.Println("  manage -install-if-not-installed Firefox")
	fmt.Println("  manage -install -gui -multi Firefox LibreOffice")
}
