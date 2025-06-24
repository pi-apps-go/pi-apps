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

// Module: i18n.go
// Description: Internationalization support for the settings package using gotext

package settings

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"
)

// Locale is the global locale instance for the settings package
var Locale *gotext.Locale

// InitializeI18n initializes the internationalization system
func InitializeI18n() error {
	// Get the base directory for Pi-Apps
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		// Fallback to current directory structure
		// First try current directory (for tests)
		currentDir, _ := os.Getwd()
		if _, err := os.Stat(filepath.Join(currentDir, "locales")); err == nil {
			directory = currentDir
		} else {
			// Then try parent directory (for installed binaries)
			directory = filepath.Join(filepath.Dir(os.Args[0]), "..")
		}
	}

	// Set up the locale directory path
	localeDir := filepath.Join(directory, "locales")

	// Get system locale
	systemLocale := getSystemLocale()

	// Set the system locale environment variables for GTK
	setSystemLocale(systemLocale)

	// Create locale object, try full locale first, then just language code
	Locale = gotext.NewLocale(localeDir, systemLocale)
	Locale.AddDomain("pi-apps")

	// Check if the locale works, if not try just the language part
	if Locale.Get("Pi-Apps Settings") == "Pi-Apps Settings" && systemLocale != "en_US" {
		// Try just the language code (e.g., "pl" instead of "pl_PL")
		if underscoreIndex := strings.Index(systemLocale, "_"); underscoreIndex != -1 {
			languageCode := systemLocale[:underscoreIndex]
			Locale = gotext.NewLocale(localeDir, languageCode)
			Locale.AddDomain("pi-apps")
		}
	}

	return nil
}

// getSystemLocale attempts to determine the system locale
func getSystemLocale() string {
	// Try various environment variables
	for _, envVar := range []string{"LANG", "LC_ALL", "LC_MESSAGES"} {
		if locale := os.Getenv(envVar); locale != "" {
			// Extract language and country code (e.g., "en_US.UTF-8" -> "en_US")
			if dotIndex := strings.Index(locale, "."); dotIndex != -1 {
				locale = locale[:dotIndex]
			}
			return locale
		}
	}

	// Fallback to English
	return "en_US"
}

// setSystemLocale sets the appropriate locale environment variables for GTK
func setSystemLocale(locale string) {
	// Ensure we have the full locale format with UTF-8
	fullLocale := locale
	if !strings.Contains(locale, ".") {
		fullLocale = locale + ".UTF-8"
	}

	// Set the locale environment variables
	os.Setenv("LC_ALL", fullLocale)
	os.Setenv("LANG", fullLocale)
	os.Setenv("LC_MESSAGES", fullLocale)
}

// T is a shorthand function for translation
func T(msgid string, args ...interface{}) string {
	if Locale == nil {
		// Fallback to original string if locale is not initialized
		if len(args) > 0 {
			return gotext.GetC("", msgid, args...)
		}
		return msgid
	}

	if len(args) > 0 {
		return Locale.Get(msgid, args...)
	}
	return Locale.Get(msgid)
}

// Tf is a shorthand function for translation with formatting
func Tf(msgid string, args ...interface{}) string {
	return T(msgid, args...)
}

// Tn is a shorthand function for plural translation
func Tn(msgid, msgidPlural string, n int, args ...interface{}) string {
	if Locale == nil {
		// Fallback to simple plural logic
		if n == 1 {
			if len(args) > 0 {
				return gotext.GetC("", msgid, args...)
			}
			return msgid
		}
		if len(args) > 0 {
			return gotext.GetC("", msgidPlural, args...)
		}
		return msgidPlural
	}

	if len(args) > 0 {
		return Locale.GetN(msgid, msgidPlural, n, args...)
	}
	return Locale.GetN(msgid, msgidPlural, n)
}

// GetAvailableLocales returns a list of available locales
func GetAvailableLocales() []string {
	// Use the same directory resolution logic as InitializeI18n
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		// Fallback to current directory structure
		// First try current directory (for tests)
		currentDir, _ := os.Getwd()
		if _, err := os.Stat(filepath.Join(currentDir, "locales")); err == nil {
			directory = currentDir
		} else {
			// Then try parent directory (for installed binaries)
			directory = filepath.Join(filepath.Dir(os.Args[0]), "..")
		}
	}

	localeDir := filepath.Join(directory, "locales")
	var locales []string

	if files, err := os.ReadDir(localeDir); err == nil {
		for _, file := range files {
			if file.IsDir() && file.Name() != "." && file.Name() != ".." {
				locales = append(locales, file.Name())
			}
		}
	}

	return locales
}

// SetLocale changes the current locale
func SetLocale(locale string) error {
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		directory = filepath.Join(filepath.Dir(os.Args[0]), "..")
	}

	// Set the system locale environment variables for GTK
	setSystemLocale(locale)

	localeDir := filepath.Join(directory, "locales")
	Locale = gotext.NewLocale(localeDir, locale)
	Locale.AddDomain("pi-apps")

	return nil
}

// GetCurrentLocale returns the current locale
func GetCurrentLocale() string {
	if Locale == nil {
		return "en_US"
	}
	return Locale.GetLanguage()
}

// translateSettingName translates setting names from their file names
func translateSettingName(settingName string) string {
	// Map of setting file names to translatable strings
	settingNameMap := map[string]string{
		"App List Style":        "App List Style",
		"Check for updates":     "Check for updates",
		"Enable analytics":      "Enable analytics",
		"Preferred text editor": "Preferred text editor",
		"Show Edit button":      "Show Edit button",
		"Show apps":             "Show apps",
		"Shuffle App list":      "Shuffle App list",
	}

	if translatable, exists := settingNameMap[settingName]; exists {
		return T(translatable)
	}

	// If not in map, try to translate directly
	return T(settingName)
}

// TranslateSettingName exports the translation function for use in other parts of the package
func TranslateSettingName(settingName string) string {
	return translateSettingName(settingName)
}

// translateSettingValue translates setting values (Yes/No, Never/Daily, etc.)
func translateSettingValue(value string) string {
	// Map of setting values to translatable strings
	valueMap := map[string]string{
		// Yes/No values
		"Yes": "Yes",
		"No":  "No",

		// Update frequency values
		"Never":  "Never",
		"Daily":  "Daily",
		"Always": "Always",
		"Weekly": "Weekly",

		// App display values
		"All":      "All",
		"packages": "packages",
		"standard": "standard",

		// Theme values
		"default": "default",
	}

	if translatable, exists := valueMap[value]; exists {
		return T(translatable)
	}

	// If not in map, try to translate directly
	return T(value)
}

// TranslateSettingValue exports the translation function for use in other parts of the package
func TranslateSettingValue(value string) string {
	return translateSettingValue(value)
}

// TranslateTooltip translates multi-line tooltips by translating each line separately
func TranslateTooltip(tooltip string) string {
	if tooltip == "" {
		return ""
	}

	// Split into lines
	lines := strings.Split(tooltip, "\n")
	var translatedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			translatedLines = append(translatedLines, "")
			continue
		}

		// Try to translate the line
		translated := T(line)
		translatedLines = append(translatedLines, translated)
	}

	return strings.Join(translatedLines, "\n")
}
