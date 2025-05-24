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

// Module: api.go
// Description: This module hosts the core functions that all
// other non Go scripts can interact via functions in the Bash API (or plugins via the plugin API).

package api

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
)

// Debug mode flag
var piAppsDebug = false

// Error displays an error message in red and exits the program
func Error(msg string) {
	// Use the exact same ANSI sequence as the original bash script
	fmt.Fprintln(os.Stderr, "\033[91m"+msg+"\033[0m")
	os.Exit(1)
}

// Error displays an error message in red but does not wexit the program
func ErrorNoExit(msg string) {
	// Use the exact same ANSI sequence as the original bash script
	fmt.Fprintln(os.Stderr, "\033[91m"+msg+"\033[0m")
}

// Warning displays a warning message in yellow with a flashing icon
func Warning(msg string) {
	// Use the exact same ANSI sequence as the original bash script
	// \e[93m = yellow, \e[5m = blink, \e[25m = no blink
	fmt.Fprintln(os.Stderr, "\033[93m\033[5m◢◣\033[25m WARNING: "+msg+"\033[0m")
}

// Status displays a status message in cyan
func Status(msg string, args ...string) {
	// Use the exact same ANSI sequence as the original bash script
	if len(args) > 0 && strings.HasPrefix(msg, "-") {
		// Handle flags passed to echo
		fmt.Fprintln(os.Stderr, msg, "\033[96m"+args[0]+"\033[0m")
	} else {
		// Regular status message
		fmt.Fprintln(os.Stderr, "\033[96m"+msg+"\033[0m")
	}
}

// StatusGreen announces the success of a major action in green
func StatusGreen(msg string) {
	// Use the exact same ANSI sequence as the original bash script
	fmt.Fprintln(os.Stderr, "\033[92m"+msg+"\033[0m")
}

// Debug outputs debug information when debug mode is enabled
func Debug(msg string) {
	if piAppsDebug {
		// The original bash script just does a simple echo without any color
		fmt.Println(msg)
	}
}

// SetDebugMode enables or disables debug mode
func SetDebugMode(enabled bool) {
	piAppsDebug = enabled
}

