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

// Module: xlunch_native.go
// Description: Native Go implementation of xlunch-style grid interface
// This replaces external xlunch dependency with native libX11 and image processing

//go:build cgo
// +build cgo

package gui

/*
#cgo pkg-config: x11 xft imlib2
#cgo LDFLAGS: -lX11 -lXft -lImlib2 -lm

#include "xlunch_native.h"
*/
import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/botspot/pi-apps/pkg/api"
	"github.com/davidbyttow/govips/v2/vips"
)

// XLunchNativeConfig holds configuration for native xlunch
type XLunchNativeConfig struct {
	Width       int
	Height      int
	IconSize    int
	Theme       string // "light", "dark", "3d-light", "3d-dark"
	Title       string
	Background  string
	Transparent bool
}

// XLunchNativeEntry represents an app entry for native xlunch
type XLunchNativeEntry struct {
	Title    string
	IconPath string
	Command  string
	IsDir    bool
}

// XLunchNative wraps the native C implementation
type XLunchNative struct {
	config  XLunchNativeConfig
	entries []XLunchNativeEntry
	handle  *C.XLunchNative
	running bool
}

type XLunchEvent struct {
	Type     int
	Selected int
}

// DefaultXLunchNativeConfig returns default configuration
func DefaultXLunchNativeConfig() XLunchNativeConfig {
	return XLunchNativeConfig{
		Width:       800,
		Height:      600,
		IconSize:    64,
		Theme:       "dark",
		Title:       "Pi-Apps: Raspberry Pi app store",
		Background:  "",
		Transparent: false,
	}
}

// NewXLunchNative creates a new native xlunch instance
func NewXLunchNative(config XLunchNativeConfig) *XLunchNative {
	return &XLunchNative{
		config:  config,
		entries: make([]XLunchNativeEntry, 0),
		running: false,
	}
}

// SetEntries sets the application entries
func (xl *XLunchNative) SetEntries(entries []XLunchNativeEntry) {
	xl.entries = entries
}

// SetThemeColors sets the xlunch theme colors
func (xl *XLunchNative) SetThemeColors(theme string) {
	if xl.handle != nil {
		themeC := C.CString(theme)
		C.xlunch_native_set_theme_colors(xl.handle, themeC)
		C.free(unsafe.Pointer(themeC))
	}
}

// AddEntry adds a single entry
func (xl *XLunchNative) AddEntry(entry XLunchNativeEntry) {
	xl.entries = append(xl.entries, entry)
}

// LoadEntriesFromPreload loads entries using the preload system
func (xl *XLunchNative) LoadEntriesFromPreload(directory, prefix string) error {
	// Use the existing preload system
	list, err := PreloadAppList(directory, prefix)
	if err != nil {
		return fmt.Errorf("failed to preload app list: %w", err)
	}

	// Convert AppListItems to XLunchNativeEntries
	xl.entries = make([]XLunchNativeEntry, 0, len(list.Items))
	for _, item := range list.Items {
		entry := XLunchNativeEntry{
			Title:    item.Name,
			IconPath: item.IconPath,
			Command:  item.Path,
			IsDir:    item.Type == "category", // Only categories are directories for navigation
		}
		xl.entries = append(xl.entries, entry)
	}

	return nil
}

