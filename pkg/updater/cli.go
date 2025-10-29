package updater

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// UpdaterCLI handles the command-line interface for the updater
type UpdaterCLI struct {
	updater *Updater
	reader  *bufio.Reader
}

// NewUpdaterCLI creates a new CLI updater
func NewUpdaterCLI(updater *Updater) *UpdaterCLI {
	return &UpdaterCLI{
		updater: updater,
		reader:  bufio.NewReader(os.Stdin),
	}
}

// RunCLI runs the CLI interface based on the mode
func (c *UpdaterCLI) RunCLI() error {
	switch c.updater.mode {
	case ModeCLI:
		return c.runInteractiveCLI()
	case ModeCLIYes:
		return c.runAutomaticCLI()
	default:
		return fmt.Errorf("unsupported CLI mode: %v", c.updater.mode)
	}
}

// runInteractiveCLI runs the interactive CLI mode with user prompts
func (c *UpdaterCLI) runInteractiveCLI() error {
	ctx := context.Background()

	// Check repository
	if err := c.updater.CheckRepo(ctx); err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	// Get updates
	files, err := c.updater.GetUpdatableFiles()
	if err != nil {
		return fmt.Errorf("failed to get updatable files: %w", err)
	}

	apps, err := c.updater.GetUpdatableApps()
	if err != nil {
		return fmt.Errorf("failed to get updatable apps: %w", err)
	}

	if len(files) == 0 && len(apps) == 0 {
		fmt.Println("\n✓ Everything is up to date.")
		return nil
	}

	// Display available updates
	selectedFiles, selectedApps := c.selectUpdates(files, apps)

	if len(selectedFiles) == 0 && len(selectedApps) == 0 {
		fmt.Println("\nNo updates selected.")
		return nil
	}

	// Show countdown and perform update
	c.showCountdown()
	return c.performUpdate(selectedFiles, selectedApps)
}

// runAutomaticCLI runs the automatic CLI mode without user interaction
func (c *UpdaterCLI) runAutomaticCLI() error {
	ctx := context.Background()

	// Check repository
	if err := c.updater.CheckRepo(ctx); err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	// Get updates
	files, err := c.updater.GetUpdatableFiles()
	if err != nil {
		return fmt.Errorf("failed to get updatable files: %w", err)
	}

	apps, err := c.updater.GetUpdatableApps()
	if err != nil {
		return fmt.Errorf("failed to get updatable apps: %w", err)
	}

	if len(files) == 0 && len(apps) == 0 {
		fmt.Println("\n✓ Nothing to update.")
		return nil
	}

	// Display what will be updated
	c.displayUpdateSummary(files, apps)

	// Perform update
	return c.performUpdate(files, apps)
}

// selectUpdates allows user to select which updates to apply
func (c *UpdaterCLI) selectUpdates(files []FileChange, apps []string) ([]FileChange, []string) {
	fmt.Println("\n📦 Available Updates")
	fmt.Println("=" + strings.Repeat("=", 50))

	var allItems []interface{}
	var selectedItems []bool

	// Add files to selection
	if len(files) > 0 {
		fmt.Println("\n📄 File Updates:")
		for i, file := range files {
			note := ""
			if file.IsModuleFile {
				note = " (module update and recompilation required)"
			} else if file.RequiresRecompile {
				note = " (requires recompile)"
			}
			fmt.Printf("  [%d] %s%s\n", i+1, file.Path, note)
			allItems = append(allItems, file)
			selectedItems = append(selectedItems, true) // Selected by default
		}
	}

	// Add apps to selection
	if len(apps) > 0 {
		fmt.Println("\n📱 App Updates:")
		offset := len(files)
		for i, app := range apps {
			reinstallNote := ""
			willReinstall, err := api.WillReinstall(app)
			if err != nil {
				fmt.Printf("Warning: Failed to check if %s will be reinstalled: %v\n", app, err)
			} else if willReinstall {
				reinstallNote = " (will reinstall)"
			}
			fmt.Printf("  [%d] %s%s\n", offset+i+1, app, reinstallNote)
			allItems = append(allItems, app)
			selectedItems = append(selectedItems, true) // Selected by default
		}
	}

	// Interactive selection
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("Commands:")
	fmt.Println("  <number>     - Toggle selection")
	fmt.Println("  all          - Select all")
	fmt.Println("  none         - Select none")
	fmt.Println("  list         - Show current selection")
	fmt.Println("  continue     - Proceed with selected items")
	fmt.Println("  quit         - Exit without updating")

	for {
		fmt.Print("\n> ")
		input, _ := c.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch strings.ToLower(input) {
		case "continue", "c", "":
			// Proceed with current selection
			return c.extractSelection(allItems, selectedItems)

		case "quit", "q", "exit":
			return nil, nil

		case "all", "a":
			for i := range selectedItems {
				selectedItems[i] = true
			}
			fmt.Println("✓ All items selected")

		case "none", "n":
			for i := range selectedItems {
				selectedItems[i] = false
			}
			fmt.Println("✓ All items deselected")

		case "list", "l":
			c.showCurrentSelection(allItems, selectedItems)

		default:
			// Try to parse as number
			if num, err := strconv.Atoi(input); err == nil {
				if num >= 1 && num <= len(allItems) {
					idx := num - 1
					selectedItems[idx] = !selectedItems[idx]
					status := "deselected"
					if selectedItems[idx] {
						status = "selected"
					}
					fmt.Printf("✓ Item %d %s\n", num, status)
				} else {
					fmt.Printf("❌ Invalid number. Please enter 1-%d\n", len(allItems))
				}
			} else {
				fmt.Println("❌ Unknown command. Type 'continue' to proceed or 'quit' to exit.")
			}
		}
	}
}

