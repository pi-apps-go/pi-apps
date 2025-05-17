package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/botspot/pi-apps/pkg/api"
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
		fmt.Println("Pi-Apps Go Edition v0.1.0")
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
	case "install":
		// Handle installation
		if len(args) < 1 {
			fmt.Println("Error: No package specified for installation")
			fmt.Println("Usage: pi-apps install <package-name>")
			os.Exit(1)
		}

		packageName := args[0]
		if err := api.InstallPackage(packageName); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "uninstall":
		// Handle uninstallation
		if len(args) < 1 {
			fmt.Println("Error: No package specified for removal")
			fmt.Println("Usage: pi-apps uninstall <package-name>")
			os.Exit(1)
		}

		packageName := args[0]
		if err := api.RemovePackage(packageName); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "update":
		// Handle update
		if err := api.UpdatePackages(); err != nil {
			fmt.Printf("Error updating package lists: %v\n", err)
			os.Exit(1)
		}

	case "upgrade":
		// Handle upgrade
		if err := api.UpgradePackages(); err != nil {
			fmt.Printf("Error upgrading packages: %v\n", err)
			os.Exit(1)
		}

	case "list":
		// List installed packages
		fmt.Println("Listing installed packages:")
		// This is a simple implementation - we would need more sophisticated
		// package listing in a real implementation
		code := api.RunCommand("dpkg", "--get-selections")
		if code != 0 {
			os.Exit(1)
		}

	case "search":
		// Search for packages
		if len(args) < 1 {
			fmt.Println("Error: No search query specified")
			fmt.Println("Usage: pi-apps search <query>")
			os.Exit(1)
		}

		query := args[0]
		fmt.Printf("Searching for package: %s\n", query)
		api.RunCommand("apt-cache", "search", query)

	case "show":
		// Show package details
		if len(args) < 1 {
			fmt.Println("Error: No package specified")
			fmt.Println("Usage: pi-apps show <package-name>")
			os.Exit(1)
		}

		packageName := args[0]
		info, err := api.PackageInfo(packageName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(info)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Pi-Apps Go Edition - Usage:")
	fmt.Println("  pi-apps <command> [options]")
	fmt.Println("")
	fmt.Println("Available commands:")
	fmt.Println("  install <package>      Install a package")
	fmt.Println("  uninstall <package>    Uninstall a package")
	fmt.Println("  update                 Update package lists")
	fmt.Println("  upgrade                Upgrade all packages")
	fmt.Println("  list                   List installed packages")
	fmt.Println("  search <query>         Search for packages")
	fmt.Println("  show <package>         Show package details")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --debug    Enable debug output")
	fmt.Println("  --help     Show this help message")
	fmt.Println("  --version  Show version information")
	fmt.Println("  --logo     Display the Pi-Apps logo")
}
