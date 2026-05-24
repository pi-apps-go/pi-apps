// Copyright (C) 2026 pi-apps-go contributors
// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// loadSettingsState loads all settings from embedded definitions and data/settings files.
// It matches the behavior of the former SettingsWindow.loadSettings used by GTK.
func loadSettingsState(directory string) (map[string]*Setting, error) {
	settingsDir := filepath.Join(directory, "data", "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create settings directory: %w", err)
	}

	settings := make(map[string]*Setting)

	for _, def := range embeddedSettingDefinitions {
		setting := &Setting{
			Name:        def.Name,
			Description: def.Description,
			Values:      append([]string(nil), def.AcceptedValues...),
			Tooltip:     def.Description,
		}

		currentPath := filepath.Join(settingsDir, def.Name)
		if fileExists(currentPath) {
			currentBytes, err := os.ReadFile(currentPath)
			if err == nil {
				setting.Current = strings.TrimSpace(string(currentBytes))
			}
		}

		if setting.Current == "" {
			setting.Current = def.DefaultValue
			if err := os.WriteFile(currentPath, []byte(setting.Current), 0644); err != nil {
				return nil, fmt.Errorf("failed to write default setting: %w", err)
			}
		}

		processAppListStyleSetting(setting)
		settings[def.Name] = setting
	}

	return settings, nil
}

// sortedSettingNames returns setting names in stable sorted order (same as GTK tab).
func sortedSettingNames(settings map[string]*Setting) []string {
	names := make([]string, 0, len(settings))
	for n := range settings {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// canonicalValueFromTranslatedSelect maps a translated combo label back to the on-disk value.
func canonicalValueFromTranslatedSelect(setting *Setting, display string) string {
	if setting == nil {
		return display
	}
	for _, v := range setting.Values {
		if TranslateSettingValue(v) == display {
			return v
		}
	}
	return display
}

// writeCanonicalSettings writes canonical setting values to data/settings/<name>.
func writeCanonicalSettings(directory string, values map[string]string) error {
	settingsDir := filepath.Join(directory, "data", "settings")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}
	for name, val := range values {
		p := filepath.Join(settingsDir, name)
		if err := os.WriteFile(p, []byte(val), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
