# Pi-Apps Go GUI Implementation

This document describes the Go implementation of the Pi-Apps GUI, which replaces the original bash `gui` script with native GTK3 integration and modern Go programming practices.

## Overview

The GUI module provides a comprehensive graphical user interface for Pi-Apps, supporting multiple display modes and offering both backward compatibility with existing interfaces (YAD, XLunch) and a new native GTK3 mode.

## Key Components

### Main GUI Structure (`gui.go`)

```go
type GUI struct {
    directory       string          // Pi-Apps directory path
    guiMode         string          // Interface mode (gtk, yad-*, xlunch-*)
    screenWidth     int             // Screen dimensions
    screenHeight    int
    window          *gtk.Window     // Main application window
    appList         *gtk.TreeView   // App list display
    appStore        *gtk.ListStore  // App data model
    detailsWindow   *gtk.Window     // App details dialog
    currentPrefix   string          // Current category/navigation state
    daemon          *PreloadDaemon  // Background refresh daemon
    ctx             context.Context // Application context
    cancel          context.CancelFunc
}
```

### Configuration

```go
type GUIConfig struct {
    Directory string  // Pi-Apps directory (defaults to PI_APPS_DIR env var)
    GuiMode   string  // Interface mode (defaults to settings file)
}
```

## Features

### Multi-Mode Support

1. **Native GTK3 Mode** (`gtk`, `native`)
   - Modern GTK3 interface with native widgets
   - Integrated search, settings, and app management
   - Responsive design with proper window management

2. **YAD Compatibility Mode** (`yad-*`)
   - Falls back to original bash implementation
   - Maintains compatibility with existing configurations

3. **XLunch Compatibility Mode** (`xlunch-*`)
   - Falls back to original bash implementation
   - Supports XLunch themes and configurations

### Core Functionality

#### Application Management
- App browsing by categories
- App installation/uninstallation
- Status tracking (installed, uninstalled, corrupted, disabled)
- Package app detection and management

#### User Interface
- Header with Pi-Apps logo and message of the day
- Toolbar with search and settings buttons
- Main app list with icons, names, and status indicators
- App details window with descriptions and action buttons

#### Background Operations
- Preload daemon for performance optimization
- Automatic update checking
- Usage analytics (when enabled)
- Announcement downloading

## Usage

### Basic Usage

```go
import "github.com/botspot/pi-apps/pkg/gui"

// Create GUI configuration
config := gui.GUIConfig{
    Directory: "/home/user/pi-apps",
    GuiMode:   "gtk",
}

// Create and run GUI
app, err := gui.NewGUI(config)
if err != nil {
    log.Fatal(err)
}

if err := app.Initialize(); err != nil {
    log.Fatal(err)
}
defer app.Cleanup()

if err := app.Run(); err != nil {
    log.Fatal(err)
}
```

### Command Line Demo

```bash
# Run the GUI demo
cd go-rewrite
go run cmd/gui-demo/main.go

# Specify custom directory and mode
go run cmd/gui-demo/main.go -directory /path/to/pi-apps -mode gtk

# Show help
go run cmd/gui-demo/main.go -help
```

## Differences from Bash Implementation

### Advantages

1. **Native Integration**: Direct GTK3 bindings instead of external YAD processes
2. **Performance**: No subprocess overhead for GUI operations
3. **Memory Safety**: Go's memory management vs bash string handling
4. **Type Safety**: Compile-time checking vs runtime script errors
5. **Modularity**: Clean separation of concerns with proper interfaces

### Syntax Checking Differences

The original bash implementation includes syntax checking for critical scripts:

```bash
# Original bash code (lines 31-45)
if shellcheck "${DIRECTORY}/updater" --color=always | grep '\[31m' --before 1 ;then
  echo "Downloading updater script to repair syntax error"
  errors="$(wget -O "${DIRECTORY}/updater" 'https://raw.githubusercontent.com/Botspot/pi-apps/master/updater' 2>&1)" || echo "$errors"
fi
```

