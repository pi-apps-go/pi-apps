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

// Package builder provides build-time plugin system for Pi-Apps Go
// This is a work in progress, things must be adapted to keep the same functionality
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Builder builds pi-apps with plugins at build time
type Builder struct {
	// PiAppsVersion is the version of pi-apps to build
	PiAppsVersion string `json:"pi_apps_version,omitempty"`

	// Plugins is the list of plugins to include
	Plugins []Plugin `json:"plugins,omitempty"`

	// Replacements is the list of module replacements
	Replacements []Replacement `json:"replacements,omitempty"`

	// BuildFlags are additional flags to pass to go build
	BuildFlags string `json:"build_flags,omitempty"`

	// ModFlags are additional flags to pass to go mod
	ModFlags string `json:"mod_flags,omitempty"`

	// RaceDetector enables race detection
	RaceDetector bool `json:"race_detector,omitempty"`

	// Debug enables debug symbols
	Debug bool `json:"debug,omitempty"`

	// SkipCleanup prevents cleanup of build artifacts
	SkipCleanup bool `json:"skip_cleanup,omitempty"`

	// TimeoutBuild is the timeout for build operations
	TimeoutBuild time.Duration `json:"timeout_build,omitempty"`

	// TimeoutGet is the timeout for go get operations
	TimeoutGet time.Duration `json:"timeout_get,omitempty"`
}

// Plugin represents a pi-apps plugin
type Plugin struct {
	// ModulePath is the Go module path
	ModulePath string `json:"module_path,omitempty"`

	// Version is the module version
	Version string `json:"version,omitempty"`

	// ReplacementPath is the local replacement path
	ReplacementPath string `json:"replacement_path,omitempty"`
}

// Replacement represents a module replacement
type Replacement struct {
	// Old is the module path to replace
	Old string `json:"old,omitempty"`

	// New is the replacement module path
	New string `json:"new,omitempty"`
}

// String returns a string representation of the plugin
func (p Plugin) String() string {
	if p.Version != "" {
		return fmt.Sprintf("%s@%s", p.ModulePath, p.Version)
	}
	return p.ModulePath
}

// DefaultBuilder returns a builder with default settings
func DefaultBuilder() *Builder {
	return &Builder{
		PiAppsVersion: "latest",
		TimeoutBuild:  time.Minute * 10,
		TimeoutGet:    time.Minute * 5,
	}
}

// Build builds pi-apps with the configured plugins
func (b *Builder) Build(ctx context.Context, outputFile string) error {
	// Create temporary directory for build
	tmpDir, err := os.MkdirTemp("", "xpi-apps-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	if !b.SkipCleanup {
		defer os.RemoveAll(tmpDir)
	}

	// Create main.go with plugin imports
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := b.createMainFile(mainFile); err != nil {
		return fmt.Errorf("failed to create main.go: %w", err)
	}

	// Initialize go module
	if err := b.initGoModule(ctx, tmpDir); err != nil {
		return fmt.Errorf("failed to initialize go module: %w", err)
	}

	// Add plugins and replacements
	if err := b.addPlugins(ctx, tmpDir); err != nil {
		return fmt.Errorf("failed to add plugins: %w", err)
	}

	if err := b.addReplacements(ctx, tmpDir); err != nil {
		return fmt.Errorf("failed to add replacements: %w", err)
	}

	// Build the binary
	if err := b.buildBinary(ctx, tmpDir, outputFile); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}

	return nil
}