// extractSelection extracts selected files and apps from the selection
func (c *UpdaterCLI) extractSelection(allItems []interface{}, selectedItems []bool) ([]FileChange, []string) {
	var selectedFiles []FileChange
	var selectedApps []string

	for i, selected := range selectedItems {
		if !selected {
			continue
		}

		switch item := allItems[i].(type) {
		case FileChange:
			selectedFiles = append(selectedFiles, item)
		case string:
			selectedApps = append(selectedApps, item)
		}
	}

	return selectedFiles, selectedApps
}

// showCurrentSelection displays the current selection status
func (c *UpdaterCLI) showCurrentSelection(allItems []interface{}, selectedItems []bool) {
	fmt.Println("\n📋 Current Selection:")
	selectedCount := 0

	for i, item := range allItems {
		marker := "❌"
		if selectedItems[i] {
			marker = "✅"
			selectedCount++
		}

		switch v := item.(type) {
		case FileChange:
			note := ""
			if v.IsModuleFile {
				note = " (module update and recompilation required)"
			} else if v.RequiresRecompile {
				note = " (requires recompile)"
			}
			fmt.Printf("  %s [%d] %s%s\n", marker, i+1, v.Path, note)
		case string:
			reinstallNote := ""
			willReinstall, err := api.WillReinstall(v)
			if err != nil {
				fmt.Printf("Warning: Failed to check if %s will be reinstalled: %v\n", v, err)
			} else if willReinstall {
				reinstallNote = " (will reinstall)"
			}
			fmt.Printf("  %s [%d] %s%s\n", marker, i+1, v, reinstallNote)
		}
	}

	fmt.Printf("\nSelected: %d/%d items\n", selectedCount, len(allItems))
}

// displayUpdateSummary shows what will be updated in automatic mode
func (c *UpdaterCLI) displayUpdateSummary(files []FileChange, apps []string) {
	fmt.Println("\n📦 Update Summary")
	fmt.Println("=" + strings.Repeat("=", 50))

	if len(files) > 0 {
		fmt.Println("\n📄 Files to update:")
		for _, file := range files {
			note := ""
			if file.IsModuleFile {
				note = " (module update and recompilation required)"
			} else if file.RequiresRecompile {
				note = " (requires recompile)"
			}
			fmt.Printf("  • %s%s\n", file.Path, note)
		}
	}

	if len(apps) > 0 {
		fmt.Println("\n📱 Apps to update:")
		for _, app := range apps {
			reinstallNote := ""
			willReinstall, err := api.WillReinstall(app)
			if err != nil {
				fmt.Printf("Warning: Failed to check if %s will be reinstalled: %v\n", app, err)
			} else if willReinstall {
				reinstallNote = " (will reinstall)"
			}
			fmt.Printf("  • %s%s\n", app, reinstallNote)
		}
	}

	// Check for recompilation or module updates
	needsRecompile := false
	needsModule := false
	for _, file := range files {
		if file.IsModuleFile {
			needsModule = true
		}
		if file.RequiresRecompile {
			needsRecompile = true
		}
	}

	if needsModule && needsRecompile {
		fmt.Println("\n⚠️  Some updates require module dependency updates and recompilation. This may take several minutes.")
	} else if needsModule {
		fmt.Println("\n⚠️  Some updates require module dependency updates. This may take several minutes.")
	} else if needsRecompile {
		fmt.Println("\n⚠️  Some updates require recompilation. This may take several minutes.")
	}

	fmt.Println()
}