// GenerateLogo displays colorized Pi-Apps logo in terminal
func GenerateLogo() string {
	// Check if Unicode 13 is supported (libicu66+)
	unicodeSupported := checkUnicodeSupport()

	// Exact ANSI color codes from the original bash script
	// Foreground colors
	blue1 := "\033[38;5;75m"
	blue2 := "\033[38;5;26m"
	blue3 := "\033[38;5;21m"
	blue4 := "\033[38;5;93m"

	green := "\033[38;5;46m"
	darkgreen := "\033[38;5;34m"
	red := "\033[38;5;197m"
	white := "\033[97m"
	black := "\033[30m"
	default_ := "\033[39m"

	// Background colors
	bg_default := "\033[49m"
	bg_black := "\033[40m"
	// bg_white := "\033[107m" // Unused in this implementation

	var logoStr string

	if unicodeSupported {
		// Complex logo requires Unicode 13 support (libicu66+)
		// This matches the original bash implementation character-for-character
		bg_black = "\033[48;2;10;10;10m"

		logoStr = bg_default + "    \033[38;2;5;220;75m🭊\033[38;2;4;150;29m🬹🬹🬹\033[38;2;6;188;64m🬿" + default_ + "                                          " + darkgreen + "                " + default_ + "\n" +
			" \033[38;2;83;213;255m🭈🬭\033[38;2;83;214;255m🬭\033[38;2;5;220;75m\033[48;2;83;212;255m🬎" + bg_default + "\033[38;2;84;201;251m🬭\033[38;2;84;190;248m🬭\033[38;2;85;178;244m🬭\033[38;2;6;188;64m\033[48;2;86;168;241m🬎" + bg_default + "\033[38;2;87;154;237m🬭🬭\033[38;2;87;136;231m🬽" + default_ + "                                       " + darkgreen + "                " + default_ + "\n" +
			" \033[38;2;83;213;255m" + bg_black + "▋  \033[38;2;255;38;101m▄ \033[38;2;255;28;92m▄ \033[38;2;255;13;83m▄\033[38;2;89;114;225m  🮉" + bg_default + default_ + "   █▀▀🭍 ▄    🭋🭡🭖🭀                      " + darkgreen + "  " + black + "    " + darkgreen + "    " + black + "    " + darkgreen + "  " + default_ + "\n" +
			" \033[38;2;85;191;249m" + bg_black + "▋  \033[38;2;255;13;85m▄ \033[38;2;255;0;75m▄ \033[38;2;246;0;73m▄\033[38;2;90;83;215m  🮉" + bg_default + default_ + "   █▄▄🭞 ▄ " + blue3 + "▄▄" + default_ + " 🭅▙▟🭐 █▀▀🭍 █▀▀🭍 🭂🬰🬰🬰       " + darkgreen + "  " + black + "    " + darkgreen + "    " + black + "    " + darkgreen + "  " + default_ + "\n" +
			" \033[38;2;86;164;240m" + bg_black + "▋  \033[38;2;249;0;73m▄ \033[38;2;239;0;69m▄ \033[38;2;229;0;66m▄\033[38;2;92;58;207m  🮉" + bg_default + default_ + "   █    █   🭋🭡  🭖🭀█▄▄🭞 █▄▄🭞 ▄▄▄🭞       " + darkgreen + "      " + black + "    " + darkgreen + "      " + default_ + "\n" +
			" \033[38;2;87;137;232m🭕" + bg_black + "🭏\033[38;2;89;111;224m🬭\033[38;2;89;100;220m🬭\033[38;2;90;89;217m🬭\033[38;2;91;76;213m🬭\033[38;2;92;68;211m🬭\033[38;2;92;59;208m🬭\033[38;2;92;56;207m🬭🭄" + bg_default + "🭠" + default_ + "                  █    █               " + darkgreen + "    " + black + "        " + darkgreen + "    " + default_ + "\n" +
			"\033[0m                                                   " + darkgreen + "    " + black + "        " + darkgreen + "    " + default_ + "\n" +
			"                                                   " + darkgreen + "    " + black + "  " + darkgreen + "    " + black + "  " + darkgreen + "    " + default_
	} else {
		// Simple logo for systems without Unicode 13 support
		// This matches the original bash implementation character-for-character
		logoStr = white + bg_default + "    " + green + "▅" + darkgreen + "▅▅▅" + green + "▅" + default_ + "                                          " + darkgreen + "                " + default_ + "\n" +
			" " + blue1 + "▂▂▂" + green + "\033[48;5;26m\033[7m▂\033[27m" + bg_default + blue2 + "▂▂▂" + blue3 + green + "\033[48;5;26m\033[7m▂\033[27m" + bg_default + blue3 + "▂▂▂" + white + default_ + "                                       " + darkgreen + "                " + default_ + "\n" +
			" " + bg_black + blue1 + "▌  " + red + "▄ ▄ ▄" + blue3 + "  ▐" + bg_default + default_ + "   █▀▀◣ ▄    ◢▀▀◣                      " + darkgreen + "  " + black + "    " + darkgreen + "    " + black + "    " + darkgreen + "  " + default_ + "\n" +
			" " + bg_black + blue2 + "▌  " + red + "▄ ▄ ▄" + blue3 + "  ▐" + bg_default + default_ + "   █▄▄◤ ▄ " + blue3 + "▄▄" + default_ + " █▄▄█ █▀▀◣ █▀▀◣ ◢\033[7m━━━\033[27m       " + darkgreen + "  " + black + "    " + darkgreen + "    " + black + "    " + darkgreen + "  " + default_ + "\n" +
			" " + bg_black + blue2 + "▌  " + red + "▄ ▄ ▄" + blue4 + "  ▐" + bg_default + default_ + "   █    █    █  █ █▄▄◤ █▄▄◤ ▄▄▄◤       " + darkgreen + "      " + black + "    " + darkgreen + "      " + default_ + "\n" +
			" " + blue3 + "◥" + bg_black + "▃▃▃▃" + blue4 + "▃▃▃▃▃" + bg_default + "◤" + default_ + "                  █    █               " + darkgreen + "    " + black + "        " + darkgreen + "    " + default_ + "\n" +
			"\033[0m                                                   " + darkgreen + "    " + black + "        " + darkgreen + "    " + default_ + "\n" +
			"                                                   " + darkgreen + "    " + black + "  " + darkgreen + "    " + black + "  " + darkgreen + "    " + default_
	}

	return logoStr + "\n"
}

