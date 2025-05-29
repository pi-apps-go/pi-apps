# Pi-Apps Settings Package

This package provides a native GTK3 reimplementation of the Pi-Apps settings interface using GOTK3 bindings.

## Features

- **Native GTK3 Interface**: Uses GOTK3 bindings for a true native Linux experience
- **Tabbed Interface**: Organizes settings and actions into separate tabs
- **Dynamic Theme Detection**: Automatically detects available GTK3 themes for the App List Style setting
- **Command Line Support**: Supports the same command-line arguments as the original bash script

## Architecture

The package is organized into several modules:

- `settings.go`: Core settings window and data structures
- `ui.go`: UI components and tab creation
- `themes.go`: Theme detection and App List Style handling
- `cmd.go`: Command-line interface and entry points
- `main.go`: Standalone executable entry point

## Usage

### As a Package

```go
import "github.com/botspot/pi-apps/pkg/settings"

// Create and show settings window
window, err := settings.NewSettingsWindow()
if err != nil {
    log.Fatal(err)
}
window.Show()
window.Run()
```

### As a Command Line Tool

```bash
# Show settings GUI
./settings

# Refresh settings (create defaults if missing)
./settings refresh

# Revert all settings to defaults
./settings revert
```

### Building

To build the standalone settings application:

```bash
cd go-rewrite
go build -o settings ./cmd/settings
```

## Dependencies

- GTK3 development libraries
- GOTK3 Go bindings (already included in go.mod)

On Ubuntu/Debian:
```bash
sudo apt install libgtk-3-dev
```

## Features Replicated from Original

1. **Settings Management**: Reads from `etc/setting-params/` and saves to `data/settings/`
2. **Theme Detection**: Dynamically generates theme options from system themes
3. **Action Buttons**: Provides buttons for categories, logs, multi-install, etc.
4. **Desktop Integration**: Creates `.desktop` file for launcher integration
5. **Command Line Compatibility**: Supports `refresh` and `revert` commands

## Settings Tab

The Settings tab displays all configuration options as dropdown menus with:
- Setting name labels with tooltips
- Combo boxes with available options
- Current value pre-selected

Special handling for:
- **App List Style**: Dynamically detects GTK3 themes and xlunch presets

## Actions Tab

The Actions tab provides buttons for:
- **Categories**: Edit app categories
- **Log files**: View installation logs
- **Multi-Install**: Install multiple apps
- **New App**: Create a new app
- **Import App**: Import external apps
- **Multi-Uninstall**: Remove multiple apps

## Window Features

- **Tabbed Interface**: Settings and Actions in separate tabs
- **Responsive Layout**: Properly sized and positioned windows
- **Icon Support**: Uses Pi-Apps icons when available
- **Tooltips**: Helpful descriptions for all options
- **Button Actions**: Save, Cancel, and Reset functionality

## Theming

The App List Style setting dynamically detects:
- System GTK3 themes from standard directories
- User-installed themes from `~/.themes` and `~/.local/share/themes`
- Built-in xlunch theme presets

Since this implementation uses direct GTK3 bindings (not YAD), themes are applied via the `GTK_THEME` environment variable. This provides better integration with the native desktop environment.

### Theme Application

- **GTK Themes**: Applied immediately via `GTK_THEME` environment variable
- **Xlunch Themes**: Represent different display modes for the app list interface
- **Live Preview**: Theme changes are applied immediately to the settings window

### For Other Packages

Other parts of Pi-Apps can use the theme system:

```go
import "github.com/botspot/pi-apps/pkg/settings"

// Get current theme environment for launching GUI applications
env, err := settings.GetCurrentThemeEnvironment()
if err != nil {
    log.Printf("Failed to get theme environment: %v", err)
    env = os.Environ() // fallback to default
}

cmd := exec.Command("some-gui-app")
cmd.Env = env
cmd.Start()

// Or get just the theme name
themeName, err := settings.GetCurrentAppListStyle()
if err == nil {
    fmt.Printf("Current theme: %s", themeName)
}
```

## Error Handling

The package includes comprehensive error handling for:
- Missing PI_APPS_DIR environment variable
- File I/O operations
- GTK widget creation
- Theme detection failures

## Future Enhancements

Possible improvements for future versions:
- Live theme preview
- Settings search/filter
- Keyboard shortcuts
- More sophisticated layout options
- Integration with Pi-Apps theming system 
- Plugin support for different UI toolkit support (that will likely be in the api package first)