// ProcessIcon processes an icon to the required size and format using govips
func (xl *XLunchNative) ProcessIcon(iconPath string) (string, error) {
	if iconPath == "" {
		return "", nil
	}

	// Check if icon exists
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		return "", nil
	}

	// Create cache directory
	tempDir := filepath.Join(os.TempDir(), "pi-apps-xlunch-icons")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return iconPath, nil // Fallback to original on error
	}

	// Generate cache filename based on icon path and size
	hasher := fmt.Sprintf("%x", iconPath)
	outputPath := filepath.Join(tempDir, fmt.Sprintf("icon_%d_%s.png",
		xl.config.IconSize, hasher[:8]))

	// Check if already processed
	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	// Load image with govips
	image, err := vips.NewImageFromFile(iconPath)
	if err != nil {
		// If govips fails, try to copy original file to a temporary location
		return xl.copyIconToTemp(iconPath, tempDir)
	}
	defer image.Close()

	// Resize image with proper aspect ratio
	err = image.Resize(float64(xl.config.IconSize)/float64(image.Width()), vips.KernelAuto)
	if err != nil {
		return xl.copyIconToTemp(iconPath, tempDir)
	}

	// Crop to square if needed
	if image.Width() != image.Height() {
		size := xl.config.IconSize
		err = image.ExtractArea(0, 0, size, size)
		if err != nil {
			return xl.copyIconToTemp(iconPath, tempDir)
		}
	}

	// Export as PNG
	ep := vips.NewDefaultPNGExportParams()
	imageBytes, _, err := image.Export(ep)
	if err != nil {
		return xl.copyIconToTemp(iconPath, tempDir)
	}

	// Save processed image
	if err := os.WriteFile(outputPath, imageBytes, 0644); err != nil {
		return xl.copyIconToTemp(iconPath, tempDir)
	}

	return outputPath, nil
}

// copyIconToTemp copies an icon to temp directory as fallback
func (xl *XLunchNative) copyIconToTemp(iconPath, tempDir string) (string, error) {
	outputPath := filepath.Join(tempDir, fmt.Sprintf("fallback_%s", filepath.Base(iconPath)))

	// Simple file copy as fallback
	sourceFile, err := os.Open(iconPath)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}

// Helper function to convert bool to int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateEntries updates the content seamlessly without recreating the window
func (xl *XLunchNative) UpdateEntries(entries []XLunchNativeEntry) {
	xl.entries = entries
}

// draw renders the entries on the screen with enhanced visual styling
func (xl *XLunchNative) draw(entries []XLunchNativeEntry) {
	if xl.handle == nil {
		return
	}

	// Clear the window
	C.xlunch_native_clear(xl.handle)

	// Note: Background drawing would be handled by the C implementation
	// For now, the C code should handle background rendering during initialization

	// Draw Pi-Apps branding at the top
	xl.drawBranding()

	// Calculate layout for entries (starting lower to accommodate branding)
	cols := int(xl.handle.cols)
	if cols <= 0 {
		cols = 1
	}

	cellWidth := int(xl.handle.cell_width)
	cellHeight := int(xl.handle.cell_height)
	padding := int(xl.handle.padding)

	// Draw entries
	for i, entry := range entries {
		if i >= cols*int(xl.handle.rows) {
			break // Don't overflow visible area
		}

		row := i / cols
		col := i % cols

		x := padding/2 + col*cellWidth
		y := padding/2 + row*cellHeight

		// Draw icon (keep original positioning for now)
		cTitle := C.CString(entry.Title)
		C.xlunch_native_draw_icon(xl.handle, C.int(x), C.int(y),
			C.int(xl.config.IconSize), C.int(xl.config.IconSize), cTitle)
		C.free(unsafe.Pointer(cTitle))

		// Draw title below icon
		titleY := y + xl.config.IconSize + 15
		cTitleText := C.CString(entry.Title)
		C.xlunch_native_draw_text(xl.handle, C.int(x), C.int(titleY), cTitleText, 0)
		C.free(unsafe.Pointer(cTitleText))
	}
}

// drawBranding draws the Pi-Apps branding at the top
func (xl *XLunchNative) drawBranding() {
	// For now, we'll keep the branding simple since we need to enhance the C code
	// to support full image rendering and advanced graphics

	// The background image and advanced styling will be handled by
	// enhancing the C implementation later
}

