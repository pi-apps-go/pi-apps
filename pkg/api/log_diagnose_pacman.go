// Copyright (C) 2026 pi-apps-go contributors
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

// Module: log_diagnose_pacman.go
// Description: Provides functions for diagnosing errors when using the Pacman package manager.
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build pacman

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
	// Repository/Sync issues
	//------------------------------------------

	// Check for repository sync errors
	regexSyncError := regexp.MustCompile(`error: failed to synchronize all databases|error: failed retrieving file.*from|error: failed to update|error: failed to download`)
	if regexSyncError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman failed to synchronize package databases.\n\n"+
				"This could be due to:\n"+
				"1. Network connectivity issues\n"+
				"2. Repository server problems\n"+
				"3. Incorrect repository configuration\n\n"+
				"Try running: sudo pacman -Sy\n\n"+
				"If the problem persists, check your internet connection and review /etc/pacman.conf for any misconfigured repositories.")
		diagnosis.ErrorType = "internet"
	}

	// Check for repository not found or 404 errors
	regexRepoNotFound := regexp.MustCompile(`error:.*repository.*not found|error:.*404.*Not Found|error:.*failed to retrieve.*404`)
	if regexRepoNotFound.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman reported a repository that could not be found.\n\n"+
				"This usually means:\n"+
				"1. The repository URL is incorrect\n"+
				"2. The repository has been moved or removed\n"+
				"3. Your system architecture is not supported by the repository\n\n"+
				"Check /etc/pacman.conf and /etc/pacman.d/ for misconfigured repositories and remove or fix them.")
		diagnosis.ErrorType = "system"
	}

	// Check for signature/key errors
	regexSignatureError := regexp.MustCompile(`error:.*signature from.*is unknown trust|error:.*signature.*is invalid|error:.*key.*is unknown|error:.*required signature missing`)
	if regexSignatureError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman reported a signature verification error.\n\n"+
				"This means a package signature could not be verified. To fix this:\n\n"+
				"1. Update the keyring: sudo pacman -Sy archlinux-keyring\n"+
				"2. If that doesn't work, try: sudo pacman-key --refresh-keys\n"+
				"3. For specific keys, you may need to manually import them\n\n"+
				"If this is for a custom repository, you may need to add its key manually.")
		diagnosis.ErrorType = "system"
	}

	// Check for database lock errors
	regexLockError := regexp.MustCompile(`error:.*failed to lock database|error:.*could not lock database|error:.*database.*locked`)
	if regexLockError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman database is locked, likely because another pacman process is running.\n\n"+
				"Wait for any other pacman operations to complete, or if you're sure no other pacman process is running:\n\n"+
				"sudo rm /var/lib/pacman/db.lck\n\n"+
				"Only do this if you're absolutely certain no other pacman process is active!")
		diagnosis.ErrorType = "system"
	}

	//------------------------------------------
	// Package issues
	//------------------------------------------

	// Check for package conflicts
	regexConflictError := regexp.MustCompile(`error: failed to commit transaction.*conflicting files|error:.*conflicts with|error:.*file conflicts`)
	if regexConflictError.MatchString(errors) {
		// Try to extract package names from the error
		re := regexp.MustCompile(`error: failed to commit transaction.*conflicting files.*?:\s*(.*?)\n`)
		matches := re.FindStringSubmatch(errors)

		var conflictInfo string
		if len(matches) > 1 {
			conflictInfo = "\n\nConflicting files: " + matches[1]
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman reported file conflicts during package installation."+conflictInfo+"\n\n"+
				"This happens when files from different packages would overwrite each other.\n\n"+
				"It's possible that this issue has been announced on the official Arch Linux page if intervention is needed, so check the Arch Linux' news page for more information.\n\n"+
				"Options:\n"+
				"1. Remove the conflicting package first: sudo pacman -R <conflicting-package>\n"+
				"2. Force overwrite (use with caution): sudo pacman -S --overwrite='*' <package>\n"+
				"3. Check if the package is available from AUR instead")
		diagnosis.ErrorType = "package"
	}

	// Check for missing dependencies
	regexDependencyError := regexp.MustCompile(`error: failed to prepare transaction.*could not satisfy dependencies|error:.*unresolvable package conflicts|error:.*dependency.*not found|error:.*failed to prepare transaction.*conflicts`)
	if regexDependencyError.MatchString(errors) {
		// Try to extract the missing package
		re := regexp.MustCompile(`error:.*could not satisfy dependencies.*?:\s*(.*?)\n|error:.*dependency.*?:\s*(.*?)\s+not found`)
		matches := re.FindStringSubmatch(errors)

		var missingPkg string
		if len(matches) > 1 && matches[1] != "" {
			missingPkg = matches[1]
		} else if len(matches) > 2 && matches[2] != "" {
			missingPkg = matches[2]
		}

		var suggestion string
		if missingPkg != "" {
			// Check if it might be an NVIDIA driver or other package that moved to AUR
			if strings.Contains(strings.ToLower(missingPkg), "nvidia") {
				// NVIDIA drivers often get moved to AUR with different names
				// For example, nvidia-580xx-dkms for the 580 series
				var aurPackageName string
				var exampleInstall string
				if strings.Contains(strings.ToLower(missingPkg), "580") {
					aurPackageName = "nvidia-580xx-dkms"
					exampleInstall = "yay -S nvidia-580xx-dkms"
				} else if strings.Contains(strings.ToLower(missingPkg), "390") {
					aurPackageName = "nvidia-390xx-dkms"
					exampleInstall = "yay -S nvidia-390xx-dkms"
				} else if strings.Contains(strings.ToLower(missingPkg), "340") {
					aurPackageName = "nvidia-340xx-dkms"
					exampleInstall = "yay -S nvidia-340xx-dkms"
				} else {
					// Generic NVIDIA package - suggest searching
					aurPackageName = missingPkg + "-dkms or " + missingPkg + "-xx-dkms"
					exampleInstall = "yay -S <aur-package-name>"
				}

				suggestion = "\n\nNote: NVIDIA driver packages (especially older series like 580, 390, 340) have been moved to AUR with different names.\n\n" +
					"Important: According to Arch Linux news, you MUST uninstall the old package before installing the AUR replacement!\n\n" +
					"Steps:\n" +
					"1. Uninstall the old package: sudo pacman -R " + missingPkg + "\n" +
					"2. Search for the AUR replacement: yay -Ss nvidia-*xx-dkms\n" +
					"3. Install from AUR (example: " + exampleInstall + ", likely package: " + aurPackageName + ")\n\n" +
					"Check the Arch Linux news page (https://archlinux.org/news/) for the exact package name and migration instructions.\n\n" +
					"If yay is not installed, you can install it from AUR manually."
			} else {
				suggestion = "\n\nThis package might be available from AUR. Try searching for it:\n" +
					"yay -Ss " + missingPkg
			}
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman reported missing or conflicting dependencies."+suggestion+"\n\n"+
				"To resolve:\n"+
				"1. Update your package database: sudo pacman -Sy\n"+
				"2. Check if the package exists: pacman -Ss <package-name>\n"+
				"3. If not found in official repos, check AUR: yay -Ss <package-name>\n"+
				"4. Some packages may have been moved to AUR (like older NVIDIA drivers)\n"+
				"5. Check Arch Linux news (https://archlinux.org/news/) for migration announcements")
		diagnosis.ErrorType = "package"
	}

	// Check for package not found
	regexPackageNotFound := regexp.MustCompile(`error:.*package.*not found|error:.*target not found|error:.*no package found`)
	if regexPackageNotFound.MatchString(errors) {
		// Try to extract package name
		re := regexp.MustCompile(`error:.*package ['"]?([^'"]+)['"]?.*not found|error:.*target ['"]?([^'"]+)['"]?.*not found`)
		matches := re.FindStringSubmatch(errors)

		var pkgName string
		if len(matches) > 1 && matches[1] != "" {
			pkgName = matches[1]
		} else if len(matches) > 2 && matches[2] != "" {
			pkgName = matches[2]
		}

		var aurSuggestion string
		if pkgName != "" {
			// Check if it's an NVIDIA driver that might have been moved to AUR
			if strings.Contains(strings.ToLower(pkgName), "nvidia") {
				var aurPackageName string
				if strings.Contains(strings.ToLower(pkgName), "580") {
					aurPackageName = "nvidia-580xx-dkms"
				} else if strings.Contains(strings.ToLower(pkgName), "390") {
					aurPackageName = "nvidia-390xx-dkms"
				} else if strings.Contains(strings.ToLower(pkgName), "340") {
					aurPackageName = "nvidia-340xx-dkms"
				}

				if aurPackageName != "" {
					aurSuggestion = "\n\nThis NVIDIA driver package has been moved to AUR with a different name.\n\n" +
						"Important: You MUST uninstall the old package before installing the AUR replacement!\n\n" +
						"Steps:\n" +
						"1. Uninstall the old package: sudo pacman -R " + pkgName + "\n" +
						"2. Install from AUR: yay -S " + aurPackageName + "\n\n" +
						"Check the Arch Linux news page (https://archlinux.org/news/) for the exact package name and migration instructions."
				} else {
					aurSuggestion = "\n\nThis NVIDIA driver package may have been moved to AUR with a different name.\n\n" +
						"Important: If migrating from an official package, uninstall it first before installing the AUR replacement!\n\n" +
						"Steps:\n" +
						"1. Search for the AUR replacement: yay -Ss nvidia-*xx-dkms\n" +
						"2. Uninstall the old package if installed: sudo pacman -R " + pkgName + "\n" +
						"3. Install from AUR: yay -S <aur-package-name>\n\n" +
						"Check the Arch Linux news page (https://archlinux.org/news/) for migration announcements."
				}
			} else {
				aurSuggestion = "\n\nThis package is not in the official repositories. " +
					"It may be available from AUR (Arch User Repository).\n\n" +
					"To install from AUR, you'll need an AUR helper like yay:\n" +
					"1. Install yay if not already installed\n" +
					"2. Search for the package: yay -Ss " + pkgName + "\n" +
					"3. Install it: yay -S " + pkgName + "\n\n" +
					"Note: Some packages (like older NVIDIA drivers) have been moved from official repos to AUR."
			}
		} else {
			aurSuggestion = "\n\nThis package is not in the official repositories. " +
				"It may be available from AUR (Arch User Repository). " +
				"Try searching with: yay -Ss <package-name>"
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman could not find the requested package in the official repositories."+aurSuggestion)
		diagnosis.ErrorType = "package"
	}

	// Check for broken packages or partial upgrades
	regexBrokenPackage := regexp.MustCompile(`error:.*broken|error:.*unresolvable|error:.*invalid package`)
	if regexBrokenPackage.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman reported broken or invalid packages.\n\n"+
				"To fix:\n"+
				"1. Update package database: sudo pacman -Sy\n"+
				"2. Try to fix broken packages: sudo pacman -Syu\n"+
				"3. If that doesn't work, you may need to reinstall the problematic package\n"+
				"4. Check for partial upgrades - make sure all packages are up to date")
		diagnosis.ErrorType = "package"
	}

	// Check for AUR package replacing system package (like NVIDIA drivers)
	regexAurReplacement := regexp.MustCompile(`warning:.*is being replaced by.*-aur|warning:.*replacing.*with.*from.*aur|nvidia.*moved to.*aur|package.*moved to.*aur|nvidia.*moved to.*AUR`)
	if regexAurReplacement.MatchString(errors) {
		// Try to extract package names
		re := regexp.MustCompile(`(nvidia[^\s]*).*?(nvidia[^\s]*-xx-dkms|nvidia[^\s]*-aur)`)
		matches := re.FindStringSubmatch(errors)

		var migrationInfo string
		if len(matches) >= 3 {
			oldPkg := matches[1]
			newPkg := matches[2]
			migrationInfo = "\n\nDetected migration: " + oldPkg + " → " + newPkg + "\n\n"
		} else if len(matches) >= 2 {
			migrationInfo = "\n\nOld package: " + matches[1] + "\n\n"
		}

		diagnosis.Captions = append(diagnosis.Captions,
			"A package that was previously in the official repositories has been moved to AUR."+migrationInfo+
				"This commonly happens with:\n"+
				"- Older NVIDIA driver versions (nvidia-580xx-dkms, nvidia-390xx-dkms, nvidia-340xx-dkms, etc.)\n"+
				"- Packages that are no longer maintained in official repos\n"+
				"- Legacy or deprecated packages\n\n"+
				"Important: According to Arch Linux news, you MUST uninstall the old package before installing the AUR replacement!\n\n"+
				"Steps:\n"+
				"1. Check Arch Linux news (https://archlinux.org/news/) for the exact migration instructions\n"+
				"2. Uninstall the old package: sudo pacman -R <old-package-name>\n"+
				"3. Install from AUR: yay -S <new-aur-package-name>\n\n"+
				"If you don't have yay installed, you can install it from AUR manually or use makepkg directly.")
		diagnosis.ErrorType = "package"
	}

	// Check for DKMS kernel module compilation failures (common on Arch)
	regexDkmsFailure := regexp.MustCompile(`error:.*dkms.*failed|error:.*kernel.*module.*failed|error:.*make.*failed.*dkms`)
	if regexDkmsFailure.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A kernel module (DKMS) failed to compile.\n\n"+
				"This often happens when:\n"+
				"1. The kernel was updated but the module hasn't been rebuilt\n"+
				"2. The module is incompatible with the current kernel version\n\n"+
				"To fix:\n"+
				"1. Rebuild the module: sudo dkms install <module>/<version> -k $(uname -r)\n"+
				"2. Or remove the problematic module and reinstall it\n"+
				"3. Some modules may need to be installed from AUR if they're not compatible with the latest kernel")
		diagnosis.ErrorType = "package"
	}

	// Check for partial upgrade warnings
	regexPartialUpgrade := regexp.MustCompile(`warning:.*partial upgrade|error:.*partial upgrade`)
	if regexPartialUpgrade.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman detected a partial upgrade, which can cause dependency issues.\n\n"+
				"On Arch Linux, it's important to keep all packages up to date together.\n\n"+
				"To fix:\n"+
				"1. Update all packages: sudo pacman -Syu\n"+
				"2. Never use -Sy (sync without upgrade) - always use -Syu\n"+
				"3. If you have AUR packages, update them too: yay -Syu")
		diagnosis.ErrorType = "package"
	}

	// Check for corrupted package database
	regexCorruptedDb := regexp.MustCompile(`error:.*database.*corrupt|error:.*invalid.*database|error:.*failed to read.*database`)
	if regexCorruptedDb.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman database appears to be corrupted.\n\n"+
				"To fix:\n"+
				"1. Try to fix the database: sudo pacman-db-upgrade\n"+
				"2. If that doesn't work, remove the local database and resync:\n"+
				"   sudo rm -rf /var/lib/pacman/sync\n"+
				"   sudo pacman -Sy\n"+
				"3. As a last resort, you may need to restore from backup")
		diagnosis.ErrorType = "system"
	}

	// Check for insufficient disk space during package installation
	regexDiskSpace := regexp.MustCompile(`error:.*not enough.*space|error:.*insufficient.*space|error:.*failed.*extract.*space`)
	if regexDiskSpace.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Pacman failed due to insufficient disk space.\n\n"+
				"To free up space:\n"+
				"1. Clean package cache: sudo pacman -Sc\n"+
				"2. Remove unused packages: sudo pacman -Rns $(pacman -Qtdq)\n"+
				"3. Check disk usage: df -h\n"+
				"4. Consider cleaning AUR build cache if using yay: yay -Sc")
		diagnosis.ErrorType = "system"
	}

	// Non-pacman related errors below (shared with other package managers)

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
				"Try installing the appmenu packages with this command: \n"+
				"sudo pacman -S appmenu-gtk-module libdbusmenu-gtk2 libdbusmenu-gtk3 \n\n"+
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
			"Your system is missing the /usr/share/i18n/SUPPORTED file. \n\n"+
				"Try reinstalling the glibc package: \n"+
				"sudo pacman -S glibc")
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
				"sudo pacman -S xz")
		diagnosis.ErrorType = "system"
	}

	// check for "aria2c: error while loading shared libraries"
	regexAria2c := regexp.MustCompile(`aria2c: error while loading shared libraries`)
	if regexAria2c.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed because aria2c could not load required libraries. \n\n"+
				"Try reinstalling aria2: \n"+
				"sudo pacman -S aria2")
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

	// check for "No LSB modules are available" or lsb_release errors
	regexLsb := regexp.MustCompile(`ModuleNotFoundError: No module named 'lsb_release'|lsb_release: command not found`)
	if regexLsb.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your lsb_release command is missing. On Arch Linux, install it with: \n"+
				"sudo pacman -S lsb-release\n\n"+
				"Note: Arch Linux doesn't use LSB by default, so some scripts expecting lsb_release may need adjustment.")
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
