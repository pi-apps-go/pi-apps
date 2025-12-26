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

// Module: misc_apt.go
// Description: Provides functions for miscellaneous operations that require APT. This also contains strings for APT related messages.

//go:build apt

package api

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// variables for APT related messages
var (
	MissingInitMessage         = T("Congratulations, Linux tinkerer, you broke your system. The init package can not be found, which means you have removed the default debian sources from your system.\nAll apt based application installs will fail. Unless you have a backup of your /etc/apt/sources.list /etc/apt/sources.list.d you will need to reinstall your OS.")
	PackageManager             = "apt"
	PackageAppErrorMessage     = T("As this is an APT error, consider Googling the errors or asking for help in the <a href=\"https://forums.raspberrypi.com\">Raspberry Pi Forums</a>.")
	PackageAppNoErrorReporting = T("Error report cannot be sent because this \"app\" is really just a shortcut to install a Debian package. It's not a problem that Pi-Apps can fix.")
	AdoptiumInstallerMessage   = T("Install Adoptium Java Debian repository")
	LessAptMessage             = T("Format apt output for readability")
	AptLockWaitMessage         = T("Wait for APT lock")
	UbuntuPPAInstallerMessage  = T("Install Ubuntu PPA")
	DebianPPAInstallerMessage  = T("Install Debian PPA")
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
			cmd := exec.Command("sudo", "apt-get", "install", "-y", "shellcheck")
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
	// Try running dpkg -L command to list files in the package
	cmd := exec.Command("dpkg", "-L", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package not installed, try apt-file
		if commandExists("apt-file") {
			cmd = exec.Command("apt-file", "list", packageName)
			output, err = cmd.Output()
			if err != nil {
				return ""
			}
		} else {
			return ""
		}
	}

	// Look for icon files in the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// For apt-file output, extract the filepath
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				line = parts[1]
			}
		}

		// Look for icon files in standard directories
		if (strings.Contains(line, "/icons/") || strings.Contains(line, "/pixmaps/")) &&
			(strings.HasSuffix(line, ".png") || strings.HasSuffix(line, ".svg") ||
				strings.HasSuffix(line, ".xpm") || strings.HasSuffix(line, ".jpg")) {
			// Check if the file exists
			if _, err := os.Stat(line); err == nil {
				return line
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

	// Use "pipx" as the app name for tracking dependencies
	appName := "pipx"

	// Check if pipx is available with a new enough version (>= 1.0.0)
	pipxAvailable := PackageAvailable("pipx", "")

	pipxNewEnough := false
	if pipxAvailable {
		pipxNewEnough = PackageIsNewEnough("pipx", "1.0.0")
	}

	if pipxAvailable && pipxNewEnough {
		// Install pipx from package manager if it's available and new enough
		if err := InstallPackages(appName, "pipx", "python3-venv"); err != nil {
			return fmt.Errorf(T("failed to install pipx and python3-venv: %w"), err)
		}
	} else {
		// Check Python version to determine installation method
		python3NewEnough := PackageIsNewEnough("python3", "3.7")

		if python3NewEnough {
			// Python 3.7+ is available, install pipx using pip
			if err := InstallPackages(appName, "python3-venv"); err != nil {
				return fmt.Errorf(T("failed to install python3-venv: %w"), err)
			}

			fmt.Println(T("Installing pipx with pip..."))
			cmd := exec.Command("sudo", "-H", "python3", "-m", "pip", "install", "--upgrade", "pipx")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf(T("failed to install pipx with pip: %w"), err)
			}
		} else {
			// Check if Python 3.8 is available
			python38Available := PackageAvailable("python3.8", "")

			if python38Available {
				// Install Python 3.8 and its venv package
				if err := InstallPackages(appName, "python3.8", "python3.8-venv"); err != nil {
					return fmt.Errorf(T("failed to install python3.8 and python3.8-venv: %w"), err)
				}

				fmt.Println(T("Installing pipx with pip using Python 3.8..."))
				cmd := exec.Command("sudo", "-H", "python3.8", "-m", "pip", "install", "--upgrade", "pipx")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf(T("failed to install pipx with pip using python3.8: %w"), err)
				}
			} else {
				// No suitable Python version found
				return fmt.Errorf(T("pipx is not available on your distro and so cannot install %s to python venv"), strings.Join(packages, " "))
			}
		}
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
func checkFrankenDebian(osInfo *SystemOSInfo) (string, error) {
	if osInfo.ID != "Debian" && osInfo.ID != "Raspbian" && osInfo.ID != "Ubuntu" {
		return "", nil
	}

	// Execute apt-get indextargets to get available repositories
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(COMPONENT) $(TARGET_OF)")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute apt-get indextargets: %w", err)
	}

	// Process repositories
	availableRepos := []string{}
	mismatches := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[3] == "deb" {
			repo := fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2])
			if !strings.HasSuffix(repo, "$(COMPONENT)") {
				availableRepos = append(availableRepos, repo)

				// Check for mismatches with current codename
				if fields[1] != osInfo.Codename && fields[1] != osInfo.Codename+"-updates" && fields[1] != osInfo.Codename+"-security" {
					if !strings.HasPrefix(osInfo.Description, "Parrot") { // Skip check for Parrot OS
						mismatches = append(mismatches, fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2]))
					}
				}
			}
		}
	}

	if len(mismatches) > 0 {
		message := "Congratulations, Linux tinkerer, you broke your system. You have made your system a FrankenDebian.\n" +
			"This website explains your mistake in more detail: https://wiki.debian.org/DontBreakDebian\n" +
			fmt.Sprintf("Your current reported release (%s) should not be combined with other releases.\n", cases.Title(language.English).String(osInfo.Codename))

		if len(mismatches) == 1 {
			message += "Specifically, the issue is this line:\n"
		} else {
			message += "Specifically, the issue is these lines:\n"
		}

		for _, mismatch := range mismatches {
			fields := strings.Fields(mismatch)
			site := fields[0]
			release := fields[1]

			// Find source entry for this mismatch
			findCmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SOURCESENTRY)",
				fmt.Sprintf("Release: %s", release), fmt.Sprintf("Site: %s", site))
			sourceOutput, err := findCmd.Output()
			if err == nil {
				// Extract file path
				sourceLines := strings.Split(string(sourceOutput), "\n")
				if len(sourceLines) > 0 {
					sources := []string{}
					for _, line := range sourceLines {
						if line != "" {
							parts := strings.SplitN(line, ":", 2)
							if len(parts) > 0 && parts[0] != "" {
								sources = append(sources, parts[0])
							}
						}
					}
					// Remove duplicates
					uniqueSources := []string{}
					seen := make(map[string]bool)
					for _, source := range sources {
						if !seen[source] {
							seen[source] = true
							uniqueSources = append(uniqueSources, source)
						}
					}
					message += fmt.Sprintf("\u001B[4m%s\u001B[24m in %s\n", mismatch, strings.Join(uniqueSources, ", "))
				}
			} else {
				message += fmt.Sprintf("\u001B[4m%s\u001B[24m\n", mismatch)
			}
		}

		message += "Your system might be recoverable if you did this recently and have not performed an apt upgrade yet, but otherwise you should probably reinstall your OS."
		return message, nil
	}

	// Check if there are any available repositories
	if len(availableRepos) == 0 {
		return "Congratulations, Linux tinkerer, you broke your system. You have removed ALL debian sources from your system.\nAll apt based application installs will fail. Unless you have a backup of your /etc/apt/sources.list /etc/apt/sources.list.d you will need to reinstall your OS.", nil
	}

	return "", nil
}