// createMainFile creates the main.go file with plugin imports
func (b *Builder) createMainFile(mainFile string) error {
	var imports []string

	// Add plugin imports
	for _, plugin := range b.Plugins {
		imports = append(imports, fmt.Sprintf("\t_ \"%s\"", plugin.ModulePath))
	}

	mainContent := fmt.Sprintf(`package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pi-apps-go/pi-apps/pkg/api"
	"github.com/pi-apps-go/pi-apps/pkg/builder"
%s
)

func main() {
	// Parse command line flags
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	helpFlag := flag.Bool("help", false, "Show help message")
	versionFlag := flag.Bool("version", false, "Show version information")
	logoFlag := flag.Bool("logo", false, "Display the Pi-Apps logo")
	flag.Parse()

	// Set debug mode if specified
	if *debugFlag {
		api.SetDebugMode(true)
	}

	// Initialize plugins
	if err := builder.InitializePlugins(); err != nil {
		fmt.Printf("Error initializing plugins: %%v\n", err)
		os.Exit(1)
	}

	// Handle help flag
	if *helpFlag {
		fmt.Println("Pi-Apps (custom build with plugins)")
		fmt.Println("This is a custom build of Pi-Apps with plugins compiled in.")
		return
	}

	// Handle version flag
	if *versionFlag {
		fmt.Println("Pi-Apps Go Edition v0.1.0 (custom build with plugins)")
		return
	}

	// Handle logo flag
	if *logoFlag {
		fmt.Print(api.GenerateLogo())
		return
	}

	// If no arguments were provided, start the Pi-Apps GUI as usual
	if flag.NArg() == 0 {
		// obtain the Pi-Apps directory
		api.Init()
		piAppsDir := os.Getenv("PI_APPS_DIR")
		if piAppsDir == "" {
			fmt.Println("Error: PI_APPS_DIR environment variable is not set")
			os.Exit(1)
		}

		guiPath := piAppsDir + "/gui"

		// Check if gui exists
		if _, err := os.Stat(guiPath); os.IsNotExist(err) {
			fmt.Printf("Error: GUI executable not found at %%s\n", guiPath)
			os.Exit(1)
		}

		cmd := exec.Command(guiPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error launching GUI: %%v\n", err)
			os.Exit(1)
		}
		return
	}

	// Get the command and arguments
	command := flag.Arg(0)
	args := flag.Args()[1:]

	// Execute the requested command
	switch strings.ToLower(command) {
	case "install":
		// Handle installation
		if len(args) < 1 {
			fmt.Println("Error: No app specified for installation")
			fmt.Println("Usage: pi-apps install <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		if err := api.InstallApp(appName); err != nil {
			fmt.Printf("Error: %%v\n", err)
			os.Exit(1)
		}

	case "uninstall", "remove":
		// Handle uninstallation
		if len(args) < 1 {
			fmt.Println("Error: No app specified for removal")
			fmt.Println("Usage: pi-apps uninstall/remove <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		if err := api.UninstallApp(appName); err != nil {
			fmt.Printf("Error: %%v\n", err)
			os.Exit(1)
		}

	case "list":
		// Handle listing
		api.Init()
		apps, err := api.ListApps("")
		if err != nil {
			fmt.Printf("Error: %%v\n", err)
			os.Exit(1)
		}
		for _, app := range apps {
			fmt.Println(app)
		}

	case "status":
		// Handle status
		if len(args) < 1 {
			fmt.Println("Error: No app specified for status check")
			fmt.Println("Usage: pi-apps status <app-name>")
			os.Exit(1)
		}

		api.Init()
		appName := args[0]
		status, err := api.GetAppStatus(appName)
		if err != nil {
			fmt.Printf("Error: %%v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%%s: %%s\n", appName, status)

	default:
		fmt.Printf("Unknown command: %%s\n", command)
		fmt.Println("Available commands: install, uninstall, remove, list, status")
		os.Exit(1)
	}
}
`, strings.Join(imports, "\n"))

	return os.WriteFile(mainFile, []byte(mainContent), 0644)
}

// initGoModule initializes the go module
func (b *Builder) initGoModule(ctx context.Context, tmpDir string) error {
	// Initialize go module
	cmd := exec.CommandContext(ctx, "go", "mod", "init", "pi-apps-custom")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize go module: %w", err)
	}

	// Add pi-apps dependency using local replacement
	// Since we're in development, use the local pi-apps module
	cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-require", "github.com/pi-apps-go/pi-apps@latest")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add pi-apps dependency: %w", err)
	}

	// Use local replacement for pi-apps
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-replace", fmt.Sprintf("github.com/pi-apps-go/pi-apps=%s", workDir))
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add pi-apps replacement: %w", err)
	}

	// Run go mod tidy to resolve dependencies
	cmd = exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tidy dependencies: %w", err)
	}

	return nil
}

// addPlugins adds plugins to the module
func (b *Builder) addPlugins(ctx context.Context, tmpDir string) error {
	for _, plugin := range b.Plugins {
		var args []string

		if plugin.ReplacementPath != "" {
			// Add replacement
			args = []string{"mod", "edit", "-replace", fmt.Sprintf("%s=%s", plugin.ModulePath, plugin.ReplacementPath)}
		} else {
			// Add require
			moduleSpec := plugin.ModulePath
			if plugin.Version != "" {
				moduleSpec = fmt.Sprintf("%s@%s", plugin.ModulePath, plugin.Version)
			}
			args = []string{"mod", "edit", "-require", moduleSpec}
		}

		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add plugin %s: %w", plugin.ModulePath, err)
		}
	}

	return nil
}

// addReplacements adds module replacements
func (b *Builder) addReplacements(ctx context.Context, tmpDir string) error {
	for _, replacement := range b.Replacements {
		cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-replace", fmt.Sprintf("%s=%s", replacement.Old, replacement.New))
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add replacement %s=%s: %w", replacement.Old, replacement.New, err)
		}
	}

	return nil
}

// buildBinary builds the final binary
func (b *Builder) buildBinary(ctx context.Context, tmpDir, outputFile string) error {
	// Download dependencies
	cmd := exec.CommandContext(ctx, "go", "mod", "download")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download dependencies: %w", err)
	}

	// Convert output file to absolute path
	absOutputFile, err := filepath.Abs(outputFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output file: %w", err)
	}

	// Build arguments
	args := []string{"build", "-o", absOutputFile}

	// Add build flags
	if b.BuildFlags != "" {
		// Parse build flags (simple space splitting for now)
		buildFlags := strings.Fields(b.BuildFlags)
		args = append(args, buildFlags...)
	}

	// Add race detector
	if b.RaceDetector {
		args = append(args, "-race")
	}

	// Add debug symbols
	if !b.Debug {
		args = append(args, "-ldflags", "-s -w")
	}

	// Add current directory as target
	args = append(args, ".")

	cmd = exec.CommandContext(ctx, "go", args...)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}

	return nil
}
