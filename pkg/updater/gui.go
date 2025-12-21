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

// Module: gui.go
// Description: GUI interface for the updater.

package updater

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/pi-apps-go/pi-apps/pkg/api"
)

// UpdaterGUI handles the GTK3 interface for the updater
type UpdaterGUI struct {
	updater         *Updater
	window          *gtk.Window
	progressBar     *gtk.ProgressBar
	statusLabel     *gtk.Label
	updatesTreeView *gtk.TreeView
	updateButton    *gtk.Button
	cancelButton    *gtk.Button
	retryButton     *gtk.Button
	rollbackButton  *gtk.Button

	// Update tracking
	selectedFiles []FileChange
	selectedApps  []string
	lastResult    *UpdateResult
}

// UpdateItem represents an item in the updates list
type UpdateItem struct {
	Selected    bool
	Icon        string
	Name        string
	Type        string
	Description string
	Action      string
}

// NewUpdaterGUI creates a new updater GUI
func NewUpdaterGUI(updater *Updater) (*UpdaterGUI, error) {
	// Initialize GTK
	gtk.Init(nil)

	gui := &UpdaterGUI{
		updater: updater,
	}

	if err := gui.buildUI(); err != nil {
		return nil, fmt.Errorf("failed to build UI: %w", err)
	}

	return gui, nil
}

// Run starts the GUI main loop
func (g *UpdaterGUI) Run() {
	g.window.ShowAll()
	gtk.Main()
}

// buildUI constructs the main window and all UI components
func (g *UpdaterGUI) buildUI() error {
	// Create main window
	var err error
	g.window, err = gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}

	g.window.SetTitle("Pi-Apps Updater")
	g.window.SetDefaultSize(600, 500)
	g.window.SetPosition(gtk.WIN_POS_CENTER)
	g.window.SetResizable(true)

	// Set window icon
	if pixbuf, err := gdk.PixbufNewFromFile(g.updater.directory + "/icons/logo.png"); err == nil {
		g.window.SetIcon(pixbuf)
	}

	// Connect window close event
	g.window.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// Create main container
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return err
	}
	mainBox.SetMarginTop(10)
	mainBox.SetMarginBottom(10)
	mainBox.SetMarginStart(10)
	mainBox.SetMarginEnd(10)

	// Create header
	if err := g.createHeader(mainBox); err != nil {
		return err
	}

	// Create updates list
	if err := g.createUpdatesList(mainBox); err != nil {
		return err
	}

	// Create progress section
	if err := g.createProgressSection(mainBox); err != nil {
		return err
	}

	// Create button section
	if err := g.createButtonSection(mainBox); err != nil {
		return err
	}

	g.window.Add(mainBox)

	// Load initial data
	g.refreshUpdatesList()

	return nil
}

// createHeader creates the header section with title and status
func (g *UpdaterGUI) createHeader(parent *gtk.Box) error {
	headerBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		return err
	}

	// Title
	titleLabel, err := gtk.LabelNew("")
	if err != nil {
		return err
	}
	titleLabel.SetMarkup("<span size='large' weight='bold'>Pi-Apps Updates</span>")
	titleLabel.SetHAlign(gtk.ALIGN_START)

	// Status label
	g.statusLabel, err = gtk.LabelNew("Checking for updates...")
	if err != nil {
		return err
	}
	g.statusLabel.SetHAlign(gtk.ALIGN_START)

	headerBox.PackStart(titleLabel, false, false, 0)
	headerBox.PackStart(g.statusLabel, false, false, 0)

	parent.PackStart(headerBox, false, false, 0)
	return nil
}