// checkUnicodeSupport checks if the system supports Unicode 13 (libicu66+)
func checkUnicodeSupport() bool {
	paths := []string{
		"/usr/lib/aarch64-linux-gnu/libicudata.so",
		"/usr/lib/arm-linux-gnueabihf/libicudata.so",
		"/usr/lib/x86_64-linux-gnu/libicudata.so",
		"/usr/lib/i686-linux-gnu/libicudata.so",
		"/usr/lib/riscv64-linux-gnu/libicudata.so",
		"/usr/lib/riscv32-linux-gnu/libicudata.so",
	}

	// Check for direct file existence first
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			// Get the version by following the symlink
			realPath, err := os.Readlink(path)
			if err == nil {
				re := regexp.MustCompile(`libicudata\.so\.(\d+)`)
				matches := re.FindStringSubmatch(realPath)
				if len(matches) > 1 {
					version, err := strconv.Atoi(matches[1])
					if err == nil && version >= 66 {
						return true
					}
				}
			}
		}
	}

	// Check for any matching files using glob patterns
	searchPaths := []string{
		"/usr/lib/aarch64-linux-gnu/libicudata.so.*",
		"/usr/lib/arm-linux-gnueabihf/libicudata.so.*",
		"/usr/lib/x86_64-linux-gnu/libicudata.so.*",
		"/usr/lib/i686-linux-gnu/libicudata.so.*",
		"/usr/lib/riscv64-linux-gnu/libicudata.so.*",
		"/usr/lib/riscv32-linux-gnu/libicudata.so.*",
	}

	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			// Extract version from the first match
			re := regexp.MustCompile(`libicudata\.so\.(\d+)`)
			submatches := re.FindStringSubmatch(matches[0])
			if len(submatches) > 1 {
				version, err := strconv.Atoi(submatches[1])
				if err == nil && version >= 66 {
					return true
				}
			}
		}
	}

	return false
}

// AddEnglish adds en_US locale for more accurate error messages
func AddEnglish() {
	// Check if en_US.UTF-8 is supported
	supported, err := os.ReadFile("/usr/share/i18n/SUPPORTED")
	if err != nil {
		Warning("Could not read /usr/share/i18n/SUPPORTED")
		return
	}

	if !strings.Contains(string(supported), "en_US.UTF-8") {
		Warning("en_US locale is not available on your system. This may cause bad logging experience.")
		return
	}

	// Check if en_US.utf8 locale is generated
	cmd := exec.Command("locale", "-a")
	output, err := cmd.Output()
	if err != nil {
		Warning("Could not check available locales")
		return
	}

	if !strings.Contains(string(output), "en_US.utf8") {
		Status("Adding en_US locale for better logging... ")

		// Uncomment en_US.UTF-8 in /etc/locale.gen
		sedCmd := exec.Command("sudo", "sed", "-i", "/en_US.UTF-8/s/^#[ ]//g", "/etc/locale.gen")
		if err := sedCmd.Run(); err != nil {
			Warning("Failed to edit /etc/locale.gen: " + err.Error())
			return
		}

		// Generate the locale
		genCmd := exec.Command("sudo", "locale-gen")
		if err := genCmd.Run(); err != nil {
			Warning("Failed to generate locale: " + err.Error())
			return
		}
	}

	// Set environment variables
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("LANGUAGE", "en_US.UTF-8")
	os.Setenv("LC_ALL", "en_US.UTF-8")
}

