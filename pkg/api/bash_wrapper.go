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

// Module: bash_wrapper.go
// Description: Provides a function for running a command with the API bash wrapper loaded.

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunWithScriptWrappers runs a command with the API bash wrapper loaded
func RunWithScriptWrappers(cmd *exec.Cmd) error {
	// Get PI_APPS_DIR environment variable
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Path to the API bash wrapper - check both locations
	apiBashWrapper := filepath.Join(piAppsDir, "go-rewrite", "api")
	if _, err := os.Stat(apiBashWrapper); os.IsNotExist(err) {
		// Try alternate location
		apiBashWrapper = filepath.Join(piAppsDir, "api")
		if _, err := os.Stat(apiBashWrapper); os.IsNotExist(err) {
			return fmt.Errorf("API bash wrapper not found at %s or %s",
				filepath.Join(piAppsDir, "go-rewrite", "api"),
				filepath.Join(piAppsDir, "api"))
		}
	}

	// Create a temporary wrapper script that sources the API and then runs the original command
	tempDir, err := os.MkdirTemp("", "pi-apps-wrapper")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Check if this is a sudo command
	isSudo := len(cmd.Args) > 0 && cmd.Args[0] == "sudo"

	// Prepare the command arguments
	var commandArgs []string

	if isSudo {
		// Skip the sudo command itself from the original args
		commandArgs = cmd.Args[1:]
	} else {
		// For non-sudo commands, use the original command
		commandArgs = cmd.Args
	}

	// Build the command arguments string
	args := ""
	for i, arg := range commandArgs {
		if i > 0 {
			args += " "
		}
		// Escape any quotes in the arguments
		escapedArg := fmt.Sprintf("%q", arg)
		args += escapedArg
	}

	// Create the wrapper script content
	wrapperContent := fmt.Sprintf(`#!/bin/bash
# Source the Pi-Apps API
source "%s"

# Execute the original command with all arguments
"%s" %s
exit $?
`, apiBashWrapper, commandArgs[0], strings.Join(formatArgs(commandArgs[1:]), " "))

	// Write the wrapper script
	wrapperPath := filepath.Join(tempDir, "wrapper.sh")
	err = os.WriteFile(wrapperPath, []byte(wrapperContent), 0755)
	if err != nil {
		return fmt.Errorf("failed to create wrapper script: %w", err)
	}

	// Create a new command that runs our wrapper
	var wrapperCmd *exec.Cmd
	if isSudo {
		// For sudo commands, we use sudo on our wrapper script
		wrapperCmd = exec.Command("sudo", "-E", wrapperPath)
	} else {
		// For non-sudo commands, just run the wrapper directly
		wrapperCmd = exec.Command(wrapperPath)
	}

	// Copy all streams
	wrapperCmd.Stdout = cmd.Stdout
	wrapperCmd.Stderr = cmd.Stderr
	wrapperCmd.Stdin = cmd.Stdin

	// Copy working directory
	wrapperCmd.Dir = cmd.Dir

	// Ensure we preserve all environment variables from the original command
	if cmd.Env != nil {
		wrapperCmd.Env = cmd.Env
	} else {
		wrapperCmd.Env = os.Environ()
	}

	// Print debug info if in debug mode
	if os.Getenv("pi_apps_debug") == "true" {
		fmt.Fprintf(os.Stderr, "RunWithScriptWrappers: Running command through wrapper: %s\n", wrapperPath)
		fmt.Fprintf(os.Stderr, "RunWithScriptWrappers: Original command: %v\n", cmd.Args)
		fmt.Fprintf(os.Stderr, "RunWithScriptWrappers: Using API from: %s\n", apiBashWrapper)
		fmt.Fprintf(os.Stderr, "RunWithScriptWrappers: isSudo: %v\n", isSudo)
		fmt.Fprintf(os.Stderr, "RunWithScriptWrappers: Wrapper command: %v\n", wrapperCmd.Args)
	}

	// Run the wrapper command
	return wrapperCmd.Run()
}

// formatArgs properly formats and escapes command line arguments
func formatArgs(args []string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = fmt.Sprintf("%q", arg)
	}
	return result
}
