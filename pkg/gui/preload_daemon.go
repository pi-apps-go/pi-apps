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

// Module: preload_daemon.go
// Description: Background daemon for refreshing app list files.
// This replaces the bash preload-daemon script functionality for the Go rewrite.

package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/botspot/pi-apps/pkg/api"
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
			fmt.Fprintf(os.Stderr, "Preload daemon stopped due to context cancellation\n")
			return
		case <-d.stopChan:
			fmt.Fprintf(os.Stderr, "Preload daemon stopped\n")
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
		fmt.Fprintf(os.Stderr, "Warning: failed to refresh package app status: %v\n", err)
	}

	// Get timestamp checker for main directories
	tc := NewTimeStampChecker(d.directory)
	timestamps := tc.GetTimestamps()

	// Check if anything has changed since last daemon run
	daemonTimestampFile := filepath.Join(d.directory, "data", "preload", "timestamps-preload-daemon")
	savedTimestamps, err := os.ReadFile(daemonTimestampFile)
	if err == nil && string(savedTimestamps) == timestamps {
		fmt.Fprintf(os.Stderr, "Preload-daemon skipped; nothing was changed\n")
		return
	}

	fmt.Fprintf(os.Stderr, "Preload-daemon running...\n")

	// Get list of folders to preload
	folders, err := d.getFoldersToPreload()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting folders to preload: %v\n", err)
		return
	}

	// Preload each folder
	for _, folder := range folders {
		if err := d.preloadFolder(folder); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to preload folder '%s': %v\n", folder, err)
		}
	}

	// Save the current timestamps
	preloadDir := filepath.Join(d.directory, "data", "preload")
	if err := os.MkdirAll(preloadDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create preload directory: %v\n", err)
	} else {
		if err := os.WriteFile(daemonTimestampFile, []byte(timestamps), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save daemon timestamps: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Preload-daemon done\n")
}

// refreshPackageAppStatus refreshes package app status if dpkg status has changed
func (d *PreloadDaemon) refreshPackageAppStatus() error {
	dpkgStatusFile := "/var/lib/dpkg/status"
	timestampFile := filepath.Join(d.directory, "data", "preload", "timestamps-dpkg-status")

	// Get current dpkg status modification time
	stat, err := os.Stat(dpkgStatusFile)
	if err != nil {
		// dpkg status file doesn't exist or can't be read, skip
		return nil
	}
	currentTime := fmt.Sprintf("%d", stat.ModTime().Unix())

	// Check if it has changed
	savedTime, err := os.ReadFile(timestampFile)
	if err == nil && string(savedTime) == currentTime {
		// No change, skip refresh
		return nil
	}

	fmt.Fprintf(os.Stderr, "Refreshing pkgapp_status...\n")

	// Save new timestamp
	preloadDir := filepath.Join(d.directory, "data", "preload")
	if err := os.MkdirAll(preloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create preload directory: %w", err)
	}

	if err := os.WriteFile(timestampFile, []byte(currentTime), 0644); err != nil {
		return fmt.Errorf("failed to save dpkg timestamp: %w", err)
	}

	// Call API function to refresh package app status
	return api.RefreshAllPkgAppStatus()
}

// getFoldersToPreload gets the list of all folders that should be preloaded
func (d *PreloadDaemon) getFoldersToPreload() ([]string, error) {
	var folders []string

	// Add special folders
	folders = append(folders, "All Apps", "Installed", "Packages")

	// Get categories from category files
	categories, err := api.ReadCategoryFiles(d.directory)
	if err != nil {
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
		return fmt.Errorf("failed to generate app list for '%s': %w", folder, err)
	}

	// Save the generated list
	if err := saveCachedList(config, list); err != nil {
		return fmt.Errorf("failed to save cached list for '%s': %w", folder, err)
	}

	// Save timestamps
	tc := NewTimeStampChecker(d.directory)
	if err := tc.SaveTimestamps(folder); err != nil {
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
		return fmt.Errorf("failed to get folders to preload: %w", err)
	}

	for _, folder := range folders {
		if err := d.preloadFolder(folder); err != nil {
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
		return nil, fmt.Errorf("failed to start preload daemon: %w", err)
	}

	return daemon, nil
}
