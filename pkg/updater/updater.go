package updater

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
)

// UpdateMode represents different modes of running the updater
type UpdateMode string

const (
	ModeAutostarted UpdateMode = "autostarted"
	ModeGetStatus   UpdateMode = "get-status"
	ModeSetStatus   UpdateMode = "set-status"
	ModeGUI         UpdateMode = "gui"
	ModeGUIYes      UpdateMode = "gui-yes"
	ModeCLI         UpdateMode = "cli"
	ModeCLIYes      UpdateMode = "cli-yes"
)

// UpdateSpeed represents update checking speed
type UpdateSpeed string

const (
	SpeedFast   UpdateSpeed = "fast"
	SpeedNormal UpdateSpeed = "normal"
)

// Updater handles pi-apps updates
type Updater struct {
	directory   string
	mode        UpdateMode
	speed       UpdateSpeed
	gitURL      string
	noStatus    bool
	noUpdate    bool
	useTerminal bool
}

// FileChange represents a file that needs updating
type FileChange struct {
	Path              string
	Type              string // "file", "app", "script"
	RequiresRecompile bool
	IsModuleFile      bool
}

// UpdateResult represents the result of an update operation
type UpdateResult struct {
	Success      bool
	Message      string
	FailedApps   []string
	FailedFiles  []string
	Recompiled   bool
	RollbackData *RollbackData
}

// RollbackData stores information needed for rollback
type RollbackData struct {
	BackupPath       string
	OriginalFiles    map[string]string // file path -> backup path
	CompilationState string            // "success", "failed", "not-attempted"
}

// CompilableFolder represents folders that require recompilation when changed
type CompilableFolder struct {
	Path    string
	Pattern string
}

var (
	// Folders that require recompilation when files change
	recompileFolders = []CompilableFolder{
		{Path: "pkg", Pattern: "**/*.go"},
		{Path: "cmd", Pattern: "**/*.go"},
		{Path: "go.mod", Pattern: "go.mod"},
		{Path: "go.sum", Pattern: "go.sum"},
	}

	// Module files that require go mod tidy
	moduleFiles = []string{"go.mod", "go.sum"}
)

// New creates a new Updater instance
func New(directory string, mode UpdateMode, speed UpdateSpeed) (*Updater, error) {
	if directory == "" {
		return nil, fmt.Errorf("directory cannot be empty")
	}

	// Set the PI_APPS_DIR environment variable for the API package
	if err := os.Setenv("PI_APPS_DIR", directory); err != nil {
		return nil, fmt.Errorf("failed to set PI_APPS_DIR environment variable: %w", err)
	}

	// Read git URL
	gitURL := "https://github.com/pi-apps-go/pi-apps"
	if gitURLFile := filepath.Join(directory, "etc", "git_url"); fileExists(gitURLFile) {
		if data, err := os.ReadFile(gitURLFile); err == nil {
			gitURL = strings.TrimSpace(string(data))
		}
	}

	return &Updater{
		directory: directory,
		mode:      mode,
		speed:     speed,
		gitURL:    gitURL,
	}, nil
}

// CheckUpdateInterval checks if updates should be checked based on the interval setting
func (u *Updater) CheckUpdateInterval() error {
	lastUpdateFile := filepath.Join(u.directory, "data", "last-update-check")

	// Read last update check date
	var lastUpdate int64
	if data, err := os.ReadFile(lastUpdateFile); err == nil {
		if _, err := fmt.Sscanf(string(data), "%d", &lastUpdate); err != nil {
			lastUpdate = 0
		}
	}

	// Write current date
	currentDate := time.Now().Unix() / 86400 // days since epoch
	if err := os.WriteFile(lastUpdateFile, []byte(fmt.Sprintf("%d", currentDate)), 0644); err != nil {
		return fmt.Errorf("failed to write last update check: %w", err)
	}

	// Read update interval setting
	intervalFile := filepath.Join(u.directory, "data", "settings", "Check for updates")
	interval := "Weekly" // default
	if data, err := os.ReadFile(intervalFile); err == nil {
		interval = strings.TrimSpace(string(data))
	}

	switch interval {
	case "Never":
		return fmt.Errorf("update checking is disabled")
	case "Daily":
		if currentDate == lastUpdate {
			return fmt.Errorf("already checked today")
		}
	case "Weekly":
		if currentDate <= lastUpdate+7 {
			return fmt.Errorf("checked within last week")
		}
	case "Always":
		return nil
	default:
		// Unknown interval, allow update
		return nil
	}

	return nil
}

