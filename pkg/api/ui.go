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

// Module: ui.go
// Description: Provides functions for GUI user input invoked via app installation scripts.

package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// UserInputFunc displays a dialog to the user and returns their selection
// This is a Go implementation of the original bash userinput_func
func UserInputFunc(text string, options ...string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("userinput_func(): requires a description")
	}
	if len(options) == 0 {
		return "", fmt.Errorf("userinput_func(): requires at least one output selection option")
	}

	// Check if we can use GTK
	if !canUseGTK() {
		fmt.Fprintf(os.Stderr, "Using CLI for dialog\n")
		return cliUserInput(text, options...)
	}

	fmt.Fprintf(os.Stderr, "Using GTK for dialog\n")

	// Initialize application name
	glib.SetPrgname("Pi-Apps")
	glib.SetApplicationName("Pi-Apps (user input dialog)")

	// Initialize GTK
	gtk.Init(nil)

	var selection string
	var err error

	// Create the appropriate dialog based on the number of options
	if len(options) == 1 {
		// Simple OK dialog
		selection, err = createSimpleDialog(text, options[0])
	} else if len(options) == 2 {
		// Yes/No type dialog
		selection, err = createYesNoDialog(text, options[0], options[1])
	} else {
		// List selection dialog
		selection, err = createListDialog(text, options)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "GTK dialog error: %v, falling back to CLI\n", err)
		return cliUserInput(text, options...)
	}

	return selection, nil
}

// createSimpleDialog creates a simple dialog with a single button
func createSimpleDialog(text, buttonLabel string) (string, error) {
	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", fmt.Errorf("failed to create dialog: %w", err)
	}

	dialog.SetTitle("Pi-Apps")
	dialog.SetPosition(gtk.WIN_POS_CENTER)
	dialog.SetModal(true)
	dialog.SetDecorated(false)
	dialog.SetResizable(false)
	dialog.SetBorderWidth(20)

	// Add content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to get dialog content area: %w", err)
	}

	// Create a horizontal box to hold the icon and text
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create horizontal box: %w", err)
	}
	contentArea.Add(hbox)

	// Add information icon
	icon, err := gtk.ImageNewFromIconName("dialog-information", gtk.ICON_SIZE_DIALOG)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create information icon: %w", err)
	}
	hbox.PackStart(icon, false, false, 0)

	// Add text label in a vertical box
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create vertical box: %w", err)
	}
	hbox.PackStart(vbox, true, true, 0)

	// Add text label
	label, err := gtk.LabelNew(text)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create text label: %w", err)
	}
	label.SetLineWrap(true)
	label.SetSelectable(false)
	label.SetJustify(gtk.JUSTIFY_LEFT)
	vbox.PackStart(label, true, true, 0)

	// Add button
	dialog.AddButton(buttonLabel, gtk.RESPONSE_OK)
	dialog.ShowAll()
	dialog.Run()
	dialog.Destroy()

	return buttonLabel, nil
}

// createYesNoDialog creates a Yes/No dialog
func createYesNoDialog(text, yesLabel, noLabel string) (string, error) {
	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", fmt.Errorf("failed to create dialog: %w", err)
	}

	dialog.SetTitle("Pi-Apps")
	dialog.SetPosition(gtk.WIN_POS_CENTER)
	dialog.SetModal(true)
	dialog.SetDecorated(false)
	dialog.SetResizable(false)
	dialog.SetBorderWidth(20)

	// Add content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to get dialog content area: %w", err)
	}

	// Create a horizontal box to hold the icon and text
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create horizontal box: %w", err)
	}
	contentArea.Add(hbox)

	// Add question icon
	icon, err := gtk.ImageNewFromIconName("dialog-question", gtk.ICON_SIZE_DIALOG)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create question icon: %w", err)
	}
	hbox.PackStart(icon, false, false, 0)

	// Add text label in a vertical box
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create vertical box: %w", err)
	}
	hbox.PackStart(vbox, true, true, 0)

	// Add text label
	label, err := gtk.LabelNew(text)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create text label: %w", err)
	}
	label.SetLineWrap(true)
	label.SetSelectable(false)
	label.SetJustify(gtk.JUSTIFY_LEFT)
	vbox.PackStart(label, true, true, 0)

	// Add buttons
	dialog.AddButton(yesLabel, gtk.RESPONSE_YES)
	dialog.AddButton(noLabel, gtk.RESPONSE_NO)
	dialog.ShowAll()

	response := dialog.Run()
	dialog.Destroy()

	// Process the response
	if response == gtk.RESPONSE_YES {
		return yesLabel, nil
	} else {
		return noLabel, nil
	}
}

