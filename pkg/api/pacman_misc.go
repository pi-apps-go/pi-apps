// Copyright (C) 2025 pi-apps-go contributors
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
// Module: pacman_misc.go
// Description: Provides functions for miscellaneous operations when using the Pacman package manager. This also contains strings for Pacman related messages.

//go:build pacman

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// variables for pacman related messages
var (
	MissingInitMessage         = T("Congratulations, Linux tinkerer, you broke your system. The init package can not be found, which means you have removed the default Arch sources from your system.\nAll pacman based application installs will fail. Unless you have a backup of your /etc/pacman.conf /etc/pacman.d you will need to reinstall your OS.")
	PackageManager             = "pacman"
	PackageAppErrorMessage     = T("As this is a pacman error, consider Googling the errors or asking for help in the Arch Linux forums.")
	PackageAppNoErrorReporting = T("Error report cannot be sent because this \"app\" is really just a shortcut to install an Arch package. It's not a problem that Pi-Apps can fix.")
	AdoptiumInstallerMessage   = T("Install Adoptium Java from AUR (jdk-temurin)")
	LessAptMessage             = T("Format pacman output for readability")
	AptLockWaitMessage         = T("Wait for pacman lock")
	UbuntuPPAInstallerMessage  = T("Install AUR package (equivalent to Ubuntu PPA)")
	DebianPPAInstallerMessage  = T("Install AUR package (equivalent to Debian PPA, arguments beyond the package name are ignored)")
)

// checkShellcheck checks if shellcheck is installed and installs it if it isn't
func checkShellcheck() error {
	// Initialize application name
	glib.SetPrgname("Pi-Apps-Settings")
	glib.SetApplicationName("Pi-Apps Settings (app creation wizard)")

	// Initialize GTK
	gtk.Init(nil)

	// Check if shellcheck is installed
	if !commandExists("shellcheck") {
		// Ask if they want to install shellcheck
		dialog := gtk.MessageDialogNew(nil, gtk.DIALOG_MODAL, gtk.MESSAGE_QUESTION, gtk.BUTTONS_YES_NO,
			"Shellcheck is not installed, but it's useful for finding errors in shell scripts. Install it now?")
		response := dialog.Run()
		dialog.Destroy()

		if response == gtk.RESPONSE_YES {
			cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "shellcheck")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install shellcheck: %v", err)
			}
		}
	}
	return nil
}

// readPackagesFile reads and parses packages from a packages file
//
//	[]string - list of packages
//	error - error if packages file does not exist
func readPackagesFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading packages file: %w", err)
	}

	var packages []string
	for _, line := range strings.Split(string(data), "\n") {
		// Handle alternative packages (package1 | package2)
		alternativePackages := strings.Split(line, "|")
		for _, pkg := range alternativePackages {
			for _, p := range strings.Fields(pkg) {
				if p != "" {
					packages = append(packages, p)
				}
			}
		}
	}

	return packages, nil
}

// getIconFromPackage tries to find an icon for the given package
func getIconFromPackage(packageName, piAppsDir string) string {
	// ensure piAppsDir is set
	if piAppsDir == "" {
		piAppsDir = GetPiAppsDir()
		os.Setenv("PI_APPS_DIR", piAppsDir)
	}

	// Try running pacman -Ql command to list files in the package
	cmd := exec.Command("pacman", "-Ql", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package not installed, try pacman -Fl (file list from sync database)
		cmd = exec.Command("pacman", "-Fl", packageName)
		output, err = cmd.Output()
		if err != nil {
			return ""
		}
	}

	// Look for icon files in the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// For pacman -Fl output, format is: repository package-name /path/to/file
		// For pacman -Ql output, format is: package-name /path/to/file
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		// Get the last part (the file path)
		filePath := parts[len(parts)-1]

		// Look for icon files in standard directories
		if (strings.Contains(filePath, "/icons/") || strings.Contains(filePath, "/pixmaps/")) &&
			(strings.HasSuffix(filePath, ".png") || strings.HasSuffix(filePath, ".svg") ||
				strings.HasSuffix(filePath, ".xpm") || strings.HasSuffix(filePath, ".jpg")) {
			// Check if the file exists (for installed packages)
			if _, err := os.Stat(filePath); err == nil {
				return filePath
			}
		}
	}

	// If we couldn't find an icon, return empty string
	return ""
}