// CheckRepo downloads/updates the repository in the update folder
func (u *Updater) CheckRepo(ctx context.Context) error {
	if u.speed == SpeedFast {
		return nil
	}

	fmt.Fprint(os.Stderr, "Checking for online changes... ")

	updateDir := filepath.Join(u.directory, "update")
	repoDir := filepath.Join(updateDir, "pi-apps")
	updaterScript := filepath.Join(repoDir, "updater")

	// If updater exists in update folder, try git pull first
	if fileExists(updaterScript) {
		cmd := exec.CommandContext(ctx, "git", "pull", "-q")
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			// If git pull fails, remove update directory for fresh clone
			os.RemoveAll(updateDir)
		} else {
			fmt.Fprintln(os.Stderr, "Done")
			return nil
		}
	}

	// If updater still doesn't exist, do git clone
	if !fileExists(updaterScript) {
		for {
			os.RemoveAll(updateDir)
			if err := os.MkdirAll(updateDir, 0755); err != nil {
				return fmt.Errorf("failed to create update directory: %w", err)
			}

			cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", u.gitURL)
			cmd.Dir = updateDir
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "\nFailed to download Pi-Apps repository! Retrying in 60 seconds.\n")
				time.Sleep(60 * time.Second)
				continue
			}
			break
		}
	}

	fmt.Fprintln(os.Stderr, "Done")
	return nil
}

// GetUpdatableFiles returns a list of files that need updating
func (u *Updater) GetUpdatableFiles() ([]FileChange, error) {
	statusFile := filepath.Join(u.directory, "data", "update-status", "updatable-files")

	if u.speed == SpeedFast && fileExists(statusFile) {
		// Use cached results for fast mode
		return u.loadCachedFiles(statusFile)
	}

	// Compare files between update and main directory
	files, err := u.listAllFiles()
	if err != nil {
		return nil, err
	}

	var updatable []FileChange
	for _, file := range files {
		localPath := filepath.Join(u.directory, file)
		updatePath := filepath.Join(u.directory, "update", "pi-apps", file)

		// Skip if file doesn't exist locally (new file)
		if !fileExists(localPath) {
			if fileExists(updatePath) {
				fc := FileChange{
					Path:              file,
					Type:              u.getFileType(file),
					RequiresRecompile: u.requiresRecompile(file),
					IsModuleFile:      u.IsModuleFile(file),
				}
				updatable = append(updatable, fc)
			}
			continue
		}

		// Skip if file doesn't exist in update
		if !fileExists(updatePath) {
			continue
		}

		// Compare file contents
		if match, err := u.filesMatch(localPath, updatePath); err != nil {
			return nil, err
		} else if !match {
			fc := FileChange{
				Path:              file,
				Type:              u.getFileType(file),
				RequiresRecompile: u.requiresRecompile(file),
				IsModuleFile:      u.IsModuleFile(file),
			}
			updatable = append(updatable, fc)
		}
	}

	// Filter out excluded files
	updatable = u.filterExcludedFiles(updatable)

	return updatable, nil
}

// GetUpdatableApps returns a list of apps that need updating
func (u *Updater) GetUpdatableApps() ([]string, error) {
	statusFile := filepath.Join(u.directory, "data", "update-status", "updatable-apps")

	if u.speed == SpeedFast && fileExists(statusFile) {
		return u.loadCachedApps(statusFile)
	}

	// Get list of all apps from online repository
	onlineApps, err := api.ListApps("online")
	if err != nil {
		return nil, err
	}

	var updatable []string
	for _, app := range onlineApps {
		localPath := filepath.Join(u.directory, "apps", app)
		updatePath := filepath.Join(u.directory, "update", "pi-apps", "apps", app)

		// If app doesn't exist locally, it's new
		if !dirExists(localPath) {
			updatable = append(updatable, app)
			continue
		}

		// Compare app directories
		if match, err := u.directoriesMatch(localPath, updatePath); err != nil {
			return nil, err
		} else if !match {
			updatable = append(updatable, app)
		}
	}

	return updatable, nil
}

// UpdateFiles updates the specified files
func (u *Updater) UpdateFiles(files []FileChange) error {
	for _, file := range files {
		if err := u.updateFile(file.Path); err != nil {
			return fmt.Errorf("failed to update file %s: %w", file.Path, err)
		}
	}
	return nil
}

