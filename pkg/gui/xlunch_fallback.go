//go:build !cgo
// +build !cgo

package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runXlunchNativeMode runs the GUI in native xlunch mode (fallback to bash when CGO not available)
func (g *GUI) runXlunchNativeMode() error {
	// Fall back to bash implementation when CGO is not available
	fmt.Println("Native XLunch CGO not available, falling back to bash GUI")

	// Get the GUI mode to pass to bash
	guiMode := g.guiMode
	if guiMode == "" {
		guiMode = "xlunch-dark"
	}

	// Set environment variables that bash GUI expects
	env := os.Environ()
	env = append(env, fmt.Sprintf("DIRECTORY=%s", g.directory))

	// Run the bash GUI script with xlunch mode
	cmd := exec.Command(filepath.Join(g.directory, "gui"))
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Create a settings file for the GUI mode if it doesn't exist
	settingsDir := filepath.Join(g.directory, "data", "settings")
	os.MkdirAll(settingsDir, 0755)

	settingsFile := filepath.Join(settingsDir, "App List Style")
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		os.WriteFile(settingsFile, []byte(guiMode), 0644)
	} else {
		// Update settings file to use xlunch mode
		content, err := os.ReadFile(settingsFile)
		if err == nil {
			currentMode := strings.TrimSpace(string(content))
			if !strings.HasPrefix(currentMode, "xlunch") {
				os.WriteFile(settingsFile, []byte(guiMode), 0644)
			}
		}
	}

	return cmd.Run()
}
