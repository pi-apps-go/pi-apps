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
// Module: apk_misc.go
// Description: Provides functions for miscellaneous operations that require APK. This also contains strings for APK related messages.

//go:build apk

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// variables for APK related messages
var (
	MissingInitMessage     = T("Congratulations, Linux tinkerer, you broke your system. The init package can not be found, which means you have removed the default Alpine sources from your system.\nAll APK based application installs will fail. Unless you have a backup of your /etc/apk/repositories you will need to reinstall your OS.")
	PackageManager         = "apk"
	PackageAppErrorMessage = T("As this is an APK error, consider Googling the errors or asking for help in Alpine Linux forums.")
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
			// Install shellcheck using APK
			cmd := exec.Command("sudo", "apk", "add", "shellcheck")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install shellcheck: %w", err)
			}
		}
	}
	return nil
}

// readPackagesFile reads and parses packages from a packages file
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

	// Try running apk info -L command to list files in the package
	cmd := exec.Command("apk", "info", "-L", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Package not installed or doesn't exist
		return ""
	}

	// Look for icon files in the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == packageName+" contains:" {
			continue
		}

		// APK info -L output is relative paths, so prepend /
		if !strings.HasPrefix(line, "/") {
			line = "/" + line
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

// PipxInstall installs packages using pipx
func PipxInstall(packages ...string) error {
	if len(packages) == 0 {
		return fmt.Errorf("%s", T("no packages specified for pipx installation"))
	}

	// Check Python version
	cmd := exec.Command("python3", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check python3 version: %w", err)
	}
	version := strings.Split(string(output), " ")[1]

	if version >= "3.7" {
		// Check if pipx is available in APK repositories first
		if PackageAvailable("pipx", "") {
			fmt.Println(T("Installing pipx from APK repositories..."))
			cmd := exec.Command("sudo", "apk", "add", "pipx")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf(T("failed to install pipx: %w"), err)
			}
		} else {
			// Fallback to pip installation
			fmt.Println(T("Installing pipx with pip..."))
			cmd := exec.Command("sudo", "-H", "python3", "-m", "pip", "install", "--upgrade", "pipx", "--break-system-packages")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf(T("failed to install pipx with pip: %w"), err)
			}
		}
	} else {
		return fmt.Errorf(T("pipx is not available on your distro and so cannot install %s to python venv"), strings.Join(packages, " "))
	}

	// Verify pipx installation
	fmt.Println(T("Verifying pipx installation..."))
	checkCmd := exec.Command("which", "pipx")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("%s", T("pipx installation failed: command not found after installation"))
	}

	// Install the requested packages
	fmt.Printf(T("Installing %s with pipx...\n"), strings.Join(packages, ", "))

	installCmd := exec.Command("sudo", "-E", "bash", "-c",
		fmt.Sprintf("PIPX_HOME=/usr/local/pipx PIPX_BIN_DIR=/usr/local/bin pipx install %s",
			strings.Join(packages, " ")))

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf(T("failed to install %s with pipx: %w"), strings.Join(packages, " "), err)
	}

	// Upgrade packages
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

	// Uninstall the requested packages
	fmt.Printf(T("Uninstalling %s with pipx...\n"), strings.Join(packages, ", "))

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

// checkFrankenDebian checks if the system has mixed repositories (not applicable for APK)
func checkFrankenDebian(osInfo *SystemOSInfo) (string, error) {
	// APK-based systems don't have the Franken-Debian problem
	return "", nil
}