// UpdateApps updates the specified apps
func (u *Updater) UpdateApps(apps []string) error {
	for _, app := range apps {
		willReinstall, err := api.WillReinstall(app)
		if err != nil {
			return fmt.Errorf("failed to check if app %s will be reinstalled: %w", app, err)
		}

		if willReinstall {
			if err := u.updateApp(app); err != nil {
				return fmt.Errorf("failed to update app %s: %w", app, err)
			}
		} else {
			if err := u.refreshApp(app); err != nil {
				return fmt.Errorf("failed to refresh app %s: %w", app, err)
			}
		}
	}
	return nil
}

// PerformUpdate handles the complete update process with compilation
func (u *Updater) PerformUpdate(files []FileChange, apps []string) *UpdateResult {
	result := &UpdateResult{
		Success: true,
		RollbackData: &RollbackData{
			OriginalFiles: make(map[string]string),
		},
	}

	// Create backup
	backupDir, err := u.createBackup(files, apps)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to create backup: %v", err)
		return result
	}
	result.RollbackData.BackupPath = backupDir

	// Check if recompilation or module tidy will be needed
	needsRecompile := u.needsRecompilation(files)
	needsModTidy := u.needsModuleTidy(files)

	// Update files first
	if err := u.UpdateFiles(files); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to update files: %v", err)
		u.rollback(result.RollbackData)
		return result
	}

	// Update apps
	if err := u.UpdateApps(apps); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to update apps: %v", err)
		u.rollback(result.RollbackData)
		return result
	}

	// Run go mod tidy if module files were updated
	if needsModTidy {
		if err := u.runModuleTidy(); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Module update failed: %v", err)
			result.RollbackData.CompilationState = "failed"
			u.rollback(result.RollbackData)
			return result
		}

		// After go mod tidy, update the git repository state to reflect the tidied module files
		// This prevents the module files from appearing as "still needing updates"
		if err := u.updateGitAfterModTidy(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to update git after module tidy: %v\n", err)
		}
	}

	// Perform recompilation if needed
	if needsRecompile {
		if err := u.recompile(); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Compilation failed: %v", err)
			result.RollbackData.CompilationState = "failed"
			u.rollback(result.RollbackData)
			return result
		}
		result.Recompiled = true
		result.RollbackData.CompilationState = "success"
	}

	// Update git repository (final update)
	if !needsModTidy { // Only update git if we didn't already do it after mod tidy
		if err := u.updateGit(); err != nil {
			// Git update failure is not critical
			fmt.Fprintf(os.Stderr, "Warning: Failed to update git: %v\n", err)
		}
	}

	// Build appropriate success message
	message := "Update completed successfully"
	if needsModTidy && needsRecompile {
		message += " (Module dependencies updated and recompilation completed)"
	} else if needsModTidy {
		message += " (Module dependencies updated)"
	} else if needsRecompile {
		message += " (Recompilation completed)"
	}

	result.Message = message
	return result
}

// Helper functions

func (u *Updater) getFileType(path string) string {
	switch {
	case path == "go.mod" || path == "go.sum":
		return "module"
	case strings.HasSuffix(path, ".go") && (strings.HasPrefix(path, "pkg/") || strings.HasPrefix(path, "cmd/")):
		return "script"
	case strings.ToLower(filepath.Base(path)) == "makefile" || strings.HasSuffix(strings.ToLower(path), ".mk"):
		return "makefile"
	case strings.Contains(path, "/apps/"):
		return "app"
	case strings.HasSuffix(strings.ToLower(path), ".png") || strings.HasSuffix(strings.ToLower(path), ".jpg") ||
		strings.HasSuffix(strings.ToLower(path), ".jpeg") || strings.HasSuffix(strings.ToLower(path), ".gif") ||
		strings.HasSuffix(strings.ToLower(path), ".svg") || strings.HasSuffix(strings.ToLower(path), ".ico"):
		return "image"
	case u.isBinaryFile(path):
		return "binary"
	default:
		return "file"
	}
}

func (u *Updater) isBinaryFile(path string) bool {
	// Check for common binary file extensions
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{".bin", ".exe", ".so", ".dylib", ".dll", ".a", ".o"}
	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}

	// Check if it's in a bin directory
	if strings.Contains(path, "/bin/") || strings.HasPrefix(path, "bin/") {
		return true
	}

	return false
}