// drawXLunchExactStyle draws the interface exactly like the original xlunch
func (xl *XLunchNative) drawXLunchExactStyle(entries []XLunchNativeEntry, theme, prefix string) {
	if xl.handle == nil {
		logger.Error("xlunch handle is nil, cannot draw")
		return
	}

	// Clear the window first - this will show the solid dark background set in window attributes
	C.xlunch_native_clear(xl.handle)

	// Draw background image based on theme (exactly like original xlunch)
	var backgroundPath string
	switch theme {
	case "light-3d":
		exeDir := filepath.Dir(os.Args[0])
		backgroundPath = filepath.Join(filepath.Dir(exeDir), "icons", "background-3d.png")
	case "dark-3d":
		exeDir := filepath.Dir(os.Args[0])
		backgroundPath = filepath.Join(filepath.Dir(exeDir), "icons", "background-3d-dark.png")
	default: // transparent dark
		// No background image - use transparent overlay
	}

	if backgroundPath != "" {
		bgPathC := C.CString(backgroundPath)
		C.xlunch_native_draw_background_image(xl.handle, bgPathC)
		C.free(unsafe.Pointer(bgPathC))
	} else {
		// Draw semi-transparent dark background like original xlunch
		C.xlunch_native_draw_background(xl.handle)
	}

	// Draw Pi-Apps logo and buttons exactly like original xlunch
	xl.drawXLunchButtons(theme, prefix)

	// Calculate grid layout exactly like original xlunch
	iconSize := 64 // xlunch uses -s 64
	cols := 1
	if xl.config.Width >= 550 {
		cols = 2
	}

	// xlunch uses specific spacing and positioning
	cellWidth := xl.config.Width / cols
	cellHeight := iconSize + 40 // Space for icon + text
	padding := 20               // xlunch padding

	// Update handle's grid info for event handling - MUST match drawing logic
	xl.handle.cols = C.int(cols)
	xl.handle.cell_width = C.int(cellWidth)
	xl.handle.cell_height = C.int(cellHeight)
	xl.handle.padding = C.int(padding)
	xl.handle.num_entries = C.int(len(entries)) // Set actual entry count for bounds checking

	// Set entries count for scrolling calculations (like xlunch)
	xl.handle.entries_count = C.int(len(entries))

	// Start position after header (logo and buttons)
	startY := 140 // Account for logo and search area

	// Calculate visible area for scrolling
	maxVisibleEntries := cols * ((xl.config.Height - startY) / cellHeight)

	// Draw scrollbar if needed (like xlunch)
	if len(entries) > maxVisibleEntries {
		C.xlunch_native_draw_scrollbar(xl.handle)
	}

	// Draw entries in xlunch-style grid with scroll support
	entriesDrawn := 0
	scrollOffset := int(xl.handle.scrolled_past) // Get current scroll position

	for i, entry := range entries {
		if entriesDrawn >= maxVisibleEntries {
			break // Don't overflow visible area
		}

		col := i % cols
		row := i / cols

		// Skip entries that are scrolled past (like xlunch)
		if row < scrollOffset {
			continue
		}

		// Skip entries that are below visible area
		displayRow := row - scrollOffset
		if displayRow >= ((xl.config.Height - startY) / cellHeight) {
			break
		}

		// Use exact same positioning logic as event handling expects
		x := col*cellWidth + (cellWidth-iconSize)/2
		y := startY + displayRow*cellHeight

		// Bounds check
		if y+iconSize+20 > xl.config.Height {
			break
		}

		// Draw icon with exact xlunch styling
		iconPath := entry.IconPath
		if iconPath == "" {
			iconPath = entry.Title // Use title for color coding
		}

		// Safety check for string conversion
		if len(iconPath) > 0 {
			iconPathC := C.CString(iconPath)
			// Use icon size directly like original xlunch does
			C.xlunch_native_draw_icon(xl.handle, C.int(x), C.int(y),
				C.int(iconSize), C.int(iconSize), iconPathC)
			C.free(unsafe.Pointer(iconPathC))
		}

		// Draw title exactly like xlunch (centered below icon)
		if len(entry.Title) > 0 {
			titleC := C.CString(entry.Title)
			// Center text under icon (xlunch calculation)
			textWidth := len(entry.Title) * 7 // Approximate character width
			textX := x + (iconSize-textWidth)/2
			if textX < 5 {
				textX = 5 // Minimum margin
			}
			C.xlunch_native_draw_text(xl.handle, C.int(textX), C.int(y+iconSize+15), titleC, C.int(0))
			C.free(unsafe.Pointer(titleC))
		}

		entriesDrawn++
	}

	// Flush the display to ensure all drawing is visible
	// Note: The double buffering is handled internally by the C code
}

