package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/botspot/pi-apps/pkg/updater"
)

var (
	// Build information (set via ldflags)
	BuildDate string
	GitCommit string
	Version   = "1.0.0"
)

func main() {
	// Handle command line arguments
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	mode := updater.UpdateMode(os.Args[1])
	speed := updater.SpeedNormal

	// Parse speed parameter if provided
	if len(os.Args) > 2 && os.Args[2] == "fast" {
		speed = updater.SpeedFast
	}

	// Get pi-apps directory
	directory, err := getPiAppsDirectory()
	if err != nil {
		log.Fatalf("Failed to determine pi-apps directory: %v", err)
	}

	// Check if running as root
	if os.Getuid() == 0 {
		log.Fatal("Pi-Apps is not designed to be run as root! Please try again as a regular user.")
	}

	// Create updater instance
	u, err := updater.New(directory, mode, speed)
	if err != nil {
		log.Fatalf("Failed to create updater: %v", err)
	}

	// Handle different modes
	switch mode {
	case updater.ModeAutostarted:
		err = handleAutostartedMode(u)
	case updater.ModeGetStatus:
		err = handleGetStatusMode(u)
	case updater.ModeSetStatus:
		err = handleSetStatusMode(u)
	case updater.ModeGUI, updater.ModeGUIYes:
		err = handleGUIMode(u, mode)
	case updater.ModeCLI, updater.ModeCLIYes:
		err = handleCLIMode(u)
	default:
		log.Fatalf("Unknown run mode: %s", mode)
	}

	if err != nil {
		log.Fatalf("Updater failed: %v", err)
	}
}

// handleAutostartedMode handles the autostarted mode (background checking)
func handleAutostartedMode(u *updater.Updater) error {
	fmt.Printf("Updater mode: %s\n", u.Mode())

	// Check if update interval allows update checking
	if err := u.CheckUpdateInterval(); err != nil {
		fmt.Printf("Won't check for updates: %v\n", err)
		return nil
	}

	// Check that at least one app has been installed
	if !hasInstalledApps(u.Directory()) {
		fmt.Println("No apps have been installed yet, so exiting now.")
		return nil
	}

	// Wait for internet connection
	if err := waitForInternet(); err != nil {
		fmt.Printf("No internet connection available: %v\n", err)
		return nil
	}

	ctx := context.Background()

	// Check repository
	if err := u.CheckRepo(ctx); err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	// Get updatable items
	files, err := u.GetUpdatableFiles()
	if err != nil {
		return fmt.Errorf("failed to get updatable files: %w", err)
	}

	apps, err := u.GetUpdatableApps()
	if err != nil {
		return fmt.Errorf("failed to get updatable apps: %w", err)
	}

	// Auto-refresh safe updates in background
	if len(files) > 0 || len(apps) > 0 {
		// Perform background updates for safe items
		result := performBackgroundUpdates(u, files, apps)
		if result != nil && !result.Success {
			fmt.Printf("Background update failed: %s\n", result.Message)
		}

		// Re-check what's still updatable
		files, _ = u.GetUpdatableFiles()
		apps, _ = u.GetUpdatableApps()
	}

	// Save status
	if err := saveUpdateStatus(u.Directory(), files, apps); err != nil {
		return fmt.Errorf("failed to save update status: %w", err)
	}

	// If no updates or no installed apps need updates, exit
	if len(files) == 0 && len(apps) == 0 {
		fmt.Println("Nothing is updatable.")
		return nil
	}

	// Check if any installed apps are updatable
	installedApps := getInstalledApps(u.Directory())
	hasInstalledUpdates := false
	for _, app := range apps {
		for _, installed := range installedApps {
			if app == installed {
				hasInstalledUpdates = true
				break
			}
		}
		if hasInstalledUpdates {
			break
		}
	}

	if len(files) == 0 && !hasInstalledUpdates {
		fmt.Println("No installed apps are updatable.")
		return nil
	}

	// Show notification (this would integrate with system notifications)
	return showUpdateNotification(files, apps)
}

// handleGetStatusMode checks if updates are available
func handleGetStatusMode(u *updater.Updater) error {
	cli := updater.NewUpdaterCLI(u)
	return cli.GetUpdateStatus()
}

// handleSetStatusMode checks for updates and saves status
func handleSetStatusMode(u *updater.Updater) error {
	cli := updater.NewUpdaterCLI(u)
	return cli.SetUpdateStatus()
}

// handleGUIMode runs the GUI updater
func handleGUIMode(u *updater.Updater, mode updater.UpdateMode) error {
	gui, err := updater.NewUpdaterGUI(u)
	if err != nil {
		return fmt.Errorf("failed to create GUI: %w", err)
	}

	if mode == updater.ModeGUIYes {
		// Auto-confirm mode - not implemented in GUI yet
		// For now, treat as regular GUI mode
	}

	gui.Run()
	return nil
}

// handleCLIMode runs the CLI updater
func handleCLIMode(u *updater.Updater) error {
	cli := updater.NewUpdaterCLI(u)
	return cli.RunCLI()
}

// Helper functions

