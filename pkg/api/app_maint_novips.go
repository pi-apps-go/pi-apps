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

// Module: app_maint_novips.go
// Description: Provides stub functions for manipulating images for slimmer builds.
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !vips

package api

import (
	"fmt"
)

// GenerateAppIcons converts the given image into icon-24.png and icon-64.png files for the specified app
//
// This implementation uses the govips library for image processing and preserves the original aspect ratio
// of the image when resizing, similar to how ImageMagick would handle it in the bash implementation
func GenerateAppIcons(iconPath, appName string) error {
	return fmt.Errorf("GenerateAppIcons is stubbed out via the !vips build tag")
}
