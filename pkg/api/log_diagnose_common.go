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

// Module: log_diagnose_common.go
// Description: Provides common functions for diagnosing log files.
// Package manager specific errors are now split into separate files named log_diagnose_<package_manager>.go to only include code for the target package manager.

package api

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// ErrorDiagnosis contains the results of diagnosing a log file
//
// ErrorType - the type of error (system, package, internet, or unknown)
//
// Captions - a user-friendly explanation of the error
type ErrorDiagnosis struct {
	// ErrorType is the type of error (system, package, internet, or unknown)
	ErrorType string
	// ErrorCaption is a user-friendly explanation of the error
	Captions []string
}

// FormatLogfile removes ANSI escape sequences and adds OS information to the beginning of a logfile
func FormatLogfile(filename string) error {
	if filename == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	// Read the file
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Remove ANSI escape sequences
	cleanedContent := RemoveAnsiEscapes(string(content))

	// Check if the file already starts with device information
	// Look for patterns that indicate system info is already present
	if strings.HasPrefix(cleanedContent, "OS: ") {
		// File already has device info, just clean ANSI and write back
		return os.WriteFile(filename, []byte(cleanedContent), 0644)
	}

	// Get device info
	deviceInfo, err := GetDeviceInfo()
	if err != nil {
		deviceInfo = "Failed to get device info"
	}

	// Create the formatted content
	formattedContent := deviceInfo + "\n\nBEGINNING OF LOG FILE:\n-----------------------\n\n" + cleanedContent

	// Write it back to the file
	return os.WriteFile(filename, []byte(formattedContent), 0644)
}

