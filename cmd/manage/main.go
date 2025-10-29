package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pi-apps-go/pi-apps/pkg/api"
	"github.com/pi-apps-go/pi-apps/pkg/gui"
)

// Build-time variables
var (
	BuildDate string
	GitCommit string
	logger    = log.New(os.Stderr, "pi-apps-manage: ", log.LstdFlags)
)

func main() {
	// runtime crashes can happen (keep in mind Pi-Apps Go is ALPHA software)
	// so add a handler to log those runtime errors to save them to a log file
	// this option can be disabled by specifying DISABLE_ERROR_HANDLING to true
	// Edit: nevermind, cgo crashes are not handled by this handler

	errorHandling := os.Getenv("DISABLE_ERROR_HANDLING")
	if errorHandling != "true" {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace as a string
				buf := make([]byte, 1024*1024)
				n := runtime.Stack(buf, false)
				stackTrace := string(buf[:n])

				logger.Printf("Panic recovered: %v", r)

				// Format the full crash report
				crashReport := fmt.Sprintf(
					"Pi-Apps Go has encountered a error and had to shutdown.\n\nReason: %v\n\nStack trace:\n%s",
					r,
					stackTrace,
				)

				// Display the error to the user
				api.ErrorNoExit(crashReport)

				// later put a function to write it to the log file in the logs folder
				os.Exit(1)
			}
		}()
	}

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
	versionFlag := flag.Bool("version", false, "Show version information")

	// Custom error handling for undefined flags
	flag.Usage = printUsage

	// Parse flags
	err := flag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		api.ErrorNoExit("Error: " + err.Error())
		printUsage()
		os.Exit(1)
	}

	// Check for version flag first
	if *versionFlag {
		fmt.Println("Pi-Apps Go management tool (rolling release)")
		if BuildDate != "" {
			api.Status(fmt.Sprintf("Built on %s", BuildDate))
		} else {
			api.ErrorNoExit("Build date not available")
		}
		if GitCommit != "" {
			api.Status(fmt.Sprintf("Git commit: %s", GitCommit))
			account, repo := api.GetGitUrl()
			if account != "" && repo != "" {
				api.Status(fmt.Sprintf("Link to commit: https://github.com/%s/%s/commit/%s", account, repo, GitCommit))
			}
		} else {
			api.ErrorNoExit("Git commit hash not available")
		}
		return
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
		var queueStr, statusFile, queuePipe string
		if len(args) > 1 {
			queueStr = args[1]
		}
		if len(args) > 2 {
			statusFile = args[2]
		}
		if len(args) > 3 {
			queuePipe = args[3]
		}
		err := daemonTerminal(queueStr, statusFile, queuePipe)
		if err != nil {
			api.ErrorNoExit("Daemon terminal error: " + err.Error())
			os.Exit(1)
		}
		return
	}

	// Check for view_file mode (called from diagnosis dialog)
	if len(args) > 0 && args[0] == "view_file" {
		if len(args) < 2 {
			api.ErrorNoExit("Error: view_file requires a file path")
			os.Exit(1)
		}
		err := api.ViewFile(args[1])
		if err != nil {
			api.ErrorNoExit("View file error: " + err.Error())
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
		// Make it show a warning considering on the original Pi-Apps manage script, this would redirect to the updater script if you ran update-all or check-all
		api.Warning("The manage package ONLY updates apps, and this mode redirects to the updater package.\nIf you want to update Pi-Apps Go from the command-line, please use:\n" + fmt.Sprintf("%s/updater cli-yes", piAppsDir))
		api.Status("Updating \u001b[1mPi-Apps\u001b[22m...")

		cmd := exec.Command(fmt.Sprintf("%s/updater", piAppsDir), "cli-yes")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			api.ErrorNoExit("Error updating Pi-Apps: " + err.Error())
		}
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
			// Use command-line validation - convert types
			internalQueue := make([]QueueItem, len(queue))
			for i, item := range queue {
				internalQueue[i] = QueueItem{
					Action:   item.Action,
					AppName:  item.AppName,
					Status:   item.Status,
					IconPath: item.IconPath,
				}
			}

			validatedQueue, err := validateQueue(internalQueue)
			if err != nil {
				api.ErrorNoExit("Error validating apps: " + err.Error())
			}

			// Convert back to gui.QueueItem
			queue = make([]gui.QueueItem, len(validatedQueue))
			for i, item := range validatedQueue {
				queue[i] = gui.QueueItem{
					Action:   item.Action,
					AppName:  item.AppName,
					Status:   item.Status,
					IconPath: item.IconPath,
				}
			}
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
				// Check if already uninstalled and allow uninstall for corrupted apps
				appStatus, statusErr := api.GetAppStatus(queue[i].AppName)
				switch {
				case statusErr != nil:
					err = fmt.Errorf("failed to get app status: %w", statusErr)
				case appStatus == "uninstalled":
					api.Status(fmt.Sprintf("App '%s' is already uninstalled, skipping", queue[i].AppName))
					continue
				default:
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
				// Check if already uninstalled and allow uninstall for corrupted apps
				appStatus, statusErr := api.GetAppStatus(queue[i].AppName)
				switch {
				case statusErr != nil:
					err = fmt.Errorf("failed to get app status: %w", statusErr)
				case appStatus == "uninstalled":
					api.Status(fmt.Sprintf("App '%s' is already uninstalled, skipping", queue[i].AppName))
					continue
				default:
					err = api.UninstallApp(queue[i].AppName)
				}
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

	// Open the named pipe for writing
	file, err := os.OpenFile(queueFile, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open queue pipe: %w", err)
	}
	defer file.Close()

	// Write the queue items to the pipe
	_, err = file.WriteString(queueStr + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to queue pipe: %w", err)
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

	// Add mutex for queue synchronization
	var queueMutex sync.Mutex

	// Write PID file
	pidFile := filepath.Join(piAppsDir, "data", "manage-daemon", "pid")
	err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	if err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create named pipe for IPC (like the bash version)
	queuePipe := filepath.Join(piAppsDir, "data", "manage-daemon", "queue")
	if _, err := os.Stat(queuePipe); os.IsNotExist(err) {
		err = syscall.Mkfifo(queuePipe, 0644)
		if err != nil {
			return fmt.Errorf("failed to create queue pipe: %w", err)
		}
	}

	// Create status file for IPC between GUI and terminal processes
	statusFile := filepath.Join(piAppsDir, "data", "manage-daemon", "status")

	// Write initial status
	queueMutex.Lock()
	err = writeQueueStatus(statusFile, guiQueue)
	queueMutex.Unlock()
	if err != nil {
		return fmt.Errorf("failed to write initial status: %w", err)
	}

	// Set up cleanup
	defer func() {
		os.Remove(pidFile)
		os.Remove(statusFile)
		os.Remove(queuePipe)
	}()

	// Handle signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(pidFile)
		os.Remove(statusFile)
		os.Remove(queuePipe)
		os.Exit(0)
	}()

	// Start progress monitor with initial queue only (it will show static progress)
	progressDone := make(chan bool, 1)
	go func() {
		// Start with initial queue
		progressQueue := make([]gui.QueueItem, len(guiQueue))
		copy(progressQueue, guiQueue)

		// Use daemon progress monitor with the initial queue
		// It will read status updates from status file
		err := gui.ProgressMonitorDaemon(progressQueue)
		if err != nil {
			fmt.Printf("Error with progress monitor: %v\n", err)
		}
		progressDone <- true
	}()

	// Start queue listener for new incoming requests
	queueUpdate := make(chan string, 10) // Buffered channel for new queue items
	go func() {
		for {
			// Open the named pipe for reading (this will block until something writes to it)
			file, err := os.OpenFile(queuePipe, os.O_RDONLY, 0644)
			if err != nil {
				fmt.Printf("Warning: failed to open queue pipe for reading: %v\n", err)
				time.Sleep(1 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					fmt.Printf("Received new queue request: %s\n", line)
					queueUpdate <- line
				}
			}
			file.Close()

			if err := scanner.Err(); err != nil {
				fmt.Printf("Warning: error reading from queue pipe: %v\n", err)
			}
		}
	}()

	// Simplified status monitoring - just wait for terminal process to complete
	statusMonitorDone := make(chan bool, 1)
	go func() {
		for {
			// Check if terminal process is still running by checking PID file
			pidBytes, err := os.ReadFile(pidFile)
			if err != nil {
				// PID file doesn't exist, terminal process is done
				statusMonitorDone <- true
				break
			}

			if pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes))); err == nil {
				// Check if the process still exists
				if process, err := os.FindProcess(pid); err != nil || process.Signal(syscall.Signal(0)) != nil {
					// Process doesn't exist anymore, terminal process is done
					statusMonitorDone <- true
					break
				}
			}

			time.Sleep(1 * time.Second)
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
"%s" daemon-terminal "%s" "%s" "%s"
`, piAppsDir, piAppsDir, pidFile, filepath.Dir(execPath), execPath, queueStr, statusFile, queuePipe)

	// Start terminal-run with the daemon processing
	terminalRunPath := filepath.Join(piAppsDir, "etc", "terminal-run")
	terminalCmd := exec.Command(terminalRunPath, terminalScript, "Terminal Output")

	// Run terminal-run and wait for completion
	err = terminalCmd.Run()
	if err != nil {
		fmt.Printf("Unable to open a terminal.\nError: %v\n", err)

		// Show GUI error dialog if this was a GUI request (similar to bash version)
		errorText := fmt.Sprintf("Unable to open a terminal.\nDebug output below.\n%v", err)
		gui.ShowMessageDialog("Error occurred when calling terminal-run", errorText, 3) // MessageType 3 is ERROR

		// Fall back to running in current shell if terminal-run fails
		return runDaemonInCurrentShell(guiQueue, statusFile)
	}

	// Wait for status monitor to detect completion
	<-statusMonitorDone

	// Wait for progress monitor to close first
	<-progressDone

	// Read final queue state from status file for accurate summary
	finalQueue, err := readQueueStatus(statusFile)
	if err != nil {
		fmt.Printf("Warning: failed to read final queue status: %v\n", err)
		// Fall back to in-memory queue if status file read fails
		finalQueue = guiQueue
	}

	// Show summary dialog
	err = gui.ShowSummaryDialog(finalQueue)
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

	// Process the queue with retry loop for failed apps
	for {
		currentIndex := 0
		// Find next unprocessed item
		for i := range guiQueue {
			if guiQueue[i].Status == "waiting" {
				currentIndex = i
				break
			}
		}

		// Check if all items are processed
		allProcessed := true
		for _, item := range guiQueue {
			if item.Status == "waiting" {
				allProcessed = false
				break
			}
		}

		if allProcessed {
			// Check for failed apps and run diagnosis
			var failedApps []string
			for _, item := range guiQueue {
				if item.Status == "failure" {
					failedApps = append(failedApps, fmt.Sprintf("%s;%s", item.Action, item.AppName))
				}
			}

			if len(failedApps) > 0 {
				// Run diagnosis on failed apps
				fmt.Println("\nDiagnosing failed operations...")
				failureList := strings.Join(failedApps, "\n")

				// Call the diagnose_apps function from the API
				results := api.DiagnoseApps(failureList)

				// Process the diagnosis results
				var retryApps []string
				for _, result := range results {
					if result.Action == "retry" {
						retryApps = append(retryApps, result.ActionStr)
					}
				}

				if len(retryApps) > 0 {
					// User chose to retry some operations
					// Mark failed apps as "diagnosed" to avoid repeated diagnosis
					for i := range guiQueue {
						if guiQueue[i].Status == "failure" {
							guiQueue[i].Status = "diagnosed"
						}
					}

					// Add retry operations to the queue
					retryQueue := parseQueue(strings.Join(retryApps, "\n"))
					for _, retryItem := range retryQueue {
						// Ensure icon path is properly set for retry items
						iconPath := filepath.Join(os.Getenv("PI_APPS_DIR"), "apps", retryItem.AppName, "icon-64.png")
						if _, err := os.Stat(iconPath); os.IsNotExist(err) {
							iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "none-64.png")
						}

						newGuiItem := gui.QueueItem{
							Action:   retryItem.Action,
							AppName:  retryItem.AppName,
							Status:   "waiting",
							IconPath: iconPath,
						}
						guiQueue = append(guiQueue, newGuiItem)
					}

					// Reorder the queue to prioritize updates and refreshes
					guiQueue = reorderList(guiQueue)

					// Write status update to show diagnosed items
					err := writeQueueStatus(statusFile, guiQueue)
					if err != nil {
						fmt.Printf("Warning: failed to write updated status: %v\n", err)
					}

					// Add a small delay before starting retries (like in original implementation)
					fmt.Println("Preparing to retry operations...")
					time.Sleep(2 * time.Second)

					// Continue processing the new items
					continue
				} else {
					// No retries requested, we're done
					break
				}
			} else {
				// No failed apps, we're done
				break
			}
		}

		// Process next waiting item
		if currentIndex < len(guiQueue) && guiQueue[currentIndex].Status == "waiting" {
			// Update status to in-progress
			guiQueue[currentIndex].Status = "in-progress"
			err := writeQueueStatus(statusFile, guiQueue)
			if err != nil {
				fmt.Printf("Warning: failed to write status: %v\n", err)
			}

			// Set terminal title
			fmt.Printf("\033]0;%sing %s\007", strings.ToUpper(guiQueue[currentIndex].Action[:1])+guiQueue[currentIndex].Action[1:], guiQueue[currentIndex].AppName)

			// Execute the action - let API functions handle their own status messaging
			var actionErr error
			switch guiQueue[currentIndex].Action {
			case "install":
				actionErr = api.InstallApp(guiQueue[currentIndex].AppName)
			case "uninstall":
				actionErr = api.UninstallApp(guiQueue[currentIndex].AppName)
			case "update":
				actionErr = api.UpdateApp(guiQueue[currentIndex].AppName)
			case "refresh":
				actionErr = api.RefreshApp(guiQueue[currentIndex].AppName)
			case "update-file":
				actionErr = api.UpdateFile(guiQueue[currentIndex].AppName)
			}

			// Update status based on result
			if actionErr != nil {
				guiQueue[currentIndex].Status = "failure"
				guiQueue[currentIndex].ErrorMessage = actionErr.Error()

				// Format the log file to add device information for failed operations
				logFile := api.GetLogfile(guiQueue[currentIndex].AppName)
				if api.FileExists(logFile) {
					err := api.FormatLogfile(logFile)
					if err != nil {
						fmt.Printf("Warning: failed to format log file %s: %v\n", logFile, err)
					}
				}
			} else {
				guiQueue[currentIndex].Status = "success"

				// Format the log file for successful operations too (consistent with bash version)
				logFile := api.GetLogfile(guiQueue[currentIndex].AppName)
				if api.FileExists(logFile) {
					err := api.FormatLogfile(logFile)
					if err != nil {
						fmt.Printf("Warning: failed to format log file %s: %v\n", logFile, err)
					}
				}
			}

			// Write updated status
			err = writeQueueStatus(statusFile, guiQueue)
			if err != nil {
				fmt.Printf("Warning: failed to write status: %v\n", err)
			}
		}
	}

	// Signal the progress monitor that daemon processing is complete
	// Add a special completion marker to the queue
	guiQueue = append(guiQueue, gui.QueueItem{
		Action:   "daemon",
		AppName:  "completed",
		Status:   "daemon-complete",
		IconPath: "",
	})
	err := writeQueueStatus(statusFile, guiQueue)
	if err != nil {
		fmt.Printf("Warning: failed to write completion status: %v\n", err)
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

		var action, appName string

		// Check if line contains semicolon delimiter
		if strings.Contains(line, ";") {
			// Format: "action;appname" - split on semicolon
			parts := strings.SplitN(line, ";", 2)
			if len(parts) >= 2 {
				action = strings.TrimSpace(parts[0])
				appName = strings.TrimSpace(parts[1])
			}
		} else {
			// Format: "action appname" - split on space but preserve app name with spaces
			parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
			if len(parts) >= 2 {
				action = parts[0]
				appName = parts[1]
			}
		}

		if action != "" && appName != "" {
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

// validateQueue validates the queue items and shows GUI dialogs for errors if in GUI mode
func validateQueue(queue []QueueItem) ([]QueueItem, error) {
	return validateQueueWithGUI(queue, false)
}

// validateQueueWithGUI validates the queue items with optional GUI error dialogs
func validateQueueWithGUI(queue []QueueItem, useGUI bool) ([]QueueItem, error) {
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
			errorMsg := fmt.Sprintf("Invalid action '%s' for app '%s', skipping", item.Action, item.AppName)
			if useGUI {
				gui.ShowMessageDialog("Error", fmt.Sprintf("Invalid action: <b>%s</b>", item.Action), 3)
			} else {
				fmt.Println(errorMsg)
			}
			continue
		}

		// For update-file actions, skip app validation
		if item.Action == "update-file" {
			validQueue = append(validQueue, item)
			continue
		}

		// Check if app exists
		var appDir string
		if item.Action == "update" || item.Action == "refresh" {
			appDir = filepath.Join(piAppsDir, "update", "pi-apps", "apps", item.AppName)
		} else {
			appDir = filepath.Join(piAppsDir, "apps", item.AppName)
		}

		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			errorMsg := fmt.Sprintf("App '%s' does not exist, skipping", item.AppName)
			if useGUI {
				gui.ShowMessageDialog("Error", fmt.Sprintf("Invalid app \"<b>%s</b>\". Cannot %s it.", item.AppName, item.Action), 3)
			} else {
				fmt.Println(errorMsg)
			}
			continue
		}

		// Check for redundant operations
		appStatus, err := api.GetAppStatus(item.AppName)
		if err != nil {
			// If we can't get status, continue with the operation
			validQueue = append(validQueue, item)
			continue
		}

		if (item.Action == "install" && appStatus == "installed") ||
			(item.Action == "uninstall" && appStatus == "uninstalled") {
			infoMsg := fmt.Sprintf("App '%s' is already %sed, skipping", item.AppName, item.Action)
			if useGUI {
				// In GUI mode, this would typically be handled by ValidateAppsGUI, so just inform
				fmt.Println(infoMsg)
			} else {
				fmt.Println(infoMsg)
			}
			continue
		}
		// Note: corrupted apps are allowed to be both installed and uninstalled

		validQueue = append(validQueue, item)
	}

	return validQueue, nil
}

// reorderList reorders the queue to prioritize app refreshes and file updates over installs/uninstalls
func reorderList(queue []gui.QueueItem) []gui.QueueItem {
	// Split queue into completed and pending items
	var completedItems []gui.QueueItem
	var pendingItems []gui.QueueItem

	for _, item := range queue {
		// Check if item is completed (has a numeric status code)
		if _, err := strconv.Atoi(item.Status); err == nil {
			completedItems = append(completedItems, item)
		} else {
			pendingItems = append(pendingItems, item)
		}
	}

	// Split pending items by type
	var pendingRefreshes []gui.QueueItem
	var pendingFileUpdates []gui.QueueItem
	var pendingOther []gui.QueueItem

	for _, item := range pendingItems {
		switch item.Action {
		case "refresh":
			pendingRefreshes = append(pendingRefreshes, item)
		case "update-file":
			pendingFileUpdates = append(pendingFileUpdates, item)
		default:
			pendingOther = append(pendingOther, item)
		}
	}

	// Reconstruct queue in priority order:
	// 1. Completed items (unchanged)
	// 2. File updates
	// 3. App refreshes
	// 4. Other operations (installs/uninstalls)
	var reorderedQueue []gui.QueueItem
	reorderedQueue = append(reorderedQueue, completedItems...)
	reorderedQueue = append(reorderedQueue, pendingFileUpdates...)
	reorderedQueue = append(reorderedQueue, pendingRefreshes...)
	reorderedQueue = append(reorderedQueue, pendingOther...)

	return reorderedQueue
}

// daemonTerminal processes the queue in the terminal window spawned by terminal-run
func daemonTerminal(queueStr, statusFile, queuePipe string) error {
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

	// Write initial status
	err := writeQueueStatus(statusFile, guiQueue)
	if err != nil {
		fmt.Printf("Warning: failed to write initial status: %v\n", err)
	}

	// Start queue listener for new incoming requests (if pipe is provided)
	if queuePipe != "" {
		go func() {
			for {
				// Open the named pipe for reading (this will block until something writes to it)
				file, err := os.OpenFile(queuePipe, os.O_RDONLY, 0644)
				if err != nil {
					fmt.Printf("Warning: failed to open queue pipe for reading: %v\n", err)
					time.Sleep(1 * time.Second)
					continue
				}

				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line != "" {
						fmt.Printf("Received new queue request: %s\n", line)

						// Parse new queue items
						newQueue := parseQueue(line)

						// Validate new queue items
						validatedNewQueue, err := validateQueue(newQueue)
						if err != nil {
							fmt.Printf("Warning: failed to validate new queue items: %v\n", err)
							continue
						}

						// Add new items to the existing queue
						for _, newItem := range validatedNewQueue {
							newGuiItem := gui.QueueItem{
								Action:   newItem.Action,
								AppName:  newItem.AppName,
								Status:   "waiting",
								IconPath: newItem.IconPath,
							}
							guiQueue = append(guiQueue, newGuiItem)
						}

						// Update status file with new items
						err = writeQueueStatus(statusFile, guiQueue)
						if err != nil {
							fmt.Printf("Warning: failed to write updated status: %v\n", err)
						}
					}
				}
				file.Close()

				if err := scanner.Err(); err != nil {
					fmt.Printf("Warning: error reading from queue pipe: %v\n", err)
				}
			}
		}()
	}

	// Process the queue with retry loop for failed apps
	for {
		currentIndex := 0
		// Find next unprocessed item
		for i := range guiQueue {
			if guiQueue[i].Status == "waiting" {
				currentIndex = i
				break
			}
		}

		// Check if all items are processed
		allProcessed := true
		for _, item := range guiQueue {
			if item.Status == "waiting" {
				allProcessed = false
				break
			}
		}

		if allProcessed {
			// Check for failed apps and run diagnosis
			var failedApps []string
			for _, item := range guiQueue {
				if item.Status == "failure" {
					failedApps = append(failedApps, fmt.Sprintf("%s;%s", item.Action, item.AppName))
				}
			}

			if len(failedApps) > 0 {
				// Run diagnosis on failed apps
				fmt.Println("\nDiagnosing failed operations...")
				failureList := strings.Join(failedApps, "\n")

				// Call the diagnose_apps function from the API
				results := api.DiagnoseApps(failureList)

				// Process the diagnosis results
				var retryApps []string
				for _, result := range results {
					if result.Action == "retry" {
						retryApps = append(retryApps, result.ActionStr)
					}
				}

				if len(retryApps) > 0 {
					// User chose to retry some operations
					// Mark failed apps as "diagnosed" to avoid repeated diagnosis
					for i := range guiQueue {
						if guiQueue[i].Status == "failure" {
							guiQueue[i].Status = "diagnosed"
						}
					}

					// Add retry operations to the queue
					retryQueue := parseQueue(strings.Join(retryApps, "\n"))
					for _, retryItem := range retryQueue {
						// Ensure icon path is properly set for retry items
						iconPath := filepath.Join(os.Getenv("PI_APPS_DIR"), "apps", retryItem.AppName, "icon-64.png")
						if _, err := os.Stat(iconPath); os.IsNotExist(err) {
							iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "none-64.png")
						}

						newGuiItem := gui.QueueItem{
							Action:   retryItem.Action,
							AppName:  retryItem.AppName,
							Status:   "waiting",
							IconPath: iconPath,
						}
						guiQueue = append(guiQueue, newGuiItem)
					}

					// Reorder the queue to prioritize updates and refreshes
					guiQueue = reorderList(guiQueue)

					// Write status update to show diagnosed items
					err := writeQueueStatus(statusFile, guiQueue)
					if err != nil {
						fmt.Printf("Warning: failed to write updated status: %v\n", err)
					}

					// Add a small delay before starting retries (like in original implementation)
					fmt.Println("Preparing to retry operations...")
					time.Sleep(2 * time.Second)

					// Continue processing the new items
					continue
				} else {
					// No retries requested, we're done
					break
				}
			} else {
				// No failed apps, we're done
				break
			}
		}

		// Process next waiting item
		if currentIndex < len(guiQueue) && guiQueue[currentIndex].Status == "waiting" {
			// Update status to in-progress
			guiQueue[currentIndex].Status = "in-progress"
			err := writeQueueStatus(statusFile, guiQueue)
			if err != nil {
				fmt.Printf("Warning: failed to write status: %v\n", err)
			}

			// Set terminal title
			fmt.Printf("\033]0;%sing %s\007", strings.ToUpper(guiQueue[currentIndex].Action[:1])+guiQueue[currentIndex].Action[1:], guiQueue[currentIndex].AppName)

			// Execute the action - let API functions handle their own status messaging
			var actionErr error
			switch guiQueue[currentIndex].Action {
			case "install":
				actionErr = api.InstallApp(guiQueue[currentIndex].AppName)
			case "uninstall":
				actionErr = api.UninstallApp(guiQueue[currentIndex].AppName)
			case "update":
				actionErr = api.UpdateApp(guiQueue[currentIndex].AppName)
			case "refresh":
				actionErr = api.RefreshApp(guiQueue[currentIndex].AppName)
			case "update-file":
				actionErr = api.UpdateFile(guiQueue[currentIndex].AppName)
			}

			// Update status based on result
			if actionErr != nil {
				guiQueue[currentIndex].Status = "failure"
				guiQueue[currentIndex].ErrorMessage = actionErr.Error()

				// Format the log file to add device information for failed operations
				logFile := api.GetLogfile(guiQueue[currentIndex].AppName)
				if api.FileExists(logFile) {
					err := api.FormatLogfile(logFile)
					if err != nil {
						fmt.Printf("Warning: failed to format log file %s: %v\n", logFile, err)
					}
				}
			} else {
				guiQueue[currentIndex].Status = "success"

				// Format the log file for successful operations too (consistent with bash version)
				logFile := api.GetLogfile(guiQueue[currentIndex].AppName)
				if api.FileExists(logFile) {
					err := api.FormatLogfile(logFile)
					if err != nil {
						fmt.Printf("Warning: failed to format log file %s: %v\n", logFile, err)
					}
				}
			}

			// Write updated status
			err = writeQueueStatus(statusFile, guiQueue)
			if err != nil {
				fmt.Printf("Warning: failed to write status: %v\n", err)
			}
		}
	}

	fmt.Println("\nAll operations completed. Press Enter to close...")
	fmt.Scanln()

	// Signal the progress monitor that daemon processing is complete
	// Add a special completion marker to the queue
	guiQueue = append(guiQueue, gui.QueueItem{
		Action:   "daemon",
		AppName:  "completed",
		Status:   "daemon-complete",
		IconPath: "",
	})
	err = writeQueueStatus(statusFile, guiQueue)
	if err != nil {
		fmt.Printf("Warning: failed to write completion status: %v\n", err)
	}

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
		// Ensure icon path is valid (not empty or a directory)
		iconPath := item.IconPath
		if iconPath == "" || iconPath == os.Getenv("PI_APPS_DIR") {
			// Fix invalid icon paths
			iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "apps", item.AppName, "icon-64.png")
			if _, err := os.Stat(iconPath); os.IsNotExist(err) {
				iconPath = filepath.Join(os.Getenv("PI_APPS_DIR"), "icons", "none-64.png")
			}
		}

		line := fmt.Sprintf("%s;%s;%s;%s;%s\n",
			item.Action, item.AppName, item.Status, iconPath, item.ErrorMessage)
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
	fmt.Println("  -version                  Show version information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  manage -install Firefox LibreOffice")
	fmt.Println("  manage -uninstall Zoom")
	fmt.Println("  manage -update-self")
	fmt.Println("  manage -install-if-not-installed Firefox")
	fmt.Println("  manage -install -gui -multi Firefox LibreOffice")
}