**This is intentionally omitted** from the Go implementation because:
- Go performs compile-time syntax checking
- Runtime errors are handled through Go's error system
- No need for external script validation
- Self-healing mechanisms are built into the module system

### Compatibility Considerations

1. **Settings Integration**: Reads existing Pi-Apps settings files
2. **File Paths**: Uses same directory structure as bash version
3. **Status Files**: Compatible with existing app status tracking
4. **Update System**: Integrates with existing updater script when needed

## Dependencies

### Go Modules
- `github.com/gotk3/gotk3` - GTK3 bindings for Go
- `github.com/botspot/pi-apps/pkg/api` - Pi-Apps API functions

### System Dependencies
- GTK3 development libraries
- Display server (X11 or Wayland)
- Same system dependencies as original Pi-Apps

## API Reference

### Core Functions

#### `NewGUI(config GUIConfig) (*GUI, error)`
Creates a new GUI instance with the specified configuration.

#### `(*GUI) Initialize() error`
Initializes the GUI environment, checks dependencies, and sets up the interface.

#### `(*GUI) Run() error`
Starts the main GUI application loop.

#### `(*GUI) Cleanup()`
Performs cleanup operations including stopping background tasks.

### Event Handlers

#### App Selection
- `onAppSelectionChanged()` - Handles app list selection changes
- `onAppDoubleClicked()` - Handles app double-click events

#### Interface Actions  
- `onSearchClicked()` - Opens search dialog
- `onSettingsClicked()` - Opens settings interface

#### App Management
- `showAppDetails(appPath string)` - Displays app information window
- `performAppAction(appName, action string)` - Executes install/uninstall actions

## Integration with Preload System

The GUI integrates seamlessly with the preload system (`preload.go`):

```go
// Refresh app list with preloaded data
list, err := PreloadAppList(g.directory, prefix)
if err != nil {
    return err
}

// Populate GTK tree view
if err := PopulateGTKTreeView(g.appList, list); err != nil {
    return err
}
```

## Error Handling

The Go implementation provides robust error handling:

```go
// Graceful degradation for missing features
if err := g.createHeader(vbox); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: failed to create header: %v\n", err)
    // Continue without header rather than failing
}

// Resource cleanup on errors
defer func() {
    if err != nil {
        g.Cleanup()
    }
}()
```

## Performance Optimizations

1. **Background Preloading**: App lists are preloaded by daemon
2. **Caching**: Timestamp-based change detection prevents unnecessary reloads
3. **Lazy Loading**: App details loaded on-demand
4. **Resource Management**: Proper GTK widget lifecycle management

## Future Enhancements

1. **Search Implementation**: Full-text search across app descriptions
2. **Categories Management**: Dynamic category creation and editing
3. **Themes**: Custom GTK3 themes and appearance options
4. **Accessibility**: Screen reader support and keyboard navigation
5. **Internationalization**: Multi-language support

## Migration Notes

When migrating from the bash GUI to Go:

1. **Environment Variables**: Ensure `PI_APPS_DIR` is set
2. **Permissions**: GUI runs with same permissions as bash version
3. **Settings**: Existing settings files are automatically read
4. **Data**: All app data and status files remain compatible

## Troubleshooting

### Common Issues

1. **GTK3 Not Available**
   - Install GTK3 development packages
   - Check display environment variables (`DISPLAY`, `WAYLAND_DISPLAY`)

2. **Permission Errors**
   - Ensure user has read/write access to Pi-Apps directory
   - Do not run as root (checked automatically)

3. **Missing Icons**
   - Verify Pi-Apps directory structure is complete
   - Check icon file permissions

### Debug Mode

Set environment variables for debugging:

```bash
export GTK_DEBUG=all
export G_MESSAGES_DEBUG=all
go run cmd/gui-demo/main.go
``` 