// drawXLunchButtons draws the logo and settings buttons exactly like original xlunch
func (xl *XLunchNative) drawXLunchButtons(theme, prefix string) {
	// Get the correct icons directory path
	exeDir := filepath.Dir(os.Args[0])
	iconsDir := filepath.Join(filepath.Dir(exeDir), "icons")

	// Draw logo buttons exactly like original xlunch (matching bash script logic)
	switch theme {
	case "light-3d":
		// Light 3D theme uses large banner logo
		logoPath := filepath.Join(iconsDir, "logo-3d.png")
		logoX := (xl.config.Width / 2) - (300 / 2)
		logoY := 0

		// Update logo button coordinates
		xl.handle.logo_x = C.int(logoX)
		xl.handle.logo_y = C.int(logoY)
		xl.handle.logo_w = C.int(300)
		xl.handle.logo_h = C.int(60)

		if logoPath != "" {
			logoPathC := C.CString(logoPath)
			C.xlunch_native_draw_button_with_hover(xl.handle, C.int(logoX), C.int(logoY), C.int(300), C.int(60), logoPathC, xl.handle.logo_button_hovered)
			C.free(unsafe.Pointer(logoPathC))
		}

	case "dark-3d":
		// Dark 3D theme uses large banner logo
		logoPath := filepath.Join(iconsDir, "logo-3d-dark.png")
		logoX := (xl.config.Width / 2) - (300 / 2)
		logoY := 0

		// Update logo button coordinates
		xl.handle.logo_x = C.int(logoX)
		xl.handle.logo_y = C.int(logoY)
		xl.handle.logo_w = C.int(300)
		xl.handle.logo_h = C.int(60)

		if logoPath != "" {
			logoPathC := C.CString(logoPath)
			C.xlunch_native_draw_button_with_hover(xl.handle, C.int(logoX), C.int(logoY), C.int(300), C.int(60), logoPathC, xl.handle.logo_button_hovered)
			C.free(unsafe.Pointer(logoPathC))
		}

	default: // transparent theme
		// For transparent theme, draw both logo icons exactly like bash script

		// Main logo (144x144 centered)
		logoPath := filepath.Join(iconsDir, "logo-144.png")
		logoX := (xl.config.Width / 2) - (144 / 2)
		logoY := 0

		if logoPath != "" {
			logoPathC := C.CString(logoPath)
			C.xlunch_native_draw_button_with_hover(xl.handle, C.int(logoX), C.int(logoY), C.int(144), C.int(144), logoPathC, C.int(0)) // Main logo not clickable
			C.free(unsafe.Pointer(logoPathC))
		}

		// Logo text (positioned based on prefix like bash script) - this is the clickable part
		logoTextPath := filepath.Join(iconsDir, "logo-text.png")
		var logoTextX, logoTextY int
		if prefix == "" {
			logoTextX = 45
			logoTextY = 10
		} else {
			logoTextX = 65
			logoTextY = 0
		}

		// Update logo button coordinates for text logo
		xl.handle.logo_x = C.int(logoTextX)
		xl.handle.logo_y = C.int(logoTextY)
		xl.handle.logo_w = C.int(245)
		xl.handle.logo_h = C.int(100)

		if logoTextPath != "" {
			logoTextPathC := C.CString(logoTextPath)
			// Use these dimensions: 245x100 pixels, reason is to not make it look stretched and to match like the original xlunch's image scaling properties
			C.xlunch_native_draw_button_with_hover(xl.handle, C.int(logoTextX), C.int(logoTextY), C.int(245), C.int(100), logoTextPathC, xl.handle.logo_button_hovered)
			C.free(unsafe.Pointer(logoTextPathC))
		}
	}

	// Draw settings button (exactly like original xlunch)
	var settingsPath string
	switch theme {
	case "light-3d":
		settingsPath = filepath.Join(iconsDir, "settings-dark.png")
	default:
		settingsPath = filepath.Join(iconsDir, "settings-light.png")
	}

	// Fine-tuned settings button position for better visual alignment
	settingsX := xl.config.Width - 150 // Move closer to right edge
	settingsY := 30                    // Move slightly higher

	// Update settings button coordinates
	xl.handle.settings_x = C.int(settingsX)
	xl.handle.settings_y = C.int(settingsY)
	xl.handle.settings_w = C.int(140)
	xl.handle.settings_h = C.int(52)

	if settingsPath != "" {
		settingsPathC := C.CString(settingsPath)
		// Use smaller, more proportional size (original is 120x32, scale down to ~30x24)
		C.xlunch_native_draw_button_with_hover(xl.handle, C.int(settingsX), C.int(settingsY), C.int(140), C.int(52), settingsPathC, xl.handle.settings_button_hovered)
		C.free(unsafe.Pointer(settingsPathC))
	}

	// Draw search prompt exactly like xlunch
	searchText := C.CString("Search: ")
	C.xlunch_native_draw_text(xl.handle, C.int(20), C.int(100), searchText, C.int(0))
	C.free(unsafe.Pointer(searchText))
}