// checkMissingRepositories checks if important repositories are missing
func checkMissingRepositories(osInfo *SystemOSInfo) (string, error) {
	// Read repositories from multiple locations
	// APK can use:
	// - /etc/apk/repositories (traditional single file)
	// - /etc/apk/repositories.d/ (directory-based, used by Chimera)
	// - /usr/lib/apk/repositories.d/ (system defaults)

	var allRepos []string

	// Read /etc/apk/repositories
	if content, err := os.ReadFile("/etc/apk/repositories"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				allRepos = append(allRepos, line)
			}
		}
	}

	// Read /etc/apk/repositories.d/*.list
	if entries, err := os.ReadDir("/etc/apk/repositories.d"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".list") {
				filePath := filepath.Join("/etc/apk/repositories.d", entry.Name())
				if content, err := os.ReadFile(filePath); err == nil {
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" && !strings.HasPrefix(line, "#") {
							allRepos = append(allRepos, line)
						}
					}
				}
			}
		}
	}

	// Read /usr/lib/apk/repositories.d/*.list
	if entries, err := os.ReadDir("/usr/lib/apk/repositories.d"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".list") {
				filePath := filepath.Join("/usr/lib/apk/repositories.d", entry.Name())
				if content, err := os.ReadFile(filePath); err == nil {
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" && !strings.HasPrefix(line, "#") {
							allRepos = append(allRepos, line)
						}
					}
				}
			}
		}
	}

	// If no repositories are found at all, that's a problem
	if len(allRepos) == 0 {
		return "No APK repositories are configured. Please check /etc/apk/repositories or /etc/apk/repositories.d/", nil
	}

	// Check for standard repositories based on distribution
	hasStandardRepo := false

	// Define standard repo patterns for different APK-based distros
	standardPatterns := []string{
		"dl-cdn.alpinelinux.org",                           // Alpine CDN
		"alpine.global.ssl.fastly.net",                     // Alpine Fastly
		"mirrors.alpinelinux.org",                          // Alpine mirrors
		"repo.chimera-linux.org",                           // Chimera official
		"https://chimera.sakamoto.pl",                      // Chimera Polish mirror
		"https://au.mirror.7bit.org/chimera",               // Chimera Australia mirror
		"https://mirror.accum.se/mirror/chimera-linux.org", // Chimera Sweden mirror
		"https://mirror.meowsmp.net/chimera-linux",         // Chimera Vietnam mirror
	}

	for _, repo := range allRepos {
		for _, pattern := range standardPatterns {
			if strings.Contains(repo, pattern) {
				hasStandardRepo = true
				break
			}
		}
		if hasStandardRepo {
			break
		}
	}

	// If no standard repositories are found, warn the user
	if !hasStandardRepo {
		return "Warning: No standard APK repositories detected. System updates may not work correctly.", nil
	}

	return "", nil
}

// checkBrokenPackages checks if there are broken packages in the system
func checkBrokenPackages() (string, error) {
	// APK uses 'apk fix' to repair/check packages
	// We'll run it in simulate mode to check without making changes

	// First, try to run apk fix --simulate (if supported)
	cmd := exec.Command("apk", "fix", "--simulate")
	output, err := cmd.CombinedOutput()

	// Check the output for any issues
	outputStr := string(output)

	// If apk fix reports any packages that need fixing, that's a problem
	if err != nil || strings.Contains(outputStr, "ERROR") ||
		strings.Contains(outputStr, "reinstalling") ||
		strings.Contains(outputStr, "Installing") ||
		strings.Contains(outputStr, "Upgrading") {
		// There are broken packages

		// Try to get more details with apk audit
		auditCmd := exec.Command("apk", "audit", "--check-permissions")
		auditOutput, _ := auditCmd.CombinedOutput()

		var message strings.Builder
		message.WriteString("APK has detected broken or missing packages on your system.\n")
		message.WriteString("You can try to fix them by running:\n")
		message.WriteString("  sudo apk fix\n\n")

		if len(outputStr) > 0 {
			message.WriteString("APK fix output:\n")
			message.WriteString(outputStr)
		}

		if len(auditOutput) > 0 {
			message.WriteString("\nAPK audit output:\n")
			message.WriteString(string(auditOutput))
		}

		return message.String(), nil
	}

	// Also check for packages with missing files using apk audit
	// Only report ACTUAL problems (missing files marked with 'M'), not normal system differences
	auditCmd := exec.Command("apk", "audit", "--check-permissions")
	auditOutput, err := auditCmd.CombinedOutput()
	if err == nil && len(auditOutput) > 0 {
		// Filter to only actual problems (missing files)
		auditStr := string(auditOutput)
		lines := strings.Split(auditStr, "\n")
		var issues []string

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "OK:") {
				continue
			}

			// APK audit output format:
			// M = Missing file (ACTUAL PROBLEM - file should exist but doesn't)
			// A = Added file (NORMAL - user created files)
			// U = Updated file (NORMAL - modified config files)
			// D = Directory (NORMAL - directories exist)
			// e = Modified file (NORMAL - edited configs)
			// X = Changed permission (NORMAL - permission changes)

			// Only report MISSING files as actual problems
			if strings.HasPrefix(line, "M ") {
				issues = append(issues, line)
			}
		}

		if len(issues) > 0 {
			var message strings.Builder
			message.WriteString("APK audit has detected missing files from installed packages:\n\n")
			message.WriteString(strings.Join(issues, "\n"))
			message.WriteString("\n\nYou can try to fix them by running:\n")
			message.WriteString("  sudo apk fix --reinstall\n")
			return message.String(), nil
		}
	}

	// No broken packages detected
	return "", nil
}

