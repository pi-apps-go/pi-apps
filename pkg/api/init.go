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

// Module: init.go
// Description: Provides functions for initializing the API.

package api

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Global variables for system information
	PIAppsDir      string
	IsArm64        bool
	IsArm32        bool
	IsX86_64       bool
	IsX86_32       bool
	HostSystemArch string
	HostSystemID   string
	HostSystemDesc string
	HostSystemVer  string
	HostSystemCode string
	OrigSystemID   string
	OrigSystemDesc string
	OrigSystemVer  string
	OrigSystemCode string
	CPUOpModes     string
	CPUOpMode32    bool
	CPUOpMode64    bool
	GTKTheme       string
	GDKBackend     string
)

// Init intializes enviroment variables required for Pi-Apps Go to function.
//
// It should be called automatically in Pi-Apps Go related programs but in other cases (like when using the API in a Go program) it is required to call this function manually.
func Init() {
	// Initialize Pi-Apps directory
	initPiAppsDir()

	// Set GTK theme for GUI components
	initGUITheme()

	// Determine system architecture
	initSystemArch()

	// Get system information
	initSystemInfo()

	// Set GDK backend to X11 for better compatibility across desktop environments
	// (This is still useful for GTK3 regardless of whether we use YAD)
	os.Setenv("GDK_BACKEND", "x11")
	GDKBackend = "x11"

	// Add /usr/local/bin to PATH if not already present
	addUsrLocalBinToPath()
}

// initPiAppsDir determines and sets the Pi-Apps directory location
func initPiAppsDir() {
	// Check if PI_APPS_DIR environment variable is already set
	piappsDir := os.Getenv("PI_APPS_DIR")
	if piappsDir != "" {
		PIAppsDir = piappsDir
		return
	}

	// Try to determine the directory based on the executable location
	// This approach mimics the bash script behavior
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		// If this is being run from the pi-apps/go-rewrite directory,
		// go up one level to get the pi-apps directory
		if strings.HasSuffix(exeDir, "/pi-apps-go/bin") {
			PIAppsDir = filepath.Dir(filepath.Dir(exeDir))
		} else {
			// Otherwise assume the current directory is the pi-apps directory
			PIAppsDir = exeDir
		}
	}

	// If we still don't have a valid directory, use the default
	if PIAppsDir == "" || !isValidPiAppsDir(PIAppsDir) {
		homeDir, _ := os.UserHomeDir()
		PIAppsDir = filepath.Join(homeDir, "pi-apps")
	}

	// Set PI_APPS_DIR environment variable
	os.Setenv("PI_APPS_DIR", PIAppsDir)
}

// isValidPiAppsDir checks if a directory is a valid Pi-Apps directory
func isValidPiAppsDir(dir string) bool {
	if dir == "" {
		return false
	}

	// Check if the directory exists and has the expected files
	apiFile := filepath.Join(dir, "api")
	guiFile := filepath.Join(dir, "gui")
	return DirExists(dir) && FileExists(apiFile) && FileExists(guiFile)
}

// initGUITheme sets the GTK theme for GUI components based on the App List Style setting
func initGUITheme() {
	if PIAppsDir == "" {
		return
	}

	// Read the App List Style setting
	settingsFile := filepath.Join(PIAppsDir, "data", "settings", "App List Style")
	guiMode := "default"

	if FileExists(settingsFile) {
		content, err := os.ReadFile(settingsFile)
		if err == nil {
			guiMode = strings.TrimSpace(string(content))
		}
	}

	// Set the GTK theme based on the setting
	// This is now more generic and not YAD-specific
	switch {
	case strings.HasPrefix(guiMode, "yad-"):
		// Extract theme name without the yad- prefix
		themeValue := strings.TrimPrefix(guiMode, "yad-")
		if themeValue != "default" {
			os.Setenv("GTK_THEME", themeValue)
			GTKTheme = themeValue
		}
	case guiMode == "xlunch-dark-3d", guiMode == "xlunch-dark":
		os.Setenv("GTK_THEME", "Adwaita-dark")
		GTKTheme = "Adwaita-dark"
	default:
		// For light themes or default, don't set any specific theme
		os.Setenv("GTK_THEME", "")
		GTKTheme = ""
	}
}

