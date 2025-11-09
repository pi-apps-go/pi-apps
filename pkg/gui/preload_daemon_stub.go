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

// Module: preload_daemon_stub.go
// Description: Background daemon for refreshing app list files (stub).
// This replaces the bash preload-daemon script functionality for the Go rewrite.

//go:build dummy

package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// PreloadDaemon manages background refreshing of app list files
type PreloadDaemon struct {
	directory     string
	running       bool
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	refreshPeriod time.Duration
}

// DaemonConfig holds configuration for the preload daemon
type DaemonConfig struct {
	Directory     string
	RefreshPeriod time.Duration
}

// NewPreloadDaemon creates a new preload daemon
func NewPreloadDaemon(config DaemonConfig) *PreloadDaemon {
	if config.RefreshPeriod == 0 {
		config.RefreshPeriod = 30 * time.Second // Default refresh period
	}

	return &PreloadDaemon{
		directory:     config.Directory,
		stopChan:      make(chan struct{}),
		refreshPeriod: config.RefreshPeriod,
	}
}

// Start begins the daemon background operation
func (d *PreloadDaemon) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		logger.Error(api.T("daemon is already running"))
		return fmt.Errorf("daemon is already running")
	}

	d.running = true
	d.wg.Add(1)

	go d.run(ctx)

	return nil
}

// Stop gracefully stops the daemon
func (d *PreloadDaemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		logger.Warn(api.T("daemon is not running"))
		return fmt.Errorf("daemon is not running")
	}

	close(d.stopChan)
	d.running = false

	// Wait for goroutine to finish
	d.wg.Wait()

	return nil
}

// IsRunning returns whether the daemon is currently running
func (d *PreloadDaemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// run is the main daemon loop
func (d *PreloadDaemon) run(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.refreshPeriod)
	defer ticker.Stop()

	// Run initial refresh
	d.refreshAll()

	for {
		select {
		case <-ctx.Done():
			logger.Info(api.T("Preload daemon stopped due to context cancellation"))
			return
		case <-d.stopChan:
			logger.Info(api.T("Preload daemon stopped"))
			return
		case <-ticker.C:
			d.refreshAll()
		}
	}
}

// refreshAll refreshes all app list categories if needed
func (d *PreloadDaemon) refreshAll() {
	// Set environment for API functions
	os.Setenv("PI_APPS_DIR", d.directory)

	// Check and refresh package app status first
	if err := d.refreshPackageAppStatus(); err != nil {
		logger.Warn(api.Tf("failed to refresh package app status: %v\n", err))
	}

	// Get timestamp checker for main directories
	tc := NewTimeStampChecker(d.directory)
	timestamps := tc.GetTimestamps()

	// Check if anything has changed since last daemon run
	daemonTimestampFile := filepath.Join(d.directory, "data", "preload", "timestamps-preload-daemon")
	savedTimestamps, err := os.ReadFile(daemonTimestampFile)
	if err == nil && string(savedTimestamps) == timestamps {
		logger.Info(api.T("Preload-daemon skipped; nothing was changed"))
		return
	}

	logger.Info(api.T("Preload-daemon running..."))

	// Get list of folders to preload
	folders, err := d.getFoldersToPreload()
	if err != nil {
		logger.Error(api.Tf("Error getting folders to preload: %v\n", err))
		return
	}

	// Preload each folder
	for _, folder := range folders {
		if err := d.preloadFolder(folder); err != nil {
			logger.Warn(api.Tf("failed to preload folder '%s': %v\n", folder, err))
		}
	}

	// Save the current timestamps
	preloadDir := filepath.Join(d.directory, "data", "preload")
	if err := os.MkdirAll(preloadDir, 0755); err != nil {
		logger.Warn(api.Tf("failed to create preload directory: %v\n", err))
	} else {
		if err := os.WriteFile(daemonTimestampFile, []byte(timestamps), 0644); err != nil {
			logger.Warn(api.Tf("failed to save daemon timestamps: %v\n", err))
		}
	}

	logger.Info(api.T("Preload-daemon done"))
}