// EnableModule ensures a kernel module is loaded and configured to load on system startup
func EnableModule(moduleName string) error {
	if moduleName == "" {
		return fmt.Errorf("module name must be specified")
	}

	// Special handling for the fuse module
	if moduleName == "fuse" {
		// Check if fuse is available as a package
		if PackageAvailable("fuse", "") {
			cmd := exec.Command("sudo", "apk", "add", "fuse")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				Error(fmt.Sprintf("failed to install fuse: %v", err))
			}
		}
	}

	// Ensure kmod is installed
	if !commandExists("kmod") {
		// Try to install kmod
		if PackageAvailable("kmod", "") {
			cmd := exec.Command("sudo", "apk", "add", "kmod")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("kmod is not installed and cannot be installed: %w", err)
			}
		} else {
			return fmt.Errorf("kmod is not installed: command not found")
		}
	}

	// Check if module is builtin
	cmd := exec.Command("modinfo", "--filename", moduleName)
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "(builtin)" {
		return nil
	}

	// Check if module is already loaded
	sysModulePath := fmt.Sprintf("/sys/module/%s", moduleName)
	if _, err := os.Stat(sysModulePath); os.IsNotExist(err) {
		// Load the module
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

	// Make it load on boot
	procModulesPath := "/proc/modules"
	moduleConfPath := fmt.Sprintf("/etc/modules-load.d/%s.conf", moduleName)

	if _, err := os.Stat(procModulesPath); err == nil {
		if _, err := os.Stat(moduleConfPath); os.IsNotExist(err) {
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
	Status(fmt.Sprintf("Installing \033[1m%s\033[22m...", appName))

	packageListPath := filepath.Join(getPiAppsDir(), "apps", appName, "packages")

	// Read packages list
	packageListBytes, err := os.ReadFile(packageListPath)
	if err != nil {
		return fmt.Errorf("failed to read packages list: %v", err)
	}

	packageList := strings.TrimSpace(string(packageListBytes))
	packages := strings.Fields(packageList)

	if len(packages) == 0 {
		return fmt.Errorf("no packages specified in %s", packageListPath)
	}

	// Install packages with sudo
	cmd := exec.Command("sudo", append([]string{"apk", "add", "--no-interactive"}, packages...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install packages: %v", err)
	}

	Status(fmt.Sprintf("\033[1m%s\033[22m installed successfully", appName))
	return nil
}

// uninstallPackageApp uninstalls a package-based app
func uninstallPackageApp(appName string) error {
	Status(fmt.Sprintf("Uninstalling \033[1m%s\033[22m...", appName))

	packageListPath := filepath.Join(getPiAppsDir(), "apps", appName, "packages")

	// Read packages list
	packageListBytes, err := os.ReadFile(packageListPath)
	if err != nil {
		return fmt.Errorf("failed to read packages list: %v", err)
	}

	packageList := strings.TrimSpace(string(packageListBytes))
	packages := strings.Fields(packageList)

	if len(packages) == 0 {
		return fmt.Errorf("no packages specified in %s", packageListPath)
	}

	// Uninstall packages with sudo
	cmd := exec.Command("sudo", append([]string{"apk", "del", "--no-interactive"}, packages...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall packages: %v", err)
	}

	Status(fmt.Sprintf("\033[1m%s\033[22m uninstalled successfully", appName))
	return nil
}

// installPackageAppDependencies installs the dependencies for a package-based app without pi-apps having to create a new virtual package
func installPackageAppDependencies(dependencies ...string) error {
	// Install packages with sudo
	cmd := exec.Command("sudo", append([]string{"apk", "add", "--no-interactive"}, dependencies...)...)
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
	cmd := exec.Command("sudo", append([]string{"apk", "del", "--no-interactive"}, dependencies...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall dependencies: %v", err)
	}

	return nil
}
