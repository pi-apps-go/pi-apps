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

// Module: main.go
// Description: Provides a user interactible way of communicating with the Pi-Apps Go API functions.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// Build-time variables
var (
	BuildDate string
	GitCommit string
)

// Plugin functions are now handled by the build-time plugin system
// No runtime plugin management needed

func main() {
	// runtime crashes can happen (keep in mind Pi-Apps Go is ALPHA software)
	// so add a handler to log those runtime errors to save them to a log file
	// this option can be disabled by specifying DISABLE_ERROR_HANDLING to true

	errorHandling := os.Getenv("DISABLE_ERROR_HANDLING")
	if errorHandling != "true" {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace as a string
				stackTrace := string(debug.Stack())

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
	// initialize variables required for api to function
	api.Init()

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

	// Handle help flag
	if *helpFlag {
		printUsage()
		return
	}

	// Handle version flag
	if *versionFlag {
		fmt.Println(api.T("Pi-Apps Go API binary runtime (rolling release)"))
		if BuildDate != "" {
			api.StatusT(api.Tf("Built on %s", BuildDate))
		} else {
			api.ErrorNoExitT(api.T("Build date not available"))
		}
		if GitCommit != "" {
			api.StatusT(api.Tf("Git commit: %s", GitCommit))
			account, repo := api.GetGitUrl()
			if account != "" && repo != "" {
				api.StatusT(api.Tf("Link to commit: https://github.com/%s/%s/commit/%s", account, repo, GitCommit))
			}
		} else {
			api.ErrorNoExitT(api.T("Git commit hash not available"))
		}
		return
	}

	// Handle logo flag
	if *logoFlag {
		fmt.Print(api.GenerateLogo())
		return
	}

	// If no arguments were provided, print usage and exit
	if flag.NArg() == 0 {
		printUsage()
		os.Exit(1)
	}

	// Get the command and arguments
	command := flag.Arg(0)
	args := flag.Args()[1:]

	// Execute the requested command
	switch strings.ToLower(command) {
	case "package_info":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_info <package-name>")
			os.Exit(1)
		}
		info, err := api.PackageInfo(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(info)

	case "package_installed":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_installed <package-name>")
			os.Exit(1)
		}
		if api.PackageInstalled(args[0]) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "package_available":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_available <package-name> [architecture]")
			os.Exit(1)
		}

		var arch string
		if len(args) > 1 {
			arch = args[1]
		}

		if api.PackageAvailable(args[0], arch) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "package_dependencies":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_dependencies <package-name>")
			os.Exit(1)
		}
		deps, err := api.PackageDependencies(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		if len(deps) > 0 {
			fmt.Println(deps[0])
		}

	case "package_installed_version":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_installed_version <package-name>")
			os.Exit(1)
		}
		version, err := api.PackageInstalledVersion(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(version)

	case "package_latest_version":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api package_latest_version <package-name> [-t <repository>]")
			os.Exit(1)
		}

		var repoArgs []string
		if len(args) >= 3 && args[1] == "-t" {
			repoArgs = []string{"-t", args[2]}
		}

		version, err := api.PackageLatestVersion(args[0], repoArgs...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(version)

	case "package_is_new_enough":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Missing arguments")
			api.StatusT("Usage: api package_is_new_enough <package-name> <version>")
			os.Exit(1)
		}

		if api.PackageIsNewEnough(args[0], args[1]) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "download_file":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Missing arguments")
			api.StatusT("Usage: api download_file <url> <destination>")
			os.Exit(1)
		}

		if err := api.DownloadFile(args[0], args[1]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "file_exists":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No file specified")
			api.StatusT("Usage: api file_exists <file-path>")
			os.Exit(1)
		}

		if api.FileExists(args[0]) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "dir_exists":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No directory specified")
			api.StatusT("Usage: api dir_exists <directory-path>")
			os.Exit(1)
		}

		if api.DirExists(args[0]) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "ensure_dir":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No directory specified")
			api.StatusT("Usage: api ensure_dir <directory-path>")
			os.Exit(1)
		}

		if err := api.EnsureDir(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "copy_file":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Missing arguments")
			api.StatusT("Usage: api copy_file <source> <destination>")
			os.Exit(1)
		}

		if err := api.CopyFile(args[0], args[1]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "files_match":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Two files must be specified")
			api.StatusT("Usage: api files_match <file1> <file2>")
			os.Exit(1)
		}

		match, err := api.FilesMatch(args[0], args[1])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		if match {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "text_editor":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No file specified")
			api.StatusT("Usage: api text_editor <file-path>")
			os.Exit(1)
		}

		if err := api.TextEditor(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "view_file":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No file specified")
			api.StatusT("Usage: api view_file <file>")
			os.Exit(1)
		}

		// Check if the file exists
		if _, err := os.Stat(args[0]); os.IsNotExist(err) {
			api.ErrorT(api.Tf("Error: File does not exist: %s", args[0]))
		}

		// Open file viewer
		err := api.ViewFile(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error viewing file: %v", err))
		}

	case "view_log":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No file specified")
			api.StatusT("Usage: api view_log <file>")
			os.Exit(1)
		}

		// Check if the file exists
		if _, err := os.Stat(args[0]); os.IsNotExist(err) {
			api.ErrorT(api.Tf("Error: File does not exist: %s", args[0]))
		}

		// Open file viewer
		err := api.ViewLog(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error viewing file: %v", err))
		}

	case "logviewer":
		if len(args) >= 1 {
			// If a log file is specified, view it directly
			if _, err := os.Stat(args[0]); os.IsNotExist(err) {
				api.ErrorT(api.Tf("Error: File does not exist: %s", args[0]))
			}

			err := api.ViewFile(args[0])
			if err != nil {
				api.ErrorT(api.Tf("Error viewing log file: %v", err))
			}
		} else {
			// Show the log viewer GUI
			err := api.ShowLogViewer()
			if err != nil {
				api.ErrorT(api.Tf("Error showing log viewer: %v", err))
			}
		}

	case "categoryedit":
		if len(args) == 2 {
			// Command line usage: categoryedit <app> <category>
			err := api.EditAppCategory(args[0], args[1])
			if err != nil {
				api.ErrorT(api.Tf("Error editing app category: %v", err))
			}
		} else if len(args) == 0 {
			// GUI usage: show category editor
			err := api.ShowCategoryEditor()
			if err != nil {
				api.ErrorT(api.Tf("Error showing category editor: %v", err))
			}
		} else {
			api.ErrorNoExitT("Error: Invalid number of arguments")
			api.StatusT("Usage: api categoryedit [<app-name> <category>]")
			api.StatusT("  Without arguments: Shows GUI category editor")
			api.StatusT("  With arguments: Sets category for specific app")
			os.Exit(1)
		}

	case "get_device_info":
		// Call GetDeviceInfo and output the result
		info, err := api.GetDeviceInfo()
		if err != nil {
			api.ErrorT(api.Tf("Error getting device info: %v", err))
		}
		fmt.Print(info)

	case "diagnose_apps":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: diagnose_apps requires a failure list")
			os.Exit(1)
		}

		// Get the input
		input := args[0]
		var failureList string

		// If input looks like a file path and exists, read it
		if strings.Contains(input, "/") && api.FileExists(input) {
			// Read the file and parse for app name
			content, err := os.ReadFile(input)
			if err != nil {
				api.ErrorT(api.Tf("Error reading file: %s", err))
			}

			// Extract app name from log file path
			// Typical path: .../logs/AppName
			appName := filepath.Base(input)

			// Default to "install" action if we can't determine it
			action := "install"

			// Check file content for hints about the action type
			contentStr := string(content)
			if strings.Contains(contentStr, "Uninstalling") {
				action = "uninstall"
			} else if strings.Contains(contentStr, "Updating") {
				action = "update"
			}

			// Format a proper failure list
			failureList = fmt.Sprintf("%s;%s", action, appName)
		} else {
			// Input is already a failure list
			failureList = input
		}

		// Validate the format
		if !strings.Contains(failureList, ";") {
			api.ErrorT(api.T("Error: Invalid failure list format. Expected 'action;app'"))
		}

		// Run the diagnostic UI
		results := api.DiagnoseApps(failureList)

		// Process results
		for _, result := range results {
			switch result.Action {
			case "retry":
				fmt.Printf("Retrying %s...\n", result.ActionStr)
			case "send":
				logfilePath := api.GetLogfile(result.AppName)
				fmt.Printf("Sending error report for %s...\n", result.AppName)
				response, err := api.ProcessSendErrorReport(logfilePath)
				if err != nil {
					api.ErrorT(api.Tf("Error sending report: %s", err))
				} else {
					fmt.Println(response)
				}
			}
		}

	case "anything_installed_from_uri_suite_component":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Missing required arguments")
			api.StatusT("Usage: api anything_installed_from_uri_suite_component <uri> <suite> [component]")
			os.Exit(1)
		}

		uri := args[0]
		suite := args[1]
		component := ""
		if len(args) > 2 {
			component = args[2]
		}

		result, err := api.AnythingInstalledFromURISuiteComponent(uri, suite, component)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		// Exit with code 0 if packages are installed, 1 if not
		if !result {
			os.Exit(1)
		}

	case "remove_repofile_if_unused":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: Missing required arguments")
			api.StatusT("Usage: api remove_repofile_if_unused <file> [test] [key]")
			os.Exit(1)
		}

		file := args[0]
		testMode := ""
		key := ""

		if len(args) > 1 {
			testMode = args[1]
		}

		if len(args) > 2 {
			key = args[2]
		}

		err := api.RemoveRepofileIfUnused(file, testMode, key)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "repo_add":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No files specified")
			api.StatusT("Usage: api repo_add <file1> [file2] [...]")
			os.Exit(1)
		}

		if err := api.RepoAdd(args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "repo_refresh":
		if err := api.RepoRefresh(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "repo_rm":
		if err := api.RepoRm(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "app_to_pkgname":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app name specified")
			api.StatusT("Usage: api app_to_pkgname <app-name>")
			os.Exit(1)
		}

		pkgName, err := api.AppToPkgName(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(pkgName)

	case "install_packages":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No packages specified")
			api.StatusT("Usage: api install_packages <package1> [package2] ... [-t repo] - Install packages (requires $app environment variable)")
			api.StatusT("Note: This command should be used within an app installation context where $app is set")
			os.Exit(1)
		}

		// Check if we're in an app installation context (has $app environment variable)
		appName := os.Getenv("app")
		if appName == "" {
			api.ErrorNoExitT("Error: install_packages function can only be used by apps to install packages")
			api.ErrorNoExitT("The $app environment variable was not set")
			api.StatusT("This command should be called from within an app installation script")
			os.Exit(1)
		}

		if err := api.InstallPackages(appName, args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "purge_packages":
		// Check if we're in an app installation context (has $app environment variable)
		appName := os.Getenv("app")
		if appName == "" {
			api.ErrorNoExitT("Error: purge_packages function can only be used by apps to install packages")
			api.ErrorNoExitT("The $app environment variable was not set")
			api.StatusT("This command should be called from within an app installation script")
			os.Exit(1)
		}

		isUpdate := false

		// Check if update flag is present in arguments
		if len(args) > 0 && args[0] == "--update" {
			isUpdate = true
		}

		if err := api.PurgePackages(appName, isUpdate); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "get_icon_from_package":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No package specified")
			api.StatusT("Usage: api get_icon_from_package <package-name> [package-name2] [...]")
			os.Exit(1)
		}

		iconPath, err := api.GetIconFromPackage(args...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(iconPath)

	case "get_pi_app_icon":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app name specified")
			api.StatusT("Usage: api get_pi_app_icon <app-name>")
			os.Exit(1)
		}

		iconPath, err := api.GetPiAppIcon(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(iconPath)

	case "ubuntu_ppa_installer":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No PPA name specified")
			api.StatusT("Usage: api ubuntu_ppa_installer <ppa-name>")
			os.Exit(1)
		}

		if err := api.UbuntuPPAInstaller(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "debian_ppa_installer":
		if len(args) < 3 {
			api.ErrorNoExitT("Error: Missing required arguments")
			api.StatusT("Usage: api debian_ppa_installer <ppa-name> <distribution> <key>")
			os.Exit(1)
		}

		if err := api.DebianPPAInstaller(args[0], args[1], args[2]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "apt_lock_wait":
		if err := api.AptLockWait(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "apt_update":
		if err := api.AptUpdate(args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "flatpak_install":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api flatpak_install <app-id>")
			os.Exit(1)
		}

		if err := api.FlatpakInstall(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "flatpak_uninstall":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api flatpak_uninstall <app-id>")
			os.Exit(1)
		}

		if err := api.FlatpakUninstall(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "list_apps":
		var filter string
		if len(args) > 0 {
			filter = args[0]
		}

		apps, err := api.ListApps(filter)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		// Print each app on a new line
		for _, app := range apps {
			fmt.Println(app)
		}

	case "list_intersect":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: Missing list to intersect with")
			api.StatusT("Usage: list_intersect <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			api.ErrorT(api.Tf("Error reading from stdin: %v", err))
		}

		// Parse list1 from stdin
		var list1 []string
		for _, line := range strings.Split(string(bytes), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list1 = append(list1, line)
			}
		}

		// Parse list2 from args - replace literal \n with actual newlines
		arg := strings.ReplaceAll(args[0], "\\n", "\n")
		var list2 []string
		for _, line := range strings.Split(arg, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list2 = append(list2, line)
			}
		}

		// Get intersection and print results
		result := api.ListIntersect(list1, list2)
		for _, item := range result {
			fmt.Println(item)
		}

	case "list_subtract":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: Missing list to subtract")
			api.StatusT("Usage: list_subtract <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			api.ErrorT(api.Tf("Error reading from stdin: %v", err))
		}

		// Parse list1 from stdin
		var list1 []string
		for _, line := range strings.Split(string(bytes), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list1 = append(list1, line)
			}
		}

		// Parse list2 from args - replace literal \n with actual newlines
		arg := strings.ReplaceAll(args[0], "\\n", "\n")
		var list2 []string
		for _, line := range strings.Split(arg, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list2 = append(list2, line)
			}
		}

		// Get subtraction and print results
		result := api.ListSubtract(list1, list2)
		for _, item := range result {
			fmt.Println(item)
		}

	case "list_intersect_partial":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: Missing list to intersect with")
			api.StatusT("Usage: list_intersect_partial <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			api.ErrorT(api.Tf("Error reading from stdin: %v", err))
		}

		// Parse list1 from stdin
		var list1 []string
		for _, line := range strings.Split(string(bytes), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list1 = append(list1, line)
			}
		}

		// Parse list2 from args - replace literal \n with actual newlines
		arg := strings.ReplaceAll(args[0], "\\n", "\n")
		var list2 []string
		for _, line := range strings.Split(arg, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list2 = append(list2, line)
			}
		}

		// Get partial intersection and print results
		result := api.ListIntersectPartial(list1, list2)
		for _, item := range result {
			fmt.Println(item)
		}

	case "list_subtract_partial":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: Missing list to subtract")
			api.StatusT("Usage: list_subtract_partial <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			api.ErrorT(api.Tf("Error reading from stdin: %v", err))
		}

		// Parse list1 from stdin
		var list1 []string
		for _, line := range strings.Split(string(bytes), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list1 = append(list1, line)
			}
		}

		// Parse list2 from args - replace literal \n with actual newlines
		arg := strings.ReplaceAll(args[0], "\\n", "\n")
		var list2 []string
		for _, line := range strings.Split(arg, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				list2 = append(list2, line)
			}
		}

		// Get partial subtraction and print results
		result := api.ListSubtractPartial(list1, list2)
		for _, item := range result {
			fmt.Println(item)
		}

	case "read_category_files":
		// Read category files and print in app|category format
		entries, err := api.ReadCategoryFiles(api.GetPiAppsDir())
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		for _, entry := range entries {
			fmt.Println(entry)
		}

	case "app_prefix_category":
		// List apps with category prefix, or all categories if no argument
		var category string
		if len(args) > 0 {
			category = args[0]
		}

		result, err := api.AppPrefixCategory(api.GetPiAppsDir(), category)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		for _, item := range result {
			fmt.Println(item)
		}

	case "less_apt":
		var input string

		// Check if stdin has data (piped input)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Read from stdin
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				api.ErrorT(api.Tf("Error reading from stdin: %v", err))
			}
			input = string(bytes)
		} else if len(args) > 0 {
			// Use the argument as input
			input = args[0]
		} else {
			api.ErrorNoExitT("Error: No input provided")
			api.StatusT("Usage: api less_apt <text> or <command> | api less_apt")
			os.Exit(1)
		}

		// Filter the input
		output := api.LessApt(input)
		fmt.Print(output)

	case "add_external_repo":
		if len(args) < 4 {
			api.ErrorNoExitT("Error: add_external_repo: requires reponame, pubkeyurl, uris, and suites")
			api.StatusT("Usage: api add_external_repo <reponame> <pubkeyurl> <uris> <suites> [components] [options]")
			os.Exit(1)
		}

		// Get required parameters
		reponame := args[0]
		pubkeyurl := args[1]
		uris := args[2]
		suites := args[3]

		// Get optional components parameter
		components := ""
		if len(args) > 4 {
			components = args[4]
		}

		// Get any additional options
		var additionalOptions []string
		if len(args) > 5 {
			additionalOptions = args[5:]
		}

		err := api.AddExternalRepo(reponame, pubkeyurl, uris, suites, components, additionalOptions...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "rm_external_repo":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: rm_external_repo: requires reponame")
			api.StatusT("Usage: api rm_external_repo <reponame> [force]")
			os.Exit(1)
		}

		// Check if force flag is provided
		force := false
		if len(args) > 1 && args[1] == "force" {
			force = true
		}

		err := api.RmExternalRepo(args[0], force)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "adoptium_installer":
		err := api.AdoptiumInstaller()
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "pipx_install":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: pipx_install: requires at least one package name")
			api.StatusT("Usage: api pipx_install <package1> [package2] [...]")
			os.Exit(1)
		}

		err := api.PipxInstall(args...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "pipx_uninstall":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: pipx_uninstall: requires at least one package name")
			api.StatusT("Usage: api pipx_uninstall <package1> [package2] [...]")
			os.Exit(1)
		}

		err := api.PipxUninstall(args...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "remove_deprecated_app":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: remove_deprecated_app: requires an app name")
			api.StatusT("Usage: api remove_deprecated_app <app-name> [arch] [message]")
			os.Exit(1)
		}

		app := args[0]

		// Check for optional args
		removalArch := ""
		message := ""

		if len(args) > 1 {
			removalArch = args[1]
		}

		if len(args) > 2 {
			message = args[2]
		}

		err := api.RemoveDeprecatedApp(app, removalArch, message)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "terminal_manage":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: terminal_manage: requires an action and an app name")
			api.StatusT("Usage: api terminal_manage <action> <app-name>")
			os.Exit(1)
		}

		action := args[0]
		app := args[1]

		err := api.TerminalManage(action, app)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "terminal_manage_multi":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: terminal_manage_multi: requires a queue of actions")
			api.StatusT("Usage: api terminal_manage_multi <queue>")
			os.Exit(1)
		}

		queue := args[0]

		err := api.TerminalManageMulti(queue)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "userinput_func":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: userinput_func: requires a description and at least one option")
			api.StatusT("Usage: api userinput_func <description> <option1> [option2] [...]")
			os.Exit(1)
		}

		// First argument is the text description
		text := args[0]

		// Remaining arguments are the options
		options := args[1:]

		selection, err := api.UserInputFunc(text, options...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		// Print the selection so it can be captured by the calling script
		fmt.Println(selection)

	case "bitly_link":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: bitly_link: requires an app and a trigger")
			api.StatusT("Usage: api bitly_link <app> <trigger>")
			os.Exit(1)
		}

		if err := api.BitlyLink(args[0], args[1]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "shlink_link":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: shlink_link: requires an app and a trigger")
			api.StatusT("Usage: api shlink_link <app> <trigger>")
			os.Exit(1)
		}

		if err := api.ShlinkLink(args[0], args[1]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "usercount":
		var app string
		if len(args) > 0 {
			app = args[0]
		}

		result, err := api.UserCount(app)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(result)

	case "script_name":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: script_name: requires an app name")
			api.StatusT("Usage: api script_name <app-name>")
			os.Exit(1)
		}

		scriptName, err := api.ScriptName(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(scriptName)

	case "script_name_cpu":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api script_name_cpu <app-name>")
			os.Exit(1)
		}

		scriptName, err := api.ScriptNameCPU(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(scriptName)

	case "app_status":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api app_status <app-name>")
			os.Exit(1)
		}

		status, err := api.GetAppStatus(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(status)

	case "app_type":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api app_type <app-name>")
			os.Exit(1)
		}

		appType, err := api.AppType(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(appType)

	case "pkgapp_packages_required":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api pkgapp_packages_required <app-name>")
			os.Exit(1)
		}

		packages, err := api.PkgAppPackagesRequired(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		fmt.Println(packages)

	case "list_apps_missing_dummy_debs":
		// List apps with missing dummy debs
		apps, err := api.ListAppsMissingDummyDebs()
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		for _, app := range apps {
			fmt.Println(app)
		}

	case "runonce":
		// Read script from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			api.ErrorT(api.Tf("Error reading from stdin: %v", err))
		}
		script := string(bytes)

		if err := api.Runonce(script); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "will_reinstall":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api will_reinstall <app-name>")
			os.Exit(1)
		}

		willReinstall, err := api.WillReinstall(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		if willReinstall {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "app_search":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No query specified")
			api.StatusT("Usage: api app_search <query> [file1 file2 ...]")
			os.Exit(1)
		}

		// First argument is the query, remaining arguments are files to search
		results, err := api.AppSearch(args[0], args[1:]...)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		for _, app := range results {
			fmt.Println(app)
		}

	case "app_search_gui":
		// No arguments needed
		app, err := api.AppSearchGUI()
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		// If an app was selected, print it
		if app != "" {
			fmt.Println(app)
		}

	case "generate_app_icons":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: Missing required arguments")
			api.StatusT("Usage: api generate_app_icons <icon-path> <app-name>")
			os.Exit(1)
		}

		iconPath := args[0]
		appName := args[1]

		if err := api.GenerateAppIcons(iconPath, appName); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "refresh_pkgapp_status":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api refresh_pkgapp_status <app-name> [package-name]")
			os.Exit(1)
		}

		appName := args[0]
		packageName := ""
		if len(args) > 1 {
			packageName = args[1]
		}

		if err := api.RefreshPkgAppStatus(appName, packageName); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "refresh_all_pkgapp_status":
		if err := api.RefreshAllPkgAppStatus(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "refresh_app_list":
		if err := api.RefreshAppList(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "is_supported_system":
		isSupported, message := api.IsSupportedSystem()
		if message != "" {
			fmt.Println(message)
		}
		if isSupported {
			os.Exit(0)
		} else {
			os.Exit(1)
		}

	case "multi_install_gui":
		if err := api.MultiInstallGUI(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "multi_uninstall_gui":
		if err := api.MultiUninstallGUI(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "wget":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No URL specified")
			api.StatusT("Usage: api wget <url>")
			os.Exit(1)
		}

		if err := api.Wget(args); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "git_clone":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No URL specified")
			api.StatusT("Usage: api git_clone <url> [dir] [options]")
			os.Exit(1)
		}

		if err := api.GitClone(args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "chmod":
		if len(args) < 2 {
			api.ErrorNoExitT("Error: No mode specified")
			api.StatusT("Usage: api chmod <mode> <file>")
			os.Exit(1)
		}

		if err := api.ChmodWithArgs(args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "unzip":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No zip file specified")
			api.StatusT("Usage: api unzip <zipfile> [destination]")
			os.Exit(1)
		}

		if err := api.UnzipWithArgs(args...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "nproc":
		nprocs, err := api.Nproc()
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(nprocs)

	case "sudo_popup":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No command specified")
			api.StatusT("Usage: api sudo_popup <command> [args...]")
			os.Exit(1)
		}

		command := args[0]
		commandArgs := args[1:]

		if err := api.SudoPopup(command, commandArgs...); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "process_exists":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No PID specified")
			api.StatusT("Usage: api process_exists <pid>")
			os.Exit(1)
		}

		pid, err := strconv.Atoi(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: Invalid PID '%s': %v", args[0], err))
		}

		if api.ProcessExists(pid) {
			fmt.Println("true")
			os.Exit(0)
		} else {
			fmt.Println("false")
			os.Exit(1)
		}

	case "enable_module":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No module name specified")
			api.StatusT("Usage: api enable_module <module-name>")
			os.Exit(1)
		}

		if err := api.EnableModule(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	// UI/Output functions
	case "status":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No message specified")
			api.StatusT("Usage: api status <message> [args...]")
			os.Exit(1)
		}

		// Handle flags for status
		if strings.HasPrefix(args[0], "-") && len(args) > 1 {
			api.Status(args[0], args[1])
		} else {
			api.Status(args[0])
		}

	case "status_green":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No message specified")
			api.StatusT("Usage: api status_green <message>")
			os.Exit(1)
		}

		api.StatusGreenT(args[0])

	case "debug":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No message specified")
			api.StatusT("Usage: api debug <message>")
			os.Exit(1)
		}

		api.Debug(args[0])

	case "error":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No message specified")
			api.StatusT("Usage: api error <message>")
			os.Exit(1)
		}

		api.ErrorT(args[0])

	case "warning":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No message specified")
			api.StatusT("Usage: api warning <message>")
			os.Exit(1)
		}

		api.WarningT(args[0])

	case "generate_logo":
		fmt.Print(api.GenerateLogo())

	case "add_english":
		api.AddEnglish()

	case "createapp":
		// Call without arguments to launch the createapp wizard
		if err := api.CreateApp(""); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "importapp":
		// Call without arguments to launch the importapp wizard
		if err := api.ImportAppGUI(); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "install":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api install <app-name>")
			os.Exit(1)
		}
		api.StatusT("Note: This command may require sudo privileges for system operations.")
		api.StatusT("You may be prompted for your password during execution.")
		if err := api.InstallApp(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		api.StatusGreenT("Installation completed successfully")

	case "uninstall":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api uninstall <app-name>")
			os.Exit(1)
		}
		api.StatusT("Note: This command may require sudo privileges for system operations.")
		api.StatusT("You may be prompted for your password during execution.")
		if err := api.UninstallApp(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		api.StatusGreenT("Uninstallation completed successfully")

	case "update":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api update <app-name>")
			os.Exit(1)
		}
		api.StatusT("Note: This command may require sudo privileges for system operations.")
		api.StatusT("You may be prompted for your password during execution.")
		if err := api.UpdateApp(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		api.StatusGreenT("Update completed successfully")

	case "install-if-not-installed":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No app specified")
			api.StatusT("Usage: api install-if-not-installed <app-name>")
			os.Exit(1)
		}
		api.StatusT("Note: This command may require sudo privileges for system operations.")
		api.StatusT("You may be prompted for your password during execution.")
		if err := api.InstallIfNotInstalled(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		api.StatusGreenT("Command completed successfully")

	case "manage":
		// If no manage subcommand is specified, show usage
		if len(args) < 1 {
			api.StatusT("Usage: api manage <command> [args...]")
			api.StatusT("Commands:")
			api.StatusT("  install <app>           - Install specified app")
			api.StatusT("  uninstall <app>         - Uninstall specified app")
			api.StatusT("  update <app>            - Update specified app")
			api.StatusT("  install-if-not-installed <app> - Install app only if not already installed")
			api.StatusT("  multi-install <app1> <app2> ... - Install multiple apps")
			api.StatusT("  multi-uninstall <app1> <app2> ... - Uninstall multiple apps")
			os.Exit(1)
		}

		// Run the manage command with the provided arguments
		manageCmd := exec.Command(filepath.Join(filepath.Dir(os.Args[0]), "api-manage"), args...)
		manageCmd.Stdout = os.Stdout
		manageCmd.Stderr = os.Stderr
		manageCmd.Stdin = os.Stdin

		err := manageCmd.Run()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
			api.ErrorT(api.Tf("Error running manage command: %v", err))
		}

	case "log_diagnose":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No log file specified")
			api.StatusT("Usage: api log_diagnose <logfile> [--allow-write]")
			os.Exit(1)
		}

		allowWrite := false
		if len(args) > 1 && args[1] == "--allow-write" {
			allowWrite = true
		}

		diagnosis, err := api.LogDiagnose(args[0], allowWrite)
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

		// Print the diagnosis
		fmt.Printf("Error Type: %s\n", diagnosis.ErrorType)
		for _, caption := range diagnosis.Captions {
			fmt.Println(caption)
		}

	case "format_logfile":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No log file specified")
			api.StatusT("Usage: api format_logfile <logfile>")
			os.Exit(1)
		}

		if err := api.FormatLogfile(args[0]); err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}

	case "send_error_report":
		if len(args) < 1 {
			api.ErrorNoExitT("Error: No log file specified")
			api.StatusT("Usage: api send_error_report <logfile>")
			os.Exit(1)
		}

		response, err := api.SendErrorReport(args[0])
		if err != nil {
			api.ErrorT(api.Tf("Error: %v", err))
		}
		fmt.Println(response)

	case "crash":
		var a []int
		fmt.Println(a[1])

	// Plugin system commands have been removed - plugins are now build-time only

	// All plugin commands have been removed - plugins are now build-time only
	// Runtime plugin management is no longer supported

	default:
		// Plugin command checking has been removed - plugins are now build-time only
		api.ErrorNoExitT(api.Tf("Unknown command: %s", command))
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(api.T("Usage: api <command> [args...]"))
	fmt.Println("")
	fmt.Println(api.T("Package Management:"))
	fmt.Println("  package_info <package-name>                  - " + api.T("Get information about a package"))
	fmt.Println("  package_installed <package-name>             - " + api.T("Check if a package is installed"))
	fmt.Println("  package_available <package-name> [arch]      - " + api.T("Check if a package is available"))
	fmt.Println("  package_dependencies <package-name>          - " + api.T("List package dependencies"))
	fmt.Println("  package_installed_version <package-name>     - " + api.T("Get installed package version"))
	fmt.Println("  package_latest_version <package-name> [-t <repo>] - " + api.T("Get latest available package version"))
	fmt.Println("  package_is_new_enough <package-name> <version> - " + api.T("Check if package meets version requirement"))
	fmt.Println("  install_packages <package1> [package2] ... [-t repo] - " + api.T("Install packages (requires $app environment variable)"))
	fmt.Println("  purge_packages [--update]                    - " + api.T("Remove packages for app (requires $app environment variable)"))
	fmt.Println("  get_icon_from_package <package-name> [package-name2] ... - " + api.T("Get package icon"))
	fmt.Println("  get_pi_app_icon <app-name>                    - " + api.T("Get Pi-Apps app icon path"))
	fmt.Println("")
	fmt.Println(api.T("Repository Management:"))
	fmt.Println("  repo_add <file1> [file2] [...]               - " + api.T("Add repository files"))
	fmt.Println("  repo_refresh                                 - " + api.T("Refresh repository data"))
	fmt.Println("  repo_rm                                      - " + api.T("Remove repository files"))
	fmt.Println("  add_external_repo <name> <keyurl> <uri> <suite> [components] [options] - " + api.T("Add external repository"))
	fmt.Println("  rm_external_repo <name> [force]              - " + api.T("Remove external repository"))
	fmt.Println("  ubuntu_ppa_installer <ppa-name>              - " + api.T("Install Ubuntu PPA"))
	fmt.Println("  debian_ppa_installer <ppa> <dist> <key>      - " + api.T("Install Debian PPA"))
	fmt.Println("  remove_repofile_if_unused <file> [test] [key] - " + api.T("Remove repository file if not used"))
	fmt.Println("  anything_installed_from_uri_suite_component <uri> <suite> [component] - " + api.T("Check if packages from a repo are installed"))
	fmt.Println("  apt_lock_wait                                - " + api.T("Wait for APT lock"))
	fmt.Println("  apt_update                                   - " + api.T("Update package lists"))
	fmt.Println("")
	fmt.Println(api.T("File Operations:"))
	fmt.Println("  download_file <url> <destination>            - " + api.T("Download file from URL"))
	fmt.Println("  file_exists <file-path>                      - " + api.T("Check if file exists"))
	fmt.Println("  dir_exists <directory-path>                  - " + api.T("Check if directory exists"))
	fmt.Println("  ensure_dir <directory-path>                  - " + api.T("Create directory if it doesn't exist"))
	fmt.Println("  copy_file <source> <destination>             - " + api.T("Copy file"))
	fmt.Println("  view_file <file-path>                        - " + api.T("View file contents"))
	fmt.Println("  files_match <file1> <file2>                  - " + api.T("Check if two files have identical content"))
	fmt.Println("  text_editor <file-path>                      - " + api.T("Open file in preferred text editor"))
	fmt.Println("  wget [options] <url>                         - " + api.T("Download files with progress display"))
	fmt.Println("  unzip [options] <zipfile> [destination]      - " + api.T("Extract zip archives with standard options"))
	fmt.Println("  chmod <mode> <file>                          - " + api.T("Change file permissions with logging"))
	fmt.Println("  git_clone <url> [dir] [options]              - " + api.T("Clone git repositories with status display"))
	fmt.Println("  nproc                                        - " + api.T("Get optimal thread count based on available RAM"))
	fmt.Println("")
	fmt.Println(api.T("App Management:"))
	fmt.Println("  flatpak_install <app-id>                     - " + api.T("Install Flatpak application"))
	fmt.Println("  flatpak_uninstall <app-id>                   - " + api.T("Uninstall Flatpak application"))
	fmt.Println("  app_to_pkgname <app-name>                    - " + api.T("Convert app name to package name"))
	fmt.Println("  list_apps [filter]                           - " + api.T("List apps with optional filter"))
	fmt.Println("  read_category_files                          - " + api.T("Read category assignments"))
	fmt.Println("  app_prefix_category [category]               - " + api.T("List apps with category prefix"))
	fmt.Println("  terminal_manage <action> <app>               - " + api.T("Manage app via terminal"))
	fmt.Println("  terminal_manage_multi <queue>                - " + api.T("Manage multiple apps"))
	fmt.Println("  remove_deprecated_app <app> [arch] [message] - " + api.T("Remove deprecated app"))
	fmt.Println("  script_name <app-name>                       - " + api.T("Show install script name(s) for an app"))
	fmt.Println("  script_name_cpu <app-name>                   - " + api.T("Show appropriate install script for CPU architecture"))
	fmt.Println("  app_status <app-name>                        - " + api.T("Get app status (installed, uninstalled, etc.)"))
	fmt.Println("  app_type <app-name>                          - " + api.T("Get app type (standard or package)"))
	fmt.Println("  pkgapp_packages_required <app-name>          - " + api.T("Get packages required for installation"))
	fmt.Println("  will_reinstall <app-name>                    - " + api.T("Check if app will be reinstalled during update"))
	fmt.Println("  app_search <query> [file1 file2 ...]         - " + api.T("Search for apps matching query in specified files"))
	fmt.Println("  app_search_gui                               - " + api.T("Open graphical interface to search for apps"))
	fmt.Println("  multi_install_gui                            - " + api.T("Open graphical interface to install multiple apps"))
	fmt.Println("  multi_uninstall_gui                          - " + api.T("Open graphical interface to uninstall multiple apps"))
	fmt.Println("  generate_app_icons <icon-path> <app-name>    - " + api.T("Generate 24x24 and 64x64 icons for an app"))
	fmt.Println("  refresh_pkgapp_status <app-name> [pkg-name]  - " + api.T("Update status of a package-app"))
	fmt.Println("  refresh_all_pkgapp_status                    - " + api.T("Update status of all package-apps"))
	fmt.Println("  refresh_app_list                             - " + api.T("Force regeneration of the app list"))
	fmt.Println("  createapp                                    - " + api.T("Launch the Create App wizard"))
	fmt.Println("  importapp                                    - " + api.T("Launch the Import App wizard"))
	fmt.Println("  manage                                       - " + api.T("Manage apps"))
	fmt.Println("  logviewer                                    - " + api.T("View log files in a graphical interface"))
	fmt.Println("  categoryedit [<app-name> <category>]         - " + api.T("Edit app categories (GUI without args, CLI with args)"))
	fmt.Println("")
	fmt.Println(api.T("List Operations:"))
	fmt.Println("  list_intersect <list2> (list1 from stdin)    - " + api.T("Show items in both lists"))
	fmt.Println("  list_subtract <list2> (list1 from stdin)     - " + api.T("Show items in list1 not in list2"))
	fmt.Println("  list_intersect_partial <list2> (list1 from stdin) - " + api.T("Show items with partial matches"))
	fmt.Println("  list_subtract_partial <list2> (list1 from stdin) - " + api.T("Show items without partial matches"))
	fmt.Println("")
	fmt.Println(api.T("Analytics and Statistics:"))
	fmt.Println("  bitly_link <app> <trigger>                   - " + api.T("Send anonymous app usage analytics (legacy)"))
	fmt.Println("  shlink_link <app> <trigger>                  - " + api.T("Send anonymous app usage analytics"))
	fmt.Println("  usercount [app-name]                         - " + api.T("Show number of users for an app or all apps"))
	fmt.Println("")
	fmt.Println(api.T("Diagnostic Tools:"))
	fmt.Println("  log_diagnose <logfile> [--allow-write]       - " + api.T("Diagnose app error logs"))
	fmt.Println("  format_logfile <logfile>                     - " + api.T("Format log file for readability"))
	fmt.Println("  send_error_report <logfile>                  - " + api.T("Send error log to Pi-Apps developers"))
	fmt.Println("  view_log <logfile>                           - " + api.T("View log contents"))
	fmt.Println("  diagnose_apps <failure-list>                 - " + api.T("Diagnose app failures"))
	fmt.Println("  get_device_info                              - " + api.T("Show device information"))
	fmt.Println("  less_apt <command>                           - " + api.T("Format apt output for readability"))
	fmt.Println("")
	fmt.Println(api.T("User Interface:"))
	fmt.Println("  userinput_func <title> <option1> [option2]   - " + api.T("Interactive selection dialog"))
	fmt.Println("  status <message> [args]                      - " + api.T("Display status message"))
	fmt.Println("  status_green <message>                       - " + api.T("Display success message"))
	fmt.Println("  debug <message>                              - " + api.T("Display debug message"))
	fmt.Println("  error <message>                              - " + api.T("Display error message"))
	fmt.Println("  warning <message>                            - " + api.T("Display warning message"))
	fmt.Println("  add_english                                  - " + api.T("Add English (en_US.UTF-8) locale to the system for improved logging"))
	fmt.Println("  generate_logo                                - " + api.T("Display Pi-Apps logo"))
	fmt.Println("")
	fmt.Println(api.T("Additional Tools:"))
	fmt.Println("  adoptium_installer                           - " + api.T("Install Adoptium Java Debian repository"))
	fmt.Println("  pipx_install <package-name> [package2]       - " + api.T("Install Python packages with pipx"))
	fmt.Println("  pipx_uninstall <package-name> [package2]     - " + api.T("Uninstall Python packages with pipx"))
	fmt.Println("  runonce                                      - " + api.T("Run script only if it's never been run before"))
	fmt.Println("  is_supported_system                          - " + api.T("Check if the current system is supported by Pi-Apps"))
	fmt.Println("  sudo_popup <command> [args...]               - " + api.T("Run command with elevated privileges, using graphical auth if needed"))
	fmt.Println("")
	fmt.Println(api.T("System Operations:"))
	fmt.Println("  process_exists <pid>                         - " + api.T("Check if a process with the given PID exists"))
	fmt.Println("  enable_module <module-name>                  - " + api.T("Ensure a kernel module is loaded and configured to load on startup"))
	fmt.Println("")
	fmt.Println(api.T("Plugin System:"))
	fmt.Println("  " + api.T("Plugins are now build-time only. Use 'xpi-apps build --with <plugin>' to build pi-apps with plugins."))
	fmt.Println("")
	fmt.Println(api.T("General Options:"))
	fmt.Println("  --help, -h                                   - " + api.T("Show this help message"))
	fmt.Println("  --version                                    - " + api.T("Show version information"))
	fmt.Println("  --logo                                       - " + api.T("Display Pi-Apps logo"))
	fmt.Println("  --debug                                      - " + api.T("Enable debug mode"))
}