// createUpdatesList creates the scrollable list of available updates
func (g *UpdaterGUI) createUpdatesList(parent *gtk.Box) error {
	// Create scrolled window
	scrolled, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		return err
	}
	scrolled.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolled.SetShadowType(gtk.SHADOW_IN)

	// Create tree view
	g.updatesTreeView, err = gtk.TreeViewNew()
	if err != nil {
		return err
	}

	// Create list store (columns: selected, icon_pixbuf, name, type, description, action)
	store, err := gtk.ListStoreNew(
		glib.TYPE_BOOLEAN,   // Selected
		gdk.PixbufGetType(), // Icon pixbuf
		glib.TYPE_STRING,    // Name
		glib.TYPE_STRING,    // Type
		glib.TYPE_STRING,    // Description
		glib.TYPE_STRING,    // Action
	)
	if err != nil {
		return err
	}

	g.updatesTreeView.SetModel(store)

	// Create columns
	if err := g.createTreeViewColumns(); err != nil {
		return err
	}

	scrolled.Add(g.updatesTreeView)
	parent.PackStart(scrolled, true, true, 0)

	return nil
}

// createTreeViewColumns creates the columns for the updates tree view
func (g *UpdaterGUI) createTreeViewColumns() error {
	// Checkbox column
	checkRenderer, err := gtk.CellRendererToggleNew()
	if err != nil {
		return err
	}
	checkRenderer.Connect("toggled", g.onItemToggled)

	checkColumn, err := gtk.TreeViewColumnNewWithAttribute("", checkRenderer, "active", 0)
	if err != nil {
		return err
	}
	checkColumn.SetFixedWidth(50)
	g.updatesTreeView.AppendColumn(checkColumn)

	// Icon column
	iconRenderer, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return err
	}

	iconColumn, err := gtk.TreeViewColumnNewWithAttribute("", iconRenderer, "pixbuf", 1)
	if err != nil {
		return err
	}
	iconColumn.SetFixedWidth(50)
	g.updatesTreeView.AppendColumn(iconColumn)

	// Name column
	nameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}

	nameColumn, err := gtk.TreeViewColumnNewWithAttribute("Name", nameRenderer, "markup", 2)
	if err != nil {
		return err
	}
	nameColumn.SetExpand(true)
	nameColumn.SetResizable(true)
	g.updatesTreeView.AppendColumn(nameColumn)

	// Type column
	typeRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}

	typeColumn, err := gtk.TreeViewColumnNewWithAttribute("Type", typeRenderer, "text", 3)
	if err != nil {
		return err
	}
	typeColumn.SetFixedWidth(100)
	g.updatesTreeView.AppendColumn(typeColumn)

	return nil
}

// createProgressSection creates the progress bar and status display
func (g *UpdaterGUI) createProgressSection(parent *gtk.Box) error {
	progressBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		return err
	}

	// Progress bar
	g.progressBar, err = gtk.ProgressBarNew()
	if err != nil {
		return err
	}
	g.progressBar.SetShowText(true)
	g.progressBar.SetVisible(false)

	progressBox.PackStart(g.progressBar, false, false, 0)
	parent.PackStart(progressBox, false, false, 0)

	return nil
}

// createButtonSection creates the action buttons
func (g *UpdaterGUI) createButtonSection(parent *gtk.Box) error {
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return err
	}
	buttonBox.SetHAlign(gtk.ALIGN_END)

	// Cancel button with exit icon
	g.cancelButton, err = gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		return err
	}
	g.addButtonIcon(g.cancelButton, "exit.png")
	g.cancelButton.Connect("clicked", func() {
		gtk.MainQuit()
	})

	// Retry button with refresh icon (initially hidden)
	g.retryButton, err = gtk.ButtonNewWithLabel("Retry")
	if err != nil {
		return err
	}
	g.addButtonIcon(g.retryButton, "refresh.png")
	g.retryButton.SetVisible(false)
	g.retryButton.Connect("clicked", g.onRetryClicked)

	// Rollback button with backup icon (initially hidden)
	g.rollbackButton, err = gtk.ButtonNewWithLabel("Rollback")
	if err != nil {
		return err
	}
	g.addButtonIcon(g.rollbackButton, "backup.png")
	g.rollbackButton.SetVisible(false)
	g.rollbackButton.Connect("clicked", g.onRollbackClicked)

	// Update button with download icon
	g.updateButton, err = gtk.ButtonNewWithLabel("Update Now")
	if err != nil {
		return err
	}
	g.addButtonIcon(g.updateButton, "download.png")
	g.updateButton.SetSensitive(false)
	g.updateButton.Connect("clicked", g.onUpdateClicked)

	// Apply styling to update button
	styleContext, err := g.updateButton.GetStyleContext()
	if err == nil {
		styleContext.AddClass("suggested-action")
	}

	buttonBox.PackStart(g.cancelButton, false, false, 0)
	buttonBox.PackStart(g.retryButton, false, false, 0)
	buttonBox.PackStart(g.rollbackButton, false, false, 0)
	buttonBox.PackStart(g.updateButton, false, false, 0)

	parent.PackStart(buttonBox, false, false, 0)
	return nil
}

