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
	"strconv"
	"strings"

	"github.com/botspot/pi-apps/pkg/api"
)

// Build-time variables
var (
	BuildDate string
	GitCommit string
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

	// Handle help flag
	if *helpFlag {
		printUsage()
		return
	}

	// Handle version flag
	if *versionFlag {
		fmt.Println("Pi-Apps Go API (rolling release)")
		if BuildDate != "" {
			fmt.Println("Built on", BuildDate)
		} else {
			fmt.Println("Build date not available")
		}
		if GitCommit != "" {
			fmt.Println("Git commit:", GitCommit)
		} else {
			fmt.Println("Git commit hash not available")
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
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_info <package-name>")
			os.Exit(1)
		}
		info, err := api.PackageInfo(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(info)

	case "package_installed":
		if len(args) < 1 {
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_installed <package-name>")
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
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_available <package-name> [architecture]")
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
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_dependencies <package-name>")
			os.Exit(1)
		}
		deps, err := api.PackageDependencies(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if len(deps) > 0 {
			fmt.Println(deps[0])
		}

	case "package_installed_version":
		if len(args) < 1 {
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_installed_version <package-name>")
			os.Exit(1)
		}
		version, err := api.PackageInstalledVersion(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(version)

	case "package_latest_version":
		if len(args) < 1 {
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api package_latest_version <package-name> [-t <repository>]")
			os.Exit(1)
		}

		var repoArgs []string
		if len(args) >= 3 && args[1] == "-t" {
			repoArgs = []string{"-t", args[2]}
		}

		version, err := api.PackageLatestVersion(args[0], repoArgs...)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(version)

	case "package_is_new_enough":
		if len(args) < 2 {
			fmt.Println("Error: Missing arguments")
			fmt.Println("Usage: api package_is_new_enough <package-name> <version>")
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
			fmt.Println("Error: Missing arguments")
			fmt.Println("Usage: api download_file <url> <destination>")
			os.Exit(1)
		}

		if err := api.DownloadFile(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "file_exists":
		if len(args) < 1 {
			fmt.Println("Error: No file specified")
			fmt.Println("Usage: api file_exists <file-path>")
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
			fmt.Println("Error: No directory specified")
			fmt.Println("Usage: api dir_exists <directory-path>")
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
			fmt.Println("Error: No directory specified")
			fmt.Println("Usage: api ensure_dir <directory-path>")
			os.Exit(1)
		}

		if err := api.EnsureDir(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "copy_file":
		if len(args) < 2 {
			fmt.Println("Error: Missing arguments")
			fmt.Println("Usage: api copy_file <source> <destination>")
			os.Exit(1)
		}

		if err := api.CopyFile(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "files_match":
		if len(args) < 2 {
			fmt.Println("Error: Two files must be specified")
			fmt.Println("Usage: api files_match <file1> <file2>")
			os.Exit(1)
		}

		match, err := api.FilesMatch(args[0], args[1])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
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
			fmt.Println("Error: No file specified")
			fmt.Println("Usage: api text_editor <file-path>")
			os.Exit(1)
		}

		if err := api.TextEditor(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "view_file":
		if len(args) < 1 {
			fmt.Println("Error: No file specified")
			fmt.Println("Usage: api view_file <file>")
			os.Exit(1)
		}

		// Check if the file exists
		if _, err := os.Stat(args[0]); os.IsNotExist(err) {
			fmt.Printf("Error: File does not exist: %s\n", args[0])
			os.Exit(1)
		}

		// Open file viewer
		err := api.ViewFile(args[0])
		if err != nil {
			fmt.Printf("Error viewing file: %v\n", err)
			os.Exit(1)
		}

	case "view_log":
		if len(args) < 1 {
			fmt.Println("Error: No file specified")
			fmt.Println("Usage: api view_log <file>")
			os.Exit(1)
		}

		// Check if the file exists
		if _, err := os.Stat(args[0]); os.IsNotExist(err) {
			fmt.Printf("Error: File does not exist: %s\n", args[0])
			os.Exit(1)
		}

		// Open file viewer
		err := api.ViewLog(args[0])
		if err != nil {
			fmt.Printf("Error viewing file: %v\n", err)
			os.Exit(1)
		}

	case "get_device_info":
		// Call GetDeviceInfo and output the result
		info, err := api.GetDeviceInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting device info: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(info)

	case "diagnose_apps":
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Error: diagnose_apps requires a failure list\n")
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
				fmt.Fprintf(os.Stderr, "Error reading file: %s\n", err)
				os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "Error: Invalid failure list format. Expected 'action;app'\n")
			os.Exit(1)
		}

		// Run the diagnostic UI
		results := api.DiagnoseApps(failureList)

		// Process results
		for _, result := range results {
			if result.Action == "retry" {
				fmt.Printf("Retrying %s...\n", result.ActionStr)
			} else if result.Action == "send" {
				logfilePath := api.GetLogfile(result.AppName)
				fmt.Printf("Sending error report for %s...\n", result.AppName)
				response, err := api.ProcessSendErrorReport(logfilePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error sending report: %s\n", err)
				} else {
					fmt.Println(response)
				}
			}
		}

	case "anything_installed_from_uri_suite_component":
		if len(args) < 2 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api anything_installed_from_uri_suite_component <uri> <suite> [component]")
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
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Exit with code 0 if packages are installed, 1 if not
		if !result {
			os.Exit(1)
		}

	case "remove_repofile_if_unused":
		if len(args) < 1 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api remove_repofile_if_unused <file> [test] [key]")
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
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "repo_add":
		if len(args) < 1 {
			fmt.Println("Error: No files specified")
			fmt.Println("Usage: api repo_add <file1> [file2] [...]")
			os.Exit(1)
		}

		if err := api.RepoAdd(args...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "repo_refresh":
		if err := api.RepoRefresh(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "repo_rm":
		if err := api.RepoRm(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "app_to_pkgname":
		if len(args) < 1 {
			fmt.Println("Error: No app name specified")
			fmt.Println("Usage: api app_to_pkgname <app-name>")
			os.Exit(1)
		}

		pkgName, err := api.AppToPkgName(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(pkgName)

	case "install_packages":
		if len(args) < 1 {
			fmt.Println("Error: No app name specified")
			fmt.Println("Usage: api install_packages <app-name> <package1> [package2] ... [-t repo]")
			os.Exit(1)
		}

		appName := args[0]
		if err := api.InstallPackages(appName, args[1:]...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "purge_packages":
		if len(args) < 1 {
			fmt.Println("Error: No app name specified")
			fmt.Println("Usage: api purge_packages <app-name> [--update]")
			os.Exit(1)
		}

		appName := args[0]
		isUpdate := false

		// Check if update flag is present
		if len(args) > 1 && args[1] == "--update" {
			isUpdate = true
		}

		if err := api.PurgePackages(appName, isUpdate); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "get_icon_from_package":
		if len(args) < 1 {
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: api get_icon_from_package <package-name> [package-name2] [...]")
			os.Exit(1)
		}

		iconPath, err := api.GetIconFromPackage(args...)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(iconPath)

	case "ubuntu_ppa_installer":
		if len(args) < 1 {
			fmt.Println("Error: No PPA name specified")
			fmt.Println("Usage: api ubuntu_ppa_installer <ppa-name>")
			os.Exit(1)
		}

		if err := api.UbuntuPPAInstaller(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "debian_ppa_installer":
		if len(args) < 3 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api debian_ppa_installer <ppa-name> <distribution> <key>")
			os.Exit(1)
		}

		if err := api.DebianPPAInstaller(args[0], args[1], args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "apt_lock_wait":
		if err := api.AptLockWait(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "apt_update":
		if err := api.AptUpdate(args...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "flatpak_install":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api flatpak_install <app-id>")
			os.Exit(1)
		}

		if err := api.FlatpakInstall(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "flatpak_uninstall":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api flatpak_uninstall <app-id>")
			os.Exit(1)
		}

		if err := api.FlatpakUninstall(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "list_apps":
		var filter string
		if len(args) > 0 {
			filter = args[0]
		}

		apps, err := api.ListApps(filter)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Print each app on a new line
		for _, app := range apps {
			fmt.Println(app)
		}

	case "list_intersect":
		if len(args) < 1 {
			fmt.Println("Error: Missing list to intersect with")
			fmt.Println("Usage: list_intersect <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
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
			fmt.Println("Error: Missing list to subtract")
			fmt.Println("Usage: list_subtract <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
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
			fmt.Println("Error: Missing list to intersect with")
			fmt.Println("Usage: list_intersect_partial <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
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
			fmt.Println("Error: Missing list to subtract")
			fmt.Println("Usage: list_subtract_partial <list2> (list1 from stdin)")
			os.Exit(1)
		}

		// Read list1 from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
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
		entries, err := api.ReadCategoryFiles(getDirectory())
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
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

		result, err := api.AppPrefixCategory(getDirectory(), category)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
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
				fmt.Printf("Error reading from stdin: %v\n", err)
				os.Exit(1)
			}
			input = string(bytes)
		} else if len(args) > 0 {
			// Use the argument as input
			input = args[0]
		} else {
			fmt.Println("Error: No input provided")
			fmt.Println("Usage: api less_apt <text> or <command> | api less_apt")
			os.Exit(1)
		}

		// Filter the input
		output := api.LessApt(input)
		fmt.Print(output)

	case "add_external_repo":
		if len(args) < 4 {
			fmt.Println("add_external_repo: requires reponame, pubkeyurl, uris, and suites")
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
			fmt.Println(err)
			os.Exit(1)
		}

	case "rm_external_repo":
		if len(args) < 1 {
			fmt.Println("rm_external_repo: requires reponame")
			os.Exit(1)
		}

		// Check if force flag is provided
		force := false
		if len(args) > 1 && args[1] == "force" {
			force = true
		}

		err := api.RmExternalRepo(args[0], force)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "adoptium_installer":
		err := api.AdoptiumInstaller()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "pipx_install":
		if len(args) < 1 {
			fmt.Println("pipx_install: requires at least one package name")
			os.Exit(1)
		}

		err := api.PipxInstall(args...)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "pipx_uninstall":
		if len(args) < 1 {
			fmt.Println("pipx_uninstall: requires at least one package name")
			os.Exit(1)
		}

		err := api.PipxUninstall(args...)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "remove_deprecated_app":
		if len(args) < 1 {
			fmt.Println("remove_deprecated_app: requires an app name")
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
			fmt.Println(err)
			os.Exit(1)
		}

	case "terminal_manage":
		if len(args) < 2 {
			fmt.Println("terminal_manage: requires an action and an app name")
			os.Exit(1)
		}

		action := args[0]
		app := args[1]

		err := api.TerminalManage(action, app)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "terminal_manage_multi":
		if len(args) < 1 {
			fmt.Println("terminal_manage_multi: requires a queue of actions")
			os.Exit(1)
		}

		queue := args[0]

		err := api.TerminalManageMulti(queue)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "userinput_func":
		if len(args) < 2 {
			fmt.Println("userinput_func: requires a description and at least one option")
			os.Exit(1)
		}

		// First argument is the text description
		text := args[0]

		// Remaining arguments are the options
		options := args[1:]

		selection, err := api.UserInputFunc(text, options...)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Print the selection so it can be captured by the calling script
		fmt.Println(selection)

	case "bitly_link":
		if len(args) < 2 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api bitly_link <app> <trigger>")
			os.Exit(1)
		}

		if err := api.BitlyLink(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "shlink_link":
		if len(args) < 2 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api shlink_link <app> <trigger>")
			os.Exit(1)
		}

		if err := api.ShlinkLink(args[0], args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "usercount":
		var app string
		if len(args) > 0 {
			app = args[0]
		}

		result, err := api.UserCount(app)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result)

	case "script_name":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api script_name <app-name>")
			os.Exit(1)
		}

		scriptName, err := api.ScriptName(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(scriptName)

	case "script_name_cpu":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api script_name_cpu <app-name>")
			os.Exit(1)
		}

		scriptName, err := api.ScriptNameCPU(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(scriptName)

	case "app_status":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api app_status <app-name>")
			os.Exit(1)
		}

		status, err := api.GetAppStatus(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(status)

	case "app_type":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api app_type <app-name>")
			os.Exit(1)
		}

		appType, err := api.AppType(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(appType)

	case "pkgapp_packages_required":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api pkgapp_packages_required <app-name>")
			os.Exit(1)
		}

		packages, err := api.PkgAppPackagesRequired(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(packages)

	case "list_apps_missing_dummy_debs":
		// List apps with missing dummy debs
		apps, err := api.ListAppsMissingDummyDebs()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		for _, app := range apps {
			fmt.Println(app)
		}

	case "runonce":
		// Read script from stdin
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
		script := string(bytes)

		if err := api.Runonce(script); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "will_reinstall":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api will_reinstall <app-name>")
			os.Exit(1)
		}

		willReinstall, err := api.WillReinstall(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
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
			fmt.Println("Error: No query specified")
			fmt.Println("Usage: api app_search <query> [file1 file2 ...]")
			os.Exit(1)
		}

		// First argument is the query, remaining arguments are files to search
		results, err := api.AppSearch(args[0], args[1:]...)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		for _, app := range results {
			fmt.Println(app)
		}

	case "app_search_gui":
		// No arguments needed
		app, err := api.AppSearchGUI()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// If an app was selected, print it
		if app != "" {
			fmt.Println(app)
		}

	case "generate_app_icons":
		if len(args) < 2 {
			fmt.Println("Error: Missing required arguments")
			fmt.Println("Usage: api generate_app_icons <icon-path> <app-name>")
			os.Exit(1)
		}

		iconPath := args[0]
		appName := args[1]

		if err := api.GenerateAppIcons(iconPath, appName); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "refresh_pkgapp_status":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api refresh_pkgapp_status <app-name> [package-name]")
			os.Exit(1)
		}

		appName := args[0]
		packageName := ""
		if len(args) > 1 {
			packageName = args[1]
		}

		if err := api.RefreshPkgAppStatus(appName, packageName); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "refresh_all_pkgapp_status":
		if err := api.RefreshAllPkgAppStatus(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "refresh_app_list":
		if err := api.RefreshAppList(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
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
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "multi_uninstall_gui":
		if err := api.MultiUninstallGUI(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "wget":
		if err := api.Wget(args); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "git_clone":
		if err := api.GitClone(args...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "chmod":
		if err := api.ChmodWithArgs(args...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "unzip":
		if err := api.UnzipWithArgs(args...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "nproc":
		nprocs, err := api.Nproc()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(nprocs)

	case "sudo_popup":
		if len(args) < 1 {
			fmt.Println("Error: No command specified")
			fmt.Println("Usage: api sudo_popup <command> [args...]")
			os.Exit(1)
		}

		command := args[0]
		commandArgs := args[1:]

		if err := api.SudoPopup(command, commandArgs...); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "process_exists":
		if len(args) < 1 {
			fmt.Println("Error: No PID specified")
			fmt.Println("Usage: api process_exists <pid>")
			os.Exit(1)
		}

		pid, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Error: Invalid PID '%s': %v\n", args[0], err)
			os.Exit(1)
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
			fmt.Println("Error: No module name specified")
			fmt.Println("Usage: api enable_module <module-name>")
			os.Exit(1)
		}

		if err := api.EnableModule(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	// UI/Output functions
	case "status":
		if len(args) < 1 {
			fmt.Println("Error: No message specified")
			fmt.Println("Usage: api status <message> [args...]")
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
			fmt.Println("Error: No message specified")
			fmt.Println("Usage: api status_green <message>")
			os.Exit(1)
		}

		api.StatusGreen(args[0])

	case "debug":
		if len(args) < 1 {
			fmt.Println("Error: No message specified")
			fmt.Println("Usage: api debug <message>")
			os.Exit(1)
		}

		api.Debug(args[0])

	case "error":
		if len(args) < 1 {
			fmt.Println("Error: No message specified")
			fmt.Println("Usage: api error <message>")
			os.Exit(1)
		}

		api.Error(args[0])

	case "warning":
		if len(args) < 1 {
			fmt.Println("Error: No message specified")
			fmt.Println("Usage: api warning <message>")
			os.Exit(1)
		}

		api.Warning(args[0])

	case "generate_logo":
		fmt.Print(api.GenerateLogo())

	case "add_english":
		api.AddEnglish()

	case "createapp":
		// Call without arguments to launch the createapp wizard
		if err := api.CreateApp(""); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "importapp":
		// Call without arguments to launch the importapp wizard
		if err := api.ImportAppGUI(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "install":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api install <app-name>")
			os.Exit(1)
		}
		fmt.Printf("Note: This command may require sudo privileges for system operations.\n")
		fmt.Printf("You may be prompted for your password during execution.\n\n")
		fmt.Printf("Installing %s...\n", args[0])
		if err := api.InstallApp(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Installation completed successfully")

	case "uninstall":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api uninstall <app-name>")
			os.Exit(1)
		}
		fmt.Printf("Note: This command may require sudo privileges for system operations.\n")
		fmt.Printf("You may be prompted for your password during execution.\n\n")
		fmt.Printf("Uninstalling %s...\n", args[0])
		if err := api.UninstallApp(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Uninstallation completed successfully")

	case "update":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api update <app-name>")
			os.Exit(1)
		}
		fmt.Printf("Note: This command may require sudo privileges for system operations.\n")
		fmt.Printf("You may be prompted for your password during execution.\n\n")
		fmt.Printf("Updating %s...\n", args[0])
		if err := api.UpdateApp(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Update completed successfully")

	case "install-if-not-installed":
		if len(args) < 1 {
			fmt.Println("Error: No app specified")
			fmt.Println("Usage: api install-if-not-installed <app-name>")
			os.Exit(1)
		}
		fmt.Printf("Note: This command may require sudo privileges for system operations.\n")
		fmt.Printf("You may be prompted for your password during execution.\n\n")
		fmt.Printf("Installing %s if not already installed...\n", args[0])
		if err := api.InstallIfNotInstalled(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Command completed successfully")

	case "manage":
		// If no manage subcommand is specified, show usage
		if len(args) < 1 {
			fmt.Println("Usage: api manage <command> [args...]")
			fmt.Println("Commands:")
			fmt.Println("  install <app>           - Install specified app")
			fmt.Println("  uninstall <app>         - Uninstall specified app")
			fmt.Println("  update <app>            - Update specified app")
			fmt.Println("  install-if-not-installed <app> - Install app only if not already installed")
			fmt.Println("  multi-install <app1> <app2> ... - Install multiple apps")
			fmt.Println("  multi-uninstall <app1> <app2> ... - Uninstall multiple apps")
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
			fmt.Printf("Error running manage command: %v\n", err)
			os.Exit(1)
		}

	case "log_diagnose":
		if len(args) < 1 {
			fmt.Println("Error: No log file specified")
			fmt.Println("Usage: api log_diagnose <logfile> [--allow-write]")
			os.Exit(1)
		}

		allowWrite := false
		if len(args) > 1 && args[1] == "--allow-write" {
			allowWrite = true
		}

		diagnosis, err := api.LogDiagnose(args[0], allowWrite)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Print the diagnosis
		fmt.Printf("Error Type: %s\n", diagnosis.ErrorType)
		for _, caption := range diagnosis.Captions {
			fmt.Println(caption)
		}

	case "format_logfile":
		if len(args) < 1 {
			fmt.Println("Error: No log file specified")
			fmt.Println("Usage: api format_logfile <logfile>")
			os.Exit(1)
		}

		if err := api.FormatLogfile(args[0]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "send_error_report":
		if len(args) < 1 {
			fmt.Println("Error: No log file specified")
			fmt.Println("Usage: api send_error_report <logfile>")
			os.Exit(1)
		}

		response, err := api.SendErrorReport(args[0])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(response)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: api <command> [args...]")
	fmt.Println("")
	fmt.Println("Package Management:")
	fmt.Println("  package_info <package-name>                  - Get information about a package")
	fmt.Println("  package_installed <package-name>             - Check if a package is installed")
	fmt.Println("  package_available <package-name> [arch]      - Check if a package is available")
	fmt.Println("  package_dependencies <package-name>          - List package dependencies")
	fmt.Println("  package_installed_version <package-name>     - Get installed package version")
	fmt.Println("  package_latest_version <package-name> [-t <repo>] - Get latest available package version")
	fmt.Println("  package_is_new_enough <package-name> <version> - Check if package meets version requirement")
	fmt.Println("  install_packages <app-name> <package1> [package2] ... [-t repo] - Install packages for an app")
	fmt.Println("  purge_packages <app-name> [--update]         - Remove packages for an app")
	fmt.Println("  get_icon_from_package <package-name> [package-name2] ... - Get package icon")
	fmt.Println("")
	fmt.Println("Repository Management:")
	fmt.Println("  repo_add <file1> [file2] [...]               - Add repository files")
	fmt.Println("  repo_refresh                                 - Refresh repository data")
	fmt.Println("  repo_rm                                      - Remove repository files")
	fmt.Println("  add_external_repo <name> <keyurl> <uri> <suite> [components] [options] - Add external repository")
	fmt.Println("  rm_external_repo <name> [force]              - Remove external repository")
	fmt.Println("  ubuntu_ppa_installer <ppa-name>              - Install Ubuntu PPA")
	fmt.Println("  debian_ppa_installer <ppa> <dist> <key>      - Install Debian PPA")
	fmt.Println("  remove_repofile_if_unused <file> [test] [key] - Remove repository file if not used")
	fmt.Println("  anything_installed_from_uri_suite_component <uri> <suite> [component] - Check if packages from a repo are installed")
	fmt.Println("  apt_lock_wait                                - Wait for APT lock")
	fmt.Println("  apt_update                                   - Update package lists")
	fmt.Println("")
	fmt.Println("File Operations:")
	fmt.Println("  download_file <url> <destination>            - Download file from URL")
	fmt.Println("  file_exists <file-path>                      - Check if file exists")
	fmt.Println("  dir_exists <directory-path>                  - Check if directory exists")
	fmt.Println("  ensure_dir <directory-path>                  - Create directory if it doesn't exist")
	fmt.Println("  copy_file <source> <destination>             - Copy file")
	fmt.Println("  view_file <file-path>                        - View file contents")
	fmt.Println("  files_match <file1> <file2>                  - Check if two files have identical content")
	fmt.Println("  text_editor <file-path>                      - Open file in preferred text editor")
	fmt.Println("  wget [options] <url>                         - Download files with progress display")
	fmt.Println("  unzip [options] <zipfile> [destination]      - Extract zip archives with standard options")
	fmt.Println("  chmod <mode> <file>                          - Change file permissions with logging")
	fmt.Println("  git_clone <url> [dir] [options]              - Clone git repositories with status display")
	fmt.Println("  nproc                                        - Get optimal thread count based on available RAM")
	fmt.Println("")
	fmt.Println("App Management:")
	fmt.Println("  flatpak_install <app-id>                     - Install Flatpak application")
	fmt.Println("  flatpak_uninstall <app-id>                   - Uninstall Flatpak application")
	fmt.Println("  app_to_pkgname <app-name>                    - Convert app name to package name")
	fmt.Println("  list_apps [filter]                           - List apps with optional filter")
	fmt.Println("  read_category_files                          - Read category assignments")
	fmt.Println("  app_prefix_category [category]               - List apps with category prefix")
	fmt.Println("  terminal_manage <action> <app>               - Manage app via terminal")
	fmt.Println("  terminal_manage_multi <queue>                - Manage multiple apps")
	fmt.Println("  remove_deprecated_app <app> [arch] [message] - Remove deprecated app")
	fmt.Println("  script_name <app-name>                       - Show install script name(s) for an app")
	fmt.Println("  script_name_cpu <app-name>                   - Show appropriate install script for CPU architecture")
	fmt.Println("  app_status <app-name>                        - Get app status (installed, uninstalled, etc.)")
	fmt.Println("  app_type <app-name>                          - Get app type (standard or package)")
	fmt.Println("  pkgapp_packages_required <app-name>          - Get packages required for installation")
	fmt.Println("  will_reinstall <app-name>                    - Check if app will be reinstalled during update")
	fmt.Println("  app_search <query> [file1 file2 ...]         - Search for apps matching query in specified files")
	fmt.Println("  app_search_gui                               - Open graphical interface to search for apps")
	fmt.Println("  multi_install_gui                            - Open graphical interface to install multiple apps")
	fmt.Println("  multi_uninstall_gui                          - Open graphical interface to uninstall multiple apps")
	fmt.Println("  generate_app_icons <icon-path> <app-name>    - Generate 24x24 and 64x64 icons for an app")
	fmt.Println("  refresh_pkgapp_status <app-name> [pkg-name]  - Update status of a package-app")
	fmt.Println("  refresh_all_pkgapp_status                    - Update status of all package-apps")
	fmt.Println("  refresh_app_list                             - Force regeneration of the app list")
	fmt.Println("  createapp                                    - Launch the Create App wizard")
	fmt.Println("  importapp                                    - Launch the Import App wizard")
	fmt.Println("  manage                                       - Manage apps")
	fmt.Println("")
	fmt.Println("List Operations:")
	fmt.Println("  list_intersect <list2> (list1 from stdin)    - Show items in both lists")
	fmt.Println("  list_subtract <list2> (list1 from stdin)     - Show items in list1 not in list2")
	fmt.Println("  list_intersect_partial <list2> (list1 from stdin) - Show items with partial matches")
	fmt.Println("  list_subtract_partial <list2> (list1 from stdin) - Show items without partial matches")
	fmt.Println("")
	fmt.Println("Analytics and Statistics:")
	fmt.Println("  bitly_link <app> <trigger>                   - Send anonymous app usage analytics (legacy)")
	fmt.Println("  shlink_link <app> <trigger>                  - Send anonymous app usage analytics")
	fmt.Println("  usercount [app-name]                         - Show number of users for an app or all apps")
	fmt.Println("")
	fmt.Println("Diagnostic Tools:")
	fmt.Println("  log_diagnose <logfile> [--allow-write]       - Diagnose app error logs")
	fmt.Println("  format_logfile <logfile>                     - Format log file for readability")
	fmt.Println("  send_error_report <logfile>                  - Send error log to Pi-Apps developers")
	fmt.Println("  view_log <logfile>                           - View log contents")
	fmt.Println("  diagnose_apps <failure-list>                 - Diagnose app failures")
	fmt.Println("  get_device_info                              - Show device information")
	fmt.Println("  less_apt <command>                           - Format apt output for readability")
	fmt.Println("")
	fmt.Println("User Interface:")
	fmt.Println("  userinput_func <title> <option1> [option2]   - Interactive selection dialog")
	fmt.Println("  status <message> [args]                      - Display status message")
	fmt.Println("  status_green <message>                       - Display success message")
	fmt.Println("  debug <message>                              - Display debug message")
	fmt.Println("  error <message>                              - Display error message")
	fmt.Println("  warning <message>                            - Display warning message")
	fmt.Println("  add_english                                  - Add English (en_US.UTF-8) locale to the system for improved logging")
	fmt.Println("  generate_logo                                - Display Pi-Apps logo")
	fmt.Println("")
	fmt.Println("Additional Tools:")
	fmt.Println("  adoptium_installer                           - Install Adoptium Java")
	fmt.Println("  pipx_install <package-name> [package2]       - Install Python packages with pipx")
	fmt.Println("  pipx_uninstall <package-name> [package2]     - Uninstall Python packages with pipx")
	fmt.Println("  runonce                                      - Run script only if it's never been run before")
	fmt.Println("  is_supported_system                          - Check if the current system is supported by Pi-Apps")
	fmt.Println("  sudo_popup <command> [args...]               - Run command with elevated privileges, using graphical auth if needed")
	fmt.Println("")
	fmt.Println("System Operations:")
	fmt.Println("  process_exists <pid>                         - Check if a process with the given PID exists")
	fmt.Println("  enable_module <module-name>                  - Ensure a kernel module is loaded and configured to load on startup")
	fmt.Println("")
	fmt.Println("General Options:")
	fmt.Println("  --help, -h                                   - Show this help message")
	fmt.Println("  --version                                    - Show version information")
	fmt.Println("  --logo                                       - Display Pi-Apps logo")
	fmt.Println("  --debug                                      - Enable debug mode")
}

// Helper function to get the PI_APPS_DIR directory
func getDirectory() string {
	dir := os.Getenv("PI_APPS_DIR")
	if dir == "" {
		fmt.Fprintln(os.Stderr, "Warning: PI_APPS_DIR environment variable not set, using current directory")
		dir = "."
	}
	return dir
}
