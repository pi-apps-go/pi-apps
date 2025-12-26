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

// Module: i18n.go
// Description: Internationalization support for Pi-Apps API package

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"
)

var (
	// Global locale object
	apiLocale *gotext.Locale
	// Current locale
	currentLocale string
	// Available locales
	availableLocales []string
	// Flag to track if i18n is initialized
	i18nInitialized bool
)

// InitializeApiI18n initializes the internationalization system for the API package
func InitializeApiI18n() error {
	// Detect system locale
	locale := detectLocale()

	// Get the directory containing translations
	translationsDir, err := getTranslationsDirectory()
	if err != nil {
		return fmt.Errorf("failed to find translations directory: %v", err)
	}

	// Initialize gotext locale
	apiLocale = gotext.NewLocale(translationsDir, locale)
	apiLocale.AddDomain("pi-apps")
	currentLocale = locale

	// Set system locale environment variables
	err = setSystemLocale(locale)
	if err != nil {
		return fmt.Errorf("failed to set system locale: %v", err)
	}

	// Check if the locale works, if not try just the language part
	if apiLocale.Get("Downloading") == "Downloading" && locale != "en_US" {
		// Try just the language code (e.g., "pl" instead of "pl_PL")
		if underscoreIndex := strings.Index(locale, "_"); underscoreIndex != -1 {
			languageCode := locale[:underscoreIndex]
			apiLocale = gotext.NewLocale(translationsDir, languageCode)
			apiLocale.AddDomain("pi-apps")
			currentLocale = languageCode
		}
	}

	// Scan for available locales
	availableLocales = scanAvailableLocales(translationsDir)

	i18nInitialized = true
	return nil
}

// T translates a string using the API locale
func T(msgid string) string {
	if !i18nInitialized || apiLocale == nil {
		return msgid
	}
	return apiLocale.Get(msgid)
}

// Tf translates a formatted string using the API locale
func Tf(format string, args ...interface{}) string {
	if !i18nInitialized || apiLocale == nil {
		return fmt.Sprintf(format, args...)
	}
	translated := apiLocale.Get(format)
	return fmt.Sprintf(translated, args...)
}

// Tn translates a string with plural support using the API locale
func Tn(msgid, msgidPlural string, n int) string {
	if !i18nInitialized || apiLocale == nil {
		if n == 1 {
			return msgid
		}
		return msgidPlural
	}
	return apiLocale.GetN(msgid, msgidPlural, n)
}

// Tnf translates a formatted string with plural support using the API locale
func Tnf(msgid, msgidPlural string, n int, args ...interface{}) string {
	if !i18nInitialized || apiLocale == nil {
		var format string
		if n == 1 {
			format = msgid
		} else {
			format = msgidPlural
		}
		return fmt.Sprintf(format, args...)
	}
	translated := apiLocale.GetN(msgid, msgidPlural, n)
	return fmt.Sprintf(translated, args...)
}

// SetApiLocale changes the current locale
func SetApiLocale(locale string) error {
	if !i18nInitialized {
		return fmt.Errorf("i18n not initialized")
	}

	translationsDir, err := getTranslationsDirectory()
	if err != nil {
		return err
	}

	// Update the locale
	apiLocale = gotext.NewLocale(translationsDir, locale)
	apiLocale.AddDomain("pi-apps")
	currentLocale = locale

	return setSystemLocale(locale)
}

// GetCurrentApiLocale returns the current locale
func GetCurrentApiLocale() string {
	return currentLocale
}

// GetAvailableApiLocales returns list of available locales
func GetAvailableApiLocales() []string {
	return availableLocales
}

// Translated versions of user-facing API functions
// These should be used by CLI interfaces, not by internal API calls

// StatusT displays a translated status message in cyan
func StatusT(msgid string, args ...interface{}) {
	if len(args) > 0 && fmt.Sprintf("%v", args[0]) != "" {
		translated := T(msgid)
		fmt.Fprintln(os.Stderr, "\033[96m"+fmt.Sprintf(translated, args...)+"\033[0m")
	} else {
		translated := T(msgid)
		fmt.Fprintln(os.Stderr, "\033[96m"+translated+"\033[0m")
	}
}

