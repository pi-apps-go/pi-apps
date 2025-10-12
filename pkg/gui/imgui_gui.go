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

// Module: imgui_gui.go
// Description: ImGui-based GUI implementation using cimgui-go with SDL2 backend
// This replaces the buggy xlunch X11 implementation with a modern, stable ImGui interface
// Uses SDL2 backend for better cross-platform compatibility compared to GLFW

//go:build cgo
// +build cgo

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	_ "github.com/AllenDang/cimgui-go/impl/opengl3"
	"github.com/botspot/pi-apps/pkg/api"
	"github.com/davidbyttow/govips/v2/vips"
)

// ImGuiAppEntry represents an app entry for ImGui interface
type ImGuiAppEntry struct {
	Title       string
	IconPath    string
	Command     string
	IsDir       bool
	Description string
	Status      string
}

// ImGuiTheme holds theme configuration
type ImGuiTheme struct {
	Name          string
	Background    imgui.Vec4
	Text          imgui.Vec4
	Button        imgui.Vec4
	ButtonHover   imgui.Vec4
	ButtonActive  imgui.Vec4
	WindowBg      imgui.Vec4
	Border        imgui.Vec4
	Scrollbar     imgui.Vec4
	ScrollbarGrab imgui.Vec4
}

// ImGuiGUI represents the ImGui-based GUI
type ImGuiGUI struct {
	directory      string
	config         ImGuiConfig
	entries        []ImGuiAppEntry
	currentPrefix  string
	searchQuery    string
	showSearch     bool
	theme          ImGuiTheme
	iconTextures   map[string]*backend.Texture
	scrollPos      float32
	selectedApp    int
	showAppDetails bool
	appDetails     ImGuiAppEntry
	backend        backend.Backend[sdlbackend.SDLWindowFlags]
}

// ImGuiConfig holds configuration for ImGui GUI
type ImGuiConfig struct {
	Width       int
	Height      int
	IconSize    int
	Theme       string
	Title       string
	WindowFlags imgui.WindowFlags
}

// DefaultImGuiConfig returns default configuration
func DefaultImGuiConfig() ImGuiConfig {
	return ImGuiConfig{
		Width:       900,
		Height:      700,
		IconSize:    80,
		Theme:       "dark",
		Title:       "Pi-Apps: Raspberry Pi app store",
		WindowFlags: imgui.WindowFlagsNoResize | imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoMove,
	}
}

// NewImGuiGUI creates a new ImGui-based GUI
func NewImGuiGUI(directory string, config ImGuiConfig) *ImGuiGUI {
	return &ImGuiGUI{
		directory:      directory,
		config:         config,
		entries:        make([]ImGuiAppEntry, 0),
		currentPrefix:  "",
		searchQuery:    "",
		showSearch:     false,
		theme:          getTheme(config.Theme),
		iconTextures:   make(map[string]*backend.Texture),
		scrollPos:      0,
		selectedApp:    -1,
		showAppDetails: false,
	}
}

