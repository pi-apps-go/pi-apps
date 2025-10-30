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

// Module: log_diagnose_dummy.go
// Description: Provides functions for diagnosing errors if no package manager is available.

//go:build dummy

package api

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LogDiagnose analyzes a logfile and returns diagnostic information
//
// It takes a logfile path and an allowWrite parameter
//
//	ErrorDiagnosis - the error diagnosis
//	error - error if logfile is not specified
func LogDiagnose(logfilePath string, allowWrite bool) (*ErrorDiagnosis, error) {
	// Read the logfile
	content, err := os.ReadFile(logfilePath)
	if err != nil {
		return nil, err
	}

	errors := string(content)

	// Determine the actual log file path to write to - used when implementing write functionality
	// Currently not used in this implementation
	_ = logfilePath
	if !allowWrite {
		// If not allowed to write, we would use /dev/null in bash
		// In Go, we simply won't write to the file
	}

	// Initialize the diagnosis struct
	diagnosis := &ErrorDiagnosis{
		ErrorType: "",
		Captions:  []string{},
	}

	// Check for various error patterns

	//------------------------------------------
	// package issues below
	//------------------------------------------

	// check for "Consult /var/lib/dkms/anbox-ashmem/1/build/make.log for more information."
	regexAnboxCompileFailure := regexp.MustCompile(`Consult /var/lib/dkms/anbox-ashmem/1/build/make.log for more information.`)
	if regexAnboxCompileFailure.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Anbox kernel modules no longer compile on the latest kernel. You need to remove it for the kernel to fully install and for APT to work.\n"+
				"Run this command to remove anbox kernel modules, then retry the operation.\n\n"+
				"sudo rm -rf /etc/modules-load.d/anbox.conf /lib/udev/rules.d/99-anbox.rules /usr/src/anbox-ashmem-1/ /usr/src/anbox-binder-1/ /var/lib/dkms/anbox-*")
		diagnosis.ErrorType = "package"
	}

	// check for "M=/var/lib/dkms/xone.*bad exit status"
	regexXoneCompileFailure := regexp.MustCompile(`M=/var/lib/dkms/xone.*bad exit status`)
	if regexXoneCompileFailure.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The Xone kernel module no longer compile on the latest kernel. You need to remove it for the kernel to fully install and for APT to work.\n"+
				"Run this command to remove the xone kernel module, then retry the operation:\n\n"+
				"sudo rm -rf /etc/modules-load.d/xone.conf /etc/udev/rules.d/50-xone.rules /usr/src/xone-*/ /var/lib/dkms/xone-*")
		diagnosis.ErrorType = "package"
	}

	// check for "Reinstalling Vulkan driver"
	regexReinstallVulkan := regexp.MustCompile(`Reinstalling Vulkan driver`)
	if regexReinstallVulkan.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"PiKiss has installed a broken custom vulkan reinstallation apt hook. As a result, dpkg and APT won't work properly.")
		diagnosis.ErrorType = "package"
	}

	// Non-APT related errors below

	// cargo package errors below

	// Check for incompatible dependencies
	regexDependencyConflict := regexp.MustCompile(`error: failed to select a version for the requirement.*version conflict`)
	if regexDependencyConflict.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Rust compilation failed due to conflicting dependency requirements.\n\n"+
				"This typically happens when different parts of your project require incompatible versions of the same crate.\n"+
				"You may need to update the Cargo.toml file to resolve these conflicts or contact the application developer.")
		diagnosis.ErrorType = "package"
	}

	// internet errors below

	// check for "Could not resolve host: github\.com\|Failed to connect to github\.com port 443: Connection timed out" aka internet errors
	regexInternetError := regexp.MustCompile(`Could not resolve host: github\.com\|Failed to connect to github\.com port 443: Connection timed out`)
	if regexInternetError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Failed to connect to github.com.\n\n"+
				"Check your internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "fetch-pack: unexpected disconnect while reading sideband packet" aka git errors
	regexFetchPack := regexp.MustCompile(`fetch-pack: unexpected disconnect while reading sideband packet`)
	if regexFetchPack.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"fetch-pack: unexpected disconnect while reading sideband packet\" Check the stability of your Internet connection and try again. \n\n"+
				"If this keeps happening, see: https://stackoverflow.com/questions/66366582")
		diagnosis.ErrorType = "internet"
	}

	// check for "fatal: did not receive expected object" aka git errors
	regexFatalObject := regexp.MustCompile(`fatal: did not receive expected object`)
	if regexFatalObject.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"fatal: did not receive expected object\" Check the stability of your Internet connection and try again.\n\n"+
				"If this keeps happening, see: https://stackoverflow.com/questions/66366582")
		diagnosis.ErrorType = "internet"
	}

	// check for "fatal: the remote end hung up unexpectedly" aka git errors
	regexRemoteEndHungUp := regexp.MustCompile(`fatal: the remote end hung up unexpectedly`)
	if regexRemoteEndHungUp.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"fatal: the remote end hung up unexpectedly\" Check the stability of your Internet connection and try again.\n\n"+
				"If this keeps happening, see: https://stackoverflow.com/questions/66366582")
		diagnosis.ErrorType = "internet"
	}

	// check for SSL/TLS handshake failure, total length mismatch, failed to establish connection, timeout, connection reset by peer, name resolution failed, temporary failure in name resolution, unable to establish SSL connection, connection closed at byte, read error at byte, failed: No route to host, invalid range header, curl error, response status not successful, download snap, dial tcp, lookup api.snapcraft.io, fatal: unable to access 'https://github.com.*': Failed to connect to github.com port 443 after .* ms: Couldn't connect to server, RPC failed; curl .* transfer closed with outstanding read data remaining, RPC failed; curl .* GnuTLS recv error (-9): A TLS packet with unexpected length was received., SSL error, failure when receiving data from the peer, java.net.SocketTimeoutException: Read timed out which include git errors
	regexSslError := regexp.MustCompile(`SSL/TLS handshake failure\|total length mismatch\|failed to establish connection\|timeout\|connection reset by peer\|name resolution failed\|temporary failure in name resolution\|unable to establish SSL connection\|connection closed at byte\|read error at byte\|failed: No route to host\|invalid range header\|curl error\|response status not successful\|download snap\|dial tcp\|lookup api\.snapcraft\.io\|fatal: unable to access 'https://github.com.*': Failed to connect to github.com port 443 after .* ms: Couldn't connect to server\|RPC failed; curl .* transfer closed with outstanding read data remaining\|RPC failed; curl .* GnuTLS recv error (-9): A TLS packet with unexpected length was received.\|SSL error\|failure when receiving data from the peer\|java\.net\.SocketTimeoutException: Read timed out`)
	if regexSslError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"SSL/TLS handshake failure\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)"
	regexCurlError := regexp.MustCompile(`curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)`)
	if regexCurlError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed due to an internal curl error. This could be an internet issue or hardware problem. \n"+
				"If you are overclocking, try reverting to stock clocks. Additionally, check your internet connection and firewall, then try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "errorCode=24 Authorization failed."
	regexAuthorizationFailed := regexp.MustCompile(`errorCode=24 Authorization failed.`)
	if regexAuthorizationFailed.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The curl command encountered this error: \"errorCode=24 Authorization failed.\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "flathub: Error resolving .dl\.flathub\.org."
	regexFlathubError := regexp.MustCompile(`flathub: Error resolving .dl\.flathub\.org.`)
	if regexFlathubError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The flathub command encountered this error: \"flathub: Error resolving .dl.flathub.org.\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "The TLS connection was non-properly terminated\.\|Can't load uri .* Unacceptable TLS certificate"
	regexTlsError := regexp.MustCompile(`The TLS connection was non-properly terminated\.\|Can't load uri .* Unacceptable TLS certificate`)
	if regexTlsError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The TLS connection was non-properly terminated. Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// Check for "GnuTLS recv error (-54): Error in the pull function."
	regexGnuTlsError := regexp.MustCompile(`GnuTLS recv error (-54): Error in the pull function.`)
	if regexGnuTlsError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed due to an issue with your internet, not Pi-Apps. The connection was terminated before the download completed. \n\n"+
				"This can be caused by your or your ISPs configured firewalls. \n\n"+
				"Here are some suggested mitigations for your bad internet connection: https://stackoverflow.com/questions/38378914/how-to-fix-git-error-rpc-failed-curl-56-gnutls")
		diagnosis.ErrorType = "internet"
	}

	// check for "java.net.ConnectException: Connection refused"
	regexConnectException := regexp.MustCompile(`java\.net\.ConnectException: Connection refused`)
	if regexConnectException.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed. Check your internet connection and firewall, then try again.")
		diagnosis.ErrorType = "internet"
	}

	// Check for cargo crate not found error
	regexCrateNotFound := regexp.MustCompile(`error: failed to get .*? as a dependency.*no matching package named`)
	if regexCrateNotFound.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Rust compilation failed because a required crate dependency was not found.\n\n"+
				"This could be due to:\n"+
				"1. Network connectivity issues when fetching crates\n"+
				"2. The crate being removed from crates.io\n"+
				"3. Version incompatibility issues\n\n"+
				"Try running 'cargo clean' and attempt the installation again with an active internet connection.")
		diagnosis.ErrorType = "internet"
	}

	// Check for cargo network errors
	regexCargoNetwork := regexp.MustCompile(`error: failed to fetch from.*could not connect to server|error: failed to fetch.*Network is unreachable`)
	if regexCargoNetwork.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Cargo couldn't connect to the crates.io registry or other dependency sources.\n\n"+
				"Please check your internet connection and try again. If you're behind a proxy, make sure it's configured correctly for Cargo.")
		diagnosis.ErrorType = "internet"
	}

	// Check for ERROR: Downloaded system image hash doesn't match, expected <hash> from Waydroid
	regexHashDoesNotMatch := regexp.MustCompile(`ERROR: Downloaded system image hash doesn't match, expected`)
	if regexHashDoesNotMatch.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Waydroid OS image download failed. Check your internet connection and firewall, then try again.")
		diagnosis.ErrorType = "internet"
	}

	// other system errors below

	// check for modprobe: FATAL: Module .* not found in directory
	regexModuleNotFound := regexp.MustCompile(`modprobe: FATAL: Module .* not found in directory`)
	if regexModuleNotFound.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Something is wrong with the kernel modules. Try rebooting if your kernel was upgraded. \n\n"+
				"Otherwise, try reinstalling the kernel using this command: \n"+
				"sudo apt install --reinstall raspberrypi-bootloader raspberrypi-kernel \n\n"+
				"See this forum thread: https://raspberrypi.org/forums/viewtopic.php?t=262963")
		diagnosis.ErrorType = "system"
	}

	// check for "Failed to load module \"appmenu-gtk-module\""
	regexAppmenuGtkModule := regexp.MustCompile(`Failed to load module "appmenu-gtk-module"`)
	if regexAppmenuGtkModule.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"This error occurred: Failed to load module \"appmenu-gtk-module\" \n\n"+
				"Try installing two packages with this command: \n"+
				"sudo apt install appmenu-gtk2-module appmenu-gtk3-module \n\n"+
				"And if that doesn't work, try Googling the errors or reach out to Pi-Apps developers for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "error: Unable to connect to system bus\|error: Message recipient disconnected from message bus without replying\|Failed to connect to bus: Host is down"
	regexDBus := regexp.MustCompile(`error: Unable to connect to system bus\|error: Message recipient disconnected from message bus without replying\|Failed to connect to bus: Host is down`)
	if regexDBus.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Something is wrong with your dbus connection. \n\n"+
				"Try rebooting. \n\n"+
				"Make sure systemd is setup correctly. \n\n"+
				"If that doesn't help please read through this: https://github.com/WhitewaterFoundry/Fedora-Remix-for-WSL/issues/81 \n\n"+
				"You may want to reinstall your OS. \n\n"+
				"Also consider reaching out to Pi-Apps developers for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "cat: /usr/share/i18n/SUPPORTED: No such file or directory"
	regexI18n := regexp.MustCompile(`cat: /usr/share/i18n/SUPPORTED: No such file or directory`)
	if regexI18n.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your system is messed up - the /usr/share/i18n/SUPPORTED file does not exist. \n\n"+
				"Try reinstalling the locales package: \n"+
				"sudo apt install --reinstall locales")
		diagnosis.ErrorType = "system"
	}

	// check for "is not in the sudoers file.  This incident will be reported."
	regexSudoers := regexp.MustCompile(`is not in the sudoers file.  This incident will be reported.`)
	if regexSudoers.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}
		diagnosis.Captions = append(diagnosis.Captions,
			"Unable to use the sudo command - the current user "+currentUser+" is not allowed to use it. \n\n"+
				"Please enable passwordless sudo or switch to a more privelaged user-account. \n\n"+
				"See: https://www.tecmint.com/fix-user-is-not-in-the-sudoers-file-the-incident-will-be-reported-ubuntu/")
		diagnosis.ErrorType = "system"
	}

	// check for "sudo: .* incorrect password attempts"
	regexIncorrectPassword := regexp.MustCompile(`sudo: .* incorrect password attempts`)
	if regexIncorrectPassword.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Process could not complete because you failed to type in the correct sudo password. \n\n"+
				"Try again, and consider enabling passwordless sudo.")
		diagnosis.ErrorType = "system"
	}

	// check for "sudo: unable to resolve host\|sudo: no valid sudoers sources found, quitting"
	regexSudoHost := regexp.MustCompile(`sudo: unable to resolve host\|sudo: no valid sudoers sources found, quitting`)
	if regexSudoHost.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Process could not complete because your sudo command is incorrectly set up. \n\n"+
				"For solutions, see: https://askubuntu.com/a/59517")
		diagnosis.ErrorType = "system"
	}

	// check for "cpp.o: file not recognized: file truncated"
	regexCpp := regexp.MustCompile(`cpp.o: file not recognized: file truncated`)
	if regexCpp.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Compiling failed. Try again, but please reach out to Pi-Apps developers for help if this same error keeps occurring.")
		diagnosis.ErrorType = "system"
	}

	// check for "tar: Unexpected EOF in archive\|xz: (stdin): Unexpected end of input\|xz: (stdin): Compressed data is corrupt\|xz: (stdin): File format not recognized\|gzip: stdin: invalid compressed data\-\-length error\|gzip: stdin: invalid compressed data\-\-crc error\|corrupted filesystem tarfile in package archive: invalid tar header size field (Invalid argument)\|member 'data.tar': internal gzip read error: '<fd:4>: incorrect data check\|error inflating zlib stream;"
	regexTar := regexp.MustCompile(`tar: Unexpected EOF in archive\|xz: (stdin): Unexpected end of input\|xz: (stdin): Compressed data is corrupt\|xz: (stdin): File format not recognized\|gzip: stdin: invalid compressed data\-\-length error\|gzip: stdin: invalid compressed data\-\-crc error\|corrupted filesystem tarfile in package archive: invalid tar header size field (Invalid argument)\|member 'data.tar': internal gzip read error: '<fd:4>: incorrect data check\|error inflating zlib stream;`)
	if regexTar.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Extraction failed. Most likely this was a corrupted download, so please try again. \n\n"+
				"If this problem continues occurring, please reach out to the Pi-Apps developers for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "xz: Cannot exec: No such file or directory"
	regexXz := regexp.MustCompile(`xz: Cannot exec: No such file or directory`)
	if regexXz.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Extraction failed because XZ is not installed. \n\n"+
				"To install XZ, run this in a terminal: \n"+
				"echo 'No package manager is available to install XZ.'")
		diagnosis.ErrorType = "system"
	}

	// check for "aria2c: error while loading shared libraries: /lib/arm-linux-gnueabihf/libaria2.so.0: unexpected reloc type 0xc8"
	regexAria2c := regexp.MustCompile(`aria2c: error while loading shared libraries: /lib/arm-linux-gnueabihf/libaria2.so.0: unexpected reloc type 0xc8`)
	if regexAria2c.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed because aria2c could not load the libaria2 library. \n\n"+
				"Try reinstalling the package: \n"+
				"echo 'No package manager is available to reinstall libaria2-0.'")
		diagnosis.ErrorType = "system"
	}

	// check for "errorCode=16 Failed to open the file .*, cause: Permission denied"
	regexPermissionDenied := regexp.MustCompile(`errorCode=16 Failed to open the file .*, cause: Permission denied`)
	if regexPermissionDenied.MatchString(errors) {
		// Extract the file path from the error message
		re := regexp.MustCompile(`errorCode=16 Failed to open the file (.*), cause: Permission denied`)
		matches := re.FindStringSubmatch(errors)

		var folderPath string
		if len(matches) > 1 {
			// Get the directory path from the file path
			folderPath = filepath.Dir(matches[1])
		} else {
			folderPath = "<unknown folder>"
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed because this folder was unable to be written: \n"+folderPath)
		diagnosis.ErrorType = "system"
	}

	// check for "Structure needs cleaning"
	regexStructureNeedsCleaning := regexp.MustCompile(`Structure needs cleaning`)
	if regexStructureNeedsCleaning.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your have encountered the dreaded \"Structure needs cleaning\" error. This indicates file-corruption caused by improperly shutting down your computer. You are lucky your computer booted at all.\n\n"+
				"You can try scheduling a filesystem cleanup: \n"+
				"sudo touch /forcefsck \n\n"+
				"After running that command, reboot and see if that fixes the problem. \n\n"+
				"If that doesn't work, then now is the time to restore your backup. Oh, you don't have one? Then you will have to re-flash your SD card and start over. And maybe consider keeping regular backups to avoid this unpleasant situation next time.")
		diagnosis.ErrorType = "system"
	}

	// check for "VCHI initialization failed"
	regexVCHI := regexp.MustCompile(`VCHI initialization failed`)
	if regexVCHI.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}
		diagnosis.Captions = append(diagnosis.Captions,
			"You have encountered the 'VCHI initialization failed' error. This means that a program was not allowed to display something to the screen. \n\n"+
				"You can try to fix the error by adding your user to the video group. Run this command in a terminal: \n"+
				"sudo usermod -a -G video "+currentUser+" \n\n"+
				"See: https://raspberrypi.stackexchange.com/a/8423/107602")
		diagnosis.ErrorType = "system"
	}

	// check for "Error: Failed to read commit .* No such metadata object\|error: Failed to install org.freedesktop.Platform: Failed to read commit .* No such metadata object\|Error: Error deploying: .* No such metadata object"
	regexFlatpak := regexp.MustCompile(`Error: Failed to read commit .* No such metadata object\|error: Failed to install org.freedesktop.Platform: Failed to read commit .* No such metadata object\|Error: Error deploying: .* No such metadata object`)
	if regexFlatpak.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Flatpak failed to install something due to a past incompleted download. \n\n"+
				"To repair it, please run this command in a terminal: \n"+
				"flatpak repair --user \n\n"+
				"See: https://github.com/flatpak/flatpak/issues/3479")
		diagnosis.ErrorType = "system"
	}

	// check for "No space left on device"
	regexSpace := regexp.MustCompile(`No space left on device\|Not enough disk space to complete this operation\|You don't have enough free space in\|Cannot write to .* (Success)\.`)
	if regexSpace.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your system has insufficient disk space.\n\n"+
				"Please free up some space, then try again.")
		diagnosis.ErrorType = "system"
	}

	// check for permission denied when creating autostart entries
	regexAutostart := regexp.MustCompile(`: line .*: \$HOME/\.config/autostart/.*\.desktop: Permission denied`)
	if regexAutostart.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Failed to create an autostart entry because the folder permissions are incorrect.\n\n"+
				"This was most likely caused by running an install script as root in the past. Don't do that.\n\n"+
				"You can fix the folder's permissions by running this command in a terminal:\n"+
				"sudo chown "+currentUser+":"+currentUser+" ~/.config/autostart")
		diagnosis.ErrorType = "system"
	}

	// check for "The directory '$HOME/\.cache/pip' or its parent directory is not owned by the current user"
	regexCache := regexp.MustCompile(`The directory '(\$HOME|\$\{HOME\}|/home/[^/]+)/\.cache/pip' or its parent directory is not owned by the current user`)
	if regexCache.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}

		// Get home directory
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "$HOME" // Fallback if we can't get the actual home directory
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"The Python package manager (pip3) could not make changes to its own cache folder: "+homeDir+"/.cache/pip\n\n"+
				"Most likely, you tried running pip3 with sudo in the past, or you tried running a Pi-Apps script with sudo in the past. (not recommended!)\n\n"+
				"To fix this, run this command: \n"+
				"sudo chown -R "+currentUser+":"+currentUser+" "+homeDir+"/.cache/pip")
		diagnosis.ErrorType = "system"
	}

	// check for "mkdir: cannot create directory .*/home/username/pi-apps-.*: Permission denied"
	regexMkdir := regexp.MustCompile(`mkdir: cannot create directory .*/home/[^/]+/pi-apps-.*: Permission denied|rm: cannot remove .*/home/[^/]+/.*: Permission denied`)
	if regexMkdir.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Your HOME directory cannot be written to by the current user. \n\n"+
				"Most likely, you ran some command that made your HOME directory root owned.\n\n"+
				"To fix this, run this command: \n"+
				"sudo chown -R "+currentUser+":"+currentUser+" $HOME")
		diagnosis.ErrorType = "system"
	}

	// check for segmentation fault from linker
	regexSegfault := regexp.MustCompile(`collect2: fatal error: ld terminated with signal 11 \[Segmentation fault\]`)
	if regexSegfault.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Failed to compile! The error was: \"collect2: fatal error: ld terminated with signal 11 [Segmentation fault]\"\n\n"+
				"This usually indicates a hardware problem, most likely with your RAM. Try rebooting your computer.")
		diagnosis.ErrorType = "system"
	}

	// check for "No LSB modules are available"
	regexLsb := regexp.MustCompile(`ModuleNotFoundError: No module named 'lsb_release'`)
	if regexLsb.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your lsb_release command seems to be incompletely installed. Try running this command to fix it: \n"+
				"sudo apt install --reinstall lsb-release")
		diagnosis.ErrorType = "system"
	}

	// check for "c++: fatal error: Killed signal terminated program cc1plus"
	regexKilled := regexp.MustCompile(`c\+\+: fatal error: Killed signal terminated program cc1plus`)
	if regexKilled.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Compiling failed because cc1plus was killed due to insufficient RAM.\n\n"+
				"Please try installing the application again, but this time keep all other programs closed to preserve more free RAM.\n"+
				"If this error persists, try installing the More RAM app from Pi-Apps. Find it in the Tools category.")
		diagnosis.ErrorType = "system"
	}

	// check for error: system does not fully support snapd: cannot mount squashfs image
	regexSnapd := regexp.MustCompile(`error: system does not fully support snapd: cannot mount squashfs image`)
	if regexSnapd.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Snap failed to fully install due to this error: 'error: system does not fully support snapd: cannot mount squashfs image'\n\n"+
				"Your Operating System is likely custom to some extent, or otherwise unusual to be having this issue. Try searching the internet. Include your setup and the error message.")
		diagnosis.ErrorType = "system"
	}

	// check for "Error: All VeraCrypt volumes must be dismounted first."
	regexVeraCrypt := regexp.MustCompile(`Error: All VeraCrypt volumes must be dismounted first.`)
	if regexVeraCrypt.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Error encountered: 'Error: All VeraCrypt volumes must be dismounted first.'\n\n"+
				"You need to do as it says and unmount any VeraCrypt volumes first. Rebooting might help.")
		diagnosis.ErrorType = "system"
	}

	// check for "Failed to mount squashfs image"
	regexMount := regexp.MustCompile(`Failed to mount squashfs image`)
	if regexMount.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Failed to mount squashfs image. This is most likely due to a failed installation of a package. Try reinstalling the package.")
		diagnosis.ErrorType = "system"
	}

	// Check for Rust version mismatch
	regexRustVersion := regexp.MustCompile(`error: the current.*rustc .* is older than the minimum version required`)
	if regexRustVersion.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Rust compilation failed because your Rust compiler (rustc) is too old for this project.\n\n"+
				"You need to update your Rust installation. Run the following command:\n"+
				"rustup update")
		diagnosis.ErrorType = "system"
	}

	// Check for permission issues with .cargo directory
	regexCargoPermission := regexp.MustCompile(`failed to get metadata for.*: permission denied: .*\.cargo`)
	if regexCargoPermission.MatchString(errors) {
		// Get current user
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER" // Fallback if we can't get the actual username
		}

		// Get home directory
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "$HOME" // Fallback if we can't get the actual home directory
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Cargo couldn't access its own cache directory due to permission issues.\n\n"+
				"This likely happened because you ran cargo with sudo in the past. To fix this, run:\n"+
				"sudo chown -R "+currentUser+":"+currentUser+" "+homeDir+"/.cargo")
		diagnosis.ErrorType = "system"
	}

	// Check for out of memory during Rust compilation
	regexRustOOM := regexp.MustCompile(`(LLVM ERROR: out of memory|rustc.*internal compiler error.*out of memory|killed by the OOM killer)`)
	if regexRustOOM.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Rust compilation failed because the system ran out of memory.\n\n"+
				"Rust compilation can be memory-intensive, especially with optimizations enabled. Try:\n"+
				"1. Close other applications to free up memory\n"+
				"2. Add swap space to your system\n"+
				"3. Try compiling with fewer parallel jobs: CARGO_BUILD_JOBS=1 cargo build\n"+
				"4. If on Raspberry Pi, consider installing the More RAM app from Pi-Apps")
		diagnosis.ErrorType = "system"
	}

	// Check for target architecture issues
	regexRustTarget := regexp.MustCompile(`error: failed to run custom build command for.*cross-compil|error: failed to run rustc to learn about target-specific information`)
	if regexRustTarget.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Rust compilation failed due to cross-compilation or target architecture issues.\n\n"+
				"This could be because:\n"+
				"1. You're missing required target-specific toolchains\n"+
				"2. The project doesn't support your hardware architecture\n\n"+
				"Try installing the required rustc target with: rustup target add <target>")
		diagnosis.ErrorType = "system"
	}

	// Check for user errors - these are errors that scripts deliberately output to diagnose issues

	// Regular user error (reporting blocked)
	regexUserError := regexp.MustCompile(`(?m)^User error: `)
	if regexUserError.MatchString(errors) {
		// Extract the error message - get only the lines that are part of the actual error
		scanner := bufio.NewScanner(strings.NewReader(errors))
		var errorMessage string
		found := false

		for scanner.Scan() {
			line := scanner.Text()
			if found {
				// Stop capturing if we hit an empty line or common boilerplate patterns
				if line == "" ||
					strings.HasPrefix(line, "Failed to install") ||
					strings.HasPrefix(line, "Need help?") ||
					strings.HasPrefix(line, "Please ask on Github:") ||
					strings.HasPrefix(line, "Or on Discord:") {
					break
				}
				errorMessage += line + "\n"
			} else if strings.HasPrefix(line, "User error: ") {
				found = true
				errorMessage = strings.TrimPrefix(line, "User error: ") + "\n"
			}
		}

		// Remove trailing newline
		errorMessage = strings.TrimSuffix(errorMessage, "\n")
		diagnosis.Captions = append(diagnosis.Captions, errorMessage)
		diagnosis.ErrorType = "system" // Blocks error reporting
	}

	// User error with reporting allowed
	regexUserErrorAllowed := regexp.MustCompile(`(?m)^User error \(reporting allowed\): `)
	if regexUserErrorAllowed.MatchString(errors) {
		// Extract the error message - get only the lines that are part of the actual error
		scanner := bufio.NewScanner(strings.NewReader(errors))
		var errorMessage string
		found := false

		for scanner.Scan() {
			line := scanner.Text()
			if found {
				// Stop capturing if we hit an empty line or common boilerplate patterns
				if line == "" ||
					strings.HasPrefix(line, "Failed to install") ||
					strings.HasPrefix(line, "Need help?") ||
					strings.HasPrefix(line, "Please ask on Github:") ||
					strings.HasPrefix(line, "Or on Discord:") {
					break
				}
				errorMessage += line + "\n"
			} else if strings.HasPrefix(line, "User error (reporting allowed): ") {
				found = true
				errorMessage = strings.TrimPrefix(line, "User error (reporting allowed): ") + "\n"
			}
		}

		// Remove trailing newline
		errorMessage = strings.TrimSuffix(errorMessage, "\n")
		diagnosis.Captions = append(diagnosis.Captions, errorMessage)
		diagnosis.ErrorType = "unknown" // Allows error reporting
	}

	// If no error type was set, default to "unknown" (allows error reporting)
	if diagnosis.ErrorType == "" {
		diagnosis.ErrorType = "unknown"
	}

	// Always return nil error (equivalent to bash's "return 0") for consistent behavior
	return diagnosis, nil
}
