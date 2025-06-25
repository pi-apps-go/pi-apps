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

// Module: system.go
// Description: Provides functions for system related tasks. (like detecting if a system is supported)

package api

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SystemSupportStatus contains detailed information about the system support status
type SystemSupportStatus struct {
	IsSupported bool
	Message     string
	OSInfo      *SystemOSInfo
}

// SystemOSInfo contains information about the operating system
type SystemOSInfo struct {
	ID           string // Debian, Ubuntu, Raspbian, etc.
	OriginalID   string // For derivative distributions
	Release      string // Version number (11, 22.04, etc.)
	Codename     string // bullseye, jammy, etc.
	Description  string // Full OS description
	PrettyName   string // User-friendly name
	Architecture string // arm64, armhf, amd64, etc.
}

// IsSystemSupported checks if the current system is supported by Pi-Apps
//
// # It returns a status object containing information about support status
//
// If the system is not supported, the message field will contain the reason
//
//	IsSupported - is the current system supported or not (true - supported, otherwaise false)
//	Message - A message explaining in the current state if the system is supported or not
func IsSystemSupported() (*SystemSupportStatus, error) {
	status := &SystemSupportStatus{
		IsSupported: true,
		Message:     "",
	}

	// Get OS information
	osInfo, err := getSystemOSInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get OS information: %w", err)
	}
	status.OSInfo = osInfo

	// Check if running as root
	if os.Geteuid() == 0 {
		status.IsSupported = false
		status.Message = "Pi-Apps is not designed to be run as root user."
		return status, nil
	}

	// Check for x86 architecture
	if strings.HasPrefix(runtime.GOARCH, "386") || strings.HasPrefix(runtime.GOARCH, "amd64") {
		// We're adding x86 support, so we'll just show a warning but not mark as unsupported
		status.Message = "Running on x86 architecture. ARM-specific apps will be hidden from the app list."
	}

	// Check for riscv64 architecture
	if strings.HasPrefix(runtime.GOARCH, "riscv64") {
		// We're adding riscv64 support in the future, so we'll just show a warning but not mark as unsupported
		Warning("You are running on riscv64 architecture. Pi-Apps Go is not yet to be confirmed to be supported on this architecture due to lack of hardware to test on.\nTo help us test, please report any issues you encounter while running Pi-Apps Go on this architecture by reporting an issue on the Pi-Apps Go GitHub repository/Discord server or consider donating to the project to fund RISC-V hardware.")
	}

	// Check for non-glibc C library (like musl)
	// Note: This check is currently being marked as supported as there are plans for Alpine Linux to be supported in Pi-Apps Go.
	if isMuslSystem() {
		//status.IsSupported = false
		Warning("While Pi-Apps Go (and the Go ecosystem in general) is meant to be portable, you are running a system with non-glibc C library (like musl). Many apps, especially Electron-based ones, will fail to run properly without a glibc-based compatibility layer. Pi-Apps will automatically hide apps that are proven to be broken on non-glibc systems even with a glibc compatiblity layer.")
		//return status, nil
	}

	// Check for Android environment
	// Note: This check will dissapear once Pi-Apps Go will be proven portable and tested on Android.
	if isAndroidSystem() {
		status.IsSupported = false
		status.Message = "Pi-Apps is not supported on Android. Some apps will work, but others won't."
		return status, nil
	}

	// Check for Windows Subsystem for Linux (WSL)
	if isWSLSystem() {
		status.IsSupported = false
		status.Message = "Pi-Apps is not supported on WSL."
		return status, nil
	}

	// Check for BusyBox commands
	// Pi-Apps Go does not use any shell commands because this is a rewrite, so checking for BusyBox commands is not needed.
	// TODO: Remove the check for BusyBox commands once Pi-Apps Go ditches the use of shell specific commands.
	if busyboxIssue := checkBusyBoxIssue(); busyboxIssue != "" {
		status.IsSupported = false
		status.Message = busyboxIssue
		return status, nil
	}

	// Check OS version
	if versionMessage := checkOSVersion(osInfo); versionMessage != "" {
		status.IsSupported = false
		status.Message = versionMessage
		return status, nil
	}

	// Check for FrankenDebian
	if isDebian := osInfo.ID == "Debian" || osInfo.ID == "Raspbian"; isDebian {
		frankenDebianMsg, err := checkFrankenDebian(osInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to check for FrankenDebian: %w", err)
		}
		if frankenDebianMsg != "" {
			status.IsSupported = false
			status.Message = frankenDebianMsg
			return status, nil
		}
	}

	// Check for missing init package
	initAvailable := PackageAvailable("init", "")
	if !initAvailable {
		status.IsSupported = false
		status.Message = "Congratulations, Linux tinkerer, you broke your system. The init package can not be found, which means you have removed the default debian sources from your system.\nAll apt based application installs will fail. Unless you have a backup of your /etc/apt/sources.list /etc/apt/sources.list.d you will need to reinstall your OS."
		return status, nil
	}

	// Check for missing repositories
	repoMsg, err := checkMissingRepositories(osInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to check for missing repositories: %w", err)
	}
	if repoMsg != "" {
		status.IsSupported = false
		status.Message = repoMsg
		return status, nil
	}

	// Check for broken packages
	broken, err := checkBrokenPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to check for broken packages: %w", err)
	}
	if broken != "" {
		status.IsSupported = false
		status.Message = broken
		return status, nil
	}

	// Check disk space
	const minDiskSpace = 500 * 1024 * 1024 // 500 MB
	freeSpace, err := getFreeSpace("/")
	if err != nil {
		return nil, fmt.Errorf("failed to check free disk space: %w", err)
	}
	if freeSpace < minDiskSpace {
		status.Message = "Your system drive has less than 500MB of free space. Watch out for \"disk full\" errors."
	}

	return status, nil
}