// getTheme returns theme configuration based on theme name
func getTheme(themeName string) ImGuiTheme {
	switch themeName {
	case "light-3d":
		return ImGuiTheme{
			Name:          "Light 3D",
			Background:    imgui.Vec4{X: 0.9, Y: 0.9, Z: 0.9, W: 1.0},
			Text:          imgui.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0},
			Button:        imgui.Vec4{X: 0.8, Y: 0.8, Z: 0.8, W: 1.0},
			ButtonHover:   imgui.Vec4{X: 0.7, Y: 0.7, Z: 0.7, W: 1.0},
			ButtonActive:  imgui.Vec4{X: 0.6, Y: 0.6, Z: 0.6, W: 1.0},
			WindowBg:      imgui.Vec4{X: 0.95, Y: 0.95, Z: 0.95, W: 1.0},
			Border:        imgui.Vec4{X: 0.5, Y: 0.5, Z: 0.5, W: 1.0},
			Scrollbar:     imgui.Vec4{X: 0.3, Y: 0.3, Z: 0.3, W: 0.5},
			ScrollbarGrab: imgui.Vec4{X: 0.2, Y: 0.2, Z: 0.2, W: 0.8},
		}
	case "dark-3d":
		return ImGuiTheme{
			Name:          "Dark 3D",
			Background:    imgui.Vec4{X: 0.1, Y: 0.1, Z: 0.1, W: 1.0},
			Text:          imgui.Vec4{X: 0.9, Y: 0.9, Z: 0.9, W: 1.0},
			Button:        imgui.Vec4{X: 0.2, Y: 0.2, Z: 0.2, W: 1.0},
			ButtonHover:   imgui.Vec4{X: 0.3, Y: 0.3, Z: 0.3, W: 1.0},
			ButtonActive:  imgui.Vec4{X: 0.4, Y: 0.4, Z: 0.4, W: 1.0},
			WindowBg:      imgui.Vec4{X: 0.05, Y: 0.05, Z: 0.05, W: 1.0},
			Border:        imgui.Vec4{X: 0.5, Y: 0.5, Z: 0.5, W: 1.0},
			Scrollbar:     imgui.Vec4{X: 0.7, Y: 0.7, Z: 0.7, W: 0.5},
			ScrollbarGrab: imgui.Vec4{X: 0.8, Y: 0.8, Z: 0.8, W: 0.8},
		}
	default: // dark (transparent)
		return ImGuiTheme{
			Name:          "Dark",
			Background:    imgui.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 0.8},
			Text:          imgui.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0},
			Button:        imgui.Vec4{X: 0.2, Y: 0.2, Z: 0.2, W: 0.8},
			ButtonHover:   imgui.Vec4{X: 0.3, Y: 0.3, Z: 0.3, W: 0.9},
			ButtonActive:  imgui.Vec4{X: 0.4, Y: 0.4, Z: 0.4, W: 1.0},
			WindowBg:      imgui.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 0.7},
			Border:        imgui.Vec4{X: 0.5, Y: 0.5, Z: 0.5, W: 0.5},
			Scrollbar:     imgui.Vec4{X: 0.7, Y: 0.7, Z: 0.7, W: 0.3},
			ScrollbarGrab: imgui.Vec4{X: 0.8, Y: 0.8, Z: 0.8, W: 0.6},
		}
	}
}

// LoadEntriesFromPreload loads entries using the preload system
func (gui *ImGuiGUI) LoadEntriesFromPreload(directory, prefix string) error {
	// Use the existing preload system
	list, err := PreloadAppList(directory, prefix)
	if err != nil {
		return fmt.Errorf("failed to preload app list: %w", err)
	}

	// Convert AppListItems to ImGuiAppEntries
	gui.entries = make([]ImGuiAppEntry, 0, len(list.Items))
	for _, item := range list.Items {
		entry := ImGuiAppEntry{
			Title:       item.Name,
			IconPath:    item.IconPath,
			Command:     item.Path,
			IsDir:       item.Type == "category",
			Description: item.Description,
			Status:      gui.getAppStatus(item.Name),
		}
		gui.entries = append(gui.entries, entry)
	}

	gui.currentPrefix = prefix
	return nil
}

