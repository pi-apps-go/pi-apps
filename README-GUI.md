# Pi-Apps GTK3 GUI Implementation

This document describes the GTK3 GUI components implemented in the Go rewrite of Pi-Apps, focusing on the management functions that were previously using YAD in the bash implementation.

## GUI Components Implemented

The GUI components implemented include:

1. **App Validation GUI (`ValidateAppsGUI`)**: This checks if apps are valid, confirms installation/uninstallation of already installed/uninstalled apps, and checks if apps are outdated.

2. **Progress Monitor (`ProgressMonitor`)**: Shows a dialog with the current progress of operations, displaying a list of pending operations and their status.

3. **Summary Window with Donation Reminders (`ShowSummaryDialog`)**: A window that summarizes completed actions and includes donation reminders with links to sponsor the developers.

4. **Broken Local Packages Repo Prompt (`ShowBrokenPackagesDialog`)**: A dialog to ask for sudo password to repair broken package repositories.

## How to Use

### Command Line Interface

The management functionality can be accessed through the `manage` command, which provides a variety of options:

```bash
# Installing an app with GUI validation and progress monitoring
./bin/manage -install -gui -multi Firefox

# Uninstalling multiple apps with GUI
./bin/manage -uninstall -gui -multi Zoom LibreOffice

# Installing an app only if it's not already installed
./bin/manage -install-if-not-installed Firefox

# Updating Pi-Apps itself
./bin/manage -update-self
```

### API Usage

The GUI components can also be used directly in Go code:

```go
import (
    "github.com/botspot/pi-apps/pkg/gui"
)

// Create a queue of operations
queue := []gui.QueueItem{
    {
        Action:   "install",
        AppName:  "Firefox",
        Status:   "waiting",
        IconPath: "/path/to/firefox/icon.png",
    },
}

// Validate the queue using GUI
validatedQueue, err := gui.ValidateAppsGUI(queue)
if err != nil {
    // Handle error
}

// Show progress monitor
go func() {
    err := gui.ProgressMonitor(validatedQueue)
    if err != nil {
        // Handle error
    }
}()

// After operations are complete, show summary dialog
err = gui.ShowSummaryDialog(completedQueue)
if err != nil {
    // Handle error
}

// Show broken packages dialog
password, err := gui.ShowBrokenPackagesDialog()
if err != nil {
    // User canceled or another error occurred
} else {
    // Use password to repair broken packages
}
```

## Implementation Details

### QueueItem

The core data structure used by the GUI components is the `QueueItem`, which represents an item in the installation/uninstallation queue:

```go
type QueueItem struct {
    Action   string // install, uninstall, update, refresh
    AppName  string
    Status   string // waiting, in-progress, success, failure
    IconPath string
}
```

### Icon Mappings

The GUI uses consistent icons across all dialogs, mapped by status and action:

```go
// StatusIconMapping maps status to icon paths
var StatusIconMapping = map[string]string{
    "waiting":     "icons/wait.png",
    "in-progress": "icons/prompt.png",
    "success":     "icons/success.png",
    "failure":     "icons/failure.png",
}

// ActionIconMapping maps actions to icon paths
var ActionIconMapping = map[string]string{
    "install":     "icons/install.png",
    "uninstall":   "icons/uninstall.png",
    "update":      "icons/update.png",
    "refresh":     "icons/refresh.png",
    "update-file": "icons/update.png",
}
```

## Building

To build the Pi-Apps API and management tools:

```bash
./build.sh
```

This will compile both the main API (`bin/api`) and the management command-line interface (`bin/manage`).

## Environment Variables

The GUI components rely on the `PI_APPS_DIR` environment variable to locate icons and app directories. If not set, it will attempt to find the Pi-Apps directory in common locations or use a default path.

## Future Improvements

Planned future improvements include:

1. Implementing command-line validation for non-GUI usage
2. Adding more terminal-related functionality and progress monitoring
3. Enhancing error reporting and user feedback
4. Implementing a proper update mechanism for Pi-Apps itself 