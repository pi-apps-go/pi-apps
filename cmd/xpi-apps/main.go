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

// xpi-apps - Custom Pi-Apps Builder (similar to xcaddy)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/botspot/pi-apps/pkg/builder"
)

const (
	version = "0.1.0"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "build":
		if err := buildCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("xpi-apps version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		// If no build command is specified, try to run pi-apps with plugins
		// similar to xcaddy's plugin development mode
		if err := devCommand(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf(`xpi-apps - Custom Pi-Apps Builder

Usage:
    xpi-apps build [<pi-apps-version>]
        [--output <file>]
        [--with <module[@version][=replacement]>...]
        [--replace <module[@version]=replacement>...]
        [--race]
        [--debug]
        [--skip-cleanup]

    xpi-apps version
    xpi-apps help

    xpi-apps <args...>  (development mode - runs pi-apps with current module)

Commands:
    build           Build custom pi-apps binary with plugins
    version         Show version information
    help            Show this help message

Build Options:
    <pi-apps-version>    Pi-Apps version to build (default: latest)
    --output <file>      Output file path (default: ./pi-apps)
    --with <module>      Add plugin module (can be used multiple times)
    --replace <module>   Replace module with local version
    --race              Enable race detector
    --debug             Enable debug symbols
    --skip-cleanup      Don't clean up build artifacts

Examples:
    # Build pi-apps with custom plugins
    xpi-apps build --with github.com/example/pi-apps-plugin

    # Build specific version with local plugin
    xpi-apps build v1.0.0 --with github.com/example/plugin=../my-plugin

    # Build with multiple plugins
    xpi-apps build \
        --with github.com/example/plugin1 \
        --with github.com/example/plugin2@v1.2.3

    # Development mode (run pi-apps with current module as plugin)
    xpi-apps list
    xpi-apps install Chrome
`)
}

func buildCommand(args []string) error {
	var (
		output      = flag.String("output", "./pi-apps", "output file path")
		withModules = make([]string, 0)
		replaceModules = make([]string, 0)
		race        = flag.Bool("race", false, "enable race detector")
		debug       = flag.Bool("debug", false, "enable debug symbols")
		skipCleanup = flag.Bool("skip-cleanup", false, "don't clean up build artifacts")
	)

	// Custom flag parsing for --with and --replace
	var piAppsVersion string
	var i int
	for i = 0; i < len(args); i++ {
		arg := args[i]
		
		if arg == "--with" && i+1 < len(args) {
			withModules = append(withModules, args[i+1])
			i++ // skip next arg
		} else if arg == "--replace" && i+1 < len(args) {
			replaceModules = append(replaceModules, args[i+1])
			i++ // skip next arg
		} else if strings.HasPrefix(arg, "--with=") {
			withModules = append(withModules, arg[7:])
		} else if strings.HasPrefix(arg, "--replace=") {
			replaceModules = append(replaceModules, arg[10:])
		} else if arg == "--race" {
			*race = true
		} else if arg == "--debug" {
			*debug = true
		} else if arg == "--skip-cleanup" {
			*skipCleanup = true
		} else if strings.HasPrefix(arg, "--output=") {
			*output = arg[9:]
		} else if arg == "--output" && i+1 < len(args) {
			*output = args[i+1]
			i++ // skip next arg
		} else if !strings.HasPrefix(arg, "--") {
			// This is the pi-apps version
			piAppsVersion = arg
		}
	}

	if piAppsVersion == "" {
		piAppsVersion = "latest"
	}

	// Parse plugin specifications
	plugins := make([]builder.Plugin, 0, len(withModules))
	for _, module := range withModules {
		plugin, err := parsePluginSpec(module)
		if err != nil {
			return fmt.Errorf("invalid plugin specification %s: %w", module, err)
		}
		plugins = append(plugins, plugin)
	}

	// Parse replacements
	replacements := make([]builder.Replacement, 0, len(replaceModules))
	for _, replace := range replaceModules {
		replacement, err := parseReplaceSpec(replace)
		if err != nil {
			return fmt.Errorf("invalid replacement specification %s: %w", replace, err)
		}
		replacements = append(replacements, replacement)
	}

	// Create builder
	b := &builder.Builder{
		PiAppsVersion: piAppsVersion,
		Plugins:       plugins,
		Replacements:  replacements,
		RaceDetector:  *race,
		Debug:         *debug,
		SkipCleanup:   *skipCleanup,
		TimeoutBuild:  time.Minute * 10,
		TimeoutGet:    time.Minute * 5,
	}

	// Build
	ctx := context.Background()
	fmt.Printf("Building pi-apps %s with %d plugins...\n", piAppsVersion, len(plugins))
	
	if err := b.Build(ctx, *output); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("Build completed successfully: %s\n", *output)
	return nil
}

func devCommand(args []string) error {
	// Development mode - build and run pi-apps with current module as plugin
	// Similar to xcaddy's plugin development mode
	
	// Check if we're in a go module
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return fmt.Errorf("development mode requires a go.mod file in the current directory")
	}

	// Get current module path
	// This is a simplified approach - in reality we'd parse go.mod
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	// For development mode, we'll use the current directory as a local plugin
	plugin := builder.Plugin{
		ModulePath:      "github.com/example/current-plugin", // This would be parsed from go.mod
		ReplacementPath: currentDir,
	}

	// Create temporary binary
	tmpBinary := filepath.Join(os.TempDir(), "pi-apps-dev")
	defer os.Remove(tmpBinary)

	// Build with current module
	b := &builder.Builder{
		PiAppsVersion: "latest",
		Plugins:       []builder.Plugin{plugin},
		TimeoutBuild:  time.Minute * 10,
		TimeoutGet:    time.Minute * 5,
	}

	ctx := context.Background()
	fmt.Println("Building pi-apps with current module...")
	
	if err := b.Build(ctx, tmpBinary); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Run the binary with provided arguments
	fmt.Println("Running pi-apps...")
	// In reality, we'd exec the binary with the arguments
	// For now, just print what we would do
	fmt.Printf("Would run: %s %s\n", tmpBinary, strings.Join(args, " "))
	
	return nil
}

func parsePluginSpec(spec string) (builder.Plugin, error) {
	// Format: module[@version][=replacement]
	plugin := builder.Plugin{}
	
	// Check for replacement
	if strings.Contains(spec, "=") {
		parts := strings.SplitN(spec, "=", 2)
		spec = parts[0]
		plugin.ReplacementPath = parts[1]
	}
	
	// Check for version
	if strings.Contains(spec, "@") {
		parts := strings.SplitN(spec, "@", 2)
		plugin.ModulePath = parts[0]
		plugin.Version = parts[1]
	} else {
		plugin.ModulePath = spec
	}
	
	if plugin.ModulePath == "" {
		return plugin, fmt.Errorf("module path is required")
	}
	
	return plugin, nil
}

func parseReplaceSpec(spec string) (builder.Replacement, error) {
	// Format: old=new
	parts := strings.SplitN(spec, "=", 2)
	if len(parts) != 2 {
		return builder.Replacement{}, fmt.Errorf("replacement must be in format old=new")
	}
	
	return builder.Replacement{
		Old: parts[0],
		New: parts[1],
	}, nil
} 