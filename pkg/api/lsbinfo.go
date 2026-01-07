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

// Module: lsbinfo.go
// Description: A Go version of the lsb_release command to replace usage of the Python/Bash version of lsb_release.
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type LSBRelease struct {
	ID               string
	NAME             string
	PRETTY_NAME      string
	VERSION_ID       string
	VERSION_CODENAME string
}

func (lsb *LSBRelease) CapitalizeID() {
	if lsb.ID == "" {
		return
	}
	runes := []rune(lsb.ID)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		lsb.ID = string(runes)
	}
}

func (lsb *LSBRelease) AdjustIDFromName() {
	if lsb.NAME == "" {
		return
	}

	lowerID := strings.ToLower(lsb.ID)
	lowerName := strings.ToLower(lsb.NAME)

	if lowerID == lowerName {
		lsb.ID = lsb.NAME
	}
}

func LoadLSBOSRelease() *LSBRelease {
	lsb := &LSBRelease{}

	// Check for os-release files in order of preference
	osReleasePaths := []string{
		"/usr/lib/os-release",
		"/etc/os-release",
	}

	// Check environment variable
	if envPath := os.Getenv("LSB_OS_RELEASE"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			osReleasePaths = append([]string{envPath}, osReleasePaths...)
		}
	}

	var osReleasePath string
	for _, path := range osReleasePaths {
		if _, err := os.Stat(path); err == nil {
			osReleasePath = path
			break
		}
	}

	if osReleasePath == "" {
		return lsb
	}

	file, err := os.Open(osReleasePath)
	if err != nil {
		return lsb
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], `"'`)

		switch key {
		case "ID":
			lsb.ID = value
		case "NAME":
			lsb.NAME = value
		case "PRETTY_NAME":
			lsb.PRETTY_NAME = value
		case "VERSION_ID":
			lsb.VERSION_ID = value
		case "VERSION_CODENAME":
			lsb.VERSION_CODENAME = value
		}
	}

	return lsb
}

func DisplayLine(label, value string, shortFormat bool) {
	if value == "" {
		value = "n/a"
	}

	if shortFormat {
		fmt.Println(value)
	} else {
		fmt.Printf("%s:\t%s\n", label, value)
	}
}