// getAppStatus returns the status of an app
func (gui *ImGuiGUI) getAppStatus(appName string) string {
	statusFile := filepath.Join(gui.directory, "data", "status", appName)
	if api.FileExists(statusFile) {
		if content, err := os.ReadFile(statusFile); err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return "not installed"
}

// loadIconTexture loads an icon and returns its texture
func (gui *ImGuiGUI) loadIconTexture(iconPath string) *backend.Texture {
	if iconPath == "" {
		return nil
	}

	// Check if already loaded
	if texture, exists := gui.iconTextures[iconPath]; exists {
		return texture
	}

	// Load and process icon
	processedPath, err := gui.processIcon(iconPath)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to process icon %s: %v", iconPath, err))
		return nil
	}

	// Load image with govips
	image, err := vips.NewImageFromFile(processedPath)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load icon %s: %v", processedPath, err))
		return nil
	}
	defer image.Close()

	// Export as PNG bytes
	ep := vips.NewDefaultPNGExportParams()
	_, _, err = image.Export(ep)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to export icon %s: %v", processedPath, err))
		return nil
	}

	// Create texture from bytes
	// Note: This is a simplified version - in a real implementation,
	// you'd need to properly create OpenGL textures from the image data
	// For now, we'll return a placeholder
	textureRef := imgui.NewTextureRefFromC(uintptr(len(gui.iconTextures) + 1))
	texture := &backend.Texture{
		ID:     *textureRef,
		Width:  gui.config.IconSize,
		Height: gui.config.IconSize,
	}
	gui.iconTextures[iconPath] = texture

	return texture
}

// processIcon processes an icon to the required size and format
func (gui *ImGuiGUI) processIcon(iconPath string) (string, error) {
	if iconPath == "" {
		return "", nil
	}

	// Check if icon exists
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		return "", nil
	}

	// Create cache directory
	tempDir := filepath.Join(os.TempDir(), "pi-apps-imgui-icons")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return iconPath, nil // Fallback to original on error
	}

	// Generate cache filename based on icon path and size
	hasher := fmt.Sprintf("%x", iconPath)
	outputPath := filepath.Join(tempDir, fmt.Sprintf("icon_%d_%s.png",
		gui.config.IconSize, hasher[:8]))

	// Check if already processed
	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	// Load image with govips
	image, err := vips.NewImageFromFile(iconPath)
	if err != nil {
		return gui.copyIconToTemp(iconPath, tempDir)
	}
	defer image.Close()

	// Resize image with proper aspect ratio
	err = image.Resize(float64(gui.config.IconSize)/float64(image.Width()), vips.KernelAuto)
	if err != nil {
		return gui.copyIconToTemp(iconPath, tempDir)
	}

	// Crop to square if needed
	if image.Width() != image.Height() {
		size := gui.config.IconSize
		err = image.ExtractArea(0, 0, size, size)
		if err != nil {
			return gui.copyIconToTemp(iconPath, tempDir)
		}
	}

	// Export as PNG
	ep := vips.NewDefaultPNGExportParams()
	imageBytes, _, err := image.Export(ep)
	if err != nil {
		return gui.copyIconToTemp(iconPath, tempDir)
	}

	// Save processed image
	if err := os.WriteFile(outputPath, imageBytes, 0644); err != nil {
		return gui.copyIconToTemp(iconPath, tempDir)
	}

	return outputPath, nil
}

// copyIconToTemp copies an icon to temp directory as fallback
func (gui *ImGuiGUI) copyIconToTemp(iconPath, tempDir string) (string, error) {
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

// applyTheme applies the current theme to ImGui
func (gui *ImGuiGUI) applyTheme() {
	// Note: Theme application would be done here
	// For now, we'll use ImGui's default styling
	// In a full implementation, you would set the appropriate colors
	// using the ImGui style system
}

// drawHeaderRegion draws the header region directly
func (gui *ImGuiGUI) drawHeaderRegion() {
	// Header background and styling
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.1, 0.1, 0.1, 0.9))
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(20, 15))

	// Create a temporary window for the header to get proper styling
	if imgui.BeginV("Header", nil, imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoScrollbar) {
		// Logo text (font handling would need to be implemented separately)
		imgui.Text("Pi-Apps")

		imgui.SameLine()
		imgui.SetCursorPosX(float32(gui.config.Width) - 250)

		// Search input
		imgui.Text("Search:")
		imgui.SameLine()
		imgui.SetNextItemWidth(150)
		if imgui.InputTextWithHint("##Search", "Search apps...", &gui.searchQuery, 0, nil) {
			gui.performSearch()
		}

		imgui.SameLine()
		if imgui.Button("Settings") {
			gui.showSettings()
		}
	}
	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}