func (u *Updater) requiresRecompile(path string) bool {
	for _, folder := range recompileFolders {
		if strings.HasPrefix(path, folder.Path) {
			return true
		}
		if path == folder.Path {
			return true
		}
	}
	return false
}

func (u *Updater) needsRecompilation(files []FileChange) bool {
	for _, file := range files {
		if file.RequiresRecompile {
			return true
		}
	}
	return false
}

func (u *Updater) recompile() error {
	fmt.Println("Recompiling Pi-Apps...")
	cmd := exec.Command("make", "install")
	cmd.Dir = u.directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make install failed: %w", err)
	}

	fmt.Println("Recompilation completed successfully")
	return nil
}

func (u *Updater) createBackup(files []FileChange, apps []string) (string, error) {
	backupDir := filepath.Join(u.directory, "update-backup", fmt.Sprintf("%d", time.Now().Unix()))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	// Backup files
	for _, file := range files {
		src := filepath.Join(u.directory, file.Path)
		if fileExists(src) {
			dst := filepath.Join(backupDir, "files", file.Path)
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return "", err
			}
			if err := copyFile(src, dst); err != nil {
				return "", err
			}
		}
	}

	// Backup apps
	for _, app := range apps {
		src := filepath.Join(u.directory, "apps", app)
		if dirExists(src) {
			dst := filepath.Join(backupDir, "apps", app)
			if err := copyDir(src, dst); err != nil {
				return "", err
			}
		}
	}

	return backupDir, nil
}

func (u *Updater) rollback(data *RollbackData) error {
	if data.BackupPath == "" {
		return fmt.Errorf("no backup to rollback to")
	}

	fmt.Println("Rolling back changes...")

	// Restore files
	filesBackup := filepath.Join(data.BackupPath, "files")
	if dirExists(filesBackup) {
		if err := copyDir(filesBackup, u.directory); err != nil {
			return fmt.Errorf("failed to restore files: %w", err)
		}
	}

	// Restore apps
	appsBackup := filepath.Join(data.BackupPath, "apps")
	if dirExists(appsBackup) {
		appsDir := filepath.Join(u.directory, "apps")
		if err := copyDir(appsBackup, appsDir); err != nil {
			return fmt.Errorf("failed to restore apps: %w", err)
		}
	}

	// If compilation was successful before rollback, recompile again
	if data.CompilationState == "success" {
		if err := u.recompile(); err != nil {
			return fmt.Errorf("failed to recompile during rollback: %w", err)
		}
	}

	fmt.Println("Rollback completed")
	return nil
}

func (u *Updater) updateFile(filePath string) error {
	src := filepath.Join(u.directory, "update", "pi-apps", filePath)
	dst := filepath.Join(u.directory, filePath)

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return copyFile(src, dst)
}

func (u *Updater) updateApp(app string) error {
	// Uninstall app first
	status, err := api.GetAppStatus(app)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}

	if status != "uninstalled" {
		if err := api.ManageApp(api.ActionUninstall, app, false); err != nil {
			return fmt.Errorf("failed to uninstall app: %w", err)
		}
	}

	// Refresh app folder
	if err := u.refreshApp(app); err != nil {
		return err
	}

	// Reinstall app
	if err := api.ManageApp(api.ActionInstall, app, true); err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}

	return nil
}

func (u *Updater) refreshApp(app string) error {
	appDir := filepath.Join(u.directory, "apps", app)
	updateAppDir := filepath.Join(u.directory, "update", "pi-apps", "apps", app)

	// Remove existing app directory
	if err := os.RemoveAll(appDir); err != nil {
		return err
	}

	// Copy new version
	return copyDir(updateAppDir, appDir)
}

func (u *Updater) updateGit() error {
	// Remove old .git folder
	gitDir := filepath.Join(u.directory, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		return err
	}

	// Copy new .git folder
	updateGitDir := filepath.Join(u.directory, "update", "pi-apps", ".git")
	return copyDir(updateGitDir, gitDir)
}

