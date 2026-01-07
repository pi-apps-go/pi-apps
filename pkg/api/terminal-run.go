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

// Module: terminal-run.go
// Description: Provides functions for starting a new GUI terminal (replicates terminal-run shell script behavior)
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Run starts a new terminal window on the host OS, sets its title,
// executes the provided command, and blocks until the terminal exits.
func TerminalRun(cmd string, title string) error {
	switch runtime.GOOS {
	case "linux":
		return runLinux(cmd, title)
	case "darwin":
		return runDarwin(cmd, title)
	case "windows":
		return runWindows(cmd, title)
	default:
		return fmt.Errorf("terminal-run: unsupported OS %s", runtime.GOOS)
	}
}

var terminalsLinux = []string{
	"lxterminal",
	"xfce4-terminal",
	"mate-terminal",
	"lxterm",
	"uxterm",
	"xterm",
	"urxvt",
	"konsole",
	"terminator",
	"ptyxis",
	"gnome-terminal",
	"gnome-terminal.wrapper",
	"tilix",
	"tilix.wrapper",
	"qterminal",
	"alacritty",
	"kitty",
}

func findTerminalLinux() (binaryPath string, terminalName string, err error) {
	// Try x-terminal-emulator first
	if p, err := exec.LookPath("x-terminal-emulator"); err == nil {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			realName := filepath.Base(resolved)

			for _, t := range terminalsLinux {
				if t == realName {
					binaryPath = resolved
					terminalName = realName

					// IMPORTANT: wrapper fix
					if realName == "gnome-terminal.wrapper" {
						if real, err := exec.LookPath("gnome-terminal"); err == nil {
							binaryPath = real
							terminalName = "gnome-terminal"
						}
					}

					return binaryPath, terminalName, nil
				}
			}
		}
	}

	// Fallback search
	for _, t := range terminalsLinux {
		if path, err := exec.LookPath(t); err == nil {
			// If it's the wrapper, replace with real GNOME terminal
			if t == "gnome-terminal.wrapper" {
				if real, err := exec.LookPath("gnome-terminal"); err == nil {
					return real, "gnome-terminal", nil
				}
			}
			return path, t, nil
		}
	}

	return "", "", errors.New("no supported Linux terminal found")
}

// waitForPidFile waits for a PID file to appear and returns the PID.
// Times out after maxWait seconds.
func waitForPidFile(pidFilePath string, maxWait int) (int, error) {
	for i := 0; i < maxWait; i++ {
		if _, err := os.Stat(pidFilePath); err == nil {
			// File exists, read the PID
			data, err := os.ReadFile(pidFilePath)
			if err != nil {
				return 0, fmt.Errorf("failed to read PID file: %w", err)
			}
			pidStr := strings.TrimSpace(string(data))
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				return 0, fmt.Errorf("invalid PID in file: %s", pidStr)
			}
			return pid, nil
		}
		time.Sleep(1 * time.Second)
	}

	// Check for common /tmp issues like the shell script does
	if _, err := os.Stat("/tmp"); os.IsNotExist(err) {
		return 0, errors.New("terminal-run: Terminal failed to launch because your /tmp directory is missing")
	}

	// Try to create a file in /tmp to check permissions
	testFile := filepath.Join("/tmp", "terminalrun_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return 0, fmt.Errorf("terminal-run: Terminal failed to launch due to bad permissions in your /tmp directory")
	}
	os.Remove(testFile)

	return 0, errors.New("terminal-run: No terminal detected as it never created the PID file within 10 seconds")
}

