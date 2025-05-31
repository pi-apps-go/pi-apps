# Pi-Apps Updater Package

The Pi-Apps Updater package provides a comprehensive update system for Pi-Apps, reimplemented in Go with native GTK3 support and enhanced compilation handling.

## Features

### Core Functionality
- **Repository Synchronization**: Downloads and checks for updates from the Pi-Apps repository
- **File Update Detection**: Compares local files with remote versions to detect changes
- **App Update Detection**: Identifies apps that need updating or are newly available
- **Compilation Handling**: Automatically recompiles Pi-Apps when Go source files change
- **Module Dependency Management**: Runs `go mod tidy` when module files (`go.mod`, `go.sum`) are updated
- **Rollback Support**: Creates backups and allows rolling back failed updates
- **Background Updates**: Performs safe updates automatically without user intervention
- **Real API Integration**: Uses the full Pi-Apps API package for app management

### User Interfaces
- **GUI Mode**: Modern GTK3-based graphical interface with progress tracking
- **CLI Mode**: Interactive command-line interface with selection capabilities
- **Automatic Mode**: Background checking and notification system

### Go-Specific Features
- **Smart Recompilation**: Detects when changes require recompilation (`pkg/`, `cmd/`, `go.mod`, `go.sum`)
- **Module Dependency Updates**: Automatically runs `go mod tidy` before compilation when module files change
- **Compilation Error Handling**: Automatically rolls back on compilation failures
- **Build Integration**: Uses `make install` for proper binary replacement

## Package Structure

```
pkg/updater/
├── updater.go      # Core updater logic with real API integration
├── gui.go          # GTK3 GUI implementation
├── cli.go          # Command-line interface
└── README.md       # This file

cmd/updater/
└── main.go         # Main entry point
```

## Usage

### Command Line Interface

```bash
# Show available updates with interactive selection
updater cli

# Automatically update without prompts
updater cli-yes

# Show GUI update dialog
updater gui

# Check if updates are available (exit code)
updater get-status

# Check for updates and cache results
updater set-status

# Background mode (used by autostart)
updater autostarted
```

### Speed Options

```bash
# Use cached results (faster)
updater cli fast

# Check repository for latest updates (default)
updater cli
```

## Compilation Handling

The updater intelligently handles Go compilation requirements:

### Files that Trigger Recompilation
- Any `.go` file in `pkg/` directory
- Any `.go` file in `cmd/` directory  
- Changes to `go.mod` or `go.sum`

### Module Files that Trigger Dependency Updates
- `go.mod` - Go module definition file
- `go.sum` - Module checksums and versions

### Compilation Process
1. **Backup Creation**: Creates timestamped backups before updates
2. **File Updates**: Updates all files first
3. **Module Dependency Update**: Runs `go mod tidy` if module files were updated
4. **Compilation Check**: Runs `make install` if needed
5. **Error Handling**: Rolls back on compilation or module update failure
6. **Success**: Completes update if all steps succeed

### Display Annotations
The updater provides clear visual indicators for different types of updates:

- **Module Files**: `go.mod (module update and recompilation required)`
- **Go Source Files**: `main.go (requires recompile)`
- **Regular Files**: `README.md` (no special annotation)
- **Apps with Updates**: `AppName (new update)` for reinstalls
- **New Apps**: `AppName (new app)` for first-time availability

### Rollback System
- Automatic rollback on compilation failures
- Automatic rollback on module dependency failures
- Manual rollback option through GUI/CLI
- Preserves original state until update is confirmed successful

## API Integration

The updater now uses the full Pi-Apps API package directly:

### Direct API Functions Used
- `api.ListApps(category)` - Lists apps by category
- `api.GetAppStatus(app)` - Gets app installation status
- `api.WillReinstall(app)` - Checks if app needs reinstallation
- `api.ManageApp(action, app, isUpdate)` - Installs/uninstalls apps

### Environment Setup
The updater automatically sets the `PI_APPS_DIR` environment variable that the API package requires.

## Configuration

### Update Intervals
Configured via `data/settings/Check for updates`:
- `Always` - Check every time
- `Daily` - Check once per day
- `Weekly` - Check once per week  
- `Never` - Disable automatic checking

### Update Exclusions
Files listed in `data/update-exclusion` are skipped during updates.

## Integration

### Build System
The updater is integrated into the main Makefile:

```makefile
build-updater:
    go build -o bin/updater -ldflags "$(LDFLAGS) -w -s" -trimpath ./cmd/updater

install: build
    install -m 755 bin/updater updater
```

### Autostart Integration
Can be configured to run on system startup to check for updates automatically.

## Error Handling

The updater provides comprehensive error handling:

1. **Network Issues**: Retries repository access with exponential backoff
2. **Compilation Errors**: Automatic rollback with user notification
3. **File Permission Issues**: Clear error messages and suggestions
4. **Disk Space**: Checks available space before updates
5. **API Errors**: Graceful handling of app management failures

## Safety Features

- **Backup System**: All changes are backed up before applying
- **Safe Background Updates**: Only applies non-critical updates automatically
- **User Confirmation**: Prompts for confirmation on major changes
- **Recompilation Warnings**: Warns users about time required for compilation

## Future Enhancements

- System notification integration
- Progress reporting for long operations
- Parallel update processing
- Update scheduling
- Delta updates for large files

## Development

To extend the updater:

1. The API integration is complete and uses the real Pi-Apps API
2. Add new update modes to the `UpdateMode` enum
3. Enhance GUI with additional features
4. Add support for more file types requiring special handling

## Dependencies

- **GTK3**: For GUI functionality (`github.com/gotk3/gotk3`)
- **Pi-Apps API**: Full integration with the real API package
- **Standard Library**: No external dependencies for core functionality
- **Make**: For compilation process
- **Git**: For repository operations 