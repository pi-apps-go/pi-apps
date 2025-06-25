package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/botspot/pi-apps/pkg/gui"
	"github.com/charmbracelet/log"
	"github.com/gotk3/gotk3/gtk"
)

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportCaller:    true,
	ReportTimestamp: true,
	TimeFormat:      time.Kitchen,
})

func runGUI() {
	// Set environment variable to indicate we're using multi-call binary
	// This will be used by the GUI to determine which binary to call for terminal_manage
	currentExecutable, err := os.Executable()
	if err == nil {
		os.Setenv("PI_APPS_MULTI_CALL_BINARY", currentExecutable)
	}

	// Reset flag.CommandLine to avoid conflicts
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	var (
		directory      = flag.String("directory", "", "Pi-Apps directory (defaults to PI_APPS_DIR env var)")
		mode           = flag.String("mode", "", "GUI mode: gtk, yad-default, xlunch-dark, etc.")
		help           = flag.Bool("help", false, "Show help message")
		version        = flag.Bool("version", false, "Show version information")
		showAppDetails = flag.Bool("show-app-details", false, "Show app details dialog (internal use)")
	)
	flag.Parse()

	// Handle special case for showing app details dialog
	if *showAppDetails {
		args := flag.Args()
		if len(args) < 2 {
			logger.Fatal("--show-app-details requires directory and app name arguments")
		}

		piAppsDir := args[0]
		appName := args[1]

		// Initialize GTK
		gtk.Init(nil)

		// Create GUI instance in native mode
		config := gui.GUIConfig{
			Directory: piAppsDir,
			GuiMode:   "native",
		}

		app, err := gui.NewGUI(config)
		if err != nil {
			logger.Fatal("Failed to create GUI for app details: %v", err)
		}

		if err := app.Initialize(); err != nil {
			logger.Fatal("Failed to initialize GUI for app details: %v", err)
		}

		// Show the app details dialog
		app.ShowAppDetailsForDialog(appName)

		// Run GTK main loop
		gtk.Main()
		return
	}

	if *help {
		printGUIUsage()
		return
	}

	// Handle version flag
	if *version {
		fmt.Println("Pi-Apps GUI binary runtime (rolling release)")
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

	// Set default directory if not provided
	if *directory == "" {
		*directory = os.Getenv("PI_APPS_DIR")
		if *directory == "" {
			logger.Fatal("PI_APPS_DIR environment variable not set and no directory specified")
		}
	}

	// Set default mode
	if *mode == "" {
		*mode = "default"
	}

	fmt.Println(api.GenerateLogo())
	properties := logger.With("compiled-on", BuildDate, "git-commit", GitCommit, "mode", *mode)
	properties.Info("Starting Pi-Apps GUI...")

	// Create GUI configuration
	config := gui.GUIConfig{
		Directory: *directory,
		GuiMode:   *mode,
	}

	// Create and initialize GUI
	app, err := gui.NewGUI(config)
	if err != nil {
		logger.Fatal("Failed to create GUI: %v", err)
	}

	if err := app.Initialize(); err != nil {
		logger.Fatal("Failed to initialize GUI: %v", err)
	}

	// Ensure cleanup on exit
	defer app.Cleanup()

	// Run the GUI
	if err := app.Run(); err != nil {
		logger.Fatal("Failed to run GUI: %v", err)
	}
}

func printGUIUsage() {
	fmt.Println("Pi-Apps GUI")
	fmt.Println("Usage: gui [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --directory <path>    Pi-Apps directory (defaults to PI_APPS_DIR env var)")
	fmt.Println("  --mode <mode>         GUI mode: gtk, yad-default, xlunch-dark, etc.")
	fmt.Println("  --help               Show this help message")
	fmt.Println("  --version            Show version information")
	fmt.Println("  --show-app-details   Show app details dialog (internal use)")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  PI_APPS_DIR  Path to Pi-Apps directory")
	fmt.Println()
	fmt.Println("GUI Modes:")
	fmt.Println("  default      Auto-detect best interface (GTK3 if available, fallback to bash)")
	fmt.Println("  gtk          Native GTK3 interface")
	fmt.Println("  native       Same as gtk")
	fmt.Println("  yad-default  YAD-based interface (compatibility, deprecated)")
	fmt.Println("  xlunch-dark  XLunch dark theme")
}