// PipxInstall installs packages using pipx, handling various distro and Python version requirements
func PipxInstall(packages ...string) error {
	if len(packages) == 0 {
		return fmt.Errorf("%s", T("no packages specified for pipx installation"))
	}

	// Check Python version to determine installation method
	// fallback to checking version using python3 --version
	cmd := exec.Command("python3", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check python3 version: %w", err)
	}
	version := strings.Split(string(output), " ")[1]

	if version >= "3.7" {
		// Python 3.7+ is available, install pipx using pip
		fmt.Println(T("Installing pipx with pip..."))
		// --break-system-packages is needed since no package manager is available to install pipx
		// no assumption can be made if it's a externally managed environment or not
		cmd := exec.Command("sudo", "-H", "python3", "-m", "pip", "install", "--upgrade", "pipx", "--break-system-packages")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf(T("failed to install pipx with pip: %w"), err)
		}
	} else {
		return fmt.Errorf(T("pipx is not available on your distro and so cannot install %s to python venv"), strings.Join(packages, " "))
	}

	// Check if pipx command exists after installation
	fmt.Println(T("Verifying pipx installation..."))
	checkCmd := exec.Command("which", "pipx")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("%s", T("pipx installation failed: command not found after installation"))
	}

	// Install the requested packages with pipx
	fmt.Printf(T("Installing %s with pipx...\n"), strings.Join(packages, ", "))

	// Create the pipx install command with environment variables
	installCmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx install %s",
			strings.Join(packages, " ")))

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf(T("failed to install %s with pipx: %w"), strings.Join(packages, " "), err)
	}

	// Create the pipx upgrade command with environment variables
	upgradeCmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx upgrade %s",
			strings.Join(packages, " ")))

	upgradeCmd.Stdout = os.Stdout
	upgradeCmd.Stderr = os.Stderr

	fmt.Printf(T("Successfully installed %s with pipx\n"), strings.Join(packages, ", "))
	return nil
}

// PipxUninstall uninstalls packages that were installed using pipx
func PipxUninstall(packages ...string) error {
	if len(packages) == 0 {
		return fmt.Errorf("%s", T("no packages specified for pipx uninstallation"))
	}

	// Check if pipx command exists
	checkCmd := exec.Command("which", "pipx")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("%s", T("pipx is not installed: command not found"))
	}

	// Uninstall the requested packages with pipx
	fmt.Printf(T("Uninstalling %s with pipx...\n"), strings.Join(packages, ", "))

	// Create the pipx uninstall command with environment variables
	cmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx uninstall %s",
			strings.Join(packages, " ")))

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(T("failed to uninstall %s with pipx: %w"), strings.Join(packages, " "), err)
	}

	fmt.Printf(T("Successfully uninstalled %s with pipx\n"), strings.Join(packages, ", "))
	return nil
}

// checkFrankenDebian checks if the system has repositories from different Debian/Ubuntu releases
// For Arch Linux, this doesn't apply as it's a rolling release without the concept of mixing releases
func checkFrankenDebian(osInfo *SystemOSInfo) (string, error) {
	// Arch Linux is a rolling release, so there's no concept of "Franken-Debian"
	// This check doesn't apply to Arch-based systems
	return "", nil
}

