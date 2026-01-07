// Copyright (C) 2026 pi-apps-go contributors
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
// Module: dummy_misc.go
// Description: Provides dummy functions for miscellaneous operations that require a package manager if no package manager build tag is set. This also contains strings for dummy related messages.
// SPDX-License-Identifier: GPL-3.0-or-later
//go:build dummy

package api

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// variables for dummy related messages
var (
	MissingInitMessage         = T("Congratulations, Linux tinkerer, you broke your system. The init package can not be found, which means you have removed the default <distro> sources from your system.\nAll <package manager> based application installs will fail. Unless you have a backup of your <required files> you will need to reinstall your OS.")
	PackageManager             = "dummy"
	PackageAppErrorMessage     = T("As this is an <package manager> error, consider Googling the errors.")
	PackageAppNoErrorReporting = T("Error report cannot be sent because this \"app\" is really just a shortcut to install a <distro> package. It's not a problem that Pi-Apps can fix.")
	AdoptiumInstallerMessage   = T("Install Adoptium Java repository - ignored, not supported by dummy")
	LessAptMessage             = T("Format <package manager> output for readability")
	AptLockWaitMessage         = T("Wait for <package manager> lock")
	UbuntuPPAInstallerMessage  = T("Install Ubuntu PPA - ignored, not supported by dummy")
	DebianPPAInstallerMessage  = T("Install Debian PPA - ignored, not supported by dummy")
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
			// assume failure if no package manager build tag is set
			return fmt.Errorf("failed to install shellcheck: no package manager build tag is set")
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

	// return empty string if no package manager build tag is set
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
func checkFrankenDebian(osInfo *SystemOSInfo) (string, error) {
	// assume success if no package manager build tag is set
	return "", nil
}

// checkMissingRepositories checks if important repositories are missing
func checkMissingRepositories(osInfo *SystemOSInfo) (string, error) {
	// assume success if no package manager build tag is set
	return "", nil
}

// checkBrokenPackages checks if there are broken packages in the system
func checkBrokenPackages() (string, error) {
	// assume success if no package manager build tag is set
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
		// no action needed if no package manager build tag is set
		Error(fmt.Sprintf("failed to enable module '%s': no package manager build tag is set", moduleName))
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
	// Show colored status message
	Status(fmt.Sprintf("Installing \033[1m%s\033[22m...", appName))

	// return error if no package manager build tag is set
	return fmt.Errorf("failed to install packages: no package manager build tag is set")
}

// uninstallPackageApp uninstalls a package-based app
func uninstallPackageApp(appName string) error {
	// Show colored status message
	Status(fmt.Sprintf("Uninstalling \033[1m%s\033[22m...", appName))

	// return error if no package manager build tag is set
	return fmt.Errorf("failed to uninstall packages: no package manager build tag is set")
}

// installPackageAppDependencies installs the dependencies for a package-based app without pi-apps having to create a new virtual package
func installPackageAppDependencies(dependencies ...string) error {
	// return error if no package manager build tag is set
	return fmt.Errorf("failed to install dependencies: no package manager build tag is set")
}

// uninstallPackageAppDependencies uninstalls the dependencies for a package-based app
func uninstallPackageAppDependencies(dependencies ...string) error {
	// return error if no package manager build tag is set
	return fmt.Errorf("failed to uninstall dependencies: no package manager build tag is set")
}