// drawMainContentRegion draws the main content region (app grid) directly
func (gui *ImGuiGUI) drawMainContentRegion() {
	// Calculate grid layout
	iconSize := float32(gui.config.IconSize)
	padding := float32(20)
	cellWidth := iconSize + padding*2
	cellHeight := iconSize + 50 // Space for text

	cols := int(float32(gui.config.Width-40) / cellWidth)
	if cols < 1 {
		cols = 1
	}

	// Create a temporary window for the main content
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.05, 0.05, 0.05, 0.95))
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(10, 10))

	if imgui.BeginV("MainContent", nil, imgui.WindowFlagsNoTitleBar|imgui.WindowFlagsNoResize|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoScrollbar) {
		// Draw back button if in a category
		if gui.currentPrefix != "" {
			imgui.PushStyleColorVec4(imgui.ColButton, imgui.NewVec4(0.2, 0.2, 0.2, 0.8))
			imgui.PushStyleColorVec4(imgui.ColButtonHovered, imgui.NewVec4(0.3, 0.3, 0.3, 0.9))
			imgui.PushStyleColorVec4(imgui.ColButtonActive, imgui.NewVec4(0.4, 0.4, 0.4, 1.0))
			if imgui.Button("← Back") {
				gui.navigateBack()
			}
			imgui.PopStyleColor()
			imgui.PopStyleColor()
			imgui.PopStyleColor()
			imgui.Separator()
		}

		// Draw apps in grid
		for i, entry := range gui.entries {
			col := i % cols
			row := i / cols

			x := float32(20 + col*int(cellWidth))
			y := float32(row * int(cellHeight))

			imgui.SetCursorPos(imgui.NewVec2(x, y))

			// Draw app button with better styling
			buttonID := fmt.Sprintf("##app_%d", i)
			imgui.PushStyleColorVec4(imgui.ColButton, gui.getCategoryColor(entry.Title))
			imgui.PushStyleColorVec4(imgui.ColButtonHovered, imgui.NewVec4(
				gui.getCategoryColor(entry.Title).X+0.1,
				gui.getCategoryColor(entry.Title).Y+0.1,
				gui.getCategoryColor(entry.Title).Z+0.1,
				1.0))
			imgui.PushStyleColorVec4(imgui.ColButtonActive, imgui.NewVec4(
				gui.getCategoryColor(entry.Title).X+0.2,
				gui.getCategoryColor(entry.Title).Y+0.2,
				gui.getCategoryColor(entry.Title).Z+0.2,
				1.0))
			imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 8)

			if imgui.ButtonV(buttonID, imgui.NewVec2(iconSize, iconSize)) {
				gui.onAppSelected(entry)
			}

			imgui.PopStyleVar()
			imgui.PopStyleColor()
			imgui.PopStyleColor()
			imgui.PopStyleColor()

			// Draw title below the button
			imgui.SetCursorPos(imgui.NewVec2(x, y+iconSize+8))
			imgui.Text(entry.Title)

			// Draw status if not installed
			if entry.Status != "installed" {
				imgui.SameLine()
				imgui.TextColored(imgui.NewVec4(0.8, 0.4, 0, 1), "("+entry.Status+")")
			}
		}
	}
	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}

// getCategoryColor returns a color for a category
func (gui *ImGuiGUI) getCategoryColor(title string) imgui.Vec4 {
	// Hash the title to get consistent colors
	hash := uint32(0)
	for _, c := range title {
		hash = hash*31 + uint32(c)
	}

	// Generate color from hash
	r := float32((hash>>16)&0xFF) / 255.0
	g := float32((hash>>8)&0xFF) / 255.0
	b := float32(hash&0xFF) / 255.0

	return imgui.NewVec4(r, g, b, 1.0)
}