// checkMissingRepositories checks if important repositories are missing
func checkMissingRepositories(osInfo *SystemOSInfo) (string, error) {
	// Read /etc/pacman.conf to check for essential repositories
	content, err := os.ReadFile("/etc/pacman.conf")
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/pacman.conf: %w", err)
	}

	// Essential repositories for Arch Linux
	essentialRepos := []string{"core", "extra", "community"}
	foundRepos := make(map[string]bool)

	// Parse pacman.conf to find repository sections
	lines := strings.Split(string(content), "\n")
	inRepoSection := false
	var currentRepo string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			continue
		}

		// Check for repository section [repo-name]
		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
			currentRepo = strings.TrimPrefix(strings.TrimSuffix(trimmedLine, "]"), "[")
			// Skip special sections
			if currentRepo == "options" {
				currentRepo = ""
				inRepoSection = false
				continue
			}
			inRepoSection = true

			// Check if this is an essential repository
			for _, essentialRepo := range essentialRepos {
				if currentRepo == essentialRepo {
					foundRepos[essentialRepo] = true
					break
				}
			}
			continue
		}

		// If we're in a repository section and see Include or Server, the repo is configured
		if inRepoSection && currentRepo != "" {
			if strings.HasPrefix(trimmedLine, "Include") || strings.HasPrefix(trimmedLine, "Server") {
				// Repository is configured
				for _, essentialRepo := range essentialRepos {
					if currentRepo == essentialRepo {
						foundRepos[essentialRepo] = true
						break
					}
				}
			}
		}
	}

	// Check if all essential repositories are present
	missingRepos := []string{}
	for _, essentialRepo := range essentialRepos {
		if !foundRepos[essentialRepo] {
			missingRepos = append(missingRepos, essentialRepo)
		}
	}

	if len(missingRepos) > 0 {
		return fmt.Sprintf("MISSING Essential Arch Linux Repositories!\nPi-Apps does NOT support systems without ALL of the following repositories: core, extra, and community.\n\nMissing repositories: %s\n\nPlease check /etc/pacman.conf and ensure these repositories are enabled.", strings.Join(missingRepos, ", ")), nil
	}

	return "", nil
}

// checkBrokenPackages checks if there are broken packages in the system
func checkBrokenPackages() (string, error) {
	// Use pacman -Qkk to check for broken packages
	// -Q: query local database
	// -kk: check file integrity (double k = more thorough)
	cmd := exec.Command("pacman", "-Qkk")
	output, err := cmd.CombinedOutput()

	// pacman -Qkk returns non-zero exit code if there are issues
	if err != nil {
		outputStr := string(output)
		// Check if there are actual problems (not just warnings)
		if strings.Contains(outputStr, "missing") || strings.Contains(outputStr, "corrupted") ||
			strings.Contains(outputStr, "modified") || strings.Contains(outputStr, "No such file") {
			var message strings.Builder
			message.WriteString(T("Pacman has detected broken or corrupted packages on your system.\n"))
			message.WriteString(T("You can try to fix them by running:\n"))
			message.WriteString("  sudo pacman -Syu\n")
			message.WriteString(T("Or reinstall specific packages if needed:\n"))
			message.WriteString("  sudo pacman -S <package-name>\n\n")
			message.WriteString(T("Pacman check output:\n"))
			message.WriteString(outputStr)
			return message.String(), nil
		}
	}

	return "", nil
}