// drawXLunchLike draws the interface in xlunch-like style (legacy method)
func (xl *XLunchNative) drawXLunchLike(entries []XLunchNativeEntry) {
	if xl.handle == nil {
		return
	}

	// Clear the window first
	C.xlunch_native_clear(xl.handle)

	// Draw xlunch-style background (semi-transparent dark overlay)
	C.xlunch_native_draw_background(xl.handle)

	// Draw Pi-Apps logo at the top center
	logoText := C.CString("Pi-Apps")
	C.xlunch_native_draw_text(xl.handle, C.int(xl.config.Width/2-50), C.int(40), logoText, C.int(1))
	C.free(unsafe.Pointer(logoText))

	// Calculate grid layout like real xlunch - centered and responsive to icon size
	iconSize := xl.config.IconSize
	iconSpacing := iconSize + 20 // Spacing between icons
	textHeight := 20             // Space for text below icons
	cellHeight := iconSize + textHeight

	headerHeight := 80 // Space for logo at top
	footerHeight := 20 // Space at bottom

	// Calculate how many can fit
	availableWidth := xl.config.Width - 40 // Leave margins
	availableHeight := xl.config.Height - headerHeight - footerHeight

	maxCols := availableWidth / iconSpacing
	if maxCols < 1 {
		maxCols = 1
	}
	maxRows := availableHeight / cellHeight
	if maxRows < 1 {
		maxRows = 1
	}

	// Limit entries to what fits on screen
	maxEntries := maxCols * maxRows
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	// Calculate actual grid size for centering
	actualCols := maxCols
	if len(entries) < maxCols {
		actualCols = len(entries)
	}
	actualRows := (len(entries) + maxCols - 1) / maxCols // Ceiling division

	// Center the grid
	gridWidth := actualCols * iconSpacing
	gridHeight := actualRows * cellHeight
	startX := (xl.config.Width - gridWidth) / 2
	startY := headerHeight + (availableHeight-gridHeight)/2

	// Draw entries in centered grid
	for i, entry := range entries {
		col := i % maxCols
		row := i / maxCols

		x := startX + col*iconSpacing
		y := startY + row*cellHeight

		// Draw icon
		iconPath := entry.IconPath
		if iconPath == "" {
			iconPath = entry.Title // Use title for color coding
		}

		iconPathC := C.CString(iconPath)
		C.xlunch_native_draw_icon(xl.handle, C.int(x), C.int(y),
			C.int(iconSize), C.int(iconSize), iconPathC)
		C.free(unsafe.Pointer(iconPathC))

		// Draw title centered below icon
		titleC := C.CString(entry.Title)
		// Center text under icon (better calculation)
		textWidth := len(entry.Title) * 7 // Approximate character width
		textX := x + (iconSize-textWidth)/2
		if textX < 0 {
			textX = 0
		}
		C.xlunch_native_draw_text(xl.handle, C.int(textX), C.int(y+iconSize+15), titleC, C.int(0))
		C.free(unsafe.Pointer(titleC))
	}
}