// GetDeviceInfo returns comprehensive system information about the device
func GetDeviceInfo() (string, error) {
	var info strings.Builder

	// Get OS info
	if _, err := os.Stat("/etc/os-release"); err == nil {
		osNameOutput, err := runCommand("grep", "PRETTY_NAME", "/etc/os-release")
		if err == nil {
			osName := strings.TrimSpace(osNameOutput)
			osName = strings.TrimPrefix(osName, "PRETTY_NAME=")
			osName = strings.Trim(osName, "\"")
			info.WriteString("OS: " + osName + "\n")
		} else {
			info.WriteString("OS: Unknown\n")
		}
	} else {
		info.WriteString("OS: Unknown\n")
	}

	// Get OS architecture using unsafe.Sizeof
	arch := fmt.Sprintf("%d", unsafe.Sizeof(uintptr(0))*8)
	info.WriteString("OS architecture: " + arch + "-bit\n")

	// Get Pi-Apps information
	piAppsDir := os.Getenv("PI_APPS_DIR") // Pi-Apps directory environment variable
	if piAppsDir != "" && fileExists(piAppsDir) {
		// Get last local update date using Go-native parsing
		cmd := exec.Command("git", "-C", piAppsDir, "show", "-s", `--format=%ad`, "--date=short")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			commitDate := strings.TrimSpace(string(output))
			if commitDate != "" {
				// commitDate should be in format YYYY-MM-DD
				parsedTime, err := time.Parse("2006-01-02", commitDate)
				if err == nil {
					// Format to system default short date (as xargs date +%x would do)
					localUpdateDate := parsedTime.Format("01/02/2006")
					info.WriteString("Last updated Pi-Apps on: " + localUpdateDate + "\n")
				} else {
					info.WriteString("Last updated Pi-Apps on: " + commitDate + "\n")
				}
			}
		}

		// Get latest Pi-Apps version
		gitURLPath := filepath.Join(piAppsDir, "etc", "git_url")
		if fileExists(gitURLPath) {
			// Read git URL from file
			gitURLBytes, err := os.ReadFile(gitURLPath)
			if err == nil {
				gitURL := strings.TrimSpace(string(gitURLBytes))

				// Parse account and repository from URL
				parts := strings.Split(gitURL, "/")
				if len(parts) >= 2 {
					account := parts[len(parts)-2]
					repo := parts[len(parts)-1]

					// Make HTTP request to GitHub API
					apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/master", account, repo)

					// Create request with optional GitHub API key
					req, err := http.NewRequest("GET", apiURL, nil)
					if err == nil {
						// Add GitHub API key if available
						if apiKey := os.Getenv("GITHUB_API_KEY"); apiKey != "" {
							req.Header.Set("Authorization", "token "+apiKey)
						}

						// Make the request
						client := &http.Client{}
						resp, err := client.Do(req)
						if err == nil {
							defer resp.Body.Close()

							// Parse JSON response
							var commits []struct {
								Commit struct {
									Author struct {
										Date string `json:"date"`
									} `json:"author"`
								} `json:"commit"`
							}

							if err := json.NewDecoder(resp.Body).Decode(&commits); err == nil && len(commits) > 0 {
								// Parse the ISO date and format it
								date, err := time.Parse(time.RFC3339, commits[0].Commit.Author.Date)
								if err == nil {
									dateStr := date.Format("01/02/2006")
									info.WriteString("Latest Pi-Apps version: " + dateStr + "\n")
								}
							}
						}
					}
				}
			}
		}
	}

	// Get kernel info
	kernelArchOutput, err := runCommand("uname", "-m")
	kernelVersionOutput, err2 := runCommand("uname", "-r")
	if err == nil && err2 == nil {
		info.WriteString("Kernel: " + strings.TrimSpace(kernelArchOutput) + " " + strings.TrimSpace(kernelVersionOutput) + "\n")
	} else {
		info.WriteString("Kernel: Unknown\n")
	}

	// Get device model and SoC information
	model, socID := GetDeviceModel()
	info.WriteString("Device model: " + model + "\n")

	// Add SoC information if available
	if socID != "" {
		info.WriteString("SOC identifier: " + socID + "\n")
	}

	// Get hashed machine-id
	if _, err := os.Stat("/etc/machine-id"); err == nil {
		machineIDBytes, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			hasher := sha1.New()
			hasher.Write(machineIDBytes)
			hash := hex.EncodeToString(hasher.Sum(nil))
			info.WriteString("Machine-id (hashed): " + hash + "\n")
		}
	}

	// Get hashed serial-number
	if _, err := os.Stat("/sys/firmware/devicetree/base/serial-number"); err == nil {
		serialBytes, err := os.ReadFile("/sys/firmware/devicetree/base/serial-number")
		if err == nil {
			hasher := sha1.New()
			hasher.Write(serialBytes)
			hash := hex.EncodeToString(hasher.Sum(nil))
			info.WriteString("Serial-number (hashed): " + hash + "\n")
		}
	}

	// Get CPU name
	if cpuInfo, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(cpuInfo))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					info.WriteString("CPU name: " + strings.TrimSpace(parts[1]) + "\n")
					break
				}
			}
		}
	}

	// Get RAM size
	if memInfo, err := os.ReadFile("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(memInfo))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal") {
				parts := strings.Fields(line)
				if len(parts) > 1 {
					memKB, _ := strconv.ParseFloat(parts[1], 64)
					memGB := memKB / 1024000.0
					info.WriteString(fmt.Sprintf("RAM size: %.2f GB\n", memGB))
					break
				}
			}
		}
	}

	// Get Raspberry Pi OS image version
	if rpiIssue, err := os.ReadFile("/etc/rpi-issue"); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(rpiIssue))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Raspberry Pi reference") {
				version := strings.TrimPrefix(line, "Raspberry Pi reference ")
				info.WriteString("Raspberry Pi OS image version: " + strings.TrimSpace(version) + "\n")
				break
			}
		}
	}

	// Get language settings
	lang := os.Getenv("LANG")
	if lang != "" {
		info.WriteString("Language: " + lang + "\n")
	} else {
		lcAll := os.Getenv("LC_ALL")
		if lcAll != "" {
			info.WriteString("Language: " + lcAll + "\n")
		}
	}

	// Get Go runtime information, including experiments if present
	goVersion := runtime.Version()
	info.WriteString("Go runtime used: " + goVersion + "\n")
	// Look for experiments of the form "X:<experiment1>,<experiment2>" in runtime.Version() output
	parts := strings.Fields(goVersion)
	for _, part := range parts {
		if strings.HasPrefix(part, "X:") {
			expNames := strings.TrimPrefix(part, "X:")
			if expNames != "" {
				expList := strings.Split(expNames, ",")
				info.WriteString("Go experiments enabled in this build: " + strings.Join(expList, ", ") + "\n")
				WarningT("Go experiments may be unstable and may cause issues with Pi-Apps Go. If you encounter any issues, please report them to the Pi-Apps Go team or disable them.")
			}
		}
	}

	return info.String(), nil
}

