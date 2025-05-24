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

// Module: multi_manage.go
// Description: Provides functions for multiple concurent app installations.

package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// MultiInstallGUI provides a graphical interface to install multiple apps
// It shows a list of installable apps that aren't hidden or already installed
func MultiInstallGUI() error {
	// Initialize GTK
	gtk.Init(nil)

	// Get PI_APPS_DIR environment variable
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get list of apps to show in dialog
	// Hide hidden apps and hide installed apps
	installableApps, err := ListApps("cpu_installable")
	if err != nil {
		return fmt.Errorf("failed to get installable apps: %w", err)
	}

	hiddenApps, err := ListApps("hidden")
	if err != nil {
		return fmt.Errorf("failed to get hidden apps: %w", err)
	}

	installedApps, err := ListApps("installed")
	if err != nil {
		return fmt.Errorf("failed to get installed apps: %w", err)
	}

	availableApps := ListSubtract(installableApps, hiddenApps)
	availableApps = ListSubtract(availableApps, installedApps)

	// If no apps are available, show a message
	if len(availableApps) == 0 {
		dialog, err := gtk.DialogNew()
		if err != nil {
			return fmt.Errorf("error creating dialog: %w", err)
		}
		defer dialog.Destroy()

		dialog.SetTitle("Pi-Apps")
		dialog.SetDefaultSize(300, 100)
		dialog.SetPosition(gtk.WIN_POS_CENTER)

		// Set icon
		iconPath := filepath.Join(piAppsDir, "icons/settings.png")
		if FileExists(iconPath) {
			if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
				dialog.SetIcon(pixbuf)
			}
		}

		dialog.AddButton("OK", gtk.RESPONSE_OK)

		contentArea, err := dialog.GetContentArea()
		if err != nil {
			return fmt.Errorf("error getting content area: %w", err)
		}

		label, err := gtk.LabelNew("No apps available for installation.\nAll installable apps are already installed.")
		if err != nil {
			return fmt.Errorf("error creating label: %w", err)
		}
		contentArea.Add(label)
		contentArea.SetMarginStart(10)
		contentArea.SetMarginEnd(10)
		contentArea.SetMarginTop(10)
		contentArea.SetMarginBottom(10)

		dialog.ShowAll()
		dialog.Run()
		return nil
	}

	// Create the dialog window
	window, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("error creating window: %w", err)
	}
	window.SetTitle("Pi-Apps - Install Apps")
	window.SetDefaultSize(400, 500)
	window.SetPosition(gtk.WIN_POS_CENTER)

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons/settings.png")
	if FileExists(iconPath) {
		if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
			window.SetIcon(pixbuf)
		}
	}

	// Connect the destroy signal to exit the application
	window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Create a vertical box to hold the widgets
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		return fmt.Errorf("error creating vbox: %w", err)
	}
	vbox.SetMarginStart(10)
	vbox.SetMarginEnd(10)
	vbox.SetMarginTop(10)
	vbox.SetMarginBottom(10)
	window.Add(vbox)

	// Create a label with instructions
	label, err := gtk.LabelNew("Install everything you want!\nNote: apps that are already installed are not shown.")
	if err != nil {
		return fmt.Errorf("error creating label: %w", err)
	}
	label.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(label, false, false, 5)

	// Create a scrolled window to hold the list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("error creating scrolled window: %w", err)
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)
	vbox.PackStart(scrolled, true, true, 0)

	// Create a tree view to display the apps
	listStore, err := gtk.ListStoreNew(glib.TYPE_BOOLEAN, gdk.PixbufGetType(), glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		return fmt.Errorf("error creating list store: %w", err)
	}

	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		return fmt.Errorf("error creating tree view: %w", err)
	}
	treeView.SetHeadersVisible(false)
	scrolled.Add(treeView)

	// Create the checkbox column
	renderer, err := gtk.CellRendererToggleNew()
	if err != nil {
		return fmt.Errorf("error creating toggle renderer: %w", err)
	}

	// Connect the toggled signal to update the model
	renderer.Connect("toggled", func(r *gtk.CellRendererToggle, pathStr string) {
		path, err := gtk.TreePathNewFromString(pathStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting path: %v\n", err)
			return
		}

		iter, err := listStore.GetIter(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting iter: %v\n", err)
			return
		}

		val, err := listStore.GetValue(iter, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting value: %v\n", err)
			return
		}

		checked, err := val.GoValue()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting go value: %v\n", err)
			return
		}

		// Toggle the value
		listStore.SetValue(iter, 0, !checked.(bool))
	})

	column, err := gtk.TreeViewColumnNewWithAttribute("", renderer, "active", 0)
	if err != nil {
		return fmt.Errorf("error creating checkbox column: %w", err)
	}
	treeView.AppendColumn(column)

	// Create the icon column
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return fmt.Errorf("error creating pixbuf renderer: %w", err)
	}
	iconColumn, err := gtk.TreeViewColumnNewWithAttribute("", iconRenderer, "pixbuf", 1)
	if err != nil {
		return fmt.Errorf("error creating icon column: %w", err)
	}
	treeView.AppendColumn(iconColumn)

	// Create the name column
	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return fmt.Errorf("error creating text renderer: %w", err)
	}
	nameColumn, err := gtk.TreeViewColumnNewWithAttribute("", nameRenderer, "text", 2)
	if err != nil {
		return fmt.Errorf("error creating name column: %w", err)
	}
	treeView.AppendColumn(nameColumn)

	// Add tooltips
	treeView.SetTooltipColumn(3)

	// Populate the list store with apps
	for _, app := range availableApps {
		appIconPath := filepath.Join(piAppsDir, "apps", app, "icon-24.png")

		// Create pixbuf from icon
		var pixbuf *gdk.Pixbuf
		if FileExists(appIconPath) {
			pixbuf, err = gdk.PixbufNewFromFile(appIconPath)
			if err != nil {
				// Use a default icon or placeholder if the app icon can't be loaded
				fmt.Fprintf(os.Stderr, "Error loading icon for %s: %v\n", app, err)
			}
		}

		// Get first line of description for tooltip
		description := ""
		descriptionBytes, err := os.ReadFile(filepath.Join(piAppsDir, "apps", app, "description"))
		if err == nil && len(descriptionBytes) > 0 {
			descLines := strings.Split(string(descriptionBytes), "\n")
			if len(descLines) > 0 {
				description = descLines[0]
			}
		}

		// Add to the list store
		iter := listStore.Append()
		if pixbuf != nil {
			listStore.Set(iter, []int{0, 1, 2, 3}, []interface{}{false, pixbuf, app, description})
		} else {
			listStore.Set(iter, []int{0, 2, 3}, []interface{}{false, app, description})
		}
	}

	// Create button box
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		return fmt.Errorf("error creating button box: %w", err)
	}
	buttonBox.SetHomogeneous(true)
	vbox.PackEnd(buttonBox, false, false, 5)

	// Cancel button
	cancelButton, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return fmt.Errorf("error creating cancel button: %w", err)
	}
	cancelButton.Connect("clicked", func() {
		window.Destroy()
	})

	// Set icon for cancel button
	cancelIconPath := filepath.Join(piAppsDir, "icons/exit.png")
	if FileExists(cancelIconPath) {
		cancelImage, err := gtk.ImageNewFromFile(cancelIconPath)
		if err == nil {
			cancelButton.SetImage(cancelImage)
			cancelButton.SetAlwaysShowImage(true)
		}
	}

	buttonBox.PackStart(cancelButton, true, true, 0)

	// Install button
	installButton, err := gtk.ButtonNewWithLabel("Install selected")
	if err != nil {
		return fmt.Errorf("error creating install button: %w", err)
	}

	// Set icon for install button
	installIconPath := filepath.Join(piAppsDir, "icons/install.png")
	if FileExists(installIconPath) {
		installImage, err := gtk.ImageNewFromFile(installIconPath)
		if err == nil {
			installButton.SetImage(installImage)
			installButton.SetAlwaysShowImage(true)
		}
	}

	installButton.Connect("clicked", func() {
		// Get the selected apps
		var selectedApps []string

		iter, valid := listStore.GetIterFirst()
		for valid {
			val, err := listStore.GetValue(iter, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting value: %v\n", err)
				valid = listStore.IterNext(iter)
				continue
			}

			checked, err := val.GoValue()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting go value: %v\n", err)
				valid = listStore.IterNext(iter)
				continue
			}

			if checked.(bool) {
				appVal, err := listStore.GetValue(iter, 2)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting app name: %v\n", err)
					valid = listStore.IterNext(iter)
					continue
				}

				app, err := appVal.GoValue()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error converting app name: %v\n", err)
					valid = listStore.IterNext(iter)
					continue
				}

				selectedApps = append(selectedApps, app.(string))
			}

			valid = listStore.IterNext(iter)
		}

		// Build queue of install commands
		if len(selectedApps) > 0 {
			var queue strings.Builder
			for _, app := range selectedApps {
				queue.WriteString(fmt.Sprintf("install %s\n", app))
			}

			queueStr := strings.TrimSpace(queue.String())
			if queueStr != "" {
				// Run terminal_manage_multi in background
				go func() {
					// Call the external command or API function
					cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("nohup %s/api terminal_manage_multi \"%s\" &",
						filepath.Join(piAppsDir, "bin"), queueStr))
					cmd.Start()
				}()
			}
		}

		window.Destroy()
	})

	buttonBox.PackEnd(installButton, true, true, 0)

	// Show all widgets
	window.ShowAll()

	// Start the GTK main loop
	gtk.Main()

	return nil
}