// refreshUpdatesList loads and displays available updates
func (g *UpdaterGUI) refreshUpdatesList() {
	go func() {
		// Update status
		glib.IdleAdd(func() {
			g.statusLabel.SetText("Checking for updates...")
			g.progressBar.SetVisible(true)
			g.progressBar.SetPulseStep(0.1)
			g.progressBar.Pulse()

			// Start pulsing animation
			go func() {
				for g.progressBar.GetVisible() {
					glib.IdleAdd(func() {
						if g.progressBar.GetVisible() {
							g.progressBar.Pulse()
						}
					})
					time.Sleep(100 * time.Millisecond)
				}
			}()
		})

		ctx := context.Background()

		// Check repository
		if err := g.updater.CheckRepo(ctx); err != nil {
			glib.IdleAdd(func() {
				g.statusLabel.SetMarkup(fmt.Sprintf("<span color='red'>Failed to check repository: %v</span>", err))
				g.progressBar.SetVisible(false)
			})
			return
		}

		// Get updatable files
		files, err := g.updater.GetUpdatableFiles()
		if err != nil {
			glib.IdleAdd(func() {
				g.statusLabel.SetMarkup(fmt.Sprintf("<span color='red'>Failed to get updatable files: %v</span>", err))
				g.progressBar.SetVisible(false)
			})
			return
		}

		// Get updatable apps
		apps, err := g.updater.GetUpdatableApps()
		if err != nil {
			glib.IdleAdd(func() {
				g.statusLabel.SetMarkup(fmt.Sprintf("<span color='red'>Failed to get updatable apps: %v</span>", err))
				g.progressBar.SetVisible(false)
			})
			return
		}

		// Update UI with results
		glib.IdleAdd(func() {
			g.populateUpdatesList(files, apps)
			g.progressBar.SetVisible(false)

			if len(files) == 0 && len(apps) == 0 {
				g.statusLabel.SetMarkup("<span color='green'>Everything is up to date!</span>")
				g.updateButton.SetSensitive(false)
			} else {
				g.statusLabel.SetText(fmt.Sprintf("Found %d file updates and %d app updates", len(files), len(apps)))
				g.updateButton.SetSensitive(true)
			}
		})
	}()
}