// waitForProcessExit polls /proc/{pid} until the process no longer exists
func waitForProcessExit(pid int) {
	procPath := fmt.Sprintf("/proc/%d", pid)
	for {
		if _, err := os.Stat(procPath); os.IsNotExist(err) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func runLinux(userCmd, title string) error {
	termBinary, termName, err := findTerminalLinux()
	if err != nil {
		return err
	}

	// Create a unique temp file path for PID tracking (matching shell script behavior)
	tempPidFile, err := os.CreateTemp("", "terminalrun_pid_*")
	if err != nil {
		return fmt.Errorf("failed to create temp PID file: %w", err)
	}
	tempPidPath := tempPidFile.Name()
	tempPidFile.Close()
	os.Remove(tempPidPath) // Remove so the terminal's bash can create it
	defer os.Remove(tempPidPath)

	// Inject PID tracking and title setting (matching shell script behavior exactly)
	// The shell script does: echo $$ > $temp_pid_file followed by title setting
	injected := fmt.Sprintf("echo $$ > %s; echo -ne '\\e]0;%s\\a'; %s", tempPidPath, title, userCmd)

	var args []string
	var scriptFile string // For terminals that need a script file

	switch termName {
	case "lxterminal", "lxterm", "uxterm", "xterm", "urxvt":
		args = []string{"-e", "bash", "-c", injected}

	case "xfce4-terminal", "mate-terminal", "terminator":
		args = []string{"-x", "bash", "-c", injected}

	case "ptyxis":
		args = []string{"--", "bash", "-c", injected}

	case "gnome-terminal", "gnome-terminal.wrapper":
		args = []string{"--", "bash", "-c", injected}

	case "konsole":
		// Konsole needs a script file passed via process substitution
		scriptFile = filepath.Join(os.TempDir(), fmt.Sprintf("terminalrun_script_%d.sh", os.Getpid()))
		if err := os.WriteFile(scriptFile, []byte("#!/bin/bash\n"+injected), 0755); err != nil {
			return fmt.Errorf("failed to create script file: %w", err)
		}
		defer os.Remove(scriptFile)
		args = []string{"-e", "bash", scriptFile}

	case "qterminal":
		// qterminal also needs a script file
		scriptFile = filepath.Join(os.TempDir(), fmt.Sprintf("terminalrun_script_%d.sh", os.Getpid()))
		if err := os.WriteFile(scriptFile, []byte("#!/bin/bash\n"+injected), 0755); err != nil {
			return fmt.Errorf("failed to create script file: %w", err)
		}
		defer os.Remove(scriptFile)
		args = []string{"-e", "bash", scriptFile}

	case "tilix", "tilix.wrapper":
		args = []string{"-e", "bash", "-c", injected}

	case "alacritty":
		args = []string{"--command", "bash", "-c", injected}

	case "kitty":
		args = []string{"bash", "-c", injected}

	default:
		return fmt.Errorf("unsupported terminal: %s", termName)
	}

	cmd := exec.Command(termBinary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start the terminal (don't wait - many terminals fork and exit immediately)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start terminal: %w", err)
	}

	// Don't block on cmd.Wait() in main goroutine for forking terminals
	// Instead, wait in a goroutine to clean up the process
	go func() {
		cmd.Wait()
	}()

	// Wait for the PID file to appear (up to 10 seconds, like shell script)
	pid, err := waitForPidFile(tempPidPath, 10)
	if err != nil {
		return err
	}

	// Wait for the actual bash process inside the terminal to exit
	waitForProcessExit(pid)

	return nil
}

func runDarwin(userCmd string, title string) error {
	// Prefer iTerm if installed
	if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
		script := fmt.Sprintf(`
tell application "iTerm"
    activate
    tell current window
        create tab with default profile
        tell current session to write text "printf '\\e]0;%s\\a'; %s"
    end tell
end tell`, title, userCmd)

		cmd := exec.Command("osascript", "-e", script)
		return cmd.Run()
	}

	// Otherwise use Terminal.app
	script := fmt.Sprintf(`
tell application "Terminal"
    activate
    do script "printf '\\e]0;%s\\a'; %s"
end tell`, title, userCmd)

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func runWindows(userCmd string, title string) error {
	// Prefer Windows Terminal if available
	if _, err := exec.LookPath("wt.exe"); err == nil {
		cmd := exec.Command("wt.exe", "powershell", "-NoExit", "-Command",
			fmt.Sprintf("$Host.UI.RawUI.WindowTitle='%s'; %s", title, userCmd))
		return cmd.Run()
	}

	// Fallback to PowerShell
	if _, err := exec.LookPath("powershell.exe"); err == nil {
		cmd := exec.Command("powershell.exe", "-NoExit", "-Command",
			fmt.Sprintf("$Host.UI.RawUI.WindowTitle='%s'; %s", title, userCmd))
		return cmd.Run()
	}

	// Last fallback: cmd.exe
	cmd := exec.Command("cmd.exe", "/k", fmt.Sprintf("title %s & %s", title, userCmd))
	return cmd.Run()
}