// initSystemArch determines the system architecture (ARM, x86, 32-bit or 64-bit)
func initSystemArch() {
	// First try using uname to get architecture information
	output, err := exec.Command("uname", "-m").Output()
	if err == nil {
		arch := strings.TrimSpace(string(output))

		// Check for various architectures
		if strings.Contains(strings.ToLower(arch), "arm64") || strings.Contains(strings.ToLower(arch), "aarch64") {
			IsArm64 = true
			HostSystemArch = "arm64"
		} else if strings.Contains(strings.ToLower(arch), "armv7") || strings.Contains(strings.ToLower(arch), "armhf") {
			IsArm32 = true
			HostSystemArch = "armhf"
		} else if strings.Contains(strings.ToLower(arch), "x86_64") || strings.Contains(strings.ToLower(arch), "amd64") {
			IsX86_64 = true
			HostSystemArch = "amd64"
		} else if strings.Contains(strings.ToLower(arch), "i386") || strings.Contains(strings.ToLower(arch), "i686") {
			IsX86_32 = true
			HostSystemArch = "i386"
		} else if strings.Contains(strings.ToLower(arch), "arm") {
			// Generic ARM detection as fallback
			IsArm32 = true
			HostSystemArch = "arm"
		}

		// Set environment variables for architecture
		os.Setenv("HOST_ARCH", HostSystemArch)
		return
	}

	// If uname fails, try checking /sbin/init as a fallback for ARM systems
	initPath, err := exec.LookPath("/sbin/init")
	if err != nil {
		return
	}

	initPath, err = filepath.EvalSymlinks(initPath)
	if err != nil {
		return
	}

	// Read the 5th byte of the file to determine architecture
	file, err := os.Open(initPath)
	if err != nil {
		return
	}
	defer file.Close()

	buf := make([]byte, 5)
	_, err = file.Read(buf)
	if err != nil {
		return
	}

	switch buf[4] {
	case 0x02:
		IsArm64 = true
		HostSystemArch = "arm64"
	case 0x01:
		IsArm32 = true
		HostSystemArch = "armhf"
	}
}