// MultiUninstallGUI provides a graphical interface to uninstall multiple apps
// It shows a list of currently installed apps
func MultiUninstallGUI() error {
	// Initialize GTK
	gtk.Init(nil)

	// Get PI_APPS_DIR environment variable
	piAppsDir := os.Getenv("PI_APPS_DIR")
	if piAppsDir == "" {
		return fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Get list of installed apps
	installedApps, err := ListApps("installed")
	if err != nil {
		return fmt.Errorf("failed to get installed apps: %w", err)
	}

	// If no apps are installed, show a message
	if len(installedApps) == 0 {
		dialog, err := gtk.DialogNew()
		if err != nil {
			return fmt.Errorf("error creating dialog: %w", err)
		}
		defer dialog.Destroy()

		dialog.SetTitle("Pi-Apps")
		dialog.SetDefaultSize(300, 100)
		dialog.SetPosition(gtk.WIN_POS_CENTER)

		// Set icon
		iconPath := filepath.Join(piAppsDir, "icons/settings.png")
		if FileExists(iconPath) {
			if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
				dialog.SetIcon(pixbuf)
			}
		}

		dialog.AddButton("OK", gtk.RESPONSE_OK)

		contentArea, err := dialog.GetContentArea()
		if err != nil {
			return fmt.Errorf("error getting content area: %w", err)
		}

		label, err := gtk.LabelNew("No apps are currently installed.")
		if err != nil {
			return fmt.Errorf("error creating label: %w", err)
		}
		contentArea.Add(label)
		contentArea.SetMarginStart(10)
		contentArea.SetMarginEnd(10)
		contentArea.SetMarginTop(10)
		contentArea.SetMarginBottom(10)

		dialog.ShowAll()
		dialog.Run()
		return nil
	}

	// Create the dialog window
	window, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return fmt.Errorf("error creating window: %w", err)
	}
	window.SetTitle("Pi-Apps - Uninstall Apps")
	window.SetDefaultSize(400, 500)
	window.SetPosition(gtk.WIN_POS_CENTER)

	// Set window icon
	iconPath := filepath.Join(piAppsDir, "icons/settings.png")
	if FileExists(iconPath) {
		if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
			window.SetIcon(pixbuf)
		}
	}

	// Connect the destroy signal to exit the application
	window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Create a vertical box to hold the widgets
	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		return fmt.Errorf("error creating vbox: %w", err)
	}
	vbox.SetMarginStart(10)
	vbox.SetMarginEnd(10)
	vbox.SetMarginTop(10)
	vbox.SetMarginBottom(10)
	window.Add(vbox)

	// Create a label with instructions
	label, err := gtk.LabelNew("Uninstall everything you want!\nNote: apps that are not installed are not shown.")
	if err != nil {
		return fmt.Errorf("error creating label: %w", err)
	}
	label.SetHAlign(gtk.ALIGN_START)
	vbox.PackStart(label, false, false, 5)

	// Create a scrolled window to hold the list
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return fmt.Errorf("error creating scrolled window: %w", err)
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)
	vbox.PackStart(scrolled, true, true, 0)

	// Create a tree view to display the apps
	listStore, err := gtk.ListStoreNew(glib.TYPE_BOOLEAN, gdk.PixbufGetType(), glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		return fmt.Errorf("error creating list store: %w", err)
	}

	treeView, err := gtk.TreeViewNewWithModel(listStore)
	if err != nil {
		return fmt.Errorf("error creating tree view: %w", err)
	}
	treeView.SetHeadersVisible(false)
	scrolled.Add(treeView)

	// Create the checkbox column
	renderer, err := gtk.CellRendererToggleNew()
	if err != nil {
		return fmt.Errorf("error creating toggle renderer: %w", err)
	}

	// Connect the toggled signal to update the model
	renderer.Connect("toggled", func(r *gtk.CellRendererToggle, pathStr string) {
		path, err := gtk.TreePathNewFromString(pathStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting path: %v\n", err)
			return
		}

		iter, err := listStore.GetIter(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting iter: %v\n", err)
			return
		}

		val, err := listStore.GetValue(iter, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting value: %v\n", err)
			return
		}

		checked, err := val.GoValue()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting go value: %v\n", err)
			return
		}

		// Toggle the value
		listStore.SetValue(iter, 0, !checked.(bool))
	})

	column, err := gtk.TreeViewColumnNewWithAttribute("", renderer, "active", 0)
	if err != nil {
		return fmt.Errorf("error creating checkbox column: %w", err)
	}
	treeView.AppendColumn(column)

	// Create the icon column
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return fmt.Errorf("error creating pixbuf renderer: %w", err)
	}
	iconColumn, err := gtk.TreeViewColumnNewWithAttribute("", iconRenderer, "pixbuf", 1)
	if err != nil {
		return fmt.Errorf("error creating icon column: %w", err)
	}
	treeView.AppendColumn(iconColumn)

	// Create the name column
	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return fmt.Errorf("error creating text renderer: %w", err)
	}
	nameColumn, err := gtk.TreeViewColumnNewWithAttribute("", nameRenderer, "text", 2)
	if err != nil {
		return fmt.Errorf("error creating name column: %w", err)
	}
	treeView.AppendColumn(nameColumn)

	// Add tooltips
	treeView.SetTooltipColumn(3)

	// Populate the list store with apps
	for _, app := range installedApps {
		appIconPath := filepath.Join(piAppsDir, "apps", app, "icon-24.png")

		// Create pixbuf from icon
		var pixbuf *gdk.Pixbuf
		if FileExists(appIconPath) {
			pixbuf, err = gdk.PixbufNewFromFile(appIconPath)
			if err != nil {
				// Use a default icon or placeholder if the app icon can't be loaded
				fmt.Fprintf(os.Stderr, "Error loading icon for %s: %v\n", app, err)
			}
		}

		// Get first line of description for tooltip
		description := ""
		descriptionBytes, err := os.ReadFile(filepath.Join(piAppsDir, "apps", app, "description"))
		if err == nil && len(descriptionBytes) > 0 {
			descLines := strings.Split(string(descriptionBytes), "\n")
			if len(descLines) > 0 {
				description = descLines[0]
			}
		}

		// Add to the list store
		iter := listStore.Append()
		if pixbuf != nil {
			listStore.Set(iter, []int{0, 1, 2, 3}, []interface{}{false, pixbuf, app, description})
		} else {
			listStore.Set(iter, []int{0, 2, 3}, []interface{}{false, app, description})
		}
	}

	// Create button box
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		return fmt.Errorf("error creating button box: %w", err)
	}
	buttonBox.SetHomogeneous(true)
	vbox.PackEnd(buttonBox, false, false, 5)

	// Cancel button
	cancelButton, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return fmt.Errorf("error creating cancel button: %w", err)
	}
	cancelButton.Connect("clicked", func() {
		window.Destroy()
	})

	// Set icon for cancel button
	cancelIconPath := filepath.Join(piAppsDir, "icons/exit.png")
	if FileExists(cancelIconPath) {
		cancelImage, err := gtk.ImageNewFromFile(cancelIconPath)
		if err == nil {
			cancelButton.SetImage(cancelImage)
			cancelButton.SetAlwaysShowImage(true)
		}
	}

	buttonBox.PackStart(cancelButton, true, true, 0)

	// Uninstall button
	uninstallButton, err := gtk.ButtonNewWithLabel("Uninstall selected")
	if err != nil {
		return fmt.Errorf("error creating uninstall button: %w", err)
	}

	// Set icon for uninstall button
	uninstallIconPath := filepath.Join(piAppsDir, "icons/uninstall.png")
	if FileExists(uninstallIconPath) {
		uninstallImage, err := gtk.ImageNewFromFile(uninstallIconPath)
		if err == nil {
			uninstallButton.SetImage(uninstallImage)
			uninstallButton.SetAlwaysShowImage(true)
		}
	}

	uninstallButton.Connect("clicked", func() {
		// Get the selected apps
		var selectedApps []string

		iter, valid := listStore.GetIterFirst()
		for valid {
			val, err := listStore.GetValue(iter, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting value: %v\n", err)
				valid = listStore.IterNext(iter)
				continue
			}

			checked, err := val.GoValue()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting go value: %v\n", err)
				valid = listStore.IterNext(iter)
				continue
			}

			if checked.(bool) {
				appVal, err := listStore.GetValue(iter, 2)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting app name: %v\n", err)
					valid = listStore.IterNext(iter)
					continue
				}

				app, err := appVal.GoValue()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error converting app name: %v\n", err)
					valid = listStore.IterNext(iter)
					continue
				}

				selectedApps = append(selectedApps, app.(string))
			}

			valid = listStore.IterNext(iter)
		}

		// Build queue of uninstall commands
		if len(selectedApps) > 0 {
			var queue strings.Builder
			for _, app := range selectedApps {
				queue.WriteString(fmt.Sprintf("uninstall %s\n", app))
			}

			queueStr := strings.TrimSpace(queue.String())
			if queueStr != "" {
				// Run terminal_manage_multi in background
				go func() {
					// Call the external command or API function
					cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("nohup %s/api terminal_manage_multi \"%s\" &",
						filepath.Join(piAppsDir, "bin"), queueStr))
					cmd.Start()
				}()
			}
		}

		window.Destroy()
	})

	buttonBox.PackEnd(uninstallButton, true, true, 0)

	// Show all widgets
	window.ShowAll()

	// Start the GTK main loop
	gtk.Main()

	return nil
}