// checkMissingRepositories checks if important repositories are missing
func checkMissingRepositories(osInfo *SystemOSInfo) (string, error) {
	if osInfo.ID != "Debian" && osInfo.ID != "Raspbian" && osInfo.ID != "Ubuntu" {
		return "", nil
	}

	// Execute apt-get indextargets to get available repositories
	cmd := exec.Command("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(COMPONENT) $(TARGET_OF)")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute apt-get indextargets: %w", err)
	}

	// Process repositories to get default repos
	defaultRepos := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[3] == "deb" {
			site := fields[0]

			// Check if the site is one of the default repositories
			if strings.Contains(site, "raspbian.raspberrypi.org/raspbian") ||
				strings.Contains(site, "archive.raspberrypi.org/debian") ||
				strings.Contains(site, "raspbian.raspberrypi.com/raspbian") ||
				strings.Contains(site, "archive.raspberrypi.com/debian") ||
				strings.Contains(site, "debian.org/debian") ||
				strings.Contains(site, "security.debian.org/") ||
				strings.Contains(site, "ports.ubuntu.com") ||
				strings.Contains(site, "esm.ubuntu.com/apps/ubuntu") ||
				strings.Contains(site, "esm.ubuntu.com/infra/ubuntu") ||
				strings.Contains(site, "repo.huaweicloud.com/debian") ||
				strings.Contains(site, "repo.huaweicloud.com/ubuntu-ports") ||
				strings.Contains(site, "deb-multimedia.org") ||
				strings.Contains(site, "apt.pop-os.org") ||
				strings.Contains(site, "apt.armbian.com") ||
				// Add additional mirrors for x86 support
				strings.Contains(site, "archive.ubuntu.com/ubuntu") ||
				strings.Contains(site, "mirror.umd.edu/ubuntu") ||
				strings.Contains(site, "us.archive.ubuntu.com/ubuntu") ||
				strings.Contains(site, "ftp.debian.org/debian") ||
				strings.Contains(site, "deb.debian.org/debian") {
				defaultRepos = append(defaultRepos, fmt.Sprintf("%s %s %s", site, fields[1], fields[2]))
			}
		}
	}

	// Check specific requirements based on OS
	switch osInfo.ID {
	case "Ubuntu":
		// Check for main and universe components
		mainCount := 0
		universeCount := 0
		mainUpdatesCount := 0
		universeUpdatesCount := 0
		mainSecurityCount := 0
		universeSecurityCount := 0

		for _, repo := range defaultRepos {
			fields := strings.Fields(repo)
			if len(fields) >= 3 {
				release := fields[1]
				component := fields[2]

				switch release {
				case osInfo.Codename:
					switch component {
					case "main":
						mainCount++
					case "universe":
						universeCount++
					}
				case osInfo.Codename + "-updates":
					switch component {
					case "main":
						mainUpdatesCount++
					case "universe":
						universeUpdatesCount++
					}
				case osInfo.Codename + "-security":
					switch component {
					case "main":
						mainSecurityCount++
					case "universe":
						universeSecurityCount++
					}
				}
			}
		}

		if mainCount == 0 || universeCount == 0 || mainUpdatesCount == 0 || universeUpdatesCount == 0 || mainSecurityCount == 0 || universeSecurityCount == 0 {
			return fmt.Sprintf("MISSING Default Ubuntu Repositories!\nPi-Apps does NOT support systems without ALL of %s, %s-updates, and %s-security dists and main and universe components present in the sources.list\nPlease refer to the default sources.list for Ubuntu and restore all required dists and components.", osInfo.Codename, osInfo.Codename, osInfo.Codename), nil
		}
	case "Debian":
		// Check for main component
		mainCount := 0
		mainUpdatesCount := 0
		mainSecurityCount := 0

		for _, repo := range defaultRepos {
			fields := strings.Fields(repo)
			if len(fields) >= 3 && strings.Contains(fields[0], "debian.org/debian") {
				release := fields[1]
				component := fields[2]

				switch {
				case release == osInfo.Codename && component == "main":
					mainCount++
				case release == osInfo.Codename+"-updates" && component == "main":
					mainUpdatesCount++
				case strings.HasSuffix(release, "-security") && component == "main":
					mainSecurityCount++
				}
			}
		}

		if mainCount == 0 || mainUpdatesCount == 0 || mainSecurityCount == 0 {
			return fmt.Sprintf("MISSING Default Debian Repositories!\nPi-Apps does NOT support systems without ALL of %s, %s-updates, and %s-security dists and main component present in the sources.list\nPlease refer to the default sources.list for Debian and restore all required dists and components.", osInfo.Codename, osInfo.Codename, osInfo.Codename), nil
		}
	case "Raspbian":
		// Check for main component in Raspbian
		mainCount := 0

		for _, repo := range defaultRepos {
			fields := strings.Fields(repo)
			if len(fields) >= 3 && strings.Contains(fields[0], "/raspbian") {
				release := fields[1]
				component := fields[2]

				if release == osInfo.Codename && component == "main" {
					mainCount++
				}
			}
		}

		if mainCount == 0 {
			return fmt.Sprintf("MISSING Default Raspbian Repositories!\nPi-Apps does NOT support systems without %s dist and main component present in the sources.list\nPlease refer to the default sources.list for Raspbian and restore all required dists and components.", osInfo.Codename), nil
		}
	}

	return "", nil
}

