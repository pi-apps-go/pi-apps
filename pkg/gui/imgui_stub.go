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

// Module: imgui_stub.go
// Description: Stub implementation of the ImGui GUI for systems that don't have ImGui support for Pi-Apps Go.
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !imgui

package gui

import "fmt"

type ImGuiConfig struct {
	Width  int
	Height int
	Theme  string
}

type ImGuiGUI struct {
	directory string
	config    ImGuiConfig
}

func DefaultImGuiConfig() ImGuiConfig {
	return ImGuiConfig{
		Width:  800,
		Height: 600,
		Theme:  "dark",
	}
}

// This file provides stub implementations for the ImGui-based GUI,
// used when ImGui is not supported on the system (when the 'imgui' build tag is absent).
// All methods simply inform the user that ImGui is unavailable.

func NewImGuiGUI(directory string, config ImGuiConfig) *ImGuiGUI {
	fmt.Println("ImGui GUI is not supported on this system. Falling back to stub implementation.")
	return &ImGuiGUI{}
}

func (gui *ImGuiGUI) Run() error {
	return fmt.Errorf("ImGui is not supported on this system")
}

func (gui *ImGuiGUI) Close() {
	// No resources to cleanup for the stub
}

func (gui *ImGuiGUI) Initialize() error {
	return fmt.Errorf("ImGui is not supported on this system")
}

func (gui *ImGuiGUI) Cleanup() {
	// No cleanup needed
}

func (gui *ImGuiGUI) getScreenDimensions() error {
	return fmt.Errorf("ImGui is not supported on this system")
}

func (gui *ImGuiGUI) createDirectories() error {
	return fmt.Errorf("ImGui is not supported on this system")
}

func (gui *ImGuiGUI) startBackgroundTasks() {
	// No background tasks in stub
}

func (gui *ImGuiGUI) runImGuiMode() error {
	return fmt.Errorf("ImGui is not supported on this system")
}
