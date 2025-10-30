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

// Module: log_diagnose_apk.go
// Description: Provides functions for diagnosing APK (Alpine Package Keeper) specific errors.

//go:build apk

package api

import (
	"bufio"
	"fmt"
	"os"
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
	// Repository issues
	//------------------------------------------

	// Check for 'ERROR: unable to select packages' or 'no such package'
	if strings.Contains(errors, "ERROR: unable to select packages") ||
		strings.Contains(errors, "(no such package)") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK reported that the requested package(s) could not be found.\n\n"+
				"This could be because:\n"+
				"1. The package name is incorrect or misspelled\n"+
				"2. The package is not available in your configured repositories\n"+
				"3. Your repository index is out of date\n\n"+
				"To fix this, try:\n"+
				"sudo apk update\n\n"+
				"If the package is still not found, you may need to enable additional repositories in /etc/apk/repositories")
		diagnosis.ErrorType = "system"
	}

	// Check for 'ERROR: unsatisfiable constraints'
	if strings.Contains(errors, "ERROR: unsatisfiable constraints") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK reported dependency conflicts that cannot be resolved.\n\n"+
				"This happens when a package requires dependencies that conflict with other installed packages or cannot be found in available repositories.\n\n"+
				"Try the following:\n"+
				"1. Update your package index: sudo apk update\n"+
				"2. Upgrade all packages: sudo apk upgrade\n"+
				"3. Check that all necessary repositories are enabled in /etc/apk/repositories\n\n"+
				"If the issue persists, the package may not be compatible with your system version.")
		diagnosis.ErrorType = "package"
	}

	// Check for 'WARNING: This apk-tools is OLD!'
	if strings.Contains(errors, "WARNING: This apk-tools is OLD!") ||
		strings.Contains(errors, "apk-tools is OLD") {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your apk-tools package manager is outdated.\n\n"+
				"An outdated apk-tools can cause package installation failures and compatibility issues.\n\n"+
				"To fix this, upgrade apk-tools first:\n"+
				"sudo apk add --upgrade apk-tools\n\n"+
				"Then update your package index:\n"+
				"sudo apk update")
		diagnosis.ErrorType = "system"
	}

	// Check for 'ERROR: BAD signature' or 'UNTRUSTED signature'
	if strings.Contains(errors, "ERROR: BAD signature") ||
		strings.Contains(errors, "UNTRUSTED signature") ||
		strings.Contains(errors, "signature verification failed") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK reported a bad or untrusted signature.\n\n"+
				"This can happen when:\n"+
				"1. A repository mirror hasn't been fully synchronized yet\n"+
				"2. Your system time is incorrect\n"+
				"3. The repository keys need to be updated\n\n"+
				"To fix this, try:\n"+
				"1. Wait a few minutes and run: sudo apk update\n"+
				"2. Check your system time is correct\n"+
				"3. Update repository keys if using a custom repository")
		diagnosis.ErrorType = "system"
	}

	// Check for 'ERROR: Not committing changes due to missing repository tags'
	if strings.Contains(errors, "ERROR: Not committing changes due to missing repository tags") ||
		strings.Contains(errors, "missing repository tags") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK cannot commit changes because repository tags are missing.\n\n"+
				"This indicates that the APK index is not properly set up or is corrupted.\n\n"+
				"To fix this, try:\n"+
				"1. Clear the APK cache: sudo rm -rf /var/cache/apk/*\n"+
				"2. Update the package index: sudo apk update\n\n"+
				"If you're using custom repositories, ensure they have proper APK indices.")
		diagnosis.ErrorType = "system"
	}

	// Check for repository fetch failures
	regexFetchFailed := regexp.MustCompile(`ERROR:.*fetch.*failed|ERROR:.*: temporary error|ERROR:.*: Network unreachable|ERROR:.*: Connection timed out`)
	if regexFetchFailed.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK failed to fetch packages from the repository.\n\n"+
				"This is usually caused by:\n"+
				"1. Network connectivity issues\n"+
				"2. Repository server being temporarily unavailable\n"+
				"3. Firewall blocking connections\n\n"+
				"Check your internet connection and try again. If the problem persists, the repository server may be down.")
		diagnosis.ErrorType = "internet"
	}

	// Check for network resolution errors
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"Could not resolve host",
			"Name or service not known",
			"Temporary failure in name resolution"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK cannot resolve repository hostnames.\n\n"+
				"This is a DNS resolution issue. Check:\n"+
				"1. Your internet connection\n"+
				"2. DNS configuration in /etc/resolv.conf\n"+
				"3. Try using a public DNS server like 8.8.8.8 or 1.1.1.1")
		diagnosis.ErrorType = "internet"
	}

	// Check for world file issues
	if strings.Contains(errors, "ERROR:") && strings.Contains(errors, "/etc/apk/world") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK encountered issues with the world file (/etc/apk/world).\n\n"+
				"The world file tracks explicitly installed packages. If it's corrupted, APK cannot function properly.\n\n"+
				"To fix this, try:\n"+
				"1. Check file permissions: sudo chmod 644 /etc/apk/world\n"+
				"2. If the file is corrupted, you may need to rebuild it based on installed packages\n"+
				"3. As a last resort, you can reset it: sudo apk fix")
		diagnosis.ErrorType = "system"
	}

	// Check for cache issues
	regexCacheError := regexp.MustCompile(`ERROR:.*cache|ERROR:.*APKINDEX`)
	if regexCacheError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK encountered cache or index issues.\n\n"+
				"To fix this, clear the cache and rebuild the index:\n"+
				"sudo rm -rf /var/cache/apk/*\n"+
				"sudo apk update")
		diagnosis.ErrorType = "system"
	}

	// Check for disk space issues
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"No space left on device",
			"not enough disk space",
			"insufficient disk space"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your system has insufficient disk space.\n\n"+
				"APK cannot install packages without enough free space.\n\n"+
				"To fix this:\n"+
				"1. Free up disk space by removing unnecessary files\n"+
				"2. Clean APK cache: sudo apk cache clean\n"+
				"3. Remove old packages: sudo apk del <package-name>")
		diagnosis.ErrorType = "system"
	}

	// Check for permission errors
	regexPermission := regexp.MustCompile(`ERROR:.*[Pp]ermission denied|ERROR:.*Operation not permitted`)
	if regexPermission.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK encountered permission errors.\n\n"+
				"This usually means you need to run the command with root privileges.\n\n"+
				"Try running the command with sudo:\n"+
				"sudo apk <command>")
		diagnosis.ErrorType = "system"
	}

	// Check for locked database
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"database is locked",
			"Unable to lock database",
			"lock file exists"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK database is locked by another process.\n\n"+
				"This happens when another APK operation is already running.\n\n"+
				"To fix this:\n"+
				"1. Wait for the other APK operation to complete\n"+
				"2. If no other operation is running, remove the lock file: sudo rm /lib/apk/db/lock")
		diagnosis.ErrorType = "system"
	}

	// Check for broken packages or world file errors
	regexBroken := regexp.MustCompile(`ERROR:.*broken|Use --force-broken-world`)
	if regexBroken.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK detected broken packages or world file inconsistencies.\n\n"+
				"To attempt to fix this, run:\n"+
				"sudo apk fix\n\n"+
				"If that doesn't work, you can try forcing a repair:\n"+
				"sudo apk fix --force-broken-world\n\n"+
				"Warning: The --force-broken-world option should only be used as a last resort.")
		diagnosis.ErrorType = "package"
	}

	// Check for virtual package errors
	if strings.Contains(errors, "ERROR:") && strings.Contains(errors, "virtual") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK encountered an error with virtual packages.\n\n"+
				"Virtual packages are meta-packages that group multiple packages together.\n\n"+
				"Try:\n"+
				"1. Update package index: sudo apk update\n"+
				"2. Fix broken dependencies: sudo apk fix")
		diagnosis.ErrorType = "package"
	}

	// Check for --simulate or --no-commit-hooks related errors
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"simulated",
			"would install",
			"would upgrade"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK was run in simulation mode.\n\n"+
				"The operation was not actually performed. Remove the --simulate flag to apply changes.")
		diagnosis.ErrorType = "system"
	}

	// Check for architecture mismatch
	regexArch := regexp.MustCompile(`ERROR:.*architecture.*not supported|ERROR:.*wrong architecture`)
	if regexArch.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK reported an architecture mismatch.\n\n"+
				"The package you're trying to install is built for a different architecture than your system.\n\n"+
				"Make sure you're using the correct repository for your architecture.")
		diagnosis.ErrorType = "system"
	}

	// Check for version conflicts
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"version conflict",
			"conflicts with",
			"incompatible with"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK detected version conflicts between packages.\n\n"+
				"To resolve this:\n"+
				"1. Try upgrading all packages: sudo apk upgrade\n"+
				"2. Check for packages held at specific versions\n"+
				"3. Consider removing conflicting packages before installing new ones")
		diagnosis.ErrorType = "package"
	}

	// Check for missing dependencies during package building
	if strings.Contains(errors, "ERROR:") &&
		containsAny(errors, []string{
			"build dependencies",
			"makedepends",
			"checkdepends"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Package build failed due to missing dependencies.\n\n"+
				"If you're building a package from source, ensure all build dependencies are installed.\n\n"+
				"Try installing the base development tools:\n"+
				"sudo apk add alpine-sdk")
		diagnosis.ErrorType = "package"
	}

	//------------------------------------------
	// Package manager issues below
	//------------------------------------------

	// Check for corrupted package files
	regexCorrupted := regexp.MustCompile(`ERROR:.*corrupted|ERROR:.*checksum.*failed|ERROR:.*integrity.*failed`)
	if regexCorrupted.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APK detected corrupted package files.\n\n"+
				"This could be due to:\n"+
				"1. Incomplete downloads\n"+
				"2. Disk corruption\n"+
				"3. Network issues during download\n\n"+
				"To fix this:\n"+
				"1. Clear the cache: sudo rm -rf /var/cache/apk/*\n"+
				"2. Update and try again: sudo apk update && sudo apk add <package>")
		diagnosis.ErrorType = "internet"
	}

	// Check for script errors
	regexScript := regexp.MustCompile(`ERROR:.*\.post-install|ERROR:.*\.pre-install|ERROR:.*\.post-upgrade|ERROR:.*\.pre-upgrade|ERROR:.*\.post-deinstall|ERROR:.*\.pre-deinstall`)
	if regexScript.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A package installation/removal script failed.\n\n"+
				"Package scripts run during installation, upgrade, or removal and can fail for various reasons.\n\n"+
				"Check the error message above for more details about what went wrong.")
		diagnosis.ErrorType = "package"
	}

	//------------------------------------------
	// Non-APK related errors below (common to all systems)
	//------------------------------------------

	// Include common non-package-manager errors from dummy file
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
	regexInternetError := regexp.MustCompile(`Could not resolve host: github\.com|Failed to connect to github\.com port 443: Connection timed out`)
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
	regexSslError := regexp.MustCompile(`SSL/TLS handshake failure|total length mismatch|failed to establish connection|timeout|connection reset by peer|name resolution failed|temporary failure in name resolution|unable to establish SSL connection|connection closed at byte|read error at byte|failed: No route to host|invalid range header|curl error|response status not successful|download snap|dial tcp|lookup api\.snapcraft\.io|fatal: unable to access 'https://github.com.*': Failed to connect to github.com port 443 after .* ms: Couldn't connect to server|RPC failed; curl .* transfer closed with outstanding read data remaining|RPC failed; curl .* GnuTLS recv error \(-9\): A TLS packet with unexpected length was received\.|SSL error|failure when receiving data from the peer|java\.net\.SocketTimeoutException: Read timed out`)
	if regexSslError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A network operation encountered SSL/TLS or connection errors. Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)"
	regexCurlError := regexp.MustCompile(`curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR \(err .*\)`)
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
	regexFlathubError := regexp.MustCompile(`flathub: Error resolving .*dl\.flathub\.org`)
	if regexFlathubError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The flathub command encountered a DNS resolution error. Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "The TLS connection was non-properly terminated\.\|Can't load uri .* Unacceptable TLS certificate"
	regexTlsError := regexp.MustCompile(`The TLS connection was non-properly terminated\.|Can't load uri .* Unacceptable TLS certificate`)
	if regexTlsError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The TLS connection was non-properly terminated. Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// Check for "GnuTLS recv error (-54): Error in the pull function."
	regexGnuTlsError := regexp.MustCompile(`GnuTLS recv error \(-54\): Error in the pull function\.`)
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
				"Otherwise, try reinstalling the kernel.")
		diagnosis.ErrorType = "system"
	}

	// check for "Failed to load module \"appmenu-gtk-module\""
	regexAppmenuGtkModule := regexp.MustCompile(`Failed to load module "appmenu-gtk-module"`)
	if regexAppmenuGtkModule.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"This error occurred: Failed to load module \"appmenu-gtk-module\" \n\n"+
				"Try installing two packages with this command: \n"+
				"sudo apk add appmenu-gtk2-module appmenu-gtk3-module \n\n"+
				"And if that doesn't work, try Googling the errors or reach out to Pi-Apps developers for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "E: gnupg, gnupg2 and gnupg1 do not seem to be installed, but one of them is required for this operation"
	regexGnupg := regexp.MustCompile(`gnupg.*do not seem to be installed`)
	if regexGnupg.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Repository-signing failed because gnupg is missing. This is installed by default on most systems, but on yours it's missing for some reason. \n\n"+
				"Try installing gnupg with this command: \n"+
				"sudo apk add gnupg")
		diagnosis.ErrorType = "system"
	}

	// check for "error: Unable to connect to system bus\|error: Message recipient disconnected from message bus without replying\|Failed to connect to bus: Host is down"
	regexDBus := regexp.MustCompile(`error: Unable to connect to system bus|error: Message recipient disconnected from message bus without replying|Failed to connect to bus: Host is down`)
	if regexDBus.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Something is wrong with your dbus connection. \n\n"+
				"Try rebooting. \n\n"+
				"Make sure systemd/dinit is setup correctly. \n\n"+
				"Also consider reaching out to Pi-Apps developers for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "is not in the sudoers file.  This incident will be reported."
	regexSudoers := regexp.MustCompile(`is not in the sudoers file\.  This incident will be reported\.`)
	if regexSudoers.MatchString(errors) {
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = "$USER"
		}
		diagnosis.Captions = append(diagnosis.Captions,
			fmt.Sprintf("Unable to use the sudo command - the current user '%s' is not allowed to use it. \n\n"+
				"Please enable passwordless sudo or switch to a more privileged user-account. \n\n"+
				"See: https://wiki.alpinelinux.org/wiki/Setting_up_a_new_user#sudo", currentUser))
		diagnosis.ErrorType = "system"
	}

	// check for "sudo: .* incorrect password attempts"
	regexIncorrectPassword := regexp.MustCompile(`sudo: .* incorrect password attempts`)
	if regexIncorrectPassword.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Process could not complete because you failed to type in the correct sudo password. \n\n"+
				"Try again, and consider enabling passwordless sudo or using doas.")
		diagnosis.ErrorType = "system"
	}

	// check for "cpp.o: file not recognized: file truncated"
	regexCpp := regexp.MustCompile(`cpp\.o: file not recognized: file truncated`)
	if regexCpp.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Compiling failed. Try again, but please reach out to Pi-Apps developers for help if this same error keeps occurring.")
		diagnosis.ErrorType = "system"
	}

	// check for "tar: Unexpected EOF in archive\|xz: (stdin): Unexpected end of input\|xz: (stdin): Compressed data is corrupt\|xz: (stdin): File format not recognized\|gzip: stdin: invalid compressed data\-\-length error\|gzip: stdin: invalid compressed data\-\-crc error\|corrupted filesystem tarfile in package archive: invalid tar header size field (Invalid argument)\|member 'data.tar': internal gzip read error: '<fd:4>: incorrect data check\|error inflating zlib stream;"
	regexTar := regexp.MustCompile(`tar: Unexpected EOF in archive|xz: \(stdin\): Unexpected end of input|xz: \(stdin\): Compressed data is corrupt|xz: \(stdin\): File format not recognized|gzip: stdin: invalid compressed data--length error|gzip: stdin: invalid compressed data--crc error|corrupted filesystem tarfile in package archive: invalid tar header size field \(Invalid argument\)|member 'data\.tar': internal gzip read error: '<fd:4>: incorrect data check|error inflating zlib stream`)
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
				"sudo apk add xz")
		diagnosis.ErrorType = "system"
	}

	// check for "Structure needs cleaning"
	regexStructureNeedsCleaning := regexp.MustCompile(`Structure needs cleaning`)
	if regexStructureNeedsCleaning.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"You have encountered the dreaded \"Structure needs cleaning\" error. This indicates file-corruption caused by improperly shutting down your computer. You are lucky your computer booted at all.\n\n"+
				"You can try scheduling a filesystem cleanup: \n"+
				"sudo touch /forcefsck \n\n"+
				"After running that command, reboot and see if that fixes the problem. \n\n"+
				"If that doesn't work, then now is the time to restore your backup. Oh, you don't have one? Then you will have to re-flash your SD card and start over. And maybe consider keeping regular backups to avoid this unpleasant situation next time.")
		diagnosis.ErrorType = "system"
	}

	// check for "Error: Failed to read commit .* No such metadata object\|error: Failed to install org.freedesktop.Platform: Failed to read commit .* No such metadata object\|Error: Error deploying: .* No such metadata object"
	regexFlatpak := regexp.MustCompile(`Error: Failed to read commit .* No such metadata object|error: Failed to install org\.freedesktop\.Platform: Failed to read commit .* No such metadata object|Error: Error deploying: .* No such metadata object`)
	if regexFlatpak.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Flatpak failed to install something due to a past incomplete download. \n\n"+
				"To repair it, please run this command in a terminal: \n"+
				"flatpak repair --user \n\n"+
				"See: https://github.com/flatpak/flatpak/issues/3479")
		diagnosis.ErrorType = "system"
	}

	// check for "No space left on device" (duplicate check but keeping for consistency)
	regexSpace := regexp.MustCompile(`No space left on device|Not enough disk space to complete this operation|You don't have enough free space in|Cannot write to .* \(Success\)\.`)
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

	// check for "c++: fatal error: Killed signal terminated program cc1plus"
	regexKilled := regexp.MustCompile(`c\+\+: fatal error: Killed signal terminated program cc1plus`)
	if regexKilled.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Compiling failed because cc1plus was killed due to insufficient RAM.\n\n"+
				"Please try installing the application again, but this time keep all other programs closed to preserve more free RAM.\n"+
				"If this error persists, try installing the More RAM app from Pi-Apps. Find it in the Tools category.")
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