// IsSupportedSystem is a simplified version of IsSystemSupported that returns a boolean
// indicating whether the system is supported, along with a message explaining why if it isn't
func IsSupportedSystem() (bool, string) {
	status, err := IsSystemSupported()
	if err != nil {
		return false, fmt.Sprintf("Failed to check system compatibility: %v", err)
	}
	return status.IsSupported, status.Message
}

// getSystemOSInfo retrieves information about the operating system from /etc/os-release
func getSystemOSInfo() (*SystemOSInfo, error) {
	osInfo := &SystemOSInfo{
		Architecture: runtime.GOARCH,
	}

	// Read /etc/os-release file
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("failed to open /etc/os-release: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "\"")

		switch key {
		case "ID":
			osInfo.ID = value
		case "VERSION_ID":
			osInfo.Release = value
		case "VERSION_CODENAME":
			osInfo.Codename = value
		case "PRETTY_NAME":
			osInfo.PrettyName = value
		case "ORIGINAL_ID":
			osInfo.OriginalID = value
		case "DESCRIPTION":
			osInfo.Description = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading /etc/os-release: %w", err)
	}

	// If architecture is not set, try to determine it
	if osInfo.Architecture == "" {
		cmd := exec.Command("uname", "-m")
		output, err := cmd.Output()
		if err == nil {
			osInfo.Architecture = strings.TrimSpace(string(output))
		}
	}

	return osInfo, nil
}

// isAndroidSystem checks if the system is running on Android
func isAndroidSystem() bool {
	// Check for Android mounts
	if FileExists("/proc/mounts") {
		content, err := os.ReadFile("/proc/mounts")
		if err == nil && strings.Contains(string(content), "/data/media ") && strings.Contains(string(content), "Android") {
			return true
		}
	}

	// Check kernel version for Android
	if FileExists("/proc/version") {
		content, err := os.ReadFile("/proc/version")
		if err == nil && (strings.Contains(strings.ToLower(string(content)), "android") || strings.Contains(strings.ToLower(string(content)), "termux")) {
			return true
		}
	}

	// Check for Android directories
	if DirExists("/system/app/") && DirExists("/system/priv-app") {
		return true
	}

	return false
}

// isWSLSystem checks if the system is running on Windows Subsystem for Linux
func isWSLSystem() bool {
	// Check kernel version for Microsoft
	if FileExists("/proc/version") {
		content, err := os.ReadFile("/proc/version")
		if err == nil && strings.Contains(strings.ToLower(string(content)), "microsoft") {
			return true
		}
	}

	// Check kernel release for WSL
	if FileExists("/proc/sys/kernel/osrelease") {
		content, err := os.ReadFile("/proc/sys/kernel/osrelease")
		if err == nil && strings.Contains(strings.ToLower(string(content)), "wsl") {
			return true
		}
	}

	// Check for WSL files
	if FileExists("/run/WSL") || FileExists("/etc/wsl.conf") {
		return true
	}

	// Check for WSL environment variable
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}

	return false
}

// checkBusyBoxIssue checks if the system has BusyBox commands that could cause issues
// Note: The Go based rewrite does not heavily depend on shell commands like date or ps unlike the original.
// this check should be removed as Alpine Linux uses busybox for base userspace and we are going to support it
func checkBusyBoxIssue() string {
	dateHelp, err := exec.Command("date", "--help").CombinedOutput()
	if err == nil && strings.HasPrefix(string(dateHelp), "BusyBox") {
		dateCmd, err := exec.LookPath("date")
		if err == nil && dateCmd != "/usr/bin/date" {
			return fmt.Sprintf("Your system has BusyBox commands overriding your main distro's commands. The BusyBox versions of ps, grep, date, and many other commands are missing options that Pi-Apps relies on.\nYou must fix this issue. Take a look at the directory the commands are stored in, and either remove it or rename it: %s", filepath.Dir(dateCmd))
		}
		return "Your system has BusyBox commands in place of the expected linux commands. ps, grep, date, and many other commands are missing options that Pi-Apps relies on.\nYou must fix this problem before Pi-Apps can function correctly."
	}

	psHelp, err := exec.Command("ps", "--help").CombinedOutput()
	if err == nil && strings.HasPrefix(string(psHelp), "BusyBox") {
		psCmd, err := exec.LookPath("ps")
		if err == nil && psCmd != "/usr/bin/ps" {
			return fmt.Sprintf("Your system has BusyBox commands overriding your main distro's commands. The BusyBox versions of ps, grep, date, and many other commands are missing options that Pi-Apps relies on.\nYou must fix this issue. Take a look at the directory the commands are stored in, and either remove it or rename it: %s", filepath.Dir(psCmd))
		}
		return "Your system has BusyBox commands in place of the expected linux commands. ps, grep, date, and many other commands are missing options that Pi-Apps relies on.\nYou must fix this problem before Pi-Apps can function correctly."
	}

	return ""
}

