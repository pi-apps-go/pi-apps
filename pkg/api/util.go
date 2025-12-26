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

// Module: util.go
// Description: Provides functions for miscellaneous operations (like runonce entries or preferred text editor options)

package api

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runonce runs a command only if it has never been run before.
// It takes a script as a string and executes it only if its hash
// doesn't exist in the runonce hashes file.
// This is useful for one-time migrations or setting changes.
//
// Deprecated: In our goals to remove bash scripts for anything other then apps,
// this function will be removed soon. Use api.RunonceFunc instead for Go native runonce functions.
func Runonce(script string) error {
	// Get the PI_APPS_DIR environment variable
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Calculate SHA1 hash of the script
	hasher := sha1.New()
	hasher.Write([]byte(script))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Check if hash exists in the runonce_hashes file
	hashesFile := filepath.Join(directory, "data", "runonce_hashes")

	// Create the file if it doesn't exist
	if !FileExists(hashesFile) {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(hashesFile), 0755); err != nil {
			return fmt.Errorf("failed to create directory for runonce_hashes: %w", err)
		}

		// Create empty file
		if _, err := os.Create(hashesFile); err != nil {
			return fmt.Errorf("failed to create runonce_hashes file: %w", err)
		}
	}

	// Check if the hash already exists in the file
	hashExists, err := hashExistsInFile(hashesFile, hash)
	if err != nil {
		return fmt.Errorf("failed to check hash existence: %w", err)
	}

	if hashExists {
		// Hash found, command already run before - do nothing
		return nil
	}

	// Hash not found, run the script
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("runonce(): script failed: %w", err)
	}

	// If script succeeds, add the hash to the list
	hashFile, err := os.OpenFile(hashesFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open runonce_hashes file: %w", err)
	}
	defer hashFile.Close()

	if _, err := hashFile.WriteString(hash + "\n"); err != nil {
		return fmt.Errorf("failed to write hash to file: %w", err)
	}

	return nil
}

// hashExistsInFile checks if a hash exists in the specified file
func hashExistsInFile(filePath, hash string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == hash {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// RunonceFunc runs a function only if it has never been run before with the given version.
// It takes a function and a version identifier (e.g., "addUserDirs-v1").
// If the version identifier doesn't exist in the runonce hashes file, the function is executed.
// This is useful for one-time migrations or setting changes using Go functions instead of bash scripts.
func RunonceFunc(version string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("runonceFunc(): function is nil")
	}

	// Get the PI_APPS_DIR environment variable
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Calculate SHA1 hash of the version identifier
	hasher := sha1.New()
	hasher.Write([]byte(version))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Check if hash exists in the runonce_hashes file
	hashesFile := filepath.Join(directory, "data", "runonce_hashes")

	// Create the file if it doesn't exist
	if !FileExists(hashesFile) {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(hashesFile), 0755); err != nil {
			return fmt.Errorf("failed to create directory for runonce_hashes: %w", err)
		}

		// Create empty file
		if _, err := os.Create(hashesFile); err != nil {
			return fmt.Errorf("failed to create runonce_hashes file: %w", err)
		}
	}

	// Check if the hash already exists in the file
	hashExists, err := hashExistsInFile(hashesFile, hash)
	if err != nil {
		return fmt.Errorf("failed to check hash existence: %w", err)
	}

	if hashExists {
		// Hash found, function already run before - do nothing
		return nil
	}

	// Hash not found, run the function
	if err := fn(); err != nil {
		return fmt.Errorf("runonceFunc(): function failed: %w", err)
	}

	// If function succeeds, add the hash to the list
	hashFile, err := os.OpenFile(hashesFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open runonce_hashes file: %w", err)
	}
	defer hashFile.Close()

	if _, err := hashFile.WriteString(hash + "\n"); err != nil {
		return fmt.Errorf("failed to write hash to file: %w", err)
	}

	return nil
}

// TextEditor opens the user's preferred text editor for the specified file
func TextEditor(filePath string) error {
	// Get the PI_APPS_DIR environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Read preferred editor setting
	settingsFile := filepath.Join(directory, "data", "settings", "Preferred text editor")

	var preferredEditor string
	if FileExists(settingsFile) {
		editorBytes, err := os.ReadFile(settingsFile)
		if err != nil {
			return fmt.Errorf("failed to read editor settings: %w", err)
		}
		preferredEditor = strings.TrimSpace(string(editorBytes))
	}

	// Map friendly name to binary name
	switch preferredEditor {
	case "Visual Studio Code":
		preferredEditor = "code"
	case "VSCodium":
		preferredEditor = "codium"
	}

	// Check if preferred editor exists, fall back to alternatives if not
	editors := []string{preferredEditor, "geany", "mousepad", "leafpad", "nano"}

	var editorFound bool
	for _, editor := range editors {
		if editor == "" {
			continue
		}

		_, err := exec.LookPath(editor)
		if err == nil {
			preferredEditor = editor
			editorFound = true
			break
		}
	}

	if !editorFound {
		return fmt.Errorf("text_editor(): no suitable text editor found")
	}

	// For terminal-based editors like nano, use terminal-run script
	if preferredEditor == "nano" {
		terminalRunPath := filepath.Join(directory, "etc", "terminal-run")
		cmd := exec.Command(terminalRunPath, fmt.Sprintf("nano \"%s\"", filePath), fmt.Sprintf("Editing %s", filepath.Base(filePath)))
		return cmd.Run()
	}

	// For GUI editors, launch with GTK_THEME and GDK_BACKEND unset
	cmd := exec.Command(preferredEditor, filePath)
	cmd.Env = os.Environ()

	// Filter out GTK_THEME and GDK_BACKEND from environment
	var newEnv []string
	for _, env := range cmd.Env {
		if !strings.HasPrefix(env, "GTK_THEME=") && !strings.HasPrefix(env, "GDK_BACKEND=") {
			newEnv = append(newEnv, env)
		}
	}
	cmd.Env = newEnv

	// Run in background
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Start()
}

// FilesMatch checks if two files have identical content
func FilesMatch(file1, file2 string) (bool, error) {
	// Check if both files exist
	if !FileExists(file1) {
		return false, fmt.Errorf("filesMatch: %s does not exist", file1)
	}
	if !FileExists(file2) {
		return false, fmt.Errorf("filesMatch: %s does not exist", file2)
	}

	// Read contents of both files
	content1, err := os.ReadFile(file1)
	if err != nil {
		return false, fmt.Errorf("filesMatch: failed to read %s: %w", file1, err)
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		return false, fmt.Errorf("filesMatch: failed to read %s: %w", file2, err)
	}

	// Compare contents
	return string(content1) == string(content2), nil
}