// EnableModule ensures a kernel module is loaded and configured to load on system startup
// It's a Go implementation of the shell 'enable_module' function
func EnableModule(moduleName string) error {
	if moduleName == "" {
		return fmt.Errorf("module name must be specified")
	}

	// Special handling for the fuse module
	if moduleName == "fuse" {
		// Get the app variable, if we're in an app installation context
		appName := os.Getenv("app")

		// Check if we're in an app installation or not
		if appName != "" {
			// Inside app installation - make dependencies
			if PackageAvailable("fuse3", "") {
				if err := InstallPackages(appName, "fuse3", "libfuse2"); err != nil {
					return fmt.Errorf("failed to install fuse3 and libfuse2: %w", err)
				}
			} else if PackageAvailable("fuse", "") {
				if err := InstallPackages(appName, "fuse", "libfuse2"); err != nil {
					return fmt.Errorf("failed to install fuse and libfuse2: %w", err)
				}
			} else {
				if err := InstallPackages(appName, "libfuse2"); err != nil {
					return fmt.Errorf("failed to install libfuse2: %w", err)
				}
			}
		} else {
			// Not in app installation
			if PackageInstalled("libfuse2") && (PackageInstalled("fuse") || PackageInstalled("fuse3")) {
				// Already installed, nothing to do
			} else {
				// Need to install
				if err := AptUpdate(); err != nil {
					return fmt.Errorf("failed to update pacman: %w", err)
				}

				// Use exec.Command to run pacman install
				var cmd *exec.Cmd
				if PackageAvailable("fuse3", "") {
					cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "--needed", "fuse3", "libfuse2")
				} else if PackageAvailable("fuse", "") {
					cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "--needed", "fuse", "libfuse2")
				} else {
					cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "--needed", "libfuse2")
				}

				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to install fuse packages: %w", err)
				}
			}
		}
		// After installing fuse packages, continue with module loading
	}

	// Ensure kmod is installed
	// fallback to checking if kmod is available as a command
	if !commandExists("kmod") {
		return fmt.Errorf("kmod is not installed: command not found, cannot proceed further because no package manager build tag is set")
	}

	// Check if module is builtin
	cmd := exec.Command("modinfo", "--filename", moduleName)
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "(builtin)" {
		// Module is built into the kernel, nothing more to do
		return nil
	}

	// Check if module is already loaded by checking /sys/module/{module} directory
	sysModulePath := fmt.Sprintf("/sys/module/%s", moduleName)
	if _, err := os.Stat(sysModulePath); os.IsNotExist(err) {
		// Module not loaded, need to load it
		cmd := exec.Command("sudo", "modprobe", moduleName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Check if user upgraded kernel but hasn't rebooted
			kernelVersion, kernelErr := exec.Command("uname", "-r").Output()
			if kernelErr == nil {
				kernelModulePath := fmt.Sprintf("/lib/modules/%s", strings.TrimSpace(string(kernelVersion)))
				if _, statErr := os.Stat(kernelModulePath); os.IsNotExist(statErr) {
					return fmt.Errorf("failed to load module '%s' because you upgraded the kernel and have not rebooted yet. Please reboot to load the new kernel, then try again", moduleName)
				}
			}
			return fmt.Errorf("failed to load module '%s': %s", moduleName, string(output))
		}
	}

	// Make it load on boot if system supports loading modules
	procModulesPath := "/proc/modules"
	moduleConfPath := fmt.Sprintf("/etc/modules-load.d/%s.conf", moduleName)

	if _, err := os.Stat(procModulesPath); err == nil {
		if _, err := os.Stat(moduleConfPath); os.IsNotExist(err) {
			// Create the module configuration file
			content := moduleName + "\n"
			cmd := exec.Command("sudo", "tee", moduleConfPath)
			cmd.Stdin = strings.NewReader(content)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to create module load configuration: %w", err)
			}
		}
	}

	return nil
}