// StatusGreenT announces the success of a major action in green with translation
func StatusGreenT(msgid string, args ...interface{}) {
	translated := T(msgid)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[92m"+translated+"\033[0m")
}

// WarningT displays a translated warning message in yellow with a flashing icon
func WarningT(msgid string, args ...interface{}) {
	translated := T(msgid)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	warningPrefix := T("WARNING:")
	fmt.Fprintln(os.Stderr, "\033[93m\033[5m◢◣\033[25m "+warningPrefix+" "+translated+"\033[0m")
}

// ErrorT displays a translated error message in red and exits the program
func ErrorT(msgid string, args ...interface{}) {
	translated := T(msgid)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[91m"+translated+"\033[0m")
	os.Exit(1)
}

// ErrorNoExitT displays a translated error message in red but does not exit the program
func ErrorNoExitT(msgid string, args ...interface{}) {
	translated := T(msgid)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[91m"+translated+"\033[0m")
}

// DebugT displays a translated debug message when debug mode is enabled
func DebugT(msg string) {
	if piAppsDebug {
		// The original bash script just does a simple echo without any color
		fmt.Println(T(msg))
	}
}

// StatusTf displays a formatted translated status message in cyan
func StatusTf(format string, args ...interface{}) {
	if len(args) > 0 && fmt.Sprintf("%v", args[0]) != "" {
		translated := T(format)
		fmt.Fprintln(os.Stderr, "\033[96m"+fmt.Sprintf(translated, args...)+"\033[0m")
	} else {
		translated := T(format)
		fmt.Fprintln(os.Stderr, "\033[96m"+translated+"\033[0m")
	}
}

// StatusGreenTf announces the success of a major action in green with translation
func StatusGreenTf(format string, args ...interface{}) {
	translated := T(format)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[92m"+translated+"\033[0m")
}

// WarningTf displays a formatted translated warning message in yellow with a flashing icon
func WarningTf(format string, args ...interface{}) {
	translated := T(format)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	warningPrefix := T("WARNING:")
	fmt.Fprintln(os.Stderr, "\033[93m\033[5m◢◣\033[25m "+warningPrefix+" "+translated+"\033[0m")
}

// ErrorTf displays a formatted translated error message in red and exits the program
func ErrorTf(format string, args ...interface{}) {
	translated := T(format)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[91m"+translated+"\033[0m")
	os.Exit(1)
}

// ErrorNoExitTf displays a formatted translated error message in red but does not exit the program
func ErrorNoExitTf(format string, args ...interface{}) {
	translated := T(format)
	if len(args) > 0 {
		translated = fmt.Sprintf(translated, args...)
	}
	fmt.Fprintln(os.Stderr, "\033[91m"+translated+"\033[0m")
}

// DebugTf translates a formatted debug message when debug mode is enabled
func DebugTf(format string, args ...any) {
	if piAppsDebug {
		translated := T(format)
		fmt.Println(fmt.Sprintf(translated, args...))
	}
}

// Helper functions for locale detection and management

func detectLocale() string {
	// Check environment variables in order of precedence
	for _, envVar := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if locale := os.Getenv(envVar); locale != "" {
			// Handle C/POSIX locale
			if locale == "C" || locale == "POSIX" {
				continue
			}

			// Extract language code from locale
			locale = strings.Split(locale, ".")[0] // Remove encoding (e.g., en_US.UTF-8 -> en_US)
			locale = strings.Split(locale, "@")[0] // Remove modifier (e.g., en_US@euro -> en_US)

			return locale
		}
	}

	// Default to English
	return "en_US"
}

func getTranslationsDirectory() (string, error) {
	// First try relative to current executable (for installed version)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		localesDir := filepath.Join(exeDir, "..", "locales")
		if apiDirExists(localesDir) {
			return localesDir, nil
		}
	}

	// Try relative to PI_APPS_DIR, development path
	if piAppsDir := GetPiAppsDir(); piAppsDir != "" {
		localesDir := filepath.Join(piAppsDir, "go-rewrite", "locales")
		if apiDirExists(localesDir) {
			return localesDir, nil
		}
	}

	// Try relative to PI_APPS_DIR
	if piAppsDir := GetPiAppsDir(); piAppsDir != "" {
		localesDir := filepath.Join(piAppsDir, "locales")
		if apiDirExists(localesDir) {
			return localesDir, nil
		}
	}

	// Try relative to working directory (for development)
	if wd, err := os.Getwd(); err == nil {
		localesDir := filepath.Join(wd, "locales")
		if apiDirExists(localesDir) {
			return localesDir, nil
		}

		// Try going up directories to find locales
		for i := 0; i < 3; i++ {
			wd = filepath.Dir(wd)
			localesDir = filepath.Join(wd, "locales")
			if apiDirExists(localesDir) {
				return localesDir, nil
			}
		}
	}

	return "", fmt.Errorf("translations directory not found")
}

func setSystemLocale(locale string) error {
	// Ensure locale has UTF-8 encoding for GTK compatibility
	if !strings.Contains(locale, ".") {
		if locale == "en_US" || locale == "en" {
			locale = "en_US.UTF-8"
		} else if strings.Contains(locale, "_") {
			locale = locale + ".UTF-8"
		} else {
			// For language-only codes, try common country mappings
			switch locale {
			case "es":
				locale = "es_ES.UTF-8"
			case "pl":
				locale = "pl_PL.UTF-8"
			case "fr":
				locale = "fr_FR.UTF-8"
			case "de":
				locale = "de_DE.UTF-8"
			default:
				locale = locale + "_" + strings.ToUpper(locale) + ".UTF-8"
			}
		}
	}

	// Only set LC_MESSAGES for GTK compatibility, don't override LC_ALL or LANG
	// This preserves the user's original environment variables for other programs (like APT)
	os.Setenv("LC_MESSAGES", locale)

	return nil
}

func scanAvailableLocales(translationsDir string) []string {
	var locales []string

	entries, err := os.ReadDir(translationsDir)
	if err != nil {
		return locales
	}

	for _, entry := range entries {
		if entry.IsDir() {
			locale := entry.Name()
			moFile := filepath.Join(translationsDir, locale, "LC_MESSAGES", "pi-apps.mo")
			if apiFileExists(moFile) {
				locales = append(locales, locale)
			}
		}
	}

	return locales
}

func apiFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func apiDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// getOriginalLang returns the original LANG environment variable set by the user
func getOriginalLang() string {
	// Now that AddEnglish() doesn't override locale variables, just use LANG directly
	return os.Getenv("LANG")
}