// checkOSVersion checks if the OS version is supported
func checkOSVersion(osInfo *SystemOSInfo) string {
	if osInfo == nil {
		return "Failed to determine OS information"
	}

	// Check for Pi OS Buster (Debian 10)
	if (osInfo.ID == "Debian" || osInfo.ID == "Raspbian") && osInfo.Release == "10" && FileExists("/etc/rpi-issue") {
		return "Pi-Apps is no longer supported on your Pi OS Buster operating system. Consider installing Pi OS Bookworm. https://www.raspberrypi.com/news/bookworm-the-new-version-of-raspberry-pi-os/"
	}

	// Check for Debian/Raspbian EOL status
	if osInfo.ID == "Debian" || osInfo.ID == "Raspbian" {
		// Get EOL info from endoflife.date API
		resp, err := http.Get("https://endoflife.date/api/debian.json")
		if err != nil {
			Warning("Failed to check Debian EOL status: " + err.Error())
		} else {
			defer resp.Body.Close()
			var debianReleases []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&debianReleases); err != nil {
				Warning("Failed to parse Debian EOL data: " + err.Error())
			} else {
				for _, release := range debianReleases {
					switch cycle := release["cycle"].(type) {
					case string:
						if cycle == osInfo.Release {
							switch extendedSupport := release["extendedSupport"].(type) {
							case string:
								// Check if the release is EOL
								eolDate, err := time.Parse("2006-01-02", extendedSupport)
								if err == nil {
									// Check if EOL date is within 4 months
									fourMonthsFromNow := time.Now().AddDate(0, 4, 0)
									oneMonthAfterEOL := eolDate.AddDate(0, 1, 0)

									if time.Now().After(eolDate) && time.Now().Before(oneMonthAfterEOL) {
										Warning(fmt.Sprintf("Your %s %s operating system reached end-of-life on %s. Please upgrade your system before Pi-Apps becomes unsupported.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), extendedSupport))
										return fmt.Sprintf("Your %s %s operating system reached end-of-life on %s. Please upgrade your system before Pi-Apps becomes unsupported.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), extendedSupport)
									} else if time.Now().After(oneMonthAfterEOL) {
										return fmt.Sprintf("Pi-Apps is not supported on your outdated %s %s operating system (EOL: %s). Expect apps to slowly fail overtime. Consider installing a newer operating system.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), extendedSupport)
									} else if time.Now().Before(eolDate) && time.Now().After(fourMonthsFromNow) {
										Warning(fmt.Sprintf("Your %s %s operating system will reach end-of-life on %s. Please consider upgrading your system.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), extendedSupport))
									}
								}
							}
							break
						}
					}
				}
			}
		}

		// Fallback to static check if API fails
		releaseNum, err := strconv.Atoi(osInfo.Release)
		if err == nil && releaseNum < 11 {
			return fmt.Sprintf("Pi-Apps is not supported on your outdated %s %s operating system. Expect many apps to fail. Consider installing a newer operating system.",
				osInfo.ID, cases.Title(language.English).String(osInfo.Codename))
		}
	}

	// Check for old Switchroot Ubuntu
	if osInfo.ID == "Ubuntu" && osInfo.Release == "18.04" && FileExists("/etc/switchroot_version.conf") {
		return "Pi-Apps is no longer supported on your outdated Switchroot Ubuntu Bionic operating system. Consider installing Switchroot Ubuntu Noble. https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-noble-installation-guide"
	}

	// Check for Ubuntu EOL status
	if osInfo.ID == "Ubuntu" {
		// Get EOL info from endoflife.date API
		resp, err := http.Get("https://endoflife.date/api/ubuntu.json")
		if err != nil {
			Warning("Failed to check Ubuntu EOL status: " + err.Error())
		} else {
			defer resp.Body.Close()
			var ubuntuReleases []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&ubuntuReleases); err != nil {
				Warning("Failed to parse Ubuntu EOL data: " + err.Error())
			} else {
				for _, release := range ubuntuReleases {
					switch cycle := release["cycle"].(type) {
					case string:
						if cycle == osInfo.Release {
							switch eol := release["eol"].(type) {
							case string:
								// Check if the release is EOL
								eolDate, err := time.Parse("2006-01-02", eol)
								if err == nil {
									// Check if EOL date is within 4 months
									fourMonthsFromNow := time.Now().AddDate(0, 4, 0)
									oneMonthAfterEOL := eolDate.AddDate(0, 1, 0)

									if time.Now().After(eolDate) && time.Now().Before(oneMonthAfterEOL) {
										Warning(fmt.Sprintf("Your %s %s operating system reached end-of-life on %s. Please upgrade your system before Pi-Apps becomes unsupported.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), eol))
										return fmt.Sprintf("Your %s %s operating system reached end-of-life on %s. Please upgrade your system before Pi-Apps becomes unsupported.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), eol)
									} else if time.Now().After(oneMonthAfterEOL) {
										return fmt.Sprintf("Pi-Apps is not supported on your outdated %s %s operating system (EOL: %s). Expect apps to slowly fail. Consider installing a newer operating system.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), eol)
									} else if time.Now().Before(eolDate) && fourMonthsFromNow.After(eolDate) {
										Warning(fmt.Sprintf("Your %s %s operating system will reach end-of-life on %s. Please consider upgrading your system.",
											osInfo.ID, cases.Title(language.English).String(osInfo.Codename), eol))
									}
								}
							}
							break
						}
					}
				}
			}
		}

		// Fallback to static check if API fails
		if !isVersionGreaterOrEqual(osInfo.Release, "22.04") {
			return fmt.Sprintf("Pi-Apps is not supported on your outdated %s %s operating system. Expect many apps to fail. Consider installing a newer operating system.",
				osInfo.ID, cases.Title(language.English).String(osInfo.Codename))
		}
	}

	// Check for Manjaro
	if strings.Contains(strings.ToLower(osInfo.PrettyName), "manjaro") {
		return "Pi-Apps is not supported on Manjaro."
	}

	// Below are checks which require a plugin before it will be supported

	// Check for Arch Linux
	// Don't mark system as unsupported, but show a warning since we are going to add support for it.
	if strings.Contains(strings.ToLower(osInfo.PrettyName), "arch") {
		// For now, give a warning saying it isn't supported without a plugin.
		// TODO: Remove this warning once we have support for Arch Linux and instead check for the plugin once the plugin interface is implemented.
		Warning("Pi-Apps is not supported on Arch Linux without a plugin. To enable support, please install the Pi-Apps Arch Linux plugin in the Pi-Apps Settings app.")
	}

	// Check for Alpine Linux
	// This is already detected in the form of checking C libraies (musl is used in Alpine Linux) but still add a separate check for it.
	if strings.Contains(strings.ToLower(osInfo.PrettyName), "alpine") {
		// Don't mark system as unsupported, but show a warning since we are going to add support for it.
		// TODO: Remove this warning once we have support for Alpine Linux and instead check for the plugin once the plugin interface is implemented.
		Warning("Pi-Apps is not supported on Alpine Linux without a plugin. To enable support, please install the Pi-Apps Alpine Linux plugin in the Pi-Apps Settings app.")
	}

	// Check for RHEL like distributions (this includes Fedora, CentOS, etc.)
	if strings.Contains(strings.ToLower(osInfo.PrettyName), "rhel") || strings.Contains(strings.ToLower(osInfo.PrettyName), "fedora") || strings.Contains(strings.ToLower(osInfo.PrettyName), "centos") || strings.Contains(strings.ToLower(osInfo.PrettyName), "rocky") || strings.Contains(strings.ToLower(osInfo.PrettyName), "alma") {
		// Don't mark system as unsupported, but show a warning since we are going to add support for it.
		// TODO: Remove this warning once we have support for RHEL like distributions and instead check for the plugin once the plugin interface is implemented.
		Warning("Pi-Apps is not supported on RHEL like distributions without a plugin. To enable support, please install the Pi-Apps RHEL like plugin in the Pi-Apps Settings app.")
	}

	// Check for openSUSE (please note that we will not be officially supporting openSUSE due to the bit harder learning curve of the package manager until the plugin interface is implemented to let the community support it)
	if strings.Contains(strings.ToLower(osInfo.PrettyName), "opensuse") {
		// Don't mark system as unsupported, but show a warning since the community can add support for it.
		// TODO: Remove this warning once the community has support for openSUSE and instead check for the plugin once the plugin interface is implemented.
		Warning("Pi-Apps is not supported on openSUSE without a plugin. To enable support, please install the Pi-Apps openSUSE plugin in the Pi-Apps Settings app.")
	}

	// Check for ARMv6
	if strings.HasPrefix(osInfo.Architecture, "armv6") {
		return "Pi-Apps is not supported on ARMv6 Raspberry Pi boards. Expect some apps to fail."
	}

	return ""
}

// isVersionGreaterOrEqual checks if version1 is greater than or equal to version2
func isVersionGreaterOrEqual(version1, version2 string) bool {
	parts1 := strings.Split(version1, ".")
	parts2 := strings.Split(version2, ".")

	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		num1, err1 := strconv.Atoi(parts1[i])
		num2, err2 := strconv.Atoi(parts2[i])

		if err1 != nil || err2 != nil {
			// If we can't parse the version numbers, fall back to string comparison
			return version1 >= version2
		}

		if num1 > num2 {
			return true
		} else if num1 < num2 {
			return false
		}
	}

	// If we've gotten this far, the versions are equal up to the shortest length
	return len(parts1) >= len(parts2)
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

// getFreeSpace gets the free space on the specified filesystem in bytes
func getFreeSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// isMuslSystem checks if the system is using musl libc as its primary C library
// This function checks what the system is actually using, not just what's installed
func isMuslSystem() bool {
	// Method 1: Check what the current process is linked against
	// This is the most reliable method as it tells us what C library this very process is using
	if output, err := exec.Command("ldd", "/proc/self/exe").Output(); err == nil {
		outputStr := string(output)
		// If we see musl in the dynamic linker path, we're definitely using musl
		if strings.Contains(outputStr, "ld-musl-") {
			return true
		}
		// If we see glibc (libc.so.6), we're definitely using glibc
		if strings.Contains(outputStr, "libc.so.6") {
			return false
		}
	}

	// Method 2: Check what fundamental system binaries are linked against
	// These binaries must use the system's primary C library
	systemBinaries := []string{"/bin/sh", "/sbin/init", "/usr/bin/ls", "/bin/ls"}
	for _, binary := range systemBinaries {
		if FileExists(binary) {
			if output, err := exec.Command("ldd", binary).Output(); err == nil {
				outputStr := string(output)
				// If we see musl in the dynamic linker, this is a musl system
				if strings.Contains(outputStr, "ld-musl-") {
					return true
				}
				// If we see glibc, this is a glibc system
				if strings.Contains(outputStr, "libc.so.6") {
					return false
				}
			}
		}
	}

	// Method 3: Check the default dynamic linker
	// On musl systems, the default linker will be ld-musl-*
	defaultLinkers := []string{
		"/lib/ld-musl-x86_64.so.1",
		"/lib/ld-musl-aarch64.so.1",
		"/lib/ld-musl-armhf.so.1",
		"/lib/ld-musl-riscv64.so.1",
	}

	glibcLinkers := []string{
		"/lib64/ld-linux-x86-64.so.2",
		"/lib/ld-linux-aarch64.so.1",
		"/lib/ld-linux-armhf.so.3",
		"/lib/ld-linux-riscv64-lp64d.so.1",
	}

	// Check if glibc linkers exist - if they do, prioritize them as the primary system
	for _, linker := range glibcLinkers {
		if FileExists(linker) {
			return false
		}
	}

	// Only if no glibc linkers exist, check for musl linkers
	for _, linker := range defaultLinkers {
		if FileExists(linker) {
			return true
		}
	}

	// Method 4: Check if ldd is provided by musl (fallback)
	if output, err := exec.Command("ldd", "--version").CombinedOutput(); err == nil {
		outputStr := string(output)
		// glibc's ldd outputs a version header with "ldd (GNU libc)" or similar
		// musl's ldd outputs an error message or nothing useful
		if strings.Contains(strings.ToLower(outputStr), "gnu libc") {
			return false
		}
		// Only consider it musl if we explicitly see musl mentioned AND no gnu libc
		if strings.Contains(strings.ToLower(outputStr), "musl") {
			return true
		}
	}

	// Method 5: Alpine Linux detection (known musl distribution)
	if osRelease, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(osRelease)
		if strings.Contains(content, "ID=alpine") || strings.Contains(content, "ID=\"alpine\"") {
			return true
		}
	}

	// Default to false - assume glibc unless we have strong evidence of musl
	return false
}

// ProcessExists checks if a process with the given PID is running
func ProcessExists(pid int) bool {
	// In Linux, check if the /proc/{pid}/status file exists
	pidStatusPath := fmt.Sprintf("/proc/%d/status", pid)
	_, err := os.Stat(pidStatusPath)
	return err == nil
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

// GitClone clones a git repository and displays output if an error occurs
// It mimics the behavior of the original bash git_clone function
func GitClone(args ...string) error {
	// Parse arguments to find the repository URL and name
	var repoURL string
	var repoName string

	for i, arg := range args {
		if strings.Contains(arg, "://") {
			repoURL = arg
			// Extract repo name from URL (remove .git extension if present)
			repoName = strings.TrimSuffix(filepath.Base(repoURL), ".git")

			// Check if there's a non-flag argument after the URL that specifies the folder name
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// If the previous argument wasn't a flag and this is a non-flag argument, it's the folder name
				if i == 0 || !strings.HasPrefix(args[i-1], "-") {
					repoName = args[i+1]
				}
			}
			break
		}
	}

	if repoURL == "" {
		return fmt.Errorf("git_clone(): no repository URL specified")
	}

	// Get user's home directory for cloning (matching original bash behavior)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	folder := filepath.Join(homeDir, repoName)

	// Display status message
	Status("Downloading " + repoName + " repository...")

	// Remove existing folder if it exists
	if FileExists(folder) || DirExists(folder) {
		if err := os.RemoveAll(folder); err != nil {
			// Try with sudo if permission denied
			cmd := exec.Command("sudo", "rm", "-rf", folder)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to remove existing directory: %w", err)
			}
		}
	}

	// Clone the repository (run from home directory)
	gitCmd := exec.Command("git", "clone", repoURL, repoName)
	gitCmd.Dir = homeDir // Set working directory to home directory
	output, err := gitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("\nFailed to download %s repository.\nErrors: %s", repoName, string(output))
	}

	StatusGreen("Done")
	return nil
}

// Chmod changes file permissions while displaying a status message
// It mimics the behavior of the original bash chmod function
func Chmod(mode os.FileMode, path string) error {
	Status("Making executable: " + path)
	return os.Chmod(path, mode)
}

// Unzip extracts a zip file with status messages and implements the flags
// of the unzip command for compatibility with original scripts
// It mimics the behavior of the original bash unzip function
func Unzip(zipFile string, destDir string, flags []string) error {
	// Parse flags
	overwrite := false
	junkPaths := false
	quiet := false

	// First display what is being extracted (unless quiet)
	if FileExists(zipFile) {
		Status("Extracting: " + zipFile)
	}

	// Process flags
	for _, flag := range flags {
		if strings.HasPrefix(flag, "-") {
			for _, c := range flag[1:] {
				switch c {
				case 'o':
					overwrite = true // Overwrite without prompting
				case 'j':
					junkPaths = true // Junk paths (do not create directories)
				case 'q':
					quiet = true // Quiet mode
				}
			}
		}
	}

	// Show status message
	Status("Extracting: " + zipFile)

	// Open the zip file
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Create destination directory if it doesn't exist
	if destDir != "" && !DirExists(destDir) {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// Extract each file
	for _, file := range reader.File {
		err := extractZipFile(file, destDir, junkPaths, overwrite, quiet)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive
func extractZipFile(file *zip.File, destDir string, junkPaths bool, overwrite bool, quiet bool) error {
	// Determine the extraction path
	var filePath string

	// Check for directory traversal attempts
	if strings.Contains(file.Name, "..") {
		return fmt.Errorf("invalid file path: %s (contains '..')", file.Name)
	}

	if junkPaths {
		// Just use the filename, without directory structure
		filePath = filepath.Join(destDir, filepath.Base(file.Name))
	} else {
		// Use the full path structure
		cleanPath := filepath.Clean(filepath.Join(destDir, file.Name))
		if !strings.HasPrefix(cleanPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}
		filePath = cleanPath
	}

	// Check if this is a directory
	if file.FileInfo().IsDir() {
		if !junkPaths {
			// Create the directory
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
		}
		return nil
	}

	// Check if file already exists and we're not overwriting
	if FileExists(filePath) && !overwrite {
		if !quiet {
			Warning("Skipping " + filePath + " (already exists)")
		}
		return nil
	}

	// Extract the file
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Nproc returns the optimal number of processor threads to use based on available memory
// It mimics the behavior of the original bash nproc function
func Nproc() (int, error) {
	// Get the total number of processors
	totalProcs := runtime.NumCPU()

	// Check if running in GitHub Actions
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return totalProcs, nil
	}

	// Get available memory
	var memInfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&memInfo); err != nil {
		return 0, fmt.Errorf("failed to get system info: %w", err)
	}

	// Convert available memory to MB (from bytes)
	// Note: Sysinfo.Freeram is in bytes, so we divide by 1024*1024 to get MB
	availableMB := int(memInfo.Freeram * uint64(memInfo.Unit) / (1024 * 1024))

	// Alternatively, read from /proc/meminfo
	if availableMB == 0 {
		availableMB, _ = getAvailableMemoryMB()
	}

	// Determine number of threads based on available memory
	if availableMB > 2000 {
		// Available memory > 2000MB, use normal number of threads
		return totalProcs, nil
	} else if availableMB > 1500 {
		// 1500MB < available memory <= 2000MB, use 3 threads
		Warning("Your system has less than 2000MB of available RAM, so this will compile with only 3 threads.")
		return int(math.Min(float64(totalProcs), 3)), nil
	} else if availableMB > 1000 {
		// 1000MB < available memory <= 1500MB, use 2 threads
		Warning("Your system has less than 1500MB of available RAM, so this will compile with only 2 threads.")
		return int(math.Min(float64(totalProcs), 2)), nil
	} else {
		// Available memory <= 1000MB, use 1 thread
		Warning("Your system has less than 1000MB of available RAM, so this will compile with only 1 thread.")
		return 1, nil
	}
}

// getAvailableMemoryMB reads available memory from /proc/meminfo
func getAvailableMemoryMB() (int, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				// Convert from kB to MB
				valuekB, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0, err
				}
				return valuekB / 1024, nil
			}
		}
	}

	return 0, fmt.Errorf("couldn't find MemAvailable in /proc/meminfo")
}

// Wget downloads a file from a URL and displays progress
// It mimics the behavior of the original bash wget function
func Wget(args []string) error {
	// Parse the arguments
	var url string
	var outputFile string
	quiet := false
	writeToStdout := false
	headers := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle flags
		if strings.HasPrefix(arg, "--") {
			// Long options
			if arg == "--quiet" {
				quiet = true
			} else if strings.HasPrefix(arg, "--header=") {
				headerParts := strings.SplitN(arg[9:], ":", 2)
				if len(headerParts) == 2 {
					headers[strings.TrimSpace(headerParts[0])] = strings.TrimSpace(headerParts[1])
				}
			}
		} else {
			switch {
			case arg == "-":
				// Writing to stdout
				writeToStdout = true
				quiet = true
			case strings.HasPrefix(arg, "-"):
				// Short options
				for _, flag := range arg[1:] {
					if flag == 'q' {
						quiet = true
					} else if flag == 'O' && i+1 < len(args) {
						// Output file specified with -O flag
						outputFile = args[i+1]
						i++ // Skip the next argument since we used it
					}
				}
			case strings.Contains(arg, "://"):
				// URL
				url = arg
			case strings.HasPrefix(arg, "/"):
				// Absolute path for output file
				outputFile = arg
			default:
				// Relative path or other argument
				// If we don't have an output file yet, assume this is it
				if outputFile == "" {
					// Get the current working directory and join with the relative path
					cwd, err := os.Getwd()
					if err != nil {
						return fmt.Errorf("failed to get current directory: %w", err)
					}
					outputFile = filepath.Join(cwd, arg)
				}
			}
		}
	}

	// Check if we have a URL
	if url == "" {
		return fmt.Errorf("no URL specified")
	}

	// If no output file is specified, use the filename from the URL
	if outputFile == "" && !writeToStdout {
		parsedURL, err := parseURL(url)
		if err != nil {
			return fmt.Errorf("failed to parse URL: %w", err)
		}

		// Get the filename from the URL path
		filename := filepath.Base(parsedURL.Path)
		// Remove trailing slashes and query parameters
		filename = strings.TrimSuffix(filename, "/")
		if filename == "" {
			filename = "index.html"
		}

		// Use current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		outputFile = filepath.Join(cwd, filename)
	}

	// Display download message
	filename := filepath.Base(outputFile)
	if !quiet {
		if outputFile != "" && outputFile != filepath.Join(filepath.Dir(outputFile), filename) {
			Status("Downloading " + filename + " to " + outputFile + "...")
		} else {
			Status("Downloading " + filename + "...")
		}
	}

	// Create the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	// Add a user agent if none specified
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", "Pi-Apps/1.0")
	}

	// Create client
	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for large files
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}

	// Prepare output
	var output io.Writer
	if writeToStdout {
		output = os.Stdout
	} else {
		// Create the output file
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	}

	// Get the total size for progress reporting
	contentLength := resp.ContentLength

	// Copy the data with progress reporting
	if !quiet && !writeToStdout && contentLength > 0 {
		// Create a progress wrapper for io.Copy
		progress := &progressWriter{
			Total:   uint64(contentLength),
			Current: 0,
			Quiet:   quiet,
		}

		// Start progress goroutine
		done := make(chan bool)
		go progress.showProgress(done)

		// Copy the data
		_, err = io.Copy(output, io.TeeReader(resp.Body, progress))

		// Signal the progress goroutine to stop
		close(done)

		// Show final progress
		if err == nil {
			fmt.Print("\033[K") // Clear the line
			StatusGreen("Done")
		}
	} else {
		// No progress reporting
		_, err = io.Copy(output, resp.Body)
	}

	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}