// refreshPackageAppStatus (stub): returns a stub status file for testing/non-APT builds
func (d *PreloadDaemon) refreshPackageAppStatus() error {
	// Write a stub file indicating this function was called
	preloadDir := filepath.Join(d.directory, "data", "preload")
	stubFile := filepath.Join(preloadDir, "timestamps-dpkg-status")
	if err := os.MkdirAll(preloadDir, 0755); err != nil {
		logger.Error(fmt.Sprintf("failed to create preload directory for stub: %v\n", err))
		return fmt.Errorf("failed to create preload directory for stub: %w", err)
	}
	stubContent := []byte("stub-dpkg-status\n")
	if err := os.WriteFile(stubFile, stubContent, 0644); err != nil {
		logger.Error(fmt.Sprintf("failed to write stub dpkg status: %v\n", err))
		return fmt.Errorf("failed to write stub dpkg status: %w", err)
	}
	// Optionally log that this is a stub action
	logger.Info("refreshPackageAppStatus stub: wrote dummy status file")
	return nil
}

// getFoldersToPreload gets the list of all folders that should be preloaded
func (d *PreloadDaemon) getFoldersToPreload() ([]string, error) {
	var folders []string

	// Add special folders
	folders = append(folders, "All Apps", "Installed", "Packages")

	// Get categories from category files
	categories, err := api.ReadCategoryFiles(d.directory)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to read category files: %v\n", err))
		return nil, fmt.Errorf("failed to read category files: %w", err)
	}

	// Extract unique category names
	categoryMap := make(map[string]bool)
	for _, categoryInfo := range categories {
		parts := strings.Split(categoryInfo, "|")
		if len(parts) >= 2 {
			category := strings.TrimSpace(parts[1])
			if category != "" {
				categoryMap[category] = true
			}
		}
	}

	// Add categories to folder list
	for category := range categoryMap {
		folders = append(folders, category)
	}

	return folders, nil
}

// preloadFolder preloads a specific folder/category
func (d *PreloadDaemon) preloadFolder(folder string) error {
	config := &AppListConfig{
		Directory: d.directory,
		Prefix:    folder,
		Format:    "gtk",
	}

	// Skip timestamp checking - we want to force regeneration
	list, err := generateAppList(config)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to generate app list for '%s': %v\n", folder, err))
		return fmt.Errorf("failed to generate app list for '%s': %w", folder, err)
	}

	// Save the generated list
	if err := saveCachedList(config, list); err != nil {
		logger.Error(fmt.Sprintf("failed to save cached list for '%s': %v\n", folder, err))
		return fmt.Errorf("failed to save cached list for '%s': %w", folder, err)
	}

	// Save timestamps
	tc := NewTimeStampChecker(d.directory)
	if err := tc.SaveTimestamps(folder); err != nil {
		logger.Error(fmt.Sprintf("failed to save timestamps for '%s': %v\n", folder, err))
		return fmt.Errorf("failed to save timestamps for '%s': %w", folder, err)
	}

	return nil
}

// RefreshSpecificCategory manually refreshes a specific category
func (d *PreloadDaemon) RefreshSpecificCategory(category string) error {
	return d.preloadFolder(category)
}

// RefreshAllCategories manually refreshes all categories
func (d *PreloadDaemon) RefreshAllCategories() error {
	folders, err := d.getFoldersToPreload()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to get folders to preload: %v\n", err))
		return fmt.Errorf("failed to get folders to preload: %w", err)
	}

	for _, folder := range folders {
		if err := d.preloadFolder(folder); err != nil {
			logger.Error(fmt.Sprintf("failed to refresh category '%s': %v\n", folder, err))
			return fmt.Errorf("failed to refresh category '%s': %w", folder, err)
		}
	}

	return nil
}

// StartPreloadDaemon is a convenience function to start the daemon with default settings
func StartPreloadDaemon(directory string) (*PreloadDaemon, error) {
	if directory == "" {
		directory = os.Getenv("PI_APPS_DIR")
		if directory == "" {
			logger.Error("PI_APPS_DIR environment variable not set")
			return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
		}
	}

	config := DaemonConfig{
		Directory:     directory,
		RefreshPeriod: 30 * time.Second,
	}

	daemon := NewPreloadDaemon(config)
	ctx := context.Background()

	if err := daemon.Start(ctx); err != nil {
		logger.Error(fmt.Sprintf("failed to start preload daemon: %v\n", err))
		return nil, fmt.Errorf("failed to start preload daemon: %w", err)
	}

	return daemon, nil
}