// populateUpdatesList fills the tree view with update items
func (g *UpdaterGUI) populateUpdatesList(files []FileChange, apps []string) {
	model, err := g.updatesTreeView.GetModel()
	if err != nil {
		log.Printf("Failed to get tree view model: %v", err)
		return
	}

	store := model.(*gtk.ListStore)
	store.Clear()

	// Add files
	for _, file := range files {
		iter := store.Append()

		// Load icon as pixbuf
		iconPixbuf := g.loadFileIconPixbuf(file.Type)
		displayName := file.Path

		// Add appropriate annotations based on file type and requirements
		if file.IsModuleFile {
			displayName += " <b>(module update and recompilation required)</b>"
		} else if file.RequiresRecompile {
			displayName += " <b>(requires recompile)</b>"
		}

		store.SetValue(iter, 0, true) // Selected by default
		store.SetValue(iter, 1, iconPixbuf)
		store.SetValue(iter, 2, displayName)
		store.SetValue(iter, 3, strings.Title(file.Type))
		store.SetValue(iter, 4, fmt.Sprintf("File: %s", file.Path))
		store.SetValue(iter, 5, fmt.Sprintf("file:%s", file.Path))
	}

	// Add apps
	for _, app := range apps {
		iter := store.Append()

		// Load icon as pixbuf
		iconPixbuf := g.loadAppIconPixbuf(app)
		displayName := app
		appType := "App Update"

		// Check if it's a new app or requires reinstall
		willReinstall, err := api.WillReinstall(app)
		if err != nil {
			log.Printf("Failed to check if app %s will be reinstalled: %v", app, err)
		} else if willReinstall {
			displayName += " <b>(new update)</b>"
			appType = "App Reinstall"
		}

		store.SetValue(iter, 0, true) // Selected by default
		store.SetValue(iter, 1, iconPixbuf)
		store.SetValue(iter, 2, displayName)
		store.SetValue(iter, 3, appType)
		store.SetValue(iter, 4, fmt.Sprintf("App: %s", app))
		store.SetValue(iter, 5, fmt.Sprintf("app:%s", app))
	}
}

// Event handlers

func (g *UpdaterGUI) onItemToggled(renderer *gtk.CellRendererToggle, pathStr string) {
	model, err := g.updatesTreeView.GetModel()
	if err != nil {
		return
	}

	store := model.(*gtk.ListStore)
	path, err := gtk.TreePathNewFromString(pathStr)
	if err != nil {
		return
	}

	iter, err := store.GetIter(path)
	if err != nil {
		return
	}

	// Toggle the value
	value, err := store.GetValue(iter, 0)
	if err != nil {
		return
	}

	current, err := value.GoValue()
	if err != nil {
		return
	}

	store.SetValue(iter, 0, !current.(bool))
}

func (g *UpdaterGUI) onUpdateClicked() {
	// Get selected items
	g.selectedFiles, g.selectedApps = g.getSelectedItems()

	if len(g.selectedFiles) == 0 && len(g.selectedApps) == 0 {
		g.showMessage("No items selected for update.")
		return
	}

	// Show confirmation dialog if needed
	if g.hasRecompileItems() {
		if !g.showRecompileConfirmation() {
			return
		}
	}

	// Start update process
	g.startUpdate()
}

func (g *UpdaterGUI) onRetryClicked() {
	g.retryButton.SetVisible(false)
	g.rollbackButton.SetVisible(false)
	g.refreshUpdatesList()
}

func (g *UpdaterGUI) onRollbackClicked() {
	if g.lastResult != nil && g.lastResult.RollbackData != nil {
		go func() {
			glib.IdleAdd(func() {
				g.statusLabel.SetText("Rolling back changes...")
				g.progressBar.SetVisible(true)
				g.progressBar.Pulse()
			})

			if err := g.updater.rollback(g.lastResult.RollbackData); err != nil {
				glib.IdleAdd(func() {
					g.statusLabel.SetMarkup(fmt.Sprintf("<span color='red'>Rollback failed: %v</span>", err))
					g.progressBar.SetVisible(false)
				})
				return
			}

			glib.IdleAdd(func() {
				g.statusLabel.SetMarkup("<span color='green'>Rollback completed successfully</span>")
				g.progressBar.SetVisible(false)
				g.rollbackButton.SetVisible(false)
				g.refreshUpdatesList()
			})
		}()
	}
}

// Helper methods