// CommandExists is a helper function that checks if a command is available in the system, return true if yes, return false if no
func CommandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// initSystemInfo gets system information using lsb_release
func initSystemInfo() {
	// Check
	if !CommandExists("lsb_release") {
		Status("Installing lsb_release, please wait...")
		runCommand("sudo", "apt", "install", "-y", "lsb-release")
	}

	// Check for upstream release first (Ubuntu derivatives)
	output, err := exec.Command("lsb_release", "-a", "-u").CombinedOutput()
	if err == nil && !bytes.Contains(output, []byte("command not found")) {
		// This is a Ubuntu Derivative
		re := regexp.MustCompile(`Distributor ID:\s+(.*)\nDescription:\s+(.*)\nRelease:\s+(.*)\nCodename:\s+(.*)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) >= 5 {
			HostSystemID = matches[1]
			HostSystemDesc = matches[2]
			HostSystemVer = matches[3]
			HostSystemCode = matches[4]

			// Now get the original info
			output, err = exec.Command("lsb_release", "-a").CombinedOutput()
			if err == nil {
				matches = re.FindStringSubmatch(string(output))
				if len(matches) >= 5 {
					OrigSystemID = matches[1]
					OrigSystemDesc = matches[2]
					OrigSystemVer = matches[3]
					OrigSystemCode = matches[4]
				}
			}
		}
	} else if FileExists("/etc/upstream-release/lsb-release") {
		// Ubuntu 22.04+ Linux Mint no longer includes the lsb_release -u option
		content, err := os.ReadFile("/etc/upstream-release/lsb-release")
		if err == nil {
			idRe := regexp.MustCompile(`DISTRIB_ID=(.*)`)
			descRe := regexp.MustCompile(`DISTRIB_DESCRIPTION=(.*)`)
			verRe := regexp.MustCompile(`DISTRIB_RELEASE=(.*)`)
			codeRe := regexp.MustCompile(`DISTRIB_CODENAME=(.*)`)

			if matches := idRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemID = strings.Trim(matches[1], "\"")
			}
			if matches := descRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemDesc = strings.Trim(matches[1], "\"")
			}
			if matches := verRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemVer = strings.Trim(matches[1], "\"")
			}
			if matches := codeRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemCode = strings.Trim(matches[1], "\"")
			}

			// Now get the original info
			output, err = exec.Command("lsb_release", "-a").CombinedOutput()
			if err == nil {
				re := regexp.MustCompile(`Distributor ID:\s+(.*)\nDescription:\s+(.*)\nRelease:\s+(.*)\nCodename:\s+(.*)`)
				matches := re.FindStringSubmatch(string(output))
				if len(matches) >= 5 {
					OrigSystemID = matches[1]
					OrigSystemDesc = matches[2]
					OrigSystemVer = matches[3]
					OrigSystemCode = matches[4]
				}
			}
		}
	} else if FileExists("/etc/lsb-release.diverted") {
		// Ubuntu 22.04+ Pop!_OS uses a different file
		content, err := os.ReadFile("/etc/lsb-release.diverted")
		if err == nil {
			idRe := regexp.MustCompile(`DISTRIB_ID=(.*)`)
			descRe := regexp.MustCompile(`DISTRIB_DESCRIPTION=(.*)`)
			verRe := regexp.MustCompile(`DISTRIB_RELEASE=(.*)`)
			codeRe := regexp.MustCompile(`DISTRIB_CODENAME=(.*)`)

			if matches := idRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemID = strings.Trim(matches[1], "\"")
			}
			if matches := descRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemDesc = strings.Trim(matches[1], "\"")
			}
			if matches := verRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemVer = strings.Trim(matches[1], "\"")
			}
			if matches := codeRe.FindStringSubmatch(string(content)); len(matches) > 1 {
				HostSystemCode = strings.Trim(matches[1], "\"")
			}

			// Now get the original info
			output, err = exec.Command("lsb_release", "-a").CombinedOutput()
			if err == nil {
				re := regexp.MustCompile(`Distributor ID:\s+(.*)\nDescription:\s+(.*)\nRelease:\s+(.*)\nCodename:\s+(.*)`)
				matches := re.FindStringSubmatch(string(output))
				if len(matches) >= 5 {
					OrigSystemID = matches[1]
					OrigSystemDesc = matches[2]
					OrigSystemVer = matches[3]
					OrigSystemCode = matches[4]
				}
			}
		}
	} else {
		// Regular system, not a derivative
		output, err := exec.Command("lsb_release", "-a").CombinedOutput()
		if err == nil {
			re := regexp.MustCompile(`Distributor ID:\s+(.*)\nDescription:\s+(.*)\nRelease:\s+(.*)\nCodename:\s+(.*)`)
			matches := re.FindStringSubmatch(string(output))
			if len(matches) >= 5 {
				HostSystemID = matches[1]
				HostSystemDesc = matches[2]
				HostSystemVer = matches[3]
				HostSystemCode = matches[4]
			}
		}
	}

	// Set environment variables for compatibility with shell scripts
	os.Setenv("__os_id", HostSystemID)
	os.Setenv("__os_desc", HostSystemDesc)
	os.Setenv("__os_release", HostSystemVer)
	os.Setenv("__os_codename", HostSystemCode)

	if OrigSystemID != "" {
		os.Setenv("__os_original_id", OrigSystemID)
		os.Setenv("__os_original_desc", OrigSystemDesc)
		os.Setenv("__os_original_release", OrigSystemVer)
		os.Setenv("__os_original_codename", OrigSystemCode)
	}

	// Get CPU operation modes
	initCPUOpModes()
}

// initCPUOpModes determines CPU operation modes (32-bit, 64-bit)
func initCPUOpModes() {
	output, err := exec.Command("lscpu").CombinedOutput()
	if err != nil {
		return
	}

	re := regexp.MustCompile(`CPU op-mode\(s\):\s+(.*)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return
	}

	opModes := matches[1]
	if strings.Contains(opModes, "32-bit, 64-bit") {
		CPUOpModes = "32/64"
		CPUOpMode32 = true
		CPUOpMode64 = true
	} else if strings.Contains(opModes, "32-bit") {
		CPUOpModes = "32"
		CPUOpMode32 = true
	} else if strings.Contains(opModes, "64-bit") {
		CPUOpModes = "64"
		CPUOpMode64 = true
	}

	// Set environment variables for compatibility with shell scripts
	os.Setenv("__cpu_op_modes", CPUOpModes)
	if CPUOpMode32 {
		os.Setenv("__cpu_op_mode_32", "true")
	}
	if CPUOpMode64 {
		os.Setenv("__cpu_op_mode_64", "true")
	}
}

// addUsrLocalBinToPath adds /usr/local/bin to PATH if not already present
func addUsrLocalBinToPath() {
	currentPath := os.Getenv("PATH")
	if !strings.Contains(currentPath, "/usr/local/bin") {
		newPath := "/usr/local/bin:" + currentPath
		os.Setenv("PATH", newPath)
	}
}

// GetPiAppsDir returns the Pi-Apps directory path
func GetPiAppsDir() string {
	// First check if the environment variable is set
	if dir := os.Getenv("PI_APPS_DIR"); dir != "" && isValidPiAppsDir(dir) {
		return dir
	}

	// Check the pi-apps directory path (old folder name for Bash based implementation)
	piAppsPath := filepath.Join(os.Getenv("HOME"), "pi-apps")
	if isValidPiAppsDir(piAppsPath) {
		return piAppsPath
	}

	// Check the pi-apps-go directory path (new folder name for Go based implementation)
	piAppsPath = filepath.Join(os.Getenv("HOME"), "pi-apps-go")
	if isValidPiAppsDir(piAppsPath) {
		return piAppsPath
	}

	// Fall back to the original path
	return PIAppsDir
}

// SetPiAppsDir allows manually setting the Pi-Apps directory
// This is useful for testing or when the detection mechanism doesn't work
func SetPiAppsDir(dir string) {
	if dir != "" && isValidPiAppsDir(dir) {
		PIAppsDir = dir
		os.Setenv("PI_APPS_DIR", dir)
	} else {
		Warning(fmt.Sprintf("Failed to set Pi-Apps directory to %s (not a valid directory)\n", dir))
	}
}
