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

// Module: app_maint_vips.go
// Description: Provides functions manipulating images using the govips library if not stubbed out via the vips build tag.
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build vips

package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidbyttow/govips/v2/vips"
)

// GenerateAppIcons converts the given image into icon-24.png and icon-64.png files for the specified app
//
// This implementation uses the govips library for image processing and preserves the original aspect ratio
// of the image when resizing, similar to how ImageMagick would handle it in the bash implementation
func GenerateAppIcons(iconPath, appName string) error {
	// Get the PI_APPS_DIR environment variable
	directory := GetPiAppsDir()
	if directory == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Create the app directory if it doesn't exist
	appDir := filepath.Join(directory, "apps", appName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("error creating app directory: %w", err)
	}

	// Initialize govips
	vips.Startup(nil)
	defer vips.Shutdown()

	// Load the source image
	image, err := vips.NewImageFromFile(iconPath)
	if err != nil {
		return fmt.Errorf("error reading source image: %w", err)
	}
	defer image.Close()

	// Get original dimensions
	originalWidth := image.Width()
	originalHeight := image.Height()

	// Create a 24x24 icon (preserving aspect ratio)
	icon24Path := filepath.Join(appDir, "icon-24.png")

	// Clone the image for 24x24 processing
	image24, err := image.Copy()
	if err != nil {
		return fmt.Errorf("error copying image for 24x24 processing: %w", err)
	}
	defer image24.Close()

	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		err = image24.Resize(24.0/float64(originalHeight), vips.KernelLanczos3)
	} else {
		// Image is taller than wide, constrain by width
		err = image24.Resize(24.0/float64(originalWidth), vips.KernelLanczos3)
	}

	if err != nil {
		return fmt.Errorf("error resizing image to 24px: %w", err)
	}

	// Export as PNG
	image24bytes, _, err := image24.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("error exporting 24x24 icon: %w", err)
	}

	if err := os.WriteFile(icon24Path, image24bytes, 0644); err != nil {
		return fmt.Errorf("error saving 24x24 icon: %w", err)
	}

	// Create a 64x64 icon (preserving aspect ratio)
	icon64Path := filepath.Join(appDir, "icon-64.png")

	// Clone the original image for 64x64 processing
	image64, err := image.Copy()
	if err != nil {
		return fmt.Errorf("error copying image for 64x64 processing: %w", err)
	}
	defer image64.Close()

	if originalWidth >= originalHeight {
		// Image is wider than tall or square, constrain by height
		err = image64.Resize(64.0/float64(originalHeight), vips.KernelLanczos3)
	} else {
		// Image is taller than wide, constrain by width
		err = image64.Resize(64.0/float64(originalWidth), vips.KernelLanczos3)
	}

	if err != nil {
		return fmt.Errorf("error resizing image to 64px: %w", err)
	}

	// Export as PNG
	image64bytes, _, err := image64.ExportPng(vips.NewPngExportParams())
	if err != nil {
		return fmt.Errorf("error exporting 64x64 icon: %w", err)
	}

	if err := os.WriteFile(icon64Path, image64bytes, 0644); err != nil {
		return fmt.Errorf("error saving 64x64 icon: %w", err)
	}

	return nil
}
