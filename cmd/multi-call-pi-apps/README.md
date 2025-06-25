# Pi-Apps Multi-Call Binary

This is a multi-call binary that embeds all Pi-Apps functionality into a single executable. It can behave as any of the separate Pi-Apps binaries (api, gui, manage, settings, updater) based on how it's invoked.

## Usage

### Direct Mode
Call the binary with the desired mode as the first argument:

```bash
./multi-call-pi-apps api package_info Firefox
./multi-call-pi-apps gui --mode gtk
./multi-call-pi-apps manage --install Firefox
./multi-call-pi-apps settings
./multi-call-pi-apps updater gui
```

### Symlink Mode
Create symlinks to the multi-call binary with the names of the individual binaries:

```bash
ln -s multi-call-pi-apps api
ln -s multi-call-pi-apps gui
ln -s multi-call-pi-apps manage
ln -s multi-call-pi-apps settings
ln -s multi-call-pi-apps updater
```

Then use them as normal:

```bash
./api package_info Firefox
./gui --mode gtk
./manage --install Firefox
./settings
./updater gui
```

## Building

```bash
go build -o multi-call-pi-apps
```

Or with build information:

```bash
go build -ldflags "-X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.GitCommit=$(git rev-parse HEAD)" -o multi-call-pi-apps
```

## Features

- **Space Efficient**: Single binary instead of multiple separate binaries
- **Unified Build**: All components built together with shared dependencies
- **Compatible**: Maintains compatibility with existing Pi-Apps scripts and workflows
- **Flexible**: Can be used directly or via symlinks

## Supported Modes

- **api**: Pi-Apps API interface for querying app information
- **gui**: Graphical user interface for Pi-Apps
- **manage**: Command-line management tool for installing/uninstalling apps
- **settings**: Pi-Apps settings interface
- **updater**: Pi-Apps update system

## Implementation Notes

This multi-call binary copies the core functionality from each separate binary:
- `cmd/api/main.go` → `api.go`
- `cmd/gui/main.go` → `gui.go` 
- `cmd/manage/main.go` → `manage.go`
- `cmd/settings/main.go` → `settings.go`
- `cmd/updater/main.go` → `updater.go`

The main dispatcher in `main.go` determines which mode to run based on the program name or first argument. 