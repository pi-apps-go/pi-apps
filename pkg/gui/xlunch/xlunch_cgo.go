//go:build xlunch && cgo
// +build xlunch,cgo

package gui

/*
#cgo pkg-config: x11 imlib2
#cgo CFLAGS: -std=c99 -Wall
#cgo LDFLAGS: -lX11 -lImlib2

#include <stdlib.h>
#include "xlunch.c"

// CGO wrapper functions
int xlunch_main_wrapper(int argc, char **argv);
int xlunch_init_wrapper(int argc, char **argv);
void xlunch_set_entries_from_string_wrapper(const char* entries_data);
void xlunch_set_config_option_wrapper(const char* option, const char* value);
int xlunch_run_loop_wrapper();
void xlunch_cleanup_wrapper();

int xlunch_main_wrapper(int argc, char **argv) {
    return xlunch_main(argc, argv);
}

int xlunch_init_wrapper(int argc, char **argv) {
    return xlunch_init(argc, argv);
}

void xlunch_set_entries_from_string_wrapper(const char* entries_data) {
    xlunch_set_entries_from_string(entries_data);
}

void xlunch_set_config_option_wrapper(const char* option, const char* value) {
    xlunch_set_config_option(option, value);
}

int xlunch_run_loop_wrapper() {
    return xlunch_run_loop();
}

void xlunch_cleanup_wrapper() {
    xlunch_cleanup();
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

// XLunchConfig holds configuration for xlunch
type XLunchConfig struct {
	// Layout options
	IconSize int
	Columns  int
	Rows     int
	Width    int
	Height   int
	XPos     int
	YPos     int

	// Behavior options
	Windowed      bool
	DesktopMode   bool
	DontQuit      bool
	Multiple      bool
	OutputOnly    bool
	VoidClickTerm bool
	FocusLostTerm bool
	HideMissing   bool

	// Appearance options
	BackgroundFile  string
	HighlightFile   string
	FontName        string
	PromptFont      string
	Prompt          string
	TextColor       string
	BackgroundColor string
	HighlightColor  string
	PromptColor     string
	ShadowColor     string
	ScrollBarColor  string
	ScrollIndColor  string

	// Input options
	NoPrompt bool
	NoTitle  bool
}

// XLunchEntry represents an application entry
type XLunchEntry struct {
	Title string
	Icon  string
	Cmd   string
}

// XLunch wraps the C xlunch implementation
type XLunch struct {
	config  XLunchConfig
	entries []XLunchEntry
	running bool
}

// Exit codes from xlunch
const (
	XLunchOkay        = 0
	XLunchEscape      = 0x20
	XLunchRightClick  = 0x21
	XLunchVoidClick   = 0x22
	XLunchFocusLost   = 0x23
	XLunchInternalCmd = 0x24
	XLunchLockError   = 0x40
	XLunchAllocError  = 0x41
	XLunchFontError   = 0x42
	XLunchConfigError = 0x43
	XLunchWinError    = 0x44
	XLunchLocaleError = 0x45
	XLunchInputError  = 0x46
	XLunchPollError   = 0x48
	XLunchExternalErr = 0x49
)

// DefaultXLunchConfig returns a default configuration
func DefaultXLunchConfig() XLunchConfig {
	return XLunchConfig{
		IconSize:        48,
		Columns:         0, // auto
		Rows:            0, // auto
		Width:           800,
		Height:          600,
		XPos:            -1, // center
		YPos:            -1, // center
		Windowed:        true,
		DesktopMode:     false,
		DontQuit:        false,
		Multiple:        false,
		OutputOnly:      false,
		VoidClickTerm:   false,
		FocusLostTerm:   false,
		HideMissing:     false,
		FontName:        "",
		PromptFont:      "",
		Prompt:          "Search: ",
		TextColor:       "",
		BackgroundColor: "",
		HighlightColor:  "",
		PromptColor:     "",
		ShadowColor:     "",
		ScrollBarColor:  "",
		ScrollIndColor:  "",
		NoPrompt:        false,
		NoTitle:         false,
	}
}

// NewXLunch creates a new XLunch instance
func NewXLunch(config XLunchConfig) *XLunch {
	return &XLunch{
		config:  config,
		entries: make([]XLunchEntry, 0),
		running: false,
	}
}

// SetEntries sets the application entries
func (xl *XLunch) SetEntries(entries []XLunchEntry) {
	xl.entries = entries
}

// AddEntry adds a single application entry
func (xl *XLunch) AddEntry(entry XLunchEntry) {
	xl.entries = append(xl.entries, entry)
}

// Run starts xlunch with the current configuration and entries
func (xl *XLunch) Run() (int, error) {
	if xl.running {
		return 0, fmt.Errorf("xlunch is already running")
	}

	// Build command line arguments from config
	args := xl.buildArgs()

	// Convert Go strings to C strings
	cargs := make([]*C.char, len(args))
	for i, arg := range args {
		cargs[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(cargs[i]))
	}

	// Initialize xlunch
	xl.running = true
	ret := C.xlunch_init_wrapper(C.int(len(args)), &cargs[0])
	if ret != 0 {
		xl.running = false
		return int(ret), fmt.Errorf("xlunch_init failed with code %d", ret)
	}

	// Set entries
	entriesData := xl.formatEntries()
	cEntriesData := C.CString(entriesData)
	defer C.free(unsafe.Pointer(cEntriesData))
	C.xlunch_set_entries_from_string_wrapper(cEntriesData)

	// Run the main loop
	ret = C.xlunch_run_loop_wrapper()
	xl.running = false

	return int(ret), nil
}

// Close cleans up xlunch resources
func (xl *XLunch) Close() {
	if xl.running {
		C.xlunch_cleanup_wrapper()
		xl.running = false
	}
}

// buildArgs builds command line arguments from configuration
func (xl *XLunch) buildArgs() []string {
	args := []string{"xlunch"}

	// Layout options
	if xl.config.IconSize > 0 {
		args = append(args, "-s", fmt.Sprintf("%d", xl.config.IconSize))
	}
	if xl.config.Columns > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", xl.config.Columns))
	}
	if xl.config.Rows > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", xl.config.Rows))
	}
	if xl.config.Width > 0 {
		args = append(args, "-w", fmt.Sprintf("%d", xl.config.Width))
	}
	if xl.config.Height > 0 {
		args = append(args, "-h", fmt.Sprintf("%d", xl.config.Height))
	}
	if xl.config.XPos >= 0 {
		args = append(args, "-x", fmt.Sprintf("%d", xl.config.XPos))
	}
	if xl.config.YPos >= 0 {
		args = append(args, "-y", fmt.Sprintf("%d", xl.config.YPos))
	}

	// Behavior options
	if xl.config.Windowed {
		args = append(args, "-W")
	}
	if xl.config.DesktopMode {
		args = append(args, "-d")
	}
	if xl.config.DontQuit {
		args = append(args, "-q")
	}
	if xl.config.Multiple {
		args = append(args, "-m")
	}
	if xl.config.OutputOnly {
		args = append(args, "-o")
	}
	if xl.config.VoidClickTerm {
		args = append(args, "-t")
	}
	if xl.config.FocusLostTerm {
		args = append(args, "--focuslostterminate")
	}
	if xl.config.HideMissing {
		args = append(args, "-e")
	}

	// Appearance options
	if xl.config.BackgroundFile != "" {
		args = append(args, "-g", xl.config.BackgroundFile)
	}
	if xl.config.HighlightFile != "" {
		args = append(args, "-L", xl.config.HighlightFile)
	}
	if xl.config.FontName != "" {
		args = append(args, "-f", xl.config.FontName)
	}
	if xl.config.PromptFont != "" {
		args = append(args, "-F", xl.config.PromptFont)
	}
	if xl.config.Prompt != "" {
		args = append(args, "-p", xl.config.Prompt)
	}
	if xl.config.TextColor != "" {
		args = append(args, "--tc", xl.config.TextColor)
	}
	if xl.config.BackgroundColor != "" {
		args = append(args, "--bc", xl.config.BackgroundColor)
	}
	if xl.config.HighlightColor != "" {
		args = append(args, "--hc", xl.config.HighlightColor)
	}
	if xl.config.PromptColor != "" {
		args = append(args, "--pc", xl.config.PromptColor)
	}
	if xl.config.ShadowColor != "" {
		args = append(args, "--sc", xl.config.ShadowColor)
	}
	if xl.config.ScrollBarColor != "" {
		args = append(args, "--scrollbarcolor", xl.config.ScrollBarColor)
	}
	if xl.config.ScrollIndColor != "" {
		args = append(args, "--scrollindicatorcolor", xl.config.ScrollIndColor)
	}

	// Input options
	if xl.config.NoPrompt {
		args = append(args, "-n")
	}
	if xl.config.NoTitle {
		args = append(args, "-N")
	}

	return args
}

// formatEntries formats entries for xlunch consumption
func (xl *XLunch) formatEntries() string {
	var lines []string
	for _, entry := range xl.entries {
		// XLunch format: Title;Icon;Command
		line := fmt.Sprintf("%s;%s;%s", entry.Title, entry.Icon, entry.Cmd)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// SetConfigOption sets a configuration option at runtime
func (xl *XLunch) SetConfigOption(option, value string) error {
	cOption := C.CString(option)
	defer C.free(unsafe.Pointer(cOption))
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))

	C.xlunch_set_config_option_wrapper(cOption, cValue)
	return nil
}
