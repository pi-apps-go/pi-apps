# Preload Package - GTK3 App List Generator

This package reimplements the functionality of the bash `preload` script in Go, specifically for the GTK3-based GUI rewrite of Pi-Apps. Instead of generating YAD or XLaunch formatted lists, it provides native GTK3 TreeView components and data structures.

## Overview

The preload package handles:

1. **Timestamp-based change detection** - Only regenerates app lists when underlying data changes
2. **App and category enumeration** - Uses the existing API package to list apps and categories
3. **GTK3 TreeView population** - Provides ready-to-use GTK3 components for displaying app lists
4. **Caching** - Stores generated lists for faster subsequent loads
5. **Updates detection** - Shows special "Updates" category when Pi-Apps updates are available

## Key Components

### Data Structures

- **`AppListItem`** - Represents a single item (app, category, or back button) in the list
- **`PreloadedList`** - Contains a collection of `AppListItem`s with metadata
- **`AppListConfig`** - Configuration for app list generation
- **`TimeStampChecker`** - Manages change detection across monitored directories

### Main Functions

#### `PreloadAppList(directory, prefix string) (*PreloadedList, error)`
The main entry point that either loads a cached list or generates a new one.

```go
// Load the main app list (all categories)
list, err := PreloadAppList("", "")

// Load apps within the "Games" category
gamesList, err := PreloadAppList("", "Games")
```

#### `CreateAppListTreeView() (*gtk.TreeView, *gtk.ListStore, error)`
Creates a GTK3 TreeView configured for displaying app lists.

```go
treeView, listStore, err := CreateAppListTreeView()
if err != nil {
    return err
}
```

#### `PopulateGTKTreeView(treeView *gtk.TreeView, list *PreloadedList) error`
Populates a TreeView with app list data.

```go
err := PopulateGTKTreeView(treeView, list)
if err != nil {
    return err
}
```

#### `GetSelectedAppPath(treeView *gtk.TreeView) (string, error)`
Gets the path of the currently selected item for navigation.

```go
path, err := GetSelectedAppPath(treeView)
if err == nil {
    fmt.Printf("Selected: %s\n", path)
}
```

## Usage Example

See `preload_demo.go` for a complete example of building an app browser using this package.

```go
// Create a simple app browser
demo, err := NewAppBrowserDemo()
if err != nil {
    log.Fatal(err)
}

demo.Show()
demo.Run()
```

## Differences from Bash Version

### Format
- **Bash version**: Generates text output for YAD/XLaunch consumption
- **Go version**: Provides GTK3 TreeView components and structured data

### Dependencies
- **Bash version**: Depends on external `yad` or `xlunch` tools
- **Go version**: Uses native GTK3 bindings (GOTK3)

### Integration
- **Bash version**: Outputs text that gets piped to GUI tools
- **Go version**: Returns structured data and ready-to-use GTK3 widgets

### Performance
- Both versions use timestamp-based caching for optimal performance
- Go version has slightly more overhead due to GTK3 object creation but provides more flexibility

## File Structure

```
pkg/gui/
├── preload.go          # Main preload functionality
├── preload_demo.go     # Demo application showing usage
└── README-preload.md   # This documentation
```

## Dependencies

- **github.com/botspot/pi-apps/pkg/api** - For app listing and category functions
- **github.com/gotk3/gotk3/gtk** - GTK3 bindings
- **github.com/gotk3/gotk3/gdk** - GDK bindings for pixbufs
- **github.com/gotk3/gotk3/glib** - GLib bindings for data types

## Environment Variables

- **`PI_APPS_DIR`** - Must be set to the Pi-Apps installation directory

## Caching

Cached files are stored in `$PI_APPS_DIR/data/preload/`:
- `timestamps-<prefix>` - Timestamp files for change detection
- `LIST-<prefix>` - Cached app list data

## Future Enhancements

1. **Custom serialization format** - Currently uses simple pipe-delimited format, could be optimized
2. **Background preloading** - Like the original bash version's daemon mode
3. **Search functionality** - Integration with app search capabilities
4. **Status coloring** - Color-coding based on app installation status
5. **Icons optimization** - Pixbuf caching for better performance

## Migration Notes

When migrating from the bash `preload` script:

1. Replace `./preload yad ""` calls with `PreloadAppList("", "")`
2. Replace YAD list parsing with GTK3 TreeView handling
3. Update navigation logic to use structured `AppListItem` data
4. Modify any scripts that parse the old text-based output format

This Go implementation provides the same core functionality while offering better integration with the native GTK3 interface and more robust error handling. 