// createListDialog creates a dialog with a list of options
func createListDialog(text string, options []string) (string, error) {
	// Create the dialog
	dialog, err := gtk.DialogNew()
	if err != nil {
		return "", fmt.Errorf("failed to create dialog: %w", err)
	}

	dialog.SetTitle("Pi-Apps")
	dialog.SetPosition(gtk.WIN_POS_CENTER)
	dialog.SetModal(true)
	dialog.SetDecorated(false)
	dialog.SetResizable(false)
	dialog.SetBorderWidth(20)

	// Add content area
	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to get dialog content area: %w", err)
	}

	// Create a horizontal box to hold the icon and content
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create horizontal box: %w", err)
	}
	contentArea.Add(hbox)

	// Add information icon
	icon, err := gtk.ImageNewFromIconName("dialog-information", gtk.ICON_SIZE_DIALOG)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create information icon: %w", err)
	}
	hbox.PackStart(icon, false, false, 0)

	// Add content in a vertical box
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create vertical box: %w", err)
	}
	hbox.PackStart(vbox, true, true, 0)

	// Add text label
	if text != "" {
		label, err := gtk.LabelNew(text)
		if err != nil {
			dialog.Destroy()
			return "", fmt.Errorf("failed to create text label: %w", err)
		}
		label.SetLineWrap(true)
		label.SetSelectable(false)
		label.SetJustify(gtk.JUSTIFY_LEFT)
		vbox.PackStart(label, false, false, 0)
	}

	// Create a box for the radio buttons
	radioBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		dialog.Destroy()
		return "", fmt.Errorf("failed to create radio button box: %w", err)
	}
	vbox.PackStart(radioBox, true, true, 0)

	// Create radio buttons
	var radioButtons []*gtk.RadioButton
	for i, opt := range options {
		var rb *gtk.RadioButton
		if i == 0 {
			rb, err = gtk.RadioButtonNewWithLabel(nil, opt)
		} else {
			rb, err = gtk.RadioButtonNewWithLabelFromWidget(radioButtons[0], opt)
		}

		if err != nil {
			dialog.Destroy()
			return "", fmt.Errorf("failed to create radio button: %w", err)
		}

		radioButtons = append(radioButtons, rb)
		radioBox.PackStart(rb, false, false, 0)

		// Set the first option as active by default
		if i == 0 {
			rb.SetActive(true)
		}
	}

	// Add OK button
	dialog.AddButton("OK", gtk.RESPONSE_OK)
	dialog.ShowAll()

	// Run dialog and wait for response
	dialog.Run()

	// Find which radio button is active
	selection := options[0] // Default to first option
	for i, rb := range radioButtons {
		if rb.GetActive() {
			selection = options[i]
			break
		}
	}

	dialog.Destroy()
	return selection, nil
}

// canUseGTK checks if GTK can be used (display available)
func canUseGTK() bool {
	// Check for --cli flag to force CLI mode
	for _, arg := range os.Args {
		if arg == "--cli" {
			fmt.Fprintf(os.Stderr, "GTK disabled by --cli flag\n")
			return false
		}
	}

	// Check essential environment variables for GUI
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintf(os.Stderr, "GTK not available: No display environment variable set\n")
		return false
	}

	// Check if we're in an SSH session without X forwarding
	if os.Getenv("SSH_CONNECTION") != "" && os.Getenv("DISPLAY") == "" {
		fmt.Fprintf(os.Stderr, "GTK not available: SSH connection without X forwarding\n")
		return false
	}

	// If we reached here, attempt to use GTK
	return true
}

// cliUserInput provides a fallback CLI-based user input when GTK is not available
func cliUserInput(text string, options ...string) (string, error) {
	// Write the prompts to stderr so they're visible during command substitution
	fmt.Fprintln(os.Stderr, text)
	fmt.Fprintln(os.Stderr)

	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "%d. %s\n", i+1, opt)
	}

	fmt.Fprintf(os.Stderr, "\nEnter your choice (1-%d): ", len(options))

	var choice string
	fmt.Scanln(&choice)

	// Try to convert to a number
	choiceNum, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || choiceNum < 1 || choiceNum > len(options) {
		fmt.Fprintln(os.Stderr, "Invalid choice. Using default:", options[0])
		return options[0], nil
	}

	return options[choiceNum-1], nil
}