// Close cleanup the native xlunch
func (xl *XLunchNative) Close() {
	if xl.handle != nil {
		C.xlunch_native_cleanup(xl.handle)
		xl.handle = nil
	}
	xl.running = false

	// Clean up temp icons
	tempDir := filepath.Join(os.TempDir(), "pi-apps-xlunch-icons")
	os.RemoveAll(tempDir)
}

// runXlunchNativeMode runs the native xlunch implementation (mimics xlunch exactly)
func (g *GUI) runXlunchNativeMode() error {
	logger.Info("Starting native xlunch mode")

	// Get theme from settings
	theme := "dark" // default
	if themeFile := filepath.Join(g.directory, "data", "settings", "App List Style"); api.FileExists(themeFile) {
		if content, err := os.ReadFile(themeFile); err == nil {
			style := strings.TrimSpace(string(content))
			if strings.HasPrefix(style, "xlunch-") {
				theme = strings.TrimPrefix(style, "xlunch-")
			}
		}
	}

	// Create native xlunch instance
	config := DefaultXLunchNativeConfig()
	config.Theme = theme
	config.Width = 800
	config.Height = 700

	xl := NewXLunchNative(config)
	defer xl.Close()

	// Initialize native C implementation
	xl.handle = C.xlunch_native_init(C.int(config.Width), C.int(config.Height), C.int(config.IconSize))
	if xl.handle == nil {
		return fmt.Errorf("failed to initialize native xlunch - check X11 display")
	}

	logger.Info("Native xlunch initialized successfully")

	// Set theme colors
	xl.SetThemeColors(theme)
	logger.Info(fmt.Sprintf("Theme colors set: %s", theme))

	// Load app entries
	if err := xl.LoadEntriesFromPreload(g.directory, ""); err != nil {
		return fmt.Errorf("failed to load app entries: %w", err)
	}

	logger.Info(fmt.Sprintf("Loaded %d app entries", len(xl.entries)))

	// Show the window
	C.xlunch_native_show(xl.handle)

	// Initial draw
	xl.drawXLunchExactStyle(xl.entries, theme, "")
	needsRedraw := false
	lastRedrawTime := time.Now()
	redrawThrottleMs := 100 // Minimum time between redraws to prevent flicker (increased)

	// Main event loop (exactly like xlunch)
	// logger.Info("Entering event loop")
	for {
		// Only redraw when necessary and throttle redraws to prevent flickering
		if needsRedraw {
			// Throttle redraws to prevent excessive flickering
			now := time.Now()
			if now.Sub(lastRedrawTime) >= time.Duration(redrawThrottleMs)*time.Millisecond {
				xl.drawXLunchExactStyle(xl.entries, theme, "")
				needsRedraw = false
				lastRedrawTime = now
			}
		}

		var selectedEntryC C.int = -1
		result := int(C.xlunch_native_handle_events(xl.handle, &selectedEntryC))
		selectedEntry := int(selectedEntryC)

		// logger.Debug(fmt.Sprintf("Event loop: result=%d, selectedEntry=%d", result, selectedEntry))

		switch result {
		case -3: // Logo button clicked
			logger.Info("Pi-Apps logo clicked - opening project homepage")
			gitURL := g.getGitURL()
			if gitURL != "" {
				// Open the git URL in default browser
				cmd := exec.Command("xdg-open", gitURL)
				if err := cmd.Start(); err != nil {
					logger.Error(fmt.Sprintf("Failed to open URL %s: %v", gitURL, err))
				} else {
					logger.Info(fmt.Sprintf("Opened %s in browser", gitURL))
				}
			}
			continue // Stay in the interface

		case -2: // Settings button clicked
			logger.Info("Settings button clicked - opening settings")
			if err := g.runSettings(); err != nil {
				logger.Error(fmt.Sprintf("Failed to run settings: %v", err))
			} else {
				logger.Info("Settings completed")
			}
			// Don't trigger redraw - settings changes will be picked up on next natural redraw
			continue // Stay in the interface

		case 0: // Quit or app selected
			if selectedEntry >= 0 && selectedEntry < len(xl.entries) {
				entry := xl.entries[selectedEntry]
				logger.Info(fmt.Sprintf("Entry selected: %s (IsDir: %v)", entry.Title, entry.IsDir))

				if entry.IsDir {
					// Category selected - reload with category prefix
					logger.Info(fmt.Sprintf("Loading category: %s", entry.Title))
					if err := xl.LoadEntriesFromPreload(g.directory, entry.Title); err != nil {
						logger.Error(fmt.Sprintf("Failed to load category %s: %v", entry.Title, err))
						continue
					}
					// Reset scroll position when entering a new category
					C.xlunch_native_set_scroll_level(xl.handle, 0)
					needsRedraw = true
					continue // Continue the event loop
				} else {
					// Non-directory entry selected (app, back, etc.)
					if entry.Title == "Back" {
						// Handle back navigation
						logger.Info("Back button clicked - navigating to parent")
						// Navigate to parent directory (entry.Command contains the parent path)
						parentPath := strings.TrimSuffix(entry.Command, "/")
						if err := xl.LoadEntriesFromPreload(g.directory, parentPath); err != nil {
							logger.Error(fmt.Sprintf("Failed to load parent directory %s: %v", parentPath, err))
							continue
						}
						needsRedraw = true
						continue // Continue the event loop
					} else {
						// Regular app selected - show details
						logger.Info(fmt.Sprintf("Showing details for app: %s", entry.Title))
						g.showAppDetails(entry.Title)
						// Stay in current view (don't reload to main)
						needsRedraw = true
						continue // Continue the event loop
					}
				}
			} else if selectedEntry >= len(xl.entries) {
				// Invalid selection - continue without quitting
				logger.Info(fmt.Sprintf("Invalid selection: %d (max: %d)", selectedEntry, len(xl.entries)))
				continue
			} else {
				// Quit without selection (selectedEntry == -1)
				return nil
			}
		case 2: // Redraw needed (including hover state changes)
			// For hover changes, we don't need immediate full redraws
			// Let the throttling handle this to prevent flicker
			needsRedraw = true
			continue
		case 1: // Continue (no events)
			// Small sleep to prevent busy waiting
			time.Sleep(10 * time.Millisecond)
			continue
		default:
			// Handle errors
			if result == -1 {
				logger.Error("xlunch native error occurred")
				return fmt.Errorf("xlunch native error occurred")
			}
			// Unknown result, continue anyway
			continue
		}
	}
}