func (g *UpdaterGUI) getSelectedItems() ([]FileChange, []string) {
	var files []FileChange
	var apps []string

	model, err := g.updatesTreeView.GetModel()
	if err != nil {
		return files, apps
	}

	store := model.(*gtk.ListStore)
	iter, valid := store.GetIterFirst()

	for valid {
		// Check if selected
		selectedVal, err := store.GetValue(iter, 0)
		if err != nil {
			valid = store.IterNext(iter)
			continue
		}

		selected, err := selectedVal.GoValue()
		if err != nil || !selected.(bool) {
			valid = store.IterNext(iter)
			continue
		}

		// Get action
		actionVal, err := store.GetValue(iter, 5)
		if err != nil {
			valid = store.IterNext(iter)
			continue
		}

		action, err := actionVal.GoValue()
		if err != nil {
			valid = store.IterNext(iter)
			continue
		}

		actionStr := action.(string)
		if strings.HasPrefix(actionStr, "file:") {
			filePath := strings.TrimPrefix(actionStr, "file:")
			files = append(files, FileChange{
				Path:              filePath,
				Type:              g.updater.getFileType(filePath),
				RequiresRecompile: g.updater.requiresRecompile(filePath),
				IsModuleFile:      g.updater.IsModuleFile(filePath),
			})
		} else if strings.HasPrefix(actionStr, "app:") {
			appName := strings.TrimPrefix(actionStr, "app:")
			apps = append(apps, appName)
		}

		valid = store.IterNext(iter)
	}

	return files, apps
}

func (g *UpdaterGUI) hasRecompileItems() bool {
	for _, file := range g.selectedFiles {
		if file.RequiresRecompile {
			return true
		}
	}
	return false
}

func (g *UpdaterGUI) hasModuleItems() bool {
	for _, file := range g.selectedFiles {
		if file.IsModuleFile {
			return true
		}
	}
	return false
}

func (g *UpdaterGUI) showRecompileConfirmation() bool {
	hasModule := g.hasModuleItems()
	hasRecompile := g.hasRecompileItems()

	var message string
	if hasModule && hasRecompile {
		message = "Some updates require module dependency updates and recompilation. This may take several minutes.\n\nDo you want to continue?"
	} else if hasModule {
		message = "Some updates require module dependency updates. This may take several minutes.\n\nDo you want to continue?"
	} else if hasRecompile {
		message = "Some updates require recompilation. This may take several minutes.\n\nDo you want to continue?"
	} else {
		return true // No confirmation needed
	}

	dialog := gtk.MessageDialogNew(
		g.window,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_QUESTION,
		gtk.BUTTONS_YES_NO,
		message,
	)
	if dialog == nil {
		return false
	}

	if hasModule {
		dialog.SetTitle("Module Update and Recompilation Required")
	} else {
		dialog.SetTitle("Recompilation Required")
	}

	response := dialog.Run()
	dialog.Destroy()

	return response == gtk.RESPONSE_YES
}

func (g *UpdaterGUI) showMessage(message string) {
	dialog := gtk.MessageDialogNew(
		g.window,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_INFO,
		gtk.BUTTONS_OK,
		message,
	)
	if dialog == nil {
		return
	}

	dialog.Run()
	dialog.Destroy()
}

func (g *UpdaterGUI) startUpdate() {
	go func() {
		glib.IdleAdd(func() {
			g.statusLabel.SetText("Updating...")
			g.progressBar.SetVisible(true)
			g.progressBar.SetPulseStep(0.1)
			g.progressBar.Pulse()
			g.updateButton.SetSensitive(false)

			// Start pulsing animation
			go func() {
				for g.progressBar.GetVisible() {
					glib.IdleAdd(func() {
						if g.progressBar.GetVisible() {
							g.progressBar.Pulse()
						}
					})
					time.Sleep(100 * time.Millisecond)
				}
			}()
		})

		result := g.updater.PerformUpdate(g.selectedFiles, g.selectedApps)
		g.lastResult = result

		glib.IdleAdd(func() {
			g.progressBar.SetVisible(false)
			g.updateButton.SetSensitive(true)

			if result.Success {
				message := result.Message
				if result.Recompiled {
					message += " (Recompilation completed)"
				}
				g.statusLabel.SetMarkup(fmt.Sprintf("<span color='green'>%s</span>", message))

				// Don't refresh immediately after an update to avoid re-detecting module files
				// Only refresh after a delay to allow file system to settle
				time.AfterFunc(2*time.Second, func() {
					g.refreshUpdatesList()
				})
			} else {
				g.statusLabel.SetMarkup(fmt.Sprintf("<span color='red'>Update failed: %s</span>", result.Message))
				g.retryButton.SetVisible(true)
				if result.RollbackData != nil {
					g.rollbackButton.SetVisible(true)
				}
			}
		})
	}()
}