// progressWriter is used to track and display download progress
type progressWriter struct {
	Total   uint64
	Current uint64
	Quiet   bool
}

// Write implements io.Writer
func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Current += uint64(n)
	return n, nil
}

// showProgress displays the download progress
func (pw *progressWriter) showProgress(done chan bool) {
	if pw.Quiet {
		return
	}

	// Get terminal width
	termWidth := 80
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		termWidth = width
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if pw.Total > 0 {
				percent := float64(pw.Current) / float64(pw.Total) * 100
				bytesRead := formatBytes(pw.Current)
				totalBytes := formatBytes(pw.Total)

				// Calculate the progress bar width
				statsLine := fmt.Sprintf("%s/%s ", bytesRead, totalBytes)
				statsLineLen := len(statsLine)
				availableWidth := termWidth - statsLineLen
				if availableWidth <= 0 {
					availableWidth = 20 // Minimum width
				}

				// Calculate how many characters to fill in the progress bar
				progressChars := int((percent * float64(availableWidth)) / 100)

				// Build the progress bar
				progressBar := "\033[92m\033[1m"
				for i := 0; i < progressChars; i++ {
					progressBar += "—" // Long dash character
				}
				progressBar += "\033[39m"

				// Hide cursor, print progress, and reset cursor position
				fmt.Print("\033[?25l") // Hide cursor
				fmt.Printf("\033[K%s%s\r", statsLine, progressBar)
			}
		case <-done:
			fmt.Print("\033[?25h") // Show cursor
			return
		}
	}
}