// onAppSelected handles app selection
func (gui *ImGuiGUI) onAppSelected(entry ImGuiAppEntry) {
	if entry.IsDir {
		// Category selected - load category apps
		logger.Info(fmt.Sprintf("Loading category: %s", entry.Title))
		if err := gui.LoadEntriesFromPreload(gui.directory, entry.Title); err != nil {
			logger.Error(fmt.Sprintf("Failed to load category %s: %v", entry.Title, err))
		}
	} else {
		// App selected - show details
		gui.appDetails = entry
		gui.showAppDetails = true
	}
}

// navigateBack navigates back to parent directory
func (gui *ImGuiGUI) navigateBack() {
	parentPath := strings.TrimSuffix(gui.currentPrefix, "/")
	if parentPath == "" {
		parentPath = ""
	}

	logger.Info(fmt.Sprintf("Navigating back to: %s", parentPath))
	if err := gui.LoadEntriesFromPreload(gui.directory, parentPath); err != nil {
		logger.Error(fmt.Sprintf("Failed to load parent directory %s: %v", parentPath, err))
	}
}

// performSearch performs a search
func (gui *ImGuiGUI) performSearch() {
	if gui.searchQuery == "" {
		// Clear search - reload current view
		gui.LoadEntriesFromPreload(gui.directory, gui.currentPrefix)
		return
	}

	// Filter entries based on search query
	filteredEntries := make([]ImGuiAppEntry, 0)
	query := strings.ToLower(gui.searchQuery)

	for _, entry := range gui.entries {
		if strings.Contains(strings.ToLower(entry.Title), query) ||
			strings.Contains(strings.ToLower(entry.Description), query) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	gui.entries = filteredEntries
}

// showSettings shows the settings dialog
func (gui *ImGuiGUI) showSettings() {
	// This would open the settings dialog
	// For now, just log
	logger.Info("Settings button clicked")
}

// drawAppDetailsModal draws the app details as a modal overlay
func (gui *ImGuiGUI) drawAppDetailsModal() {
	if !gui.showAppDetails {
		return
	}

	// Set modal properties - center it over the main window
	imgui.SetNextWindowPosV(imgui.NewVec2(
		float32(gui.config.Width/2-250),
		float32(gui.config.Height/2-200),
	), imgui.CondAlways, imgui.NewVec2(0.5, 0.5))
	imgui.SetNextWindowSizeV(imgui.NewVec2(500, 400), imgui.CondAlways)

	// Style the modal
	imgui.PushStyleColorVec4(imgui.ColWindowBg, imgui.NewVec4(0.1, 0.1, 0.1, 0.95))
	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 8)
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.NewVec2(20, 20))

	// Use a unique name to avoid conflicts with main window
	if imgui.BeginV("AppDetailsModal", &gui.showAppDetails, imgui.WindowFlagsNoCollapse|imgui.WindowFlagsNoResize) {
		// App icon and title
		imgui.Text(gui.appDetails.Title)

		imgui.Separator()

		// Status
		imgui.Text("Status:")
		imgui.SameLine()
		if gui.appDetails.Status == "installed" {
			imgui.TextColored(imgui.NewVec4(0, 1, 0, 1), gui.appDetails.Status)
		} else {
			imgui.TextColored(imgui.NewVec4(1, 0.5, 0, 1), gui.appDetails.Status)
		}

		// Description
		imgui.Spacing()
		imgui.Text("Description:")
		imgui.TextWrapped(gui.appDetails.Description)

		imgui.Spacing()
		imgui.Separator()
		imgui.Spacing()

		// Action buttons
		imgui.PushStyleColorVec4(imgui.ColButton, imgui.NewVec4(0.2, 0.6, 0.2, 0.8))
		imgui.PushStyleColorVec4(imgui.ColButtonHovered, imgui.NewVec4(0.3, 0.7, 0.3, 0.9))
		imgui.PushStyleColorVec4(imgui.ColButtonActive, imgui.NewVec4(0.4, 0.8, 0.4, 1.0))

		if gui.appDetails.Status == "not installed" {
			if imgui.Button("Install") {
				gui.installApp(gui.appDetails.Title)
			}
		} else {
			imgui.PushStyleColorVec4(imgui.ColButton, imgui.NewVec4(0.6, 0.2, 0.2, 0.8))
			imgui.PushStyleColorVec4(imgui.ColButtonHovered, imgui.NewVec4(0.7, 0.3, 0.3, 0.9))
			imgui.PushStyleColorVec4(imgui.ColButtonActive, imgui.NewVec4(0.8, 0.4, 0.4, 1.0))
			if imgui.Button("Uninstall") {
				gui.uninstallApp(gui.appDetails.Title)
			}
			imgui.PopStyleColor()
			imgui.PopStyleColor()
			imgui.PopStyleColor()
		}

		imgui.PopStyleColor()
		imgui.PopStyleColor()
		imgui.PopStyleColor()

		imgui.SameLine()
		imgui.SetCursorPosX(400)
		if imgui.Button("Close") {
			gui.showAppDetails = false
		}
	}
	imgui.End()
	imgui.PopStyleVar()
	imgui.PopStyleVar()
	imgui.PopStyleColor()
}