// installPackageApp installs a package-based app
func installPackageApp(appName string) error {
	piAppsDir := GetPiAppsDir()

	// Set up logging
	logDir := filepath.Join(piAppsDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFilename := fmt.Sprintf("install-%s-incomplete-%d.log", appName, time.Now().Unix())
	logPath := filepath.Join(logDir, logFilename)

	// Remove any existing incomplete log file
	if _, err := os.Stat(logPath); err == nil {
		os.Remove(logPath)
	}

	// Create log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Write to log file (plain text) and stdout (colored)
	fmt.Fprintf(logFile, "%s Installing %s...\n\n", time.Now().Format("2006-01-02 15:04:05"), appName)
	Status(fmt.Sprintf("Installing \033[1m%s\033[22m...", appName))

	packageListPath := filepath.Join(piAppsDir, "apps", appName, "packages")

	// Read packages list
	packageListBytes, err := os.ReadFile(packageListPath)
	if err != nil {
		fmt.Fprintf(logFile, "Failed to read packages list: %v\n", err)
		return fmt.Errorf("failed to read packages list: %v", err)
	}

	packageList := strings.TrimSpace(string(packageListBytes))
	packages := strings.Fields(packageList)

	if len(packages) == 0 {
		fmt.Fprintf(logFile, "No packages specified in %s\n", packageListPath)
		return fmt.Errorf("no packages specified in %s", packageListPath)
	}

	fmt.Fprintf(logFile, "Will install these packages: %s\n", strings.Join(packages, " "))

	// Use InstallPackages which has proper error detection
	err = InstallPackages(appName, packages...)

	if err != nil {
		// Write failure to log file
		fmt.Fprintf(logFile, "\nFailed to install the packages!\n")
		fmt.Fprintf(logFile, "Error: %v\n", err)

		// Format log file
		FormatLogfile(logPath)

		// Rename log file to indicate failure
		newLogPath := strings.Replace(logPath, "-incomplete-", "-fail-", 1)
		os.Rename(logPath, newLogPath)

		return fmt.Errorf("failed to install packages: %v", err)
	}

	// Success - write to log
	fmt.Fprintf(logFile, "\n%s installed successfully\n", appName)
	StatusGreen(fmt.Sprintf("Installed %s successfully.", appName))

	// Format log file
	FormatLogfile(logPath)

	// Rename log file to indicate success
	newLogPath := strings.Replace(logPath, "-incomplete-", "-success-", 1)
	os.Rename(logPath, newLogPath)

	// Mark app as installed
	return markAppAsInstalled(appName)
}

// uninstallPackageApp uninstalls a package-based app
func uninstallPackageApp(appName string) error {
	piAppsDir := GetPiAppsDir()

	// Set up logging
	logDir := filepath.Join(piAppsDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFilename := fmt.Sprintf("uninstall-%s-incomplete-%d.log", appName, time.Now().Unix())
	logPath := filepath.Join(logDir, logFilename)

	// Remove any existing incomplete log file
	if _, err := os.Stat(logPath); err == nil {
		os.Remove(logPath)
	}

	// Create log file
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Write to log file (plain text) and stdout (colored)
	fmt.Fprintf(logFile, "%s Uninstalling %s...\n\n", time.Now().Format("2006-01-02 15:04:05"), appName)
	Status(fmt.Sprintf("Uninstalling \033[1m%s\033[22m...", appName))

	// Use PurgePackages which has proper error detection
	err = PurgePackages(appName, false)

	if err != nil {
		// Write failure to log file
		fmt.Fprintf(logFile, "\nFailed to uninstall the packages!\n")
		fmt.Fprintf(logFile, "Error: %v\n", err)

		// Format log file
		FormatLogfile(logPath)

		// Rename log file to indicate failure
		newLogPath := strings.Replace(logPath, "-incomplete-", "-fail-", 1)
		os.Rename(logPath, newLogPath)

		return fmt.Errorf("failed to uninstall packages: %v", err)
	}

	// Success - write to log
	fmt.Fprintf(logFile, "\n%s uninstalled successfully\n", appName)
	StatusGreen(fmt.Sprintf("Uninstalled %s successfully.", appName))

	// Format log file
	FormatLogfile(logPath)

	// Rename log file to indicate success
	newLogPath := strings.Replace(logPath, "-incomplete-", "-success-", 1)
	os.Rename(logPath, newLogPath)

	// Mark app as uninstalled
	return markAppAsUninstalled(appName)
}

// installPackageAppDependencies installs the dependencies for a package-based app without pi-apps having to create a new virtual package
func installPackageAppDependencies(dependencies ...string) error {
	// Install packages with sudo
	cmd := exec.Command("sudo", append([]string{"pacman", "-S", "--noconfirm", "--needed"}, dependencies...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install dependencies: %v", err)
	}

	return nil
}

// uninstallPackageAppDependencies uninstalls the dependencies for a package-based app
func uninstallPackageAppDependencies(dependencies ...string) error {
	// Uninstall packages with sudo
	cmd := exec.Command("sudo", append([]string{"pacman", "-R", "--noconfirm", "--nosave"}, dependencies...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall dependencies: %v", err)
	}

	return nil
}
