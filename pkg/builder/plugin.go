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

// Module: plugin.go
// Description: Provides build-time plugin system for Pi-Apps Go
// SPDX-License-Identifier: GPL-3.0-or-later

package builder

import (
	"fmt"
	"sync"
)

// PiAppsPlugin represents a pi-apps plugin interface
type PiAppsPlugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Description returns the plugin description
	Description() string

	// Initialize initializes the plugin
	Initialize() error

	// Shutdown shuts down the plugin
	Shutdown() error
}

// CommandPlugin represents a plugin that adds commands
type CommandPlugin interface {
	PiAppsPlugin

	// RegisterCommands registers commands with the CLI
	RegisterCommands(registry CommandRegistry) error
}

// APIPlugin represents a plugin that adds API endpoints
type APIPlugin interface {
	PiAppsPlugin

	// RegisterRoutes registers API routes
	RegisterRoutes(registry RouteRegistry) error
}

// GUIPlugin represents a plugin that adds GUI components
type GUIPlugin interface {
	PiAppsPlugin

	// RegisterGUIComponents registers GUI components
	RegisterGUIComponents(registry GUIRegistry) error
}

// HookPlugin represents a plugin that provides hooks
type HookPlugin interface {
	PiAppsPlugin

	// RegisterHooks registers hooks
	RegisterHooks(registry HookRegistry) error
}

// CommandRegistry interface for registering commands
type CommandRegistry interface {
	RegisterCommand(name string, handler interface{}) error
}

// RouteRegistry interface for registering API routes
type RouteRegistry interface {
	RegisterRoute(method, path string, handler interface{}) error
}

// GUIRegistry interface for registering GUI components
type GUIRegistry interface {
	RegisterComponent(name string, component interface{}) error
}

// HookRegistry interface for registering hooks
type HookRegistry interface {
	RegisterHook(event string, handler interface{}) error
}

// PluginRegistry manages plugin registration
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]PiAppsPlugin
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]PiAppsPlugin),
	}
}

// Register registers a plugin
func (r *PluginRegistry) Register(plugin PiAppsPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = plugin
	return nil
}

// Get retrieves a plugin by name
func (r *PluginRegistry) Get(name string) (PiAppsPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

// List returns all registered plugins
func (r *PluginRegistry) List() []PiAppsPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]PiAppsPlugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// Initialize initializes all registered plugins
func (r *PluginRegistry) Initialize() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, plugin := range r.plugins {
		if err := plugin.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
		}
	}
	return nil
}

// Shutdown shuts down all registered plugins
func (r *PluginRegistry) Shutdown() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, plugin := range r.plugins {
		if err := plugin.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown plugin %s: %w", name, err)
		}
	}
	return nil
}

// Global plugin registry instance
var globalRegistry = NewPluginRegistry()

// RegisterPlugin registers a plugin globally
func RegisterPlugin(plugin PiAppsPlugin) error {
	return globalRegistry.Register(plugin)
}

// GetPlugin retrieves a plugin by name
func GetPlugin(name string) (PiAppsPlugin, bool) {
	return globalRegistry.Get(name)
}

// ListPlugins returns all registered plugins
func ListPlugins() []PiAppsPlugin {
	return globalRegistry.List()
}

// InitializePlugins initializes all registered plugins
func InitializePlugins() error {
	return globalRegistry.Initialize()
}

// ShutdownPlugins shuts down all registered plugins
func ShutdownPlugins() error {
	return globalRegistry.Shutdown()
}