func showUsage() {
	fmt.Printf("Pi-Apps Updater v%s\n", Version)
	if BuildDate != "" {
		fmt.Printf("Built: %s\n", BuildDate)
	}
	if GitCommit != "" {
		fmt.Printf("Commit: %s\n", GitCommit)
	}
	fmt.Println()
	fmt.Println("Usage: updater <mode> [speed] [options]")
	fmt.Println()
	fmt.Println("Modes:")
	fmt.Println("  autostarted  - Check for updates on boot (with notification)")
	fmt.Println("  get-status   - Check if updates are available (exit code)")
	fmt.Println("  set-status   - Check for updates and save status")
	fmt.Println("  gui          - Show GUI update dialog")
	fmt.Println("  gui-yes      - Show GUI and auto-confirm updates")
	fmt.Println("  cli          - Interactive command-line interface")
	fmt.Println("  cli-yes      - Automatic command-line update")
	fmt.Println()
	fmt.Println("Speed:")
	fmt.Println("  fast         - Use cached results (faster, may be outdated)")
	fmt.Println("  (default)    - Check repository for latest updates")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  updater gui")
	fmt.Println("  updater cli fast")
	fmt.Println("  updater get-status")
}

func getPiAppsDirectory() (string, error) {
	// Check DIRECTORY environment variable first
	if dir := os.Getenv("DIRECTORY"); dir != "" {
		return dir, nil
	}

	// Get directory from executable path
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	directory := filepath.Dir(executable)

	// Validate directory
	if err := validatePiAppsDirectory(directory); err != nil {
		// Try parent directory
		directory = filepath.Dir(directory)
		if err := validatePiAppsDirectory(directory); err != nil {
			return "", fmt.Errorf("invalid pi-apps directory: %w", err)
		}
	}

	return directory, nil
}

func validatePiAppsDirectory(directory string) error {
	// Check for required files/directories
	required := []string{"apps", "data", "etc"}
	for _, item := range required {
		path := filepath.Join(directory, item)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing %s directory", item)
		}
	}
	return nil
}

func hasInstalledApps(directory string) bool {
	statusDir := filepath.Join(directory, "data", "status")
	if entries, err := os.ReadDir(statusDir); err == nil {
		return len(entries) > 0
	}
	return false
}

func getInstalledApps(directory string) []string {
	var installed []string
	statusDir := filepath.Join(directory, "data", "status")

	if entries, err := os.ReadDir(statusDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				// Check if the app is actually installed (not uninstalled/corrupted)
				status, err := api.GetAppStatus(entry.Name())
				if err == nil && status == "installed" {
					installed = append(installed, entry.Name())
				}
			}
		}
	}

	return installed
}

func waitForInternet() error {
	maxAttempts := 18 // 3 minutes total
	for i := 0; i < maxAttempts; i++ {
		// Simple connectivity check - try to resolve github.com
		if err := checkConnectivity(); err == nil {
			return nil
		}

		fmt.Printf("No internet connection yet. Waiting 10 seconds... (attempt %d/%d)\n", i+1, maxAttempts)
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("no internet connection after %d attempts", maxAttempts)
}

func checkConnectivity() error {
	// Use a simple HTTP request to check connectivity
	// This is a simplified version - could be enhanced
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Execute a simple command to check connectivity
	cmd := exec.CommandContext(ctx, "wget", "--spider", "https://github.com")
	return cmd.Run()
}

func performBackgroundUpdates(u *updater.Updater, files []updater.FileChange, apps []string) *updater.UpdateResult {
	// Filter to only safe updates (no new apps, no reinstalls, no recompilation)
	var safeFiles []updater.FileChange
	var safeApps []string

	for _, file := range files {
		if !file.RequiresRecompile {
			safeFiles = append(safeFiles, file)
		}
	}

	for _, app := range apps {
		// Skip new apps and apps that require reinstallation
		appDir := filepath.Join(u.Directory(), "apps", app)
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			continue // Skip new apps
		}

		willReinstall, err := api.WillReinstall(app)
		if err != nil {
			fmt.Printf("Warning: Failed to check if %s will be reinstalled: %v\n", app, err)
			continue
		}
		if willReinstall {
			continue // Skip apps that need reinstallation
		}

		status, err := api.GetAppStatus(app)
		if err != nil {
			fmt.Printf("Warning: Failed to get status for %s: %v\n", app, err)
			continue
		}
		if status == "corrupted" {
			continue // Skip corrupted apps
		}

		safeApps = append(safeApps, app)
	}

	if len(safeFiles) == 0 && len(safeApps) == 0 {
		return nil
	}

	fmt.Printf("Performing background updates: %d files, %d apps\n", len(safeFiles), len(safeApps))
	return u.PerformUpdate(safeFiles, safeApps)
}

func saveUpdateStatus(directory string, files []updater.FileChange, apps []string) error {
	statusDir := filepath.Join(directory, "data", "update-status")
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		return err
	}

	// Save updatable files
	filesData := ""
	for _, file := range files {
		filesData += file.Path + "\n"
	}
	if err := os.WriteFile(filepath.Join(statusDir, "updatable-files"), []byte(filesData), 0644); err != nil {
		return err
	}

	// Save updatable apps
	appsData := ""
	for _, app := range apps {
		appsData += app + "\n"
	}
	if err := os.WriteFile(filepath.Join(statusDir, "updatable-apps"), []byte(appsData), 0644); err != nil {
		return err
	}

	return nil
}

func showUpdateNotification(files []updater.FileChange, apps []string) error {
	// This would show a system notification
	// For now, just print to console
	fmt.Printf("📱 Pi-Apps updates available: %d files, %d apps\n", len(files), len(apps))
	fmt.Println("Run 'updater gui' to see available updates.")
	return nil
}