// PackageInfo lists everything dpkg knows about the specified package
func PackageInfo(packageName string) (string, error) {
	if packageName == "" {
		Error("PackageInfo(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// We'll directly use exec.Command to get package info since syspkg doesn't
	// seem to have a direct method for detailed package info
	cmd := exec.Command("dpkg", "-s", packageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get package info: %w", err)
	}

	return string(output), nil
}

// PackageInstalled checks if a package is installed
func PackageInstalled(packageName string) bool {
	if packageName == "" {
		Error("PackageInstalled(): no package specified!")
		return false
	}

	// Use dpkg to check if the package is installed
	cmd := exec.Command("dpkg", "-s", packageName)
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// PackageAvailable determines if the specified package exists in a local repository
func PackageAvailable(packageName string, dpkgArch string) bool {
	if packageName == "" {
		Error("PackageAvailable(): no package name specified!")
		return false
	}

	// If dpkgArch is not specified, get the current architecture
	if dpkgArch == "" {
		cmd := exec.Command("dpkg", "--print-architecture")
		output, err := cmd.Output()
		if err != nil {
			Debug("Error getting dpkg architecture: " + err.Error())
			return false
		}
		dpkgArch = strings.TrimSpace(string(output))
	}

	// Use apt-cache to check if package is available
	cmd := exec.Command("apt-cache", "policy", packageName+":"+dpkgArch)
	output, err := cmd.Output()
	if err != nil {
		Debug("Error checking if package is available: " + err.Error())
		return false
	}

	// Parse the output to see if a candidate version is available
	return !strings.Contains(string(output), "Candidate: (none)")
}

// PackageDependencies outputs the list of dependencies for the specified package
func PackageDependencies(packageName string) ([]string, error) {
	if packageName == "" {
		Error("PackageDependencies(): no package specified!")
		return nil, fmt.Errorf("no package specified")
	}

	// Get package info like the original implementation
	info, err := PackageInfo(packageName)
	if err != nil {
		return nil, err
	}

	// Extract the Depends line from package info
	var deps []string
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "Depends:") {
			// Return the entire dependency line, which includes version requirements
			depLine := strings.TrimSpace(strings.TrimPrefix(line, "Depends:"))
			if depLine != "" {
				return []string{depLine}, nil
			}
			break
		}
	}

	return deps, nil
}

// PackageInstalledVersion returns the installed version of the specified package
func PackageInstalledVersion(packageName string) (string, error) {
	if packageName == "" {
		Error("PackageInstalledVersion(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// Use dpkg to get the installed version
	cmd := exec.Command("dpkg-query", "-W", "-f=${Version}", packageName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("package %s is not installed", packageName)
	}

	return strings.TrimSpace(string(output)), nil
}

// PackageLatestVersion returns the latest available version of the specified package
func PackageLatestVersion(packageName string, repo ...string) (string, error) {
	if packageName == "" {
		Error("PackageLatestVersion(): no package specified!")
		return "", fmt.Errorf("no package specified")
	}

	// Optional repo selection flags
	var additionalFlags []string
	if len(repo) >= 2 && repo[0] == "-t" {
		additionalFlags = []string{"-t", repo[1]}
	}

	// Get the latest version using apt-cache policy
	var cmd *exec.Cmd
	if len(additionalFlags) > 0 {
		cmd = exec.Command("apt-cache", append([]string{"policy"}, append(additionalFlags, packageName)...)...)
	} else {
		cmd = exec.Command("apt-cache", "policy", packageName)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse the output to extract the Candidate version
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Candidate:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}
			version := strings.TrimSpace(parts[1])
			return version, nil
		}
	}

	return "", fmt.Errorf("candidate version not found for package %s", packageName)
}

// PackageIsNewEnough checks if the package has an available version greater than or equal to compareVersion
func PackageIsNewEnough(packageName, compareVersion string) bool {
	if packageName == "" {
		Error("PackageIsNewEnough(): no package specified!")
		return false
	}

	if compareVersion == "" {
		Error("PackageIsNewEnough(): no comparison version number specified!")
		return false
	}

	// Get the latest available version
	packageVersion, err := PackageLatestVersion(packageName)
	if err != nil || packageVersion == "" {
		return false
	}

	// Compare versions using the string comparison
	return compareVersions(packageVersion, compareVersion) >= 0
}

// compareVersions compares two version strings and returns:
// -1 if version1 < version2
//
//	0 if version1 == version2
//	1 if version1 > version2
func compareVersions(version1, version2 string) int {
	// Split versions by dots
	v1Parts := strings.Split(version1, ".")
	v2Parts := strings.Split(version2, ".")

	// Compare each part
	for i := 0; i < len(v1Parts) && i < len(v2Parts); i++ {
		// Extract numeric part
		v1Num := extractNumber(v1Parts[i])
		v2Num := extractNumber(v2Parts[i])

		if v1Num > v2Num {
			return 1
		} else if v1Num < v2Num {
			return -1
		}

		// If numbers are equal, compare the entire strings for this part
		if v1Parts[i] > v2Parts[i] {
			return 1
		} else if v1Parts[i] < v2Parts[i] {
			return -1
		}
	}

	// If we get here, all parts so far were equal
	if len(v1Parts) > len(v2Parts) {
		return 1
	} else if len(v1Parts) < len(v2Parts) {
		return -1
	}

	return 0 // Versions are exactly equal
}

// extractNumber extracts the first numeric part of a string
func extractNumber(s string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}

	num := 0
	fmt.Sscanf(match, "%d", &num)
	return num
}