// formatBytes converts bytes to a human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// parseURL parses a URL string and handles special cases
func parseURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// Remove /download suffix that some file hosting services use
	parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/download")

	return parsedURL, nil
}

// UnzipWithArgs wraps Unzip to handle command-line style arguments
func UnzipWithArgs(args ...string) error {
	// Need at least one argument (the zip file)
	if len(args) < 1 {
		return fmt.Errorf("unzip: no zipfile specified")
	}

	zipFile := args[0]
	destDir := ""           // Default to current directory
	flags := []string{"-o"} // Default to overwrite

	// Process arguments
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-d" && i+1 < len(args) {
			// Destination directory
			destDir = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "-") {
			// Flag
			flags = append(flags, arg)
		}
	}

	return Unzip(zipFile, destDir, flags)
}

// GetPiAppIcon returns the path to an app's icon file (icon-64.png)
// Returns the full path to the icon file, or an error if not found
func GetPiAppIcon(appName string) (string, error) {
	piAppsDir := getPiAppsDir()
	iconPath := filepath.Join(piAppsDir, "apps", appName, "icon-64.png")

	// Check if the icon file exists
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		return "", fmt.Errorf("icon file not found for app '%s': %s", appName, iconPath)
	}

	return iconPath, nil
}

// ChmodWithArgs wraps Chmod to handle command-line style arguments
func ChmodWithArgs(args ...string) error {
	// Need at least two arguments (mode and filename)
	if len(args) < 2 {
		return fmt.Errorf("chmod: missing operand")
	}

	// Parse the mode
	modeStr := args[0]
	mode, err := parseChmodMode(modeStr)
	if err != nil {
		return fmt.Errorf("chmod: invalid mode: %s", modeStr)
	}

	// Apply chmod to each file
	for _, path := range args[1:] {
		if err := Chmod(mode, path); err != nil {
			return err
		}
	}

	return nil
}