func (g *UpdaterGUI) getFileIcon(fileType string) string {
	iconDir := g.updater.directory + "/icons/updater"

	switch fileType {
	case "module":
		return iconDir + "/go-module.png"
	case "script":
		return iconDir + "/golang-file.png"
	case "makefile":
		return iconDir + "/makefile.png"
	case "app":
		return iconDir + "/shell.png"
	case "image":
		return iconDir + "/image.png"
	case "binary":
		return iconDir + "/binary.png"
	default:
		return iconDir + "/text.png"
	}
}

func (g *UpdaterGUI) getAppIcon(appName string) string {
	// Try to get app-specific icon first
	iconPath := fmt.Sprintf("%s/apps/%s/icon-24.png", g.updater.directory, appName)
	if fileExists(iconPath) {
		return iconPath
	}
	// Fall back to shell icon from updater directory
	return g.updater.directory + "/icons/updater/shell.png"
}

func (g *UpdaterGUI) loadFileIconPixbuf(fileType string) interface{} {
	iconPath := g.getFileIcon(fileType)

	// Try to load the icon file
	if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
		// Scale icon to appropriate size
		if scaled, err := pixbuf.ScaleSimple(24, 24, gdk.INTERP_BILINEAR); err == nil {
			return scaled
		}
		return pixbuf
	}

	// Fallback: return nil and let GTK handle it
	log.Printf("Failed to load icon: %s", iconPath)
	return nil
}

func (g *UpdaterGUI) loadAppIconPixbuf(appName string) interface{} {
	iconPath := g.getAppIcon(appName)

	// Try to load the icon file
	if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
		// Scale icon to appropriate size (app icons are usually already 24x24)
		if scaled, err := pixbuf.ScaleSimple(24, 24, gdk.INTERP_BILINEAR); err == nil {
			return scaled
		}
		return pixbuf
	}

	// Fallback: try to load shell icon from updater directory
	shellIconPath := g.updater.directory + "/icons/updater/shell.png"
	if pixbuf, err := gdk.PixbufNewFromFile(shellIconPath); err == nil {
		if scaled, err := pixbuf.ScaleSimple(24, 24, gdk.INTERP_BILINEAR); err == nil {
			return scaled
		}
		return pixbuf
	}

	// Final fallback: return nil
	log.Printf("Failed to load icon for app: %s", appName)
	return nil
}

// addButtonIcon adds an icon to a button
func (g *UpdaterGUI) addButtonIcon(button *gtk.Button, iconName string) {
	iconPath := g.updater.directory + "/icons/" + iconName

	// Try to load the icon
	if pixbuf, err := gdk.PixbufNewFromFile(iconPath); err == nil {
		// Scale icon to appropriate size for buttons (16x16)
		if scaled, err := pixbuf.ScaleSimple(16, 16, gdk.INTERP_BILINEAR); err == nil {
			// Create image widget
			if image, err := gtk.ImageNewFromPixbuf(scaled); err == nil {
				button.SetImage(image)
				button.SetAlwaysShowImage(true) // Ensure both icon and text are shown
			}
		} else if image, err := gtk.ImageNewFromPixbuf(pixbuf); err == nil {
			button.SetImage(image)
			button.SetAlwaysShowImage(true)
		}
	} else {
		// If icon loading fails, log but don't prevent button creation
		log.Printf("Failed to load button icon: %s", iconPath)
	}
}