// checkBrokenPackages checks if there are broken packages in the system
func checkBrokenPackages() (string, error) {
	cmd := exec.Command("apt-get", "--dry-run", "check")
	err := cmd.Run()
	if err != nil {
		// Try to get more detailed error information
		output, _ := exec.Command("apt-get", "--dry-run", "check").CombinedOutput()
		return fmt.Sprintf("Congratulations, Linux tinkerer, you broke your system. There are packages on your system that are in a broken state.\nRefer to the output below for any potential solutions.\n\n%s", string(output)), nil
	}
	return "", nil
}

// EnableModule ensures a kernel module is loaded and configured to load on system startup
// It's a Go implementation of the shell 'enable_module' function
func EnableModule(moduleName string) error {
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
					return fmt.Errorf("failed to update apt: %w", err)
				}

				// Use exec.Command to run apt install
				var cmd *exec.Cmd
				if PackageAvailable("fuse3", "") {
					cmd = exec.Command("sudo", "apt", "install", "-y", "fuse3", "libfuse2", "--no-install-recommends")
				} else if PackageAvailable("fuse", "") {
					cmd = exec.Command("sudo", "apt", "install", "-y", "fuse", "libfuse2", "--no-install-recommends")
				} else {
					cmd = exec.Command("sudo", "apt", "install", "-y", "libfuse2", "--no-install-recommends")
				}

				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to install fuse packages: %w", err)
				}
			}
		}
	}

	// Ensure kmod is installed
	if !PackageInstalled("kmod") {
		appName := os.Getenv("app")
		if appName != "" {
			// Inside app installation
			if err := InstallPackages(appName, "kmod"); err != nil {
				return fmt.Errorf("failed to install kmod: %w", err)
			}
		} else {
			// Not in app installation
			if err := AptUpdate(); err != nil {
				return fmt.Errorf("failed to update apt: %w", err)
			}

			cmd := exec.Command("sudo", "apt", "install", "-y", "kmod", "--no-install-recommends")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install kmod: %w", err)
			}
		}

		// Refresh PATH-related caches (equivalent to hash -r in bash)
		// In Go, this isn't necessary as exec.Command will find executables in PATH each time
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
	piAppsDir := getPiAppsDir()

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

	// Create ANSI-stripping writer for log file
	ansiStripLogWriter := NewAnsiStripWriter(logFile)

	// Save original stdout/stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	// Create pipes for capturing output
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	// Redirect os.Stdout and os.Stderr to pipe writers
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	// Create channels to signal completion
	var stdoutDone, stderrDone = make(chan bool), make(chan bool)

	// Copy output to both log file and original stdout/stderr
	go func() {
		io.Copy(io.MultiWriter(ansiStripLogWriter, originalStdout), stdoutReader)
		stdoutDone <- true
	}()
	go func() {
		io.Copy(io.MultiWriter(ansiStripLogWriter, originalStderr), stderrReader)
		stderrDone <- true
	}()

	// Use InstallPackages which has proper error detection
	err = InstallPackages(appName, packages...)

	// Close writers to signal end of output
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait for all output to be copied
	<-stdoutDone
	<-stderrDone

	// Restore original stdout/stderr
	os.Stdout = originalStdout
	os.Stderr = originalStderr

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
	piAppsDir := getPiAppsDir()

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

	fmt.Fprintf(logFile, "Will uninstall these packages: %s\n", strings.Join(packages, " "))

	// Create ANSI-stripping writer for log file
	ansiStripLogWriter := NewAnsiStripWriter(logFile)

	// Save original stdout/stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	// Create pipes for capturing output
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()

	// Redirect os.Stdout and os.Stderr to pipe writers
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	// Create channels to signal completion
	var stdoutDone, stderrDone = make(chan bool), make(chan bool)

	// Copy output to both log file and original stdout/stderr
	go func() {
		io.Copy(io.MultiWriter(ansiStripLogWriter, originalStdout), stdoutReader)
		stdoutDone <- true
	}()
	go func() {
		io.Copy(io.MultiWriter(ansiStripLogWriter, originalStderr), stderrReader)
		stderrDone <- true
	}()

	// Use PurgePackages which has proper error detection
	err = PurgePackages(appName, false)

	// Close writers to signal end of output
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait for all output to be copied
	<-stdoutDone
	<-stderrDone

	// Restore original stdout/stderr
	os.Stdout = originalStdout
	os.Stderr = originalStderr

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
	cmd := exec.Command("sudo", append([]string{"apt-get", "install", "-yf", "--no-install-recommends"}, dependencies...)...)
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
	cmd := exec.Command("sudo", append([]string{"apt-get", "purge", "--autoremove", "-yf"}, dependencies...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall dependencies: %v", err)
	}

	return nil
}