// parseChmodMode converts a string mode (like "755") to os.FileMode
func parseChmodMode(modeStr string) (os.FileMode, error) {
	// Check if it's an octal number
	if regexp.MustCompile(`^[0-7]+$`).MatchString(modeStr) {
		// Parse octal
		modeInt, err := strconv.ParseInt(modeStr, 8, 32)
		if err != nil {
			return 0, err
		}
		return os.FileMode(modeInt), nil
	}

	// Handle symbolic mode
	return parseSymbolicMode(modeStr)
}

// parseSymbolicMode handles symbolic chmod modes like "u+x", "go-w", "a=r", etc.
func parseSymbolicMode(modeStr string) (os.FileMode, error) {
	// Default starting mode (644 - rw-r--r--)
	mode := os.FileMode(0644)

	// Split by comma for multiple operations
	operations := strings.Split(modeStr, ",")

	for _, op := range operations {
		op = strings.TrimSpace(op)
		if op == "" {
			continue
		}

		// Parse who, operator, and permissions
		var who, operator, perms string

		// Find the operator (+, -, =)
		opIndex := -1
		for i, char := range op {
			if char == '+' || char == '-' || char == '=' {
				who = op[:i]
				operator = string(char)
				perms = op[i+1:]
				opIndex = i
				break
			}
		}

		if opIndex == -1 {
			return 0, fmt.Errorf("invalid symbolic mode: %s", op)
		}

		// Default to 'a' (all) if no who is specified
		if who == "" {
			who = "a"
		}

		// Calculate permission bits
		permBits := os.FileMode(0)
		for _, perm := range perms {
			switch perm {
			case 'r':
				permBits |= 0444 // read for all
			case 'w':
				permBits |= 0222 // write for all
			case 'x':
				permBits |= 0111 // execute for all
			default:
				return 0, fmt.Errorf("invalid permission: %c", perm)
			}
		}

		// Apply to specific users
		var targetBits os.FileMode
		for _, user := range who {
			switch user {
			case 'u': // user/owner
				if permBits&0444 != 0 {
					targetBits |= 0400
				}
				if permBits&0222 != 0 {
					targetBits |= 0200
				}
				if permBits&0111 != 0 {
					targetBits |= 0100
				}
			case 'g': // group
				if permBits&0444 != 0 {
					targetBits |= 0040
				}
				if permBits&0222 != 0 {
					targetBits |= 0020
				}
				if permBits&0111 != 0 {
					targetBits |= 0010
				}
			case 'o': // others
				if permBits&0444 != 0 {
					targetBits |= 0004
				}
				if permBits&0222 != 0 {
					targetBits |= 0002
				}
				if permBits&0111 != 0 {
					targetBits |= 0001
				}
			case 'a': // all (user, group, others)
				targetBits = permBits
			default:
				return 0, fmt.Errorf("invalid user specification: %c", user)
			}
		}

		// Apply the operation
		switch operator {
		case "+":
			mode |= targetBits
		case "-":
			mode &^= targetBits
		case "=":
			// For '=', we need to clear existing bits for the specified users first
			var clearBits os.FileMode
			for _, user := range who {
				switch user {
				case 'u':
					clearBits |= 0700
				case 'g':
					clearBits |= 0070
				case 'o':
					clearBits |= 0007
				case 'a':
					clearBits |= 0777
				}
			}
			mode &^= clearBits
			mode |= targetBits
		}
	}

	return mode, nil
}

// SudoPopup executes a command with sudo if available without password, otherwise with pkexec
// It mimics the behavior of the original bash sudo_popup function, which avoids displaying
// a password prompt to an invisible terminal
func SudoPopup(command string, args ...string) error {
	Status("Requesting administrative privileges for: " + command)

	// First check if sudo can be run without a password
	checkCmd := exec.Command("sudo", "-n", "true")
	err := checkCmd.Run()

	if err == nil {
		// sudo is available without password, use it directly
		cmd := exec.Command("sudo", append([]string{command}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	} else {
		// sudo needs password, use pkexec for graphical authentication
		cmd := exec.Command("pkexec", append([]string{command}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
}
