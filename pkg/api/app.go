package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// AppStatus returns the current status of an app: installed, uninstalled, etc.
func AppStatus(app string) (string, error) {
	if app == "" {
		return "", fmt.Errorf("app_status: no app specified")
	}

	// Get the Pi-Apps directory
	directory := os.Getenv("DIRECTORY")
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("app_status: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Check if app exists
	appDir := filepath.Join(directory, "apps", app)
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return "", fmt.Errorf("app_status: app %s does not exist", app)
	}

	// Check if the app has a status file
	statusFile := filepath.Join(directory, "data", "status", app)
	if _, err := os.Stat(statusFile); os.IsNotExist(err) {
		return "uninstalled", nil
	}

	// Read the status file
	statusBytes, err := os.ReadFile(statusFile)
	if err != nil {
		return "", fmt.Errorf("app_status: failed to read status file: %w", err)
	}

	status := string(statusBytes)
	return status, nil
}

// RemoveDeprecatedApp prompts a user to uninstall a deprecated pi-apps application
// This is a Go implementation of the original bash remove_deprecated_app function
func RemoveDeprecatedApp(app, removalArch, message string) error {
	if app == "" {
		return fmt.Errorf("remove_deprecated_app(): requires a pi-apps app name")
	}

	// Get the Pi-Apps directory
	directory := os.Getenv("DIRECTORY")
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("remove_deprecated_app: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Get the app status
	appStatus, err := AppStatus(app)
	if err != nil {
		return fmt.Errorf("remove_deprecated_app: failed to get app status: %w", err)
	}

	// Get the system architecture
	archCmd := exec.Command("getconf", "LONG_BIT")
	archOutput, err := archCmd.Output()
	if err != nil {
		return fmt.Errorf("remove_deprecated_app: failed to get system architecture: %w", err)
	}
	arch := string(archOutput)

	// Check if the app directory exists
	appDir := filepath.Join(directory, "apps", app)
	appDirExists := true
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		appDirExists = false
	}

	// Construct the appropriate text for the user prompt
	var text string
	shouldPrompt := false

	// Check if we need to prompt the user based on the removal architecture and app status
	if removalArch != "" && arch == removalArch && appDirExists && appStatus == "installed" {
		shouldPrompt = true
		if message != "" {
			text = fmt.Sprintf("Pi-Apps has deprecated %s for %s-bit OSs which you currently have installed.\n\n%s\n\nWould you like to uninstall it now or leave it installed? You will NOT be able to uninstall %s with Pi-Apps later.", app, removalArch, message, app)
		} else {
			text = fmt.Sprintf("Pi-Apps has deprecated %s for %s-bit OSs which you currently have installed.\nWould you like to uninstall it now or leave it installed? You will NOT be able to uninstall %s with Pi-Apps later.", app, removalArch, app)
		}
	} else if removalArch == "" && appDirExists && appStatus == "installed" {
		shouldPrompt = true
		if message != "" {
			text = fmt.Sprintf("Pi-Apps has deprecated %s which you currently have installed.\n\n%s\n\nWould you like to uninstall it now or leave it installed? You will NOT be able to uninstall %s with Pi-Apps later.", app, message, app)
		} else {
			text = fmt.Sprintf("Pi-Apps has deprecated %s which you currently have installed.\nWould you like to uninstall it now or leave it installed? You will NOT be able to uninstall %s with Pi-Apps later.", app, app)
		}
	}

	// If we should prompt, show the dialog and process response
	if shouldPrompt {
		output, err := UserInputFunc(text, "Uninstall now", "Leave installed")
		if err != nil {
			return fmt.Errorf("remove_deprecated_app: failed to get user input: %w", err)
		}

		// If user chose to uninstall, run the uninstall command
		if output == "Uninstall now" {
			uninstallCmd := exec.Command(filepath.Join(directory, "manage"), "uninstall", app)
			uninstallCmd.Stdout = os.Stdout
			uninstallCmd.Stderr = os.Stderr
			if err := uninstallCmd.Run(); err != nil {
				return fmt.Errorf("remove_deprecated_app: failed to uninstall app: %w", err)
			}
		}
	}

	// Clean up files based on removal architecture
	if removalArch != "" {
		// Remove per-architecture script regardless of the current arch
		installScript := filepath.Join(directory, "apps", app, "install-"+removalArch)
		if _, err := os.Stat(installScript); err == nil {
			os.Remove(installScript)
		}

		// Remove unified-architecture script in case the new version has a per-architecture script
		unifiedScript := filepath.Join(directory, "apps", app, "install")
		if _, err := os.Stat(unifiedScript); err == nil {
			os.Remove(unifiedScript)
		}
	} else {
		// Only remove folder if the desired removal arch is unset (so remove on all architectures)
		if appDirExists {
			os.RemoveAll(appDir)
		}
	}

	return nil
}