// showCountdown displays a countdown before starting the update
func (c *UpdaterCLI) showCountdown() {
	fmt.Print("\nStarting update in: ")
	for i := 6; i >= 1; i-- {
		fmt.Printf("%d... ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("")
}

// performUpdate executes the update process
func (c *UpdaterCLI) performUpdate(files []FileChange, apps []string) error {
	fmt.Println("🚀 Starting update process...")

	result := c.updater.PerformUpdate(files, apps)

	if result.Success {
		fmt.Printf("\n✅ %s", result.Message)
		if result.Recompiled {
			fmt.Print(" (Recompilation completed)")
		}
		fmt.Println()

		// Update status files
		if err := c.updateStatusFiles(); err != nil {
			fmt.Printf("⚠️  Warning: Failed to update status files: %v\n", err)
		}

		return nil
	}

	// Handle failure
	fmt.Printf("\n❌ Update failed: %s\n", result.Message)

	if result.RollbackData != nil {
		fmt.Print("\nWould you like to rollback the changes? (y/N): ")
		response, _ := c.reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			fmt.Println("\n🔄 Rolling back changes...")
			if err := c.updater.rollback(result.RollbackData); err != nil {
				fmt.Printf("❌ Rollback failed: %v\n", err)
			} else {
				fmt.Println("✅ Rollback completed successfully")
			}
		}
	}

	// Offer retry option
	fmt.Print("\nWould you like to retry the update? (y/N): ")
	response, _ := c.reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		fmt.Println("\n🔄 Retrying update...")
		return c.performUpdate(files, apps)
	}

	return fmt.Errorf("update failed: %s", result.Message)
}

// updateStatusFiles updates the status tracking files
func (c *UpdaterCLI) updateStatusFiles() error {
	statusDir := c.updater.directory + "/data/update-status"
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		return err
	}

	// Get current updatable items
	files, err := c.updater.GetUpdatableFiles()
	if err != nil {
		return err
	}

	apps, err := c.updater.GetUpdatableApps()
	if err != nil {
		return err
	}

	// Write updatable files
	filesData := ""
	for _, file := range files {
		filesData += file.Path + "\n"
	}
	if err := os.WriteFile(statusDir+"/updatable-files", []byte(filesData), 0644); err != nil {
		return err
	}

	// Write updatable apps
	appsData := ""
	for _, app := range apps {
		appsData += app + "\n"
	}
	if err := os.WriteFile(statusDir+"/updatable-apps", []byte(appsData), 0644); err != nil {
		return err
	}

	return nil
}

// Status functions for different modes

// GetUpdateStatus checks if there are any updates available
func (c *UpdaterCLI) GetUpdateStatus() error {
	statusDir := c.updater.directory + "/data/update-status"

	filesStatus := statusDir + "/updatable-files"
	appsStatus := statusDir + "/updatable-apps"

	// Check if status files exist and have content
	hasFileUpdates := c.hasContent(filesStatus)
	hasAppUpdates := c.hasContent(appsStatus)

	if hasFileUpdates || hasAppUpdates {
		return nil // Updates available
	}

	return fmt.Errorf("no updates available")
}

// SetUpdateStatus checks for updates and writes status files
func (c *UpdaterCLI) SetUpdateStatus() error {
	ctx := context.Background()

	// Check repository
	if err := c.updater.CheckRepo(ctx); err != nil {
		return err
	}

	// Run runonce entries (if they exist)
	c.runOnceEntries()

	// Get updates
	files, err := c.updater.GetUpdatableFiles()
	if err != nil {
		return err
	}

	apps, err := c.updater.GetUpdatableApps()
	if err != nil {
		return err
	}

	// Write status files
	statusDir := c.updater.directory + "/data/update-status"
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		return err
	}

	// Write updatable files
	filesData := ""
	for _, file := range files {
		filesData += file.Path + "\n"
	}
	if err := os.WriteFile(statusDir+"/updatable-files", []byte(filesData), 0644); err != nil {
		return err
	}

	// Write updatable apps
	appsData := ""
	for _, app := range apps {
		appsData += app + "\n"
	}
	if err := os.WriteFile(statusDir+"/updatable-apps", []byte(appsData), 0644); err != nil {
		return err
	}

	// Return status like the bash version
	return c.GetUpdateStatus()
}

// Helper functions

func (c *UpdaterCLI) hasContent(filePath string) bool {
	if data, err := os.ReadFile(filePath); err == nil {
		return len(strings.TrimSpace(string(data))) > 0
	}
	return false
}

func (c *UpdaterCLI) runOnceEntries() {
	runoncePath := filepath.Join(c.updater.directory, "etc", "runonce-entries")
	if fileExists(runoncePath) {
		fmt.Println("Running runonce entries...")
		cmd := exec.Command(runoncePath)
		cmd.Dir = c.updater.directory
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: runonce-entries failed: %v\n", err)
		}
	}
}
