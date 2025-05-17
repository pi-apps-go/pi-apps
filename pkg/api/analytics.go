package api

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BitlyLink is a compatibility function that redirects to ShlinkLink
// It's maintained for backward compatibility with scripts that might use it
func BitlyLink(app, trigger string) error {
	return ShlinkLink(app, trigger)
}

// ShlinkLink sends anonymous analytics data when an app is installed or uninstalled
// to track app popularity. No personally identifiable information is sent.
func ShlinkLink(app, trigger string) error {
	// Run in a goroutine to avoid blocking the caller
	go func() {
		// Validate inputs
		if app == "" {
			Error("ShlinkLink(): requires an app argument")
			return
		}
		if trigger == "" {
			Error("ShlinkLink(): requires a trigger argument")
			return
		}

		// Check if analytics are enabled
		directory := os.Getenv("PI_APPS_DIR")
		if directory == "" {
			Error("ShlinkLink(): PI_APPS_DIR environment variable not set")
			return
		}

		settingsPath := filepath.Join(directory, "data", "settings", "Enable analytics")
		settingsData, err := os.ReadFile(settingsPath)
		if err == nil && strings.TrimSpace(string(settingsData)) == "No" {
			// Analytics are disabled
			return
		}

		// Get device information
		model, socID := getModel()
		machineID := getHashedFileContent("/etc/machine-id")
		serialNumber := getHashedFileContent("/sys/firmware/devicetree/base/serial-number")
		osName := getOSName()
		arch := getArchitecture()

		// Sanitize app name for URL
		sanitizedApp := sanitizeAppName(app)

		// Create the URL
		url := fmt.Sprintf("https://analytics.pi-apps.io/pi-apps-%s-%s/track", trigger, sanitizedApp)

		// Create the user agent string
		userAgent := fmt.Sprintf("Pi-Apps Raspberry Pi app store; %s; %s; %s; %s; %s; %s",
			model, socID, machineID, serialNumber, osName, arch)

		// Make the request
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			Debug(fmt.Sprintf("ShlinkLink: Error creating request: %v", err))
			return
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "image/gif")

		resp, err := client.Do(req)
		if err != nil {
			Debug(fmt.Sprintf("ShlinkLink: Error making request: %v", err))
			return
		}
		defer resp.Body.Close()

		// We don't need to do anything with the response
	}()

	return nil
}

// Helper functions

// getModel returns the device model and SOC_ID
func getModel() (string, string) {
	// First, try to get the model information
	var model, socID string

	// Initialize the model detection function
	cmd := exec.Command("bash", "-c", `grep -m1 "^Model.*:" /proc/cpuinfo | sed 's/Model.*: //g'`)
	modelOutput, err := cmd.Output()
	if err == nil {
		model = strings.TrimSpace(string(modelOutput))
		model = strings.Trim(model, `"';`)
	}

	// Get SOC_ID if available
	socCmd := exec.Command("bash", "-c", `grep -m1 "^Hardware.*:" /proc/cpuinfo | sed 's/Hardware.*: //g'`)
	socOutput, err := socCmd.Output()
	if err == nil {
		socID = strings.TrimSpace(string(socOutput))
		socID = strings.Trim(socID, `"';`)
	}

	return model, socID
}

// getHashedFileContent reads a file and returns its SHA1 hash if the file exists and has content
func getHashedFileContent(filePath string) string {
	// Check if file exists and has content
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() == 0 {
		return ""
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	// Calculate SHA1 hash
	hash := sha1.New()
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// getOSName returns the OS name and version
func getOSName() string {
	idCmd := exec.Command("bash", "-c", `cat /etc/os-release | grep ^ID= | tr -d '"'\; | awk -F= '{print $2}' | head -1`)
	idOutput, err := idCmd.Output()
	if err != nil {
		return ""
	}

	versionCmd := exec.Command("bash", "-c", `cat /etc/os-release | grep ^VERSION_ID= | tr -d '"'\; | awk -F= '{print $2}' | head -1`)
	versionOutput, err := versionCmd.Output()
	if err != nil {
		return ""
	}

	osID := strings.TrimSpace(string(idOutput))
	osVersion := strings.TrimSpace(string(versionOutput))

	osName := fmt.Sprintf("%s %s", osID, osVersion)
	// Capitalize first letter
	if len(osName) > 0 {
		osName = strings.ToUpper(osName[:1]) + osName[1:]
	}

	return osName
}

// sanitizeAppName removes any non-alphanumeric characters from the app name
func sanitizeAppName(appName string) string {
	// Remove spaces
	noSpaces := strings.ReplaceAll(appName, " ", "")

	// Keep only alphanumeric characters
	reg := regexp.MustCompile("[^a-zA-Z0-9]")
	return reg.ReplaceAllString(noSpaces, "")
}