// TerminalManage is a wrapper for executing app management actions
// This is a Go implementation of the original bash terminal_manage function
func TerminalManage(action, app string) error {
	if action == "" {
		return fmt.Errorf("terminal_manage(): Must specify an action: either 'install' or 'uninstall' or 'update' or 'refresh'")
	}

	// Forward to the multi-version with a single action
	return TerminalManageMulti(fmt.Sprintf("%s %s", action, app))
}

// TerminalManageMulti executes multiple app management actions in the Pi-Apps environment
// This is a Go implementation of the original bash terminal_manage_multi function
func TerminalManageMulti(queue string) error {
	// Get the Pi-Apps directory
	directory := os.Getenv("DIRECTORY")
	if directory == "" {
		// Default to the parent of the parent directory
		currentDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("terminal_manage_multi: failed to get current directory: %w", err)
		}
		directory = filepath.Dir(filepath.Dir(currentDir))
	}

	// Check if a daemon is already running by checking the pid file
	daemonPidFile := filepath.Join(directory, "data", "manage-daemon", "pid")

	if _, err := os.Stat(daemonPidFile); err == nil {
		// Read the PID from the file
		pidBytes, err := os.ReadFile(daemonPidFile)
		if err != nil {
			return fmt.Errorf("terminal_manage_multi: failed to read daemon pid file: %w", err)
		}

		pid := string(pidBytes)

		// Check if the process exists
		processCmd := exec.Command("ps", "-p", pid)
		if err := processCmd.Run(); err == nil {
			// Process exists, send the queue to the daemon and exit
			daemonCmd := exec.Command(filepath.Join(directory, "manage"), "daemon", queue)
			daemonCmd.Stdout = os.Stdout
			daemonCmd.Stderr = os.Stderr

			if err := daemonCmd.Run(); err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to send queue to daemon: %w", err)
			}

			return nil
		}
	}

	// If we reached here, there's no active daemon or the PID file doesn't exist
	// We'll run the daemon with our queue
	daemonCmd := exec.Command(filepath.Join(directory, "manage"), "daemon", queue)
	daemonCmd.Stdout = os.Stdout
	daemonCmd.Stderr = os.Stderr

	if err := daemonCmd.Run(); err != nil {
		return fmt.Errorf("terminal_manage_multi: failed to run daemon: %w", err)
	}

	// Refresh the app list if there's a pipe
	pipeEnv := os.Getenv("pipe")
	if pipeEnv != "" {
		// Check if the pipe exists
		if _, err := os.Stat(pipeEnv); err == nil {
			// Write form feed character to the pipe
			pipeFile, err := os.OpenFile(pipeEnv, os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to open pipe: %w", err)
			}
			defer pipeFile.Close()

			// Write form feed character to clear the screen
			if _, err := pipeFile.WriteString("\f"); err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to write to pipe: %w", err)
			}

			// Get the app list style
			prefix := os.Getenv("prefix")
			styleFile := filepath.Join(directory, "data", "settings", "App List Style")
			styleBytes, err := os.ReadFile(styleFile)
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to read app list style: %w", err)
			}

			style := string(styleBytes)

			// Reload the app list via the preload script
			preloadCmd := exec.Command(filepath.Join(directory, "preload"), style, prefix)
			preloadOutput, err := preloadCmd.Output()
			if err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to run preload: %w", err)
			}

			// Write the preload output to the pipe
			if _, err := pipeFile.Write(preloadOutput); err != nil {
				return fmt.Errorf("terminal_manage_multi: failed to write preload output to pipe: %w", err)
			}
		}
	}

	return nil
}