// installApp installs an app
func (gui *ImGuiGUI) installApp(appName string) {
	logger.Info(fmt.Sprintf("Installing app: %s", appName))
	// This would trigger the actual installation
	gui.showAppDetails = false
}

// uninstallApp uninstalls an app
func (gui *ImGuiGUI) uninstallApp(appName string) {
	logger.Info(fmt.Sprintf("Uninstalling app: %s", appName))
	// This would trigger the actual uninstallation
	gui.showAppDetails = false
}

// drawUI draws the UI directly to the window surface
func (gui *ImGuiGUI) drawUI() {
	// Draw header region directly
	gui.drawHeaderRegion()

	// Draw main content region directly
	gui.drawMainContentRegion()

	// Draw app details modal if needed
	gui.drawAppDetailsModal()
}

// Run runs the ImGui GUI
func (gui *ImGuiGUI) Run() error {
	logger.Info("Starting ImGui GUI with SDL2 backend")

	// Lock OS thread for OpenGL
	runtime.LockOSThread()

	// Initialize ImGui context before creating backend
	imgui.CreateContext()

	// Create SDL2 backend (switched from GLFW for better cross-platform support)
	var err error
	gui.backend, err = backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		return fmt.Errorf("failed to create SDL2 backend: %w", err)
	}

	// Create window first
	gui.backend.CreateWindow(gui.config.Title, gui.config.Width, gui.config.Height)

	// Set background color
	gui.backend.SetBgColor(imgui.NewVec4(0.05, 0.05, 0.05, 1.0))

	// Set up callbacks after window is created
	gui.backend.SetCloseCallback(func() {
		logger.Info("Window closing")
	})

	// Load initial entries
	if err := gui.LoadEntriesFromPreload(gui.directory, ""); err != nil {
		return fmt.Errorf("failed to load entries: %w", err)
	}

	logger.Info(fmt.Sprintf("Loaded %d entries", len(gui.entries)))

	// Main loop - let SDL2 backend handle ImGui lifecycle, just draw our UI
	gui.backend.Run(func() {
		// Draw UI directly to the window (backend handles ImGui frame lifecycle)
		gui.drawUI()
	})

	return nil
}

// Close cleans up resources
func (gui *ImGuiGUI) Close() {
	// Destroy ImGui context
	imgui.DestroyContext()

	// Clean up icon textures
	for _, texture := range gui.iconTextures {
		// Note: In a real implementation, you'd delete the OpenGL texture here
		_ = texture
	}
	gui.iconTextures = make(map[string]*backend.Texture)

	// Clean up temp icons
	tempDir := filepath.Join(os.TempDir(), "pi-apps-imgui-icons")
	os.RemoveAll(tempDir)
}