// preloadForXlunch generates app list output for xlunch format
func (g *GUI) preloadForXlunch(prefix string) (string, error) {
	// Use the existing preload system but format for xlunch
	list, err := PreloadAppList(g.directory, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to preload app list: %w", err)
	}

	var output strings.Builder
	for _, item := range list.Items {
		// Format each item for xlunch: icon_path;app_name;tooltip
		line := fmt.Sprintf("%s;%s;%s\n", item.IconPath, item.Name, item.Description)
		output.WriteString(line)
	}

	return output.String(), nil
}

// runUpdater runs the updater in GUI mode
func (g *GUI) runUpdater() error {
	// Simple implementation - just run the updater script
	cmd := exec.Command(filepath.Join(g.directory, "updater"), "gui")
	return cmd.Run()
}

// getGitURL returns the git URL for the project
func (g *GUI) getGitURL() string {
	gitURLFile := filepath.Join(g.directory, "etc", "git_url")
	if content, err := os.ReadFile(gitURLFile); err == nil {
		return strings.TrimSpace(string(content))
	}
	return "https://github.com/Botspot/pi-apps" // Fallback
}

// runSettings runs the settings GUI
func (g *GUI) runSettings() error {
	// Run the settings script from Pi-Apps root directory
	settingsPath := filepath.Join(g.directory, "settings")
	cmd := exec.Command(settingsPath)
	cmd.Dir = g.directory // Set working directory to Pi-Apps root
	return cmd.Run()
}