// DownloadFile downloads a file from URL to destination
func DownloadFile(url, destination string) error {
	if url == "" {
		return fmt.Errorf("no URL specified")
	}
	if destination == "" {
		return fmt.Errorf("no destination specified")
	}

	// Create the destination directory if it doesn't exist
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create a client
	client := grab.NewClient()
	req, err := grab.NewRequest(destination, url)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Start the download
	Status("Downloading " + url)
	resp := client.Do(req)

	// Monitor the download progress
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			progress := resp.Progress()
			fmt.Printf("%.2f%%\n", progress*100)
		case <-resp.Done:
			// Download is complete
			if err := resp.Err(); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
			StatusGreen(fmt.Sprintf("Download completed: %s", destination))
			return nil
		}
	}
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, input, 0644)
	if err != nil {
		return err
	}

	return nil
}

// GetCommandOutput executes a command and returns its output
func GetCommandOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// RunCommand executes a command and returns its exit code
func RunCommand(command string, args ...string) int {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		log.Printf("Failed to execute command: %v", err)
		return 1
	}

	return 0
}

// EnsureDir ensures a directory exists, creating it if necessary
func EnsureDir(path string) error {
	if path == "" {
		return fmt.Errorf("no path specified")
	}

	if DirExists(path) {
		return nil
	}

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// InstallPackage installs a package using apt-get
func InstallPackage(packageName string) error {
	if packageName == "" {
		return fmt.Errorf("no package specified")
	}

	// Use direct command execution for more control
	cmd := exec.Command("sudo", "apt-get", "install", "-y", packageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Status(fmt.Sprintf("Installing package %s", packageName))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install package %s: %w", packageName, err)
	}

	StatusGreen(fmt.Sprintf("Successfully installed package %s", packageName))
	return nil
}

// RemovePackage removes a package using apt-get
func RemovePackage(packageName string) error {
	if packageName == "" {
		return fmt.Errorf("no package specified")
	}

	// Use direct command execution for more control
	cmd := exec.Command("sudo", "apt-get", "remove", "-y", packageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Status(fmt.Sprintf("Removing package %s", packageName))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to remove package %s: %w", packageName, err)
	}

	StatusGreen(fmt.Sprintf("Successfully removed package %s", packageName))
	return nil
}

// UpdatePackages updates package lists
func UpdatePackages() error {
	// Use direct command execution for more control
	cmd := exec.Command("sudo", "apt-get", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Status("Updating package lists")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}

	StatusGreen("Successfully updated package lists")
	return nil
}

// UpgradePackages upgrades all packages
func UpgradePackages() error {
	// Use direct command execution for more control
	cmd := exec.Command("sudo", "apt-get", "upgrade", "-y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Status("Upgrading packages")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to upgrade packages: %w", err)
	}

	StatusGreen("Successfully upgraded packages")
	return nil
}
