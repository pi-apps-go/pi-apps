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

// Module: dummy_repo.go
// Description: Provides dummy functions for managing repositories if no package manager build tag is set.

//go:build dummy

package api

import (
	"fmt"
)

// AnythingInstalledFromURISuiteComponent checks if any packages from a specific repository
// (identified by URI, suite, and optional component) are currently installed.
//
//	false - no packages are installed from the repository
//	true - at least one package is installed from the repository
//	error - error if repository URI, suite, or component is not specified
func AnythingInstalledFromURISuiteComponent(uri, suite, component string) (bool, error) {
	Debug(fmt.Sprintf("Checking if anything is installed from %s %s %s", uri, suite, component))

	// assume false if no package manager build tag is set
	return false, nil
}

// RemoveRepofileIfUnused removes a sources.list.d file if nothing from that repository is currently installed.
//
// If testMode is "test", it only outputs the status without removing anything.
//
//	error - error if file is not specified or testMode is not "test"
func RemoveRepofileIfUnused(file, testMode, key string) error {
	// return success if no package manager build tag is set
	return nil
}

// Helper function to handle .list files
func handleListFile(file string) (bool, error) {
	// return false if no package manager build tag is set
	return false, nil
}

// Helper function to handle .sources files
func handleSourcesFile(file string) (bool, error) {
	// return false if no package manager build tag is set
	return false, nil
}

// Helper function to get the list of all installed packages
func getInstalledPackages() ([]string, error) {
	// return empty slice if no package manager build tag is set
	return []string{}, nil
}

// Helper function to get the list of packages in a repo file
func getPackagesInRepo(repoFile string) ([]string, error) {
	// return empty slice if no package manager build tag is set
	return []string{}, nil
}

// Helper function to check if any packages are installed from a specific repo
func checkIfPackagesInstalledFromRepo(packages []string, uri, suite, component string) (bool, error) {
	// return false if no package manager build tag is set
	return false, nil
}