// RemoveAnsiEscapes removes ANSI escape sequences from a string
func RemoveAnsiEscapes(input string) string {
	// Replace \r with \n
	input = strings.ReplaceAll(input, "\r", "\n")

	// Remove various ANSI escape sequences
	regexes := []*regexp.Regexp{
		regexp.MustCompile(`\x1b\[?[0-9;]*[a-zA-Z]`),
		regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`),
		regexp.MustCompile(`\x1b\[[0-9;]*`),
	}

	for _, regex := range regexes {
		input = regex.ReplaceAllString(input, "")
	}

	// Remove progress bar lines
	progressBarRegex := regexp.MustCompile(`\.{10} \.{10} \.{10} \.{10} \.{9}`)
	lines := strings.Split(input, "\n")
	var filteredLines []string

	for _, line := range lines {
		if !progressBarRegex.MatchString(line) {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.Join(filteredLines, "\n")
}

// Helper function to check if a string contains any of the given patterns
func containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// SendErrorReport sends an error report to the Pi-Apps team
func SendErrorReport(logfilePath string) (string, error) {
	// Validate arguments
	if logfilePath == "" {
		return "", fmt.Errorf("send_error_report(): requires an argument")
	}

	// Check if file exists
	if !FileExists(logfilePath) {
		return "", fmt.Errorf("send_error_report(): '%s' is not a valid file", logfilePath)
	}

	// Format the log file before sending to ensure it's readable
	if err := FormatLogfile(logfilePath); err != nil {
		return "", fmt.Errorf("send_error_report(): error formatting log file: %w", err)
	}

	// Check if the log file contains the required header before sending
	containsHeader, err := fileContainsPattern(logfilePath, "^Last updated Pi-Apps on:")
	if err != nil {
		return "", fmt.Errorf("error checking log file contents: %w", err)
	}

	if !containsHeader {
		// If header is not found, just return success but don't actually send the report
		// This is to maintain compatibility with the original bash implementation
		return "Log file not sent - missing required header", nil
	}

	// Get a token from the error report server
	client := &http.Client{}
	tokenResp, err := client.Get("http://localhost:8080/token") // localhost for development purposes
	if err != nil {
		return "", fmt.Errorf("failed to get error report token: %w", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get error report token: server returned %d", tokenResp.StatusCode)
	}

	var tokenData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// Create a filename for the upload that removes the .log extension
	filename := filepath.Base(logfilePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".txt"

	// Create a multipart form request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	// Read and write the file content
	fileContent, err := os.ReadFile(logfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return "", fmt.Errorf("failed to write file content: %w", err)
	}
	writer.Close()

	// Create the request
	req, err := http.NewRequest("POST", "http://localhost:8080/report", body) // localhost is for development purposes
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Error-Report-Token", tokenData.Token)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send error report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to send error report: server returned %d", resp.StatusCode)
	}

	return "Error report sent successfully!", nil
}

// fileContainsPattern checks if a file contains a given pattern using Go's native library functions
func fileContainsPattern(filePath, pattern string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), pattern) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// Helper function to run shell commands
func runCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Helper function to get unique strings from a slice
func uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	sort.Strings(list)
	return list
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getArchitecture returns "32" or "64" based on system architecture
func getArchitecture() string {
	output, err := runCommand("getconf", "LONG_BIT")
	if err == nil {
		return strings.TrimSpace(output)
	}

	// Try alternative method
	output, err = runCommand("uname", "-m")
	if err == nil {
		if strings.Contains(output, "64") {
			return "64"
		} else {
			return "32"
		}
	}

	// Default fallback
	return "64"
}

// GetDeviceModel returns detailed information about the hardware model and SoC
func GetDeviceModel() (string, string) {
	// Initialize variables
	model := ""
	socID := ""

	// Check all possible model locations from most to least common
	modelPaths := []string{
		"/sys/firmware/devicetree/base/model",
		"/sys/firmware/devicetree/base/banner-name",
		"/tmp/sysinfo/model",
		"/sys/devices/virtual/dmi/id/product_name",
		"/sys/class/dmi/id/product_name",
	}

	// Try each path until we find one that works
	for _, path := range modelPaths {
		if _, err := os.Stat(path); err == nil {
			modelBytes, err := os.ReadFile(path)
			if err == nil && len(modelBytes) > 0 {
				// Remove null terminator and trim whitespace
				model = strings.TrimSpace(strings.ReplaceAll(string(modelBytes), "\x00", ""))
				if model != "" {
					break
				}
			}
		}
	}

	// Check for Android environment
	// Android typically has /system/app/ and /system/priv-app directories
	if model == "" && fileExists("/system/app/") && fileExists("/system/priv-app") {
		// Try multiple Android property commands in order of preference
		androidProps := []string{
			"ro.product.marketname",
			"ro.vendor.product.display",
			"ro.config.devicename",
			"ro.config.marketing_name",
			"ro.product.vendor.model",
			"ro.product.oppo_model",
			"ro.oppo.market.name",
			"ro.product.model",
			"ro.product.product.model",
			"ro.product.odm.model",
		}

		for _, prop := range androidProps {
			cmd := exec.Command("getprop", prop)
			output, err := cmd.Output()
			if err == nil {
				propValue := strings.TrimSpace(string(output))
				if propValue != "" {
					model = propValue
					break
				}
			}
		}
	}

	// SoC (System on Chip) detection
	// Check for Tegra, Rockchip, Broadcom (RPi), etc.
	compatiblePath := "/proc/device-tree/compatible"
	if fileExists(compatiblePath) {
		compatibleBytes, err := os.ReadFile(compatiblePath)
		if err == nil {
			chip := strings.ReplaceAll(string(compatibleBytes), "\x00", "")

			// Nvidia Tegra detection
			tegraMapping := map[string]string{
				"tegra20":  "tegra-2",
				"tegra30":  "tegra-3",
				"tegra114": "tegra-4",
				"tegra124": "tegra-k1-32",
				"tegra132": "tegra-k1-64",
				"tegra210": "tegra-x1",
				"tegra186": "tegra-x2",
				"tegra194": "xavier",
				"tegra234": "orin",
				"tegra239": "switch-2-chip",
			}

			for key, value := range tegraMapping {
				if strings.Contains(chip, key) {
					socID = value
					break
				}
			}

			// Generic Tegra detection
			if socID == "" && strings.Contains(chip, "tegra") {
				socID = "jetson-unknown"
			}

			// Rockchip SoC detection
			rockchipIDs := []string{
				"rk3399", "rk3308", "rk3326", "rk3328",
				"rk3368", "rk3566", "rk3568", "rk3588",
				"rk3588s",
			}

			for _, id := range rockchipIDs {
				if strings.Contains(chip, id) {
					socID = id
					break
				}
			}

			// RISC-V SoC detection
			riscvIDs := []string{
				"jh7100", "jh7110", "jh7120", // StarFive VisionFive series
				"cv1800b", "cv1812h", // Milk-V Duo series
				"th1520", // T-Head TH1520
				"k230",   // Kendryte K230
				"sg2042", // Sophgo SG2042
				"u74",    // SiFive U74
				"fu740",  // SiFive FU740
				"kyu",    // Kyu SoC
			}

			for _, id := range riscvIDs {
				if strings.Contains(chip, id) {
					socID = id
					break
				}
			}

			// Amlogic SoC detection
			if strings.Contains(chip, "g12b") {
				socID = "g12b"
			}

			// Broadcom (Raspberry Pi) detection
			bcmIDs := []string{
				"bcm2712", "bcm2711", "bcm2837", "bcm2836", "bcm2835",
			}

			for _, id := range bcmIDs {
				if strings.Contains(chip, id) {
					socID = id
					break
				}
			}
		}
	}

	// Older Tegra detection method
	if socID == "" && fileExists("/sys/devices/soc0/family") {
		familyBytes, err := os.ReadFile("/sys/devices/soc0/family")
		if err == nil {
			chip := strings.ReplaceAll(string(familyBytes), "\x00", "")

			tegraMapping := map[string]string{
				"tegra20":  "tegra-2",
				"tegra30":  "tegra-3",
				"tegra114": "tegra-4",
				"tegra124": "tegra-k1-32",
				"tegra132": "tegra-k1-64",
				"tegra210": "tegra-x1",
			}

			for key, value := range tegraMapping {
				if strings.Contains(chip, key) {
					socID = value
					break
				}
			}
		}
	}

	// If still no model, use hostname as last resort (for RPi)
	if model == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			// Raspberry Pi hostnames often contain model info
			if strings.Contains(strings.ToLower(hostname), "raspberry") ||
				strings.Contains(strings.ToLower(hostname), "rpi") {
				model = hostname
			}
		}
	}

	// If model is still empty, set to Unknown
	if model == "" {
		model = "Unknown"
	}

	return model, socID
}
