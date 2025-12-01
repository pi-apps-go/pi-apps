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

// Module: version.go
// Description: Provides functions obtaining the version. Only relevant for FFI based bindings.

package api

// Build-time variables
var (
	BuildDate string
	GitCommit string
)

func GetPiAppsGoApiVersion() string {
	if BuildDate == "" {
		BuildDate = "unknown"
	}
	return BuildDate
}

func GetPiAppsGoApiCommit() string {
	if GitCommit == "" {
		GitCommit = "unknown"
	}
	return GitCommit
}