func (u *Updater) updateGitAfterModTidy() error {
	// Copy the current state (including tidied module files) to the update directory
	// so that subsequent file comparisons don't show module files as still needing updates

	// Copy go.mod if it exists
	if fileExists(filepath.Join(u.directory, "go.mod")) {
		src := filepath.Join(u.directory, "go.mod")
		dst := filepath.Join(u.directory, "update", "pi-apps", "go.mod")
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to update go.mod in update directory: %w", err)
		}
	}

	// Copy go.sum if it exists
	if fileExists(filepath.Join(u.directory, "go.sum")) {
		src := filepath.Join(u.directory, "go.sum")
		dst := filepath.Join(u.directory, "update", "pi-apps", "go.sum")
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to update go.sum in update directory: %w", err)
		}
	}

	return nil
}

func (u *Updater) listAllFiles() ([]string, error) {
	var files []string

	// List files in update directory
	updateDir := filepath.Join(u.directory, "update", "pi-apps")
	err := filepath.Walk(updateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(updateDir, path)
			if err != nil {
				return err
			}
			// Skip certain directories
			if !strings.HasPrefix(relPath, ".git/") &&
				!strings.HasPrefix(relPath, "apps/") &&
				!strings.HasPrefix(relPath, "data/") {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// List files in local directory
	err = filepath.Walk(u.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(u.directory, path)
			if err != nil {
				return err
			}
			// Skip certain directories
			if !strings.HasPrefix(relPath, ".git/") &&
				!strings.HasPrefix(relPath, "apps/") &&
				!strings.HasPrefix(relPath, "update/") &&
				!strings.HasPrefix(relPath, "data/") &&
				!strings.HasPrefix(relPath, "logs/") {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remove duplicates and sort
	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[file] = true
	}

	files = make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)

	return files, nil
}

func (u *Updater) filesMatch(file1, file2 string) (bool, error) {
	// Simple implementation - could be enhanced with checksums
	data1, err := os.ReadFile(file1)
	if err != nil {
		return false, err
	}

	data2, err := os.ReadFile(file2)
	if err != nil {
		return false, err
	}

	return string(data1) == string(data2), nil
}

func (u *Updater) directoriesMatch(dir1, dir2 string) (bool, error) {
	// Use diff command for directory comparison
	cmd := exec.Command("diff", "-rq", dir1, dir2)
	err := cmd.Run()
	return err == nil, nil
}

func (u *Updater) filterExcludedFiles(files []FileChange) []FileChange {
	exclusionFile := filepath.Join(u.directory, "data", "update-exclusion")
	if !fileExists(exclusionFile) {
		return files
	}

	excluded := make(map[string]bool)
	if file, err := os.Open(exclusionFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ";") {
				excluded[line] = true
			}
		}
	}

	var filtered []FileChange
	for _, file := range files {
		if !excluded[file.Path] {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

func (u *Updater) loadCachedFiles(statusFile string) ([]FileChange, error) {
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, err
	}

	var files []FileChange
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, FileChange{
				Path:              line,
				Type:              u.getFileType(line),
				RequiresRecompile: u.requiresRecompile(line),
				IsModuleFile:      u.IsModuleFile(line),
			})
		}
	}

	return files, nil
}

func (u *Updater) loadCachedApps(statusFile string) ([]string, error) {
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, err
	}

	var apps []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			apps = append(apps, line)
		}
	}

	return apps, nil
}

// Mode returns the current update mode
func (u *Updater) Mode() UpdateMode {
	return u.mode
}

// Directory returns the pi-apps directory
func (u *Updater) Directory() string {
	return u.directory
}

// AppStatus returns the status of an app using the real API
func (u *Updater) AppStatus(app string) (string, error) {
	return api.GetAppStatus(app)
}

// WillReinstall checks if an app will be reinstalled using the real API
func (u *Updater) WillReinstall(app string) (bool, error) {
	return api.WillReinstall(app)
}

// ListApps returns a list of apps using the real API
func (u *Updater) ListApps(category string) ([]string, error) {
	return api.ListApps(category)
}

// Utility functions
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

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

		return copyFile(path, dstPath)
	})
}

func (u *Updater) IsModuleFile(path string) bool {
	for _, moduleFile := range moduleFiles {
		if path == moduleFile {
			return true
		}
	}
	return false
}

func (u *Updater) needsModuleTidy(files []FileChange) bool {
	for _, file := range files {
		if file.IsModuleFile {
			return true
		}
	}
	return false
}

func (u *Updater) runModuleTidy() error {
	fmt.Println("Running go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = u.directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	fmt.Println("go mod tidy completed successfully")
	return nil
}
