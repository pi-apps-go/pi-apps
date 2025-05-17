# Pi-Apps Go Edition

This is a Go-based rewrite of the [Pi-Apps](https://github.com/Botspot/pi-apps) application store for Raspberry Pi.

## Features

- Clean, modern Go implementation
- Same command-line and graphical interface as the original Pi-Apps
- Enhanced performance and error handling
- Improved dependency management (it mostly uses native Go libraries instead of external programs)
- Separate API command for script integration
- Bash wrapper for backward compatibility with existing scripts

## Requirements

- Go 1.17 or higher (on Debian Bullseye and below this will install Go with [instruction guides similar to the install guides](https://go.dev/doc/install), while on Ubuntu this will install a snap package)
- Debian-based Linux distribution (Raspberry Pi OS, Ubuntu, etc.)

## Supported distributions: 
#### This below info is according to application testing runners properties on Pi-Apps, [Adoptium installer code](https://github.com/Botspot/pi-apps/blob/master/api#L972-L978) and [this issue](https://github.com/Botspot/pi-apps/issues/2665)

### Raspberry Pi <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="16"> (2/3/Zero 2 W/4/5)
| Version                         | Supported          |
| -------                         | ------------------ |
|  <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14">  [**Raspberry Pi OS**](https://www.raspberrypi.com/software/operating-systems/) on Debian 12 (bookworm, 32/64 bit) | :white_check_mark: |
|  <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14"> [**Raspberry Pi OS**](https://www.raspberrypi.com/software/operating-systems/) on Debian 11 (bullseye, 32/64 bit) | :white_check_mark: |
| <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14"> [**Raspberry Pi OS**](https://www.raspberrypi.com/software/operating-systems/) on Debian 10 (buster, 32 bit)   | :white_check_mark:                |
| <img src="https://pi-apps.io/img/other-icons/raspberrypi-icon.svg" height="14"> [**Raspberry Pi OS**](https://www.raspberrypi.com/software/operating-systems/) on < Debian 9 (stretch, 32 bit)   | :x:                |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) 24.04.x LTS (noble)      | :white_check_mark: |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) 22.04.x LTS (jammy)      | :white_check_mark: |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) 20.04.x LTS  (focal)   | :white_check_mark:               |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) 18.04.x LTS  (bionic)   | :white_check_mark:               |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> < [**Ubuntu**](https://ubuntu.com/download/raspberry-pi) 16.04.x LTS  (xenial)   | :x:               |


### Nintendo Switch <img src=https://pi-apps.io/img/other-icons/switch-icon.svg height="16">
| Version                         | Supported          |
| -------                         | ------------------ |
| <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [**Switchroot L4T Ubuntu (noble, 24.04)**](https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-noble-installation-guide)   | :white_check_mark:** |
| <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [**Switchroot L4T Ubuntu (jammy, 22.04)**](https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-jammy-installation-guide)   | :white_check_mark:** |
| <img src=https://pi-apps.io/img/other-icons/switchroot-icon.png height="14"> [**Switchroot L4T Ubuntu (bionic, 18.04)**](https://wiki.switchroot.org/wiki/linux/l4t-ubuntu-bionic-installation-guide)  | :white_check_mark:**                |
### Nvidia Jetson
| Version                         | Supported          |
| -------                         | ------------------ |
| <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**Nvidia Jetpack 6**](https://developer.nvidia.com/embedded/jetpack-sdk-62) (noble, 24.04)   | :white_check_mark:** |
| <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**Nvidia Jetpack 5**](https://developer.nvidia.com/embedded/jetpack-sdk-514) (jammy, 22.04)   | :white_check_mark:** |
| <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**Nvidia Jetpack 4**](https://developer.nvidia.com/jetpack-sdk-464) (bionic, 18.04)  | :white_check_mark:**               |
| <img src=https://pi-apps.io/img/other-icons/nvidia-icon.svg height="14"> [**< Nvidia Jetpack 3**](https://developer.nvidia.com/embedded/jetpack-3_3_4) (xenial, 16.04)  | :x:                |
#### Apple Silicon Macs
| Version                         | Supported          |
| -------                         | ------------------ |
| <img src="https://pi-apps.io/img/other-icons/ubuntu-icon.svg" height="14"> [**Ubuntu Asahi**](https://ubuntuasahi.org/) (noble, 24.04)   | :white_check_mark:* |

### [**Pine64**](https://www.pine64.org/), [**Orange Pi**](http://www.orangepi.org/), [**Radxa**](https://rockpi.org/), [**Banana Pi**](https://banana-pi.org/), [**Khadas**](https://www.khadas.com/), [**Inovato**](https://www.inovato.com/), [**Libre Computer**](https://libre.computer/), and other **ARMv7/ARMv8/ARMv9** Devices
| Version                         | Supported          |
| -------                         | ------------------ |
| <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian 12](https://www.debian.org/distrib/) (bookworm, 32/64 bit)*** | :white_check_mark:*
| <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian 11](https://www.debian.org/distrib/) (bullseye, 32/64 bit)*** | :white_check_mark:*
| <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> [Debian 10](https://www.debian.org/distrib/) (buster, 32/64 bit)*** | :white_check_mark:*
| <img src="https://pi-apps.io/img/other-icons/debian-icon.svg" height="14"> < [Debian 9](https://www.debian.org/distrib/) (stretch, 32 bit) | :x:

#### * Note: Not actively tested by the official Pi-Apps developers nor tested at all for the Go rewrite but should work
#### ** Note: Not tested by the maintainers of the Go rewrite but should work
#### *** Note: Only official releases from Debian are guaranteed to work by the official Pi-Apps developers


## Project Structure

```
├── cmd/
│   ├── api/          # API command binary
│   │   └── main.go
│   └── pi-apps/      # Main application binary
│       └── main.go
├── pkg/
│   ├── api/          # Shared API package
│   │   └── api.go
│   └── ...           # Other packages (future)
├── go.mod            # Go module file
├── go.sum            # Go dependencies checksum
├── Makefile          # Build configuration
└── README.md         # This file
```

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/matu6968/pi-apps-go.git
   cd pi-apps-go
   ```

2. Build the applications:
   ```bash
   make build
   ```

3. Install the applications:
   ```bash
   sudo make install
   ```

4. Install the bash wrapper for backward compatibility:
   ```bash
   sudo ln -sf $(pwd)/../bash-go-api /usr/local/bin/api
   # Or copy it to a location in your PATH
   # sudo cp ../bash-go-api /usr/local/bin/api
   ```

This will install both the `pi-apps` and `api` commands, plus the bash wrapper for compatibility with existing scripts.

## Usage

### Pi-Apps Command

```bash
pi-apps <command> [options]
```

Available commands:
- `install <package>`: Install a package
- `uninstall <package>`: Uninstall a package
- `update`: Update package lists
- `upgrade`: Upgrade all packages
- `list`: List installed packages
- `search <query>`: Search for packages
- `show <package>`: Show package details

Options:
- `--debug`: Enable debug output
- `--help`: Show help message
- `--version`: Show version information
- `--logo`: Display the Pi-Apps logo

### API Command

```bash
api <command> [options]
```

Available commands:
- `package_info <package>`: Get information about a package
- `package_installed <package>`: Check if a package is installed
- `package_available <package> [arch]`: Check if a package is available
- `package_dependencies <package>`: List package dependencies
- `package_installed_version <package>`: Get installed version of a package
- `package_latest_version <package>`: Get latest available version of a package
- `package_is_new_enough <pkg> <ver>`: Check if a package version is new enough
- `download_file <url> <destination>`: Download a file
- `file_exists <path>`: Check if a file exists
- `dir_exists <path>`: Check if a directory exists
- `ensure_dir <path>`: Ensure a directory exists
- `copy_file <source> <destination>`: Copy a file
- `error <message>`: Display error message and exit
- `warning <message>`: Display warning message
- `status <message>`: Display status message
- `status_green <message>`: Display success message
- `debug <message>`: Output debug message if debug mode is on
- `add_english`: Add en_US locale for better logging
- `generate_logo`: Display Pi-Apps logo

Options:
- `--debug`: Enable debug output
- `--help`: Show help message
- `--version`: Show version information
- `--logo`: Display the Pi-Apps logo

### Bash Wrapper

The bash wrapper provides backward compatibility with existing Pi-Apps scripts. It can be used in two ways:

1. **Direct execution** - Call it directly like the original API:
   ```bash
   ./bash-go-api generate_logo
   ```

2. **Source it in scripts** - Use it to access the API functions:
   ```bash
   source ./path/to/bash-go-api
   status "Using the API functions..."
   if package_installed "some-package"; then
     status_green "Package is installed!"
   fi
   ```

## Libraries Used

- [lipgloss](https://github.com/charmbracelet/lipgloss): For terminal styling
- [grab](https://github.com/cavaliergopher/grab): For file downloading

## Development

To contribute to the development of Pi-Apps Go Edition:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

Pi-Apps Go Edition is licensed under the same license as the original Pi-Apps aka GPLv3.