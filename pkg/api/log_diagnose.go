package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ErrorDiagnosis contains the results of diagnosing a log file
type ErrorDiagnosis struct {
	// ErrorType is the type of error (system, package, internet, or unknown)
	ErrorType string
	// ErrorCaption is a user-friendly explanation of the error
	Captions []string
}

// LogDiagnose analyzes a logfile and returns diagnostic information
// It takes a logfile path and an allowWrite parameter
// Returns the error type and captions for the user
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
	// Following the same logic as the bash script

	//------------------------------------------
	// Repo issues
	//------------------------------------------

	// Check for 'E: The repository'
	if strings.Contains(errors, "E: The repository") ||
		strings.Contains(errors, "sources.list entry misspelt") ||
		strings.Contains(errors, "component misspelt in") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a faulty repository, and you must fix it before Pi-Apps will work.\n\n"+
				"To delete the repository:\n"+
				"Remove the relevant line from /etc/apt/sources.list file or delete one file in\n"+
				"the /etc/apt/sources.list.d folder.\n\n"+
				"sources.list requires root permissions to edit: sudo mousepad /path/to/file")
		diagnosis.ErrorType = "system"
	}

	// Check for 'NO_PUBKEY' or ' is no longer signed.'
	if strings.Contains(errors, "NO_PUBKEY") ||
		strings.Contains(errors, " is no longer signed.") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported an unsigned repository. This has to be solved before APT or Pi-Apps, will work.\n\n"+
				"If you're not sure what to do, you can try to fix the problem by running this command in a terminal:\n"+
				"sudo apt update 2>&1 | sed -ne 's/.*NO_PUBKEY //p' | while read key; do if ! [[ ${keys[*]} =~ \"$key\" ]]; then sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys \"$key\"; keys+=(\"$key\"); fi; done")
		diagnosis.ErrorType = "system"
	}

	// Check for 'Could not resolve' or 'Failed to fetch'
	if containsAny(errors, []string{
		"Could not resolve",
		"Failed to fetch",
		"Temporary failure resolving",
		"Internal Server Error",
		"404 .*Not Found"}) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported an unresolvable repository.\n\n"+
				"Check your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// Check for 'is configured multiple times in'
	if strings.Contains(errors, "is configured multiple times in") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a double-configured repository, and you must fix it to fix Pi-Apps.\n\n"+
				"To delete the repository:\n"+
				"Remove the relevant line from /etc/apt/sources.list file or delete the file in\n"+
				"the /etc/apt/sources.list.d folder.\n\n"+
				"sources.list requires root permissions to edit: sudo mousepad /path/to/file")
		diagnosis.ErrorType = "system"
	}

	// Check for "W: Conflicting distribution: "
	if strings.Contains(errors, "W: Conflicting distribution: ") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a conflicting repository.\n\n"+
				"Read the installation errors, then look through /etc/apt/sources.list and /etc/apt/sources.list.d, making changes as necessary.\n\n"+
				"Perhaps doing a Google search for the exact error you received would help.")
		diagnosis.ErrorType = "system"
	}

	// Check for "Release file for <repo-url> is not valid yet"
	regexNotValid := regexp.MustCompile(`Release file for .* is not valid yet`)
	if regexNotValid.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a repository whose release file becomes valid in the future.\n\n"+
				"This is probably because your system time is set incorrectly.")
		diagnosis.ErrorType = "system"
	}

	// Check for "Release file for <repo-url> is expired"
	regexExpired := regexp.MustCompile(`Release file for .* is expired`)
	if regexExpired.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a repository whose release file was invalidated in the past.\n"+
				"Please check that your system clock is set correctly, and if it is, check if the repository is kept updated or if its developers abandoned it.\n\n"+
				"If you think think you shouldn't see this error, you can try refreshing APT with these commands:\n"+
				"sudo rm -rf /var/lib/apt\n"+
				"sudo apt update")
		diagnosis.ErrorType = "system"
	}
	// check for typo in sources.list and list.d
	regexTypo := regexp.MustCompile(`sources.list entry misspelt`)
	if regexTypo.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a typo in the sources.list file.\n"+
				"You must look around in /etc/apt/sources.list and /etc/apt/sources.list.d and fix the typo.\n")
		diagnosis.ErrorType = "system"
	}
	// check for "E: The package cache file is corrupted"
	regexCorrupted := regexp.MustCompile(`E: The package cache file is corrupted`)
	if regexCorrupted.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT found something wrong with a package list file.\n"+
				"Perhaps this link would help: https://askubuntu.com/questions/939345/the-package-cache-file-is-corrupted-error")
		diagnosis.ErrorType = "system"
	}
	// check for broken pi-apps-local-packages symlink
	regexBroken := regexp.MustCompile(`E: Could not open file /var/lib/apt/lists/_tmp_pi-apps-local-packages_._Packages`)
	if regexBroken.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported the pi-apps-local-packages list as missing.\n"+
				"The Pi-Apps developers have been receiving a few of these errors recently, but we can't figure out what the problem is without your help. Could you please reach out so we can solve this?")
		diagnosis.ErrorType = "system"
	}
	//------------------------------------------
	// repo issues above, apt/dpkg issues below
	//------------------------------------------

	// check for "--fix-broken"
	regexFixBroken := regexp.MustCompile(`--fix-broken`)
	if regexFixBroken.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a broken package.\n\n"+
				"Please run this command: sudo apt --fix-broken install")
		diagnosis.ErrorType = "package"
	}

	// Check for apt/dpkg issues
	if strings.Contains(errors, "--fix-broken") ||
		strings.Contains(errors, "needs to be reinstalled") {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT reported a broken package.\n\n"+
				"Please run this command: sudo apt --fix-broken install")
		diagnosis.ErrorType = "package"
	}
	// Check for "dpkg --configure -a"
	if strings.Contains(errors, "dpkg --configure -a") {
		diagnosis.Captions = append(diagnosis.Captions,
			"Before dpkg, apt, or Pi-Apps will work, dpkg needs to repair your system.\n\n"+
				"Please run this command: sudo dpkg --configure -a")
		diagnosis.ErrorType = "system"
	}

	// check for "package is in a very bad inconsistent state;"
	regexInconsistent := regexp.MustCompile(`package is in a very bad inconsistent state;`)
	if regexInconsistent.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Something is wrong with another package on your system.\n\n"+
				"Refer to this information while troubleshooting: https://askubuntu.com/questions/148715")
		diagnosis.ErrorType = "system"
	}

	// check for "dpkg: error: fgets gave an empty string from"
	regexEmpty := regexp.MustCompile(`dpkg: error: fgets gave an empty string from`)
	if regexEmpty.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Something strange is going on with your system and dpkg won't work.\n\n"+
				"Perhaps this link will help: https://askubuntu.com/questions/1293709/weird-error-when-trying-to-install-packages-with-apt")
		diagnosis.ErrorType = "system"
	}

	// check for "Command line option --allow-releaseinfo-change is not understood"
	regexAllow := regexp.MustCompile(`Command line option --allow-releaseinfo-change is not understood`)
	if regexAllow.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The Debian Project recently upgraded from Buster to version Bullseye. As a result, all Raspberry Pi OS Buster users will receive APT errors saying the repositories changed from 'stable' to 'oldstable'. \n\n"+
				"This error broke pi-apps. To fix it, the Pi-Apps developers added something to the 'sudo apt update' command: --allow-releaseinfo-change. \n\n"+
				"This flag allows the repository migration to succeed, thereby allowing Pi-Apps to work again.\n\n"+
				"Unfortunately for you, your operating system is too old for apt to understand this flag we added. Please upgrade your operating system for a better experience. Raspbian Stretch is unsupported and many apps will not install.\n\n"+
				"Please flash your SD card with the latest release of Raspberry Pi OS: https://www.raspberrypi.org/software")
		diagnosis.ErrorType = "system"
	}

	// check for "lzma error: compressed data is corrupt"
	regexLzma := regexp.MustCompile(`lzma error: compressed data is corrupt`)
	if regexLzma.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A package failed to install because it appears corrupted. (buggy download?)")
		diagnosis.ErrorType = "internet"
	}

	// check for "E: Could not get lock"
	regexLock := regexp.MustCompile(`E: Could not get lock`)
	if regexLock.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Some other apt-get/dpkg process is running. Wait for that one to finish, then try again.")
		diagnosis.ErrorType = "system"
	}

	// check for "dpkg: error: cannot scan updates directory"
	regexUpdates := regexp.MustCompile(`dpkg: error: cannot scan updates directory`)
	if regexUpdates.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"What did you do to your system? The '/var/lib/dpkg/updates' folder is missing. \n\n"+
				"You can try creating the folder with this command: \n"+
				"sudo mkdir -p /var/lib/dpkg/updates")
		diagnosis.ErrorType = "system"
	}

	// check for "E: Repository .* changed its 'Suite' value"
	regexSuite := regexp.MustCompile(`E: Repository .* changed its 'Suite' value`)
	if regexSuite.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"One or more APT repositories on your system have changed Suite values. Usually this occurs when a new version of Debian is released every two years. \n\n"+
				"Pi-Apps should work around this error, but somehow it did not. \n\n"+
				"Please run this command in a terminal: sudo apt update --allow-releaseinfo-change")
		diagnosis.ErrorType = "system"
	}

	// check for "E: Failed to fetch .* File has unexpected size .* Mirror sync in progress\?"
	regexMirror := regexp.MustCompile(`E: Failed to fetch .* File has unexpected size .* Mirror sync in progress\?`)
	if regexMirror.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT encountered a repository with a file that is of incorrect size. This can be caused by a periodic mirror sync, or maybe the repository is faulty. \n\n"+
				"In any case, Pi-Apps cannot work until you solve this issue. Try disabling any 3rd-party APT repos first, and if that doesn't work then ask for help.")
		diagnosis.ErrorType = "system"
	}

	// check for "E: The value 'stable' is invalid for APT::Default-Release as such a release is not available in the sources"
	regexDefault := regexp.MustCompile(`E: The value 'stable' is invalid for APT::Default-Release as such a release is not available in the sources`)
	if regexDefault.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT encountered an issue reading a source file for a repository. Most likely, you were trying to change your sources and did not format the file correctly. \n\n"+
				"Please check your sources.list file and try again.")
		diagnosis.ErrorType = "system"
	}
	// check for "E: The value 'stable' is invalid for APT::Default-Release as such a release is not available in the sources"
	regexRelease := regexp.MustCompile(`E: The value 'stable' is invalid for APT::Default-Release as such a release is not available in the sources`)
	if regexRelease.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"APT encountered an issue reading a source file for a repository. Most likely, you were trying to change your sources and did not format the file correctly. \n\n"+
				"Please check your sources.list file and try again.")
		diagnosis.ErrorType = "system"
	}

	// check for "dpkg: error processing package .*-dkms"
	regexDkms := regexp.MustCompile(`dpkg: error processing package .*-dkms`)
	if regexDkms.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A DKMS (Dynamic Kernel Module Support) package failed to install and has prevented apt from working correctly. This is likely an issue with your distribution and you should report it wherever applicable.")
		diagnosis.ErrorType = "system"
	}

	// Check for "The following packages have unmet dependencies:"
	if strings.Contains(errors, "The following packages have unmet dependencies:") {
		// If allowWrite is true, we need to write additional diagnostic information to the logfile
		if allowWrite {
			// Open the logfile for appending
			logFile, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				defer logFile.Close()

				// Write header for additional diagnostics
				logFile.WriteString("\nAdditional log diagnosis for developers below:\n\n")

				// Perform the diagnostic commands and append their output to the logfile
				// This replicates the bash script's complex package dependency analysis

				// Case 1: Extract dependencies from lines matching "^ .* : Depends:"
				dependsPattern1 := regexp.MustCompile(`^ .* : Depends:`)
				scanner := bufio.NewScanner(strings.NewReader(errors))
				var matchesCase1 []string
				var packagesCase1 []string

				for scanner.Scan() {
					line := scanner.Text()
					if dependsPattern1.MatchString(line) {
						// Extract package names from these lines
						parts := strings.Fields(line)
						if len(parts) >= 4 {
							matchesCase1 = append(matchesCase1, parts[0], parts[3])
						}
					}
				}

				// Remove duplicates and sort
				matchesCase1 = uniqueStrings(matchesCase1)

				// Process architecture-specific packages
				var processedMatches1 []string
				for _, match := range matchesCase1 {
					processedMatches1 = append(processedMatches1, match)
					if strings.Contains(match, ":armhf") {
						// Also add arm64 variant
						arm64Match := strings.Replace(match, ":armhf", ":arm64", 1)
						processedMatches1 = append(processedMatches1, arm64Match)
					}
				}

				// Run apt-cache show and policy on these packages
				if len(processedMatches1) > 0 {
					showOutput1, _ := runCommand("apt-cache", append([]string{"show"}, processedMatches1...)...)
					logFile.WriteString(showOutput1 + "\n")

					policyOutput1, _ := runCommand("apt-cache", append([]string{"policy"}, processedMatches1...)...)
					logFile.WriteString(policyOutput1 + "\n")

					// Get clean package names without architecture suffix
					for _, match := range matchesCase1 {
						cleanName := regexp.MustCompile(`:(armhf|arm64|all)`).ReplaceAllString(match, "")
						packagesCase1 = append(packagesCase1, cleanName)
					}
					packagesCase1 = uniqueStrings(packagesCase1)

					// Run apt list -a
					if len(packagesCase1) > 0 {
						listOutput1, _ := runCommand("apt", append([]string{"list", "-a"}, packagesCase1...)...)
						logFile.WriteString(listOutput1 + "\n")
					}

					// Run a dry-run install
					var installPackages []string
					for _, match := range matchesCase1 {
						if strings.Contains(match, ":") && !strings.HasPrefix(match, ":") {
							installPackages = append(installPackages, match)
						}
					}

					if len(installPackages) > 0 {
						dryRunArgs := append([]string{"install", "-fy", "--no-install-recommends", "--allow-downgrades", "--dry-run"}, installPackages...)
						dryRunOutput1, _ := runCommand("apt-get", dryRunArgs...)
						logFile.WriteString(dryRunOutput1 + "\n")

						// Save the dry-run output for analysis
						dryRunCase1 := dryRunOutput1

						// Additional diagnostic logic
						for _, pkg := range packagesCase1 {
							// Check for multiarch compatibility issues
							if strings.Contains(dryRunCase1, pkg+" : Breaks: "+pkg+":armhf") {
								diagnosis.Captions = append(diagnosis.Captions,
									"Packages failed to install because "+pkg+" does not have a multiarch (armhf) compatible version.\n"+
										"This issue does not occur on Ubuntu/Debian (where every package is multiarch compatible). "+
										"Contact your distro maintainer or the packager of "+pkg+" to have this issue resolved.")
								diagnosis.ErrorType = "system"
							}
						}
					}
				}

				// Case 2: Extract dependencies from lines matching "^ +Depends:"
				dependsPattern2 := regexp.MustCompile(`^ +Depends:`)
				scanner = bufio.NewScanner(strings.NewReader(errors))
				var matchesCase2 []string
				var packagesCase2 []string

				for scanner.Scan() {
					line := scanner.Text()
					if dependsPattern2.MatchString(line) {
						// Extract package names from these lines
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							matchesCase2 = append(matchesCase2, parts[1])
						}
					}
				}

				// Remove duplicates and sort
				matchesCase2 = uniqueStrings(matchesCase2)

				// Process architecture-specific packages
				var processedMatches2 []string
				for _, match := range matchesCase2 {
					processedMatches2 = append(processedMatches2, match)
					if strings.Contains(match, ":armhf") {
						// Also add arm64 variant
						arm64Match := strings.Replace(match, ":armhf", ":arm64", 1)
						processedMatches2 = append(processedMatches2, arm64Match)
					}
				}

				// Run apt-cache show and policy on these packages
				if len(processedMatches2) > 0 {
					showOutput2, _ := runCommand("apt-cache", append([]string{"show"}, processedMatches2...)...)
					logFile.WriteString(showOutput2 + "\n")

					policyOutput2, _ := runCommand("apt-cache", append([]string{"policy"}, processedMatches2...)...)
					logFile.WriteString(policyOutput2 + "\n")

					// Get clean package names without architecture suffix
					for _, match := range matchesCase2 {
						cleanName := regexp.MustCompile(`:(armhf|arm64|all)`).ReplaceAllString(match, "")
						packagesCase2 = append(packagesCase2, cleanName)
					}
					packagesCase2 = uniqueStrings(packagesCase2)

					// Run apt list -a
					if len(packagesCase2) > 0 {
						listOutput2, _ := runCommand("apt", append([]string{"list", "-a"}, packagesCase2...)...)
						logFile.WriteString(listOutput2 + "\n")
					}

					// Run a dry-run install
					if len(processedMatches2) > 0 {
						dryRunArgs := append([]string{"install", "-fy", "--no-install-recommends", "--allow-downgrades", "--dry-run"}, processedMatches2...)
						dryRunOutput2, _ := runCommand("apt-get", dryRunArgs...)
						logFile.WriteString(dryRunOutput2 + "\n")

						// Save the dry-run output for analysis
						dryRunCase2 := dryRunOutput2

						// Additional diagnostic logic for Case 2
						for _, pkg := range packagesCase2 {
							// Check for multiarch compatibility issues
							if strings.Contains(dryRunCase2, pkg+" : Breaks: "+pkg+":armhf") {
								diagnosis.Captions = append(diagnosis.Captions,
									"Packages failed to install because "+pkg+" does not have a multiarch (armhf) compatible version.\n"+
										"This issue does not occur on Ubuntu/Debian (where every package is multiarch compatible). "+
										"Contact your distro maintainer or the packager of "+pkg+" to have this issue resolved.")
								diagnosis.ErrorType = "system"
							}
						}
					}

					// Check if any of the packages are on hold
					if diagnosis.ErrorType == "" && len(matchesCase2) > 0 {
						showOutput, _ := runCommand("apt-cache", append([]string{"show"}, matchesCase2...)...)
						if strings.Contains(showOutput, "Status: hold ok installed") {
							pkgList := strings.Join(matchesCase2, "\n")
							diagnosis.Captions = append(diagnosis.Captions,
								"Packages failed to install because you manually marked at least one of the following packages as held:\n\n"+
									pkgList+"\n\n"+
									"You will need to unmark the packages with the following command before installation can proceed:\n"+
									"sudo apt-mark unhold "+strings.Join(matchesCase2, " "))
							diagnosis.ErrorType = "system"
						}
					}
				}

				// Case 3: Extract dependencies from lines matching "^Depends:"
				dependsPattern3 := regexp.MustCompile(`^Depends:`)
				scanner = bufio.NewScanner(strings.NewReader(errors))
				var dependsLines []string

				for scanner.Scan() {
					line := scanner.Text()
					if dependsPattern3.MatchString(line) {
						dependsLines = append(dependsLines, line)
					}
				}

				var packagesCase3 []string

				// Process each Depends: line
				for _, line := range dependsLines {
					// Extract everything after "Depends:"
					parts := strings.SplitN(line, ":", 2)
					if len(parts) < 2 {
						continue
					}

					// Split by commas and pipes to get individual package specs
					pkgSpecs := strings.Replace(parts[1], ", ", "\n", -1)
					pkgSpecs = strings.Replace(pkgSpecs, "| ", "\n", -1)

					// Remove architecture and version constraints
					re := regexp.MustCompile(`\([^)]*\)`)
					pkgSpecs = re.ReplaceAllString(pkgSpecs, "")

					// Clean up extra spaces
					pkgSpecs = strings.Replace(pkgSpecs, "  ", " ", -1)

					// Split into lines
					for _, pkg := range strings.Split(pkgSpecs, "\n") {
						pkg = strings.TrimSpace(pkg)
						if pkg != "" {
							// Exclude :any suffix
							pkg = strings.Replace(pkg, ":any", "", -1)
							packagesCase3 = append(packagesCase3, pkg)
						}
					}
				}

				// Remove duplicates and sort
				packagesCase3 = uniqueStrings(packagesCase3)

				// Run apt-cache show and policy
				if len(packagesCase3) > 0 {
					showOutput3, _ := runCommand("apt-cache", append([]string{"show"}, packagesCase3...)...)
					logFile.WriteString(showOutput3 + "\n")

					policyOutput3, _ := runCommand("apt-cache", append([]string{"policy"}, packagesCase3...)...)
					logFile.WriteString(policyOutput3 + "\n")

					// Get clean package names without architecture suffix for apt list
					var cleanPackages []string
					for _, pkg := range packagesCase3 {
						cleanName := regexp.MustCompile(`:(armhf|arm64|all)`).ReplaceAllString(pkg, "")
						cleanPackages = append(cleanPackages, cleanName)
					}
					cleanPackages = uniqueStrings(cleanPackages)

					// Run apt list -a
					if len(cleanPackages) > 0 {
						listOutput3, _ := runCommand("apt", append([]string{"list", "-a"}, cleanPackages...)...)
						logFile.WriteString(listOutput3 + "\n")
					}

					// Run a dry-run install
					dryRunArgs := append([]string{"install", "-fy", "--no-install-recommends", "--allow-downgrades", "--dry-run"}, packagesCase3...)
					dryRunOutput3, _ := runCommand("apt-get", dryRunArgs...)
					logFile.WriteString(dryRunOutput3 + "\n")

					// Save the dry-run output for analysis
					dryRunCase3 := dryRunOutput3

					// Additional diagnostic logic for Case 3
					for _, pkg := range cleanPackages {
						// Check for multiarch compatibility issues
						if strings.Contains(dryRunCase3, pkg+" : Breaks: "+pkg+":armhf") {
							diagnosis.Captions = append(diagnosis.Captions,
								"Packages failed to install because "+pkg+" does not have a multiarch (armhf) compatible version.\n"+
									"This issue does not occur on Ubuntu/Debian (where every package is multiarch compatible). "+
									"Contact your distro maintainer or the packager of "+pkg+" to have this issue resolved.")
							diagnosis.ErrorType = "system"
						}
					}

					// Check if any of the packages are on hold
					if diagnosis.ErrorType == "" {
						showOutput, _ := runCommand("apt-cache", append([]string{"show"}, packagesCase3...)...)
						if strings.Contains(showOutput, "Status: hold ok installed") {
							pkgList := strings.Join(packagesCase3, "\n")
							diagnosis.Captions = append(diagnosis.Captions,
								"Packages failed to install because you manually marked at least one of the following packages as held:\n\n"+
									pkgList+"\n\n"+
									"You will need to unmark the packages with the following command before installation can proceed:\n"+
									"sudo apt-mark unhold "+strings.Join(packagesCase3, " "))
							diagnosis.ErrorType = "system"
						}
					}
				}

				// Get apt sources and architectures
				aptSourcesOutput, _ := runCommand("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(COMPONENT) $(TARGET_OF) $(ARCHITECTURE)")
				if aptSourcesOutput != "" {
					logFile.WriteString(aptSourcesOutput + "\n")
				}

				// Get foreign architectures
				archOutput, _ := runCommand("dpkg", "--print-foreign-architectures")
				logFile.WriteString("foreign architectures: " + archOutput + "\n")

				// Check for held packages
				if len(matchesCase1) > 0 {
					// Check if any of the packages are on hold
					showOutput, _ := runCommand("apt-cache", append([]string{"show"}, matchesCase1...)...)
					if strings.Contains(showOutput, "Status: hold ok installed") {
						pkgList := strings.Join(matchesCase1, "\n")
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install because you manually marked at least one of the following packages as held:\n\n"+
								pkgList+"\n\n"+
								"You will need to unmark the packages with the following command before installation can proceed:\n"+
								"sudo apt-mark unhold "+strings.Join(matchesCase1, " "))
						diagnosis.ErrorType = "system"
					}
				}

				// If we've processed all cases but still have no specific diagnosis, add a generic unmet dependencies message
				if diagnosis.ErrorType == "" && strings.Contains(errors, "The following packages have unmet dependencies:") {
					// Extract the whole section about unmet dependencies
					unmetSection := ""
					scanner = bufio.NewScanner(strings.NewReader(errors))
					captureLines := false

					for scanner.Scan() {
						line := scanner.Text()
						if strings.Contains(line, "The following packages have unmet dependencies:") {
							captureLines = true
							unmetSection = line + "\n"
							continue
						}

						if captureLines {
							if line == "" || strings.Contains(line, "E:") {
								captureLines = false
							} else {
								unmetSection += line + "\n"
							}
						}
					}

					// Add appropriate generic messages
					if strings.Contains(errors, "not going to be installed") {
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install because the package manager requires you to install some dependencies manually.\n\n"+
								unmetSection+"\n"+
								"Either your APT repositories are broken, or you need to run:\n"+
								"sudo apt update && sudo apt full-upgrade")
						diagnosis.ErrorType = "system"
					} else if strings.Contains(errors, "but it is not installable") {
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install because at least one dependency is not available in your repositories:\n\n"+
								unmetSection+"\n"+
								"This might be fixed by enabling additional repositories or by running:\n"+
								"sudo apt update && sudo apt full-upgrade")
						diagnosis.ErrorType = "system"
					} else if strings.Contains(errors, "has no installation candidate") {
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install because one or more packages are not available in your repositories:\n\n"+
								unmetSection+"\n"+
								"This might be fixed by enabling additional repositories.")
						diagnosis.ErrorType = "system"
					} else if strings.Contains(errors, "is to be installed") || strings.Contains(errors, "Depends:") {
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install due to unmet dependencies:\n\n"+
								unmetSection+"\n"+
								"This might be fixed by running:\n"+
								"sudo apt --fix-broken install")
						diagnosis.ErrorType = "system"
					} else {
						// Generic fallback
						diagnosis.Captions = append(diagnosis.Captions,
							"Packages failed to install due to unresolved dependency issues:\n\n"+
								unmetSection+"\n"+
								"Try running these commands to resolve the issue:\n"+
								"sudo apt update\n"+
								"sudo apt --fix-broken install\n"+
								"sudo apt full-upgrade")
						diagnosis.ErrorType = "system"
					}
				}
			}
		}

		// If no specific error type was set but we have unmet dependencies,
		// use a generic message
		if diagnosis.ErrorType == "" {
			diagnosis.Captions = append(diagnosis.Captions,
				"APT reported unmet dependencies. This could be caused by:\n\n"+
					"1. Missing packages in the repositories\n"+
					"2. Incompatible package versions\n"+
					"3. Held packages preventing installation\n\n"+
					"Try running: sudo apt --fix-broken install")
			// Note: We're not setting error_type here, following the original script behavior
			// which leaves it as unknown so error reporting is still possible
		}
	}

	// Check for "trying to overwrite shared .*, which is different from other instances of package"
	overwritePattern := regexp.MustCompile(`trying to overwrite shared .*, which is different from other instances of package`)
	if overwritePattern.MatchString(errors) {
		if allowWrite {
			logFile, err := os.OpenFile(logfilePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				defer logFile.Close()

				// Write diagnostic header
				logFile.WriteString("\nAdditional log diagnosis for developers below:\n\n")

				// Extract package names from the error messages
				var packageNames []string
				scanner := bufio.NewScanner(strings.NewReader(errors))
				for scanner.Scan() {
					line := scanner.Text()
					if overwritePattern.MatchString(line) {
						// Get the last field, which should be the package name
						fields := strings.Fields(line)
						if len(fields) > 0 {
							packageNames = append(packageNames, fields[len(fields)-1])
						}
					}
				}

				// Remove duplicates and sort
				packageNames = uniqueStrings(packageNames)

				// Process architecture-specific packages
				var processedPackages []string
				for _, pkg := range packageNames {
					processedPackages = append(processedPackages, pkg)
					if strings.Contains(pkg, ":armhf") {
						// Also add arm64 variant
						arm64Pkg := strings.Replace(pkg, ":armhf", ":arm64", 1)
						processedPackages = append(processedPackages, arm64Pkg)
					}
				}
				processedPackages = uniqueStrings(processedPackages)

				// Run apt-cache show on these packages
				if len(processedPackages) > 0 {
					showOutput, _ := runCommand("apt-cache", append([]string{"show"}, processedPackages...)...)
					logFile.WriteString(showOutput + "\n")
				}

				// Get clean package names without architecture suffix
				var cleanPackages []string
				for _, pkg := range packageNames {
					cleanName := regexp.MustCompile(`:(armhf|arm64|all)`).ReplaceAllString(pkg, "")
					cleanPackages = append(cleanPackages, cleanName)
				}
				cleanPackages = uniqueStrings(cleanPackages)

				// Run apt list -a on clean package names
				if len(cleanPackages) > 0 {
					listOutput, _ := runCommand("apt", append([]string{"list", "-a"}, cleanPackages...)...)
					logFile.WriteString(listOutput + "\n")
				}

				// Run debsums on processed packages
				if len(processedPackages) > 0 {
					debsumsOutput, _ := runCommand("debsums", processedPackages...)
					logFile.WriteString(debsumsOutput + "\n")

					// Check if any debsums reports FAILED
					debsumsFailedPattern := regexp.MustCompile(`FAILED`)
					if debsumsFailedPattern.MatchString(debsumsOutput) {
						// System files have been overwritten
						packageList := strings.Join(processedPackages, "\n")
						diagnosis.Captions = append(diagnosis.Captions,
							"You have overwritten system files which prevent packages that share files from being able to install.\n"+
								"You need to reinstall the following packages to restore the integrity of your apt managed system packages:\n\n"+
								packageList)
						diagnosis.ErrorType = "system"
					} else {
						// Packages have file conflicts but none modified by user
						diagnosis.Captions = append(diagnosis.Captions,
							"Two packages which share the same files are having a problem with different file versions.\n"+
								"Try running this command to fix it:\n"+
								"sudo apt --fix-broken install -o Dpkg::Options::='--force-overwrite'")
						diagnosis.ErrorType = "system"
					}
				} else {
					// Fallback if no packages were found
					diagnosis.Captions = append(diagnosis.Captions,
						"Two packages which share the same files are having a problem with different file versions.\n"+
							"Try running this command to fix it:\n"+
							"sudo apt --fix-broken install -o Dpkg::Options::='--force-overwrite'")
					diagnosis.ErrorType = "system"
				}
			} else {
				// Error opening log file - use the fallback message
				diagnosis.Captions = append(diagnosis.Captions,
					"Two packages which share the same files are having a problem with different file versions.\n"+
						"Try running this command to fix it:\n"+
						"sudo apt --fix-broken install -o Dpkg::Options::='--force-overwrite'")
				diagnosis.ErrorType = "system"
			}
		} else {
			// Not allowed to write to log file - use the fallback message
			diagnosis.Captions = append(diagnosis.Captions,
				"Two packages which share the same files are having a problem with different file versions.\n"+
					"Try running this command to fix it:\n"+
					"sudo apt --fix-broken install -o Dpkg::Options::='--force-overwrite'")
			diagnosis.ErrorType = "system"
		}
	}

	// Downgrade errors with no actual packages listed as to be downgraded. Assume users (custom) distro is to blame
	regexDowngrade := regexp.MustCompile(`E: Packages were downgraded and -y was used without --allow-downgrades.`)
	regexDowngradeList := regexp.MustCompile(`The following packages will be DOWNGRADED:`)
	if regexDowngrade.MatchString(errors) && !regexDowngradeList.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Apt is reporting conflicting information that packages would be downgraded as a result of this standard apt install yet no packages are listed as to be downgraded. This is likely an issue with your linux distribution. Please contact the appropriate maintainer for assistance.")
		diagnosis.ErrorType = "system"
	}

	// Check for Raspberry Pi OS with missing or altered raspi.list
	rpiIssueExists := fileExists("/etc/rpi-issue")
	raspiListExists := fileExists("/etc/apt/sources.list.d/raspi.list")

	if rpiIssueExists && (!raspiListExists || !containsRaspiRepo("/etc/apt/sources.list.d/raspi.list")) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Packages failed to install because you seem to have deleted or altered an important repository file in /etc/apt/sources.list.d\n\n"+
				"This error-dialog appeared because /etc/apt/sources.list.d/raspi.list is missing or altered, but you may have deleted other files as well.\n"+
				"The raspi.list file should contain this:\n\n"+
				"deb http://archive.raspberrypi.com/debian/ "+getCodename()+" main\n"+
				"# Uncomment line below then 'apt-get update' to enable 'apt-get source'\n"+
				"#deb-src http://archive.raspberrypi.com/debian/ "+getCodename()+" main")
		diagnosis.ErrorType = "system"
	}

	// Check for missing sources.list and ubuntu.sources
	sourcesListExists := fileExists("/etc/apt/sources.list")
	ubuntuSourcesExists := fileExists("/etc/apt/sources.list.d/ubuntu.sources")

	if !sourcesListExists && !ubuntuSourcesExists {
		if rpiIssueExists && getArchitecture() == "32" {
			diagnosis.Captions = append(diagnosis.Captions,
				"Packages failed to install because you deleted an important repository file: /etc/apt/sources.list\n\n"+
					"You appear to be using Raspberry Pi OS 32-bit, so the sources.list file should contain this:\n"+
					"deb http://raspbian.raspberrypi.org/raspbian/ "+getCodename()+" main contrib non-free rpi\n"+
					"# Uncomment line below then 'apt-get update' to enable 'apt-get source'\n"+
					"deb-src http://raspbian.raspberrypi.org/raspbian/ "+getCodename()+" main contrib non-free rpi")
			diagnosis.ErrorType = "system"
		} else if rpiIssueExists && getArchitecture() == "64" {
			diagnosis.Captions = append(diagnosis.Captions,
				"Packages failed to install because you deleted an important repository file: /etc/apt/sources.list\n\n"+
					"You appear to be using Raspberry Pi OS 64-bit, so the sources.list file should contain this:\n"+
					"deb http://deb.debian.org/debian "+getCodename()+" main contrib non-free\n"+
					"deb http://security.debian.org/debian-security "+getCodename()+"-security main contrib non-free\n"+
					"deb http://deb.debian.org/debian "+getCodename()+"-updates main contrib non-free\n"+
					"# Uncomment deb-src lines below then 'apt-get update' to enable 'apt-get source'\n"+
					"#deb-src http://deb.debian.org/debian "+getCodename()+" main contrib non-free\n"+
					"#deb-src http://security.debian.org/debian-security "+getCodename()+"-security main contrib non-free\n"+
					"#deb-src http://deb.debian.org/debian "+getCodename()+"-updates main contrib non-free")
			diagnosis.ErrorType = "system"
		} else {
			diagnosis.Captions = append(diagnosis.Captions,
				"Packages failed to install because you deleted an important repository file: /etc/apt/sources.list\n\n"+
					"Refer to your Linux distro's documentation for how to restore this file.\n"+
					"You may have a backup of it in /etc/apt/sources.list.save if you have not deleted that as well.")
			diagnosis.ErrorType = "system"
		}
	}

	// check for "unable to securely remove '.*': Bad message"
	regexBadMessage := regexp.MustCompile(`unable to securely remove '.*': Bad message`)
	if regexBadMessage.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Got a 'Bad message' error when trying to remove a file in an unrelated package. This is not a Pi-Apps issue, but it may indicate hardware failure or disk corruption on your computer.\n\n"+
				"Please click the retry button to see if this keeps occuring, and if it does, try searching the internet for your specific error message.\n\n"+
				"Also it is advisable to run fsck on your root partition to try to work around disk corruption.\n\n"+
				"Open an issue on Pi-Apps if all else fails, but we will probably tell you the same things as are written here.")
		diagnosis.ErrorType = "system"
	}

	//------------------------------------------
	//apt/dpkg issues above, package issues below
	//------------------------------------------

	// check for "installed .* post-installation script subprocess returned error exit status"
	regexPostInstall := regexp.MustCompile(`installed .* post-installation script subprocess returned error exit status`)
	if regexPostInstall.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"A package failed to install because it encountered an error during the post-installation script.")
		diagnosis.ErrorType = "package"
	}

	// check for "E: Problem executing scripts DPkg::Post-Invoke '/home/.*/mesa_vulkan/reinstall-vulkan-driver.sh'"
	regexPostInvoke := regexp.MustCompile(`E: Problem executing scripts DPkg::Post-Invoke '/home/.*/mesa_vulkan/reinstall-vulkan-driver.sh'`)
	if regexPostInvoke.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"PiKiss has installed a broken custom vulkan reinstallation apt hook. As a result, dpkg and APT won't work properly.")
		diagnosis.ErrorType = "package"
	}

	// check for "Reinstalling Vulkan driver"
	regexReinstallVulkan := regexp.MustCompile(`Reinstalling Vulkan driver`)
	if regexReinstallVulkan.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"PiKiss has installed a broken custom vulkan reinstallation apt hook. As a result, dpkg and APT won't work properly.")
		diagnosis.ErrorType = "package"
	}

	// check for "error processing package dphys-swapfile"
	regexDphysSwapfile := regexp.MustCompile(`error processing package dphys-swapfile`)
	if regexDphysSwapfile.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Before dpkg, apt, or Pi-Apps will work, dphys-swapfile must be fixed. \n\n"+
				"Try Googling the above errors, or ask the Pi-Apps developers for help.")
		diagnosis.ErrorType = "package"
	}

	// check for "missing /boot/firmware, did you forget to mount it" or "u-boot-rpi"
	regexMissingFirmware := regexp.MustCompile(`missing /boot/firmware, did you forget to mount it`)
	regexUbootRpi := regexp.MustCompile(`u-boot-rpi`)
	if regexMissingFirmware.MatchString(errors) || regexUbootRpi.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Package(s) failed to install because your boot drive is not working. \n\n"+
				"You must fix the u-boot-rpi package before dpkg, apt, or Pi-Apps will work.")
		diagnosis.ErrorType = "package"
	}

	// check for "files list file for package .* is missing final newline"
	regexMissingFinalNewline := regexp.MustCompile(`files list file for package .* is missing final newline`)
	if regexMissingFinalNewline.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Before dpkg, apt, or Pi-Apps will work, your system must be repaired. \n\n"+
				"Perhaps this link will help: https://askubuntu.com/questions/909719/dpkg-unrecoverable-fatal-error-aborting-files-list-file-for-package-linux-ge")
		diagnosis.ErrorType = "package"
	}

	// check for "raspberrypi-kernel package post-installation script subprocess returned error exit status"
	regexKernelPostInstall := regexp.MustCompile(`raspberrypi-kernel package post-installation script subprocess returned error exit status`)
	if regexKernelPostInstall.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The raspberrypi-kernel package on your system is causing problems. \n\n"+
				"Pi-Apps, dpkg and APT won't work properly until the problem is fixed. \n\n"+
				"https://www.raspberrypi.org/forums")
		diagnosis.ErrorType = "package"
	}

	// check for "raspberrypi-bootloader package pre-installation script subprocess returned error exit status"
	regexBootloaderPostInstall := regexp.MustCompile(`raspberrypi-bootloader package pre-installation script subprocess returned error exit status`)
	if regexBootloaderPostInstall.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The raspberrypi-bootloader package on your system is causing problems. \n\n"+
				"Pi-Apps, dpkg and APT won't work properly until the problem is fixed. \n\n"+
				"https://www.raspberrypi.org/forums")
		diagnosis.ErrorType = "package"
	}

	// check for "error processing package nginx-full"
	regexNginxFull := regexp.MustCompile(`error processing package nginx-full`)
	if regexNginxFull.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The nginx-full package on your system encountered a problem. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "libwine-development:arm64 package post-installation script subprocess returned error exit status"
	regexLibWinePostInstall := regexp.MustCompile(`libwine-development:arm64 package post-installation script subprocess returned error exit status`)
	if regexLibWinePostInstall.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The libwine-development package on your system encountered a problem. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed firmware-microbit-micropython-dl package post-installation script subprocess returned error exit status 1"
	regexFirmwareMicrobit := regexp.MustCompile(`installed firmware-microbit-micropython-dl package post-installation script subprocess returned error exit status 1`)
	if regexFirmwareMicrobit.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The firmware-microbit-micropython-dl package on your system encountered a problem. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed flash-kernel package post-installation script subprocess returned error exit status 1"
	regexFlashKernel := regexp.MustCompile(`installed flash-kernel package post-installation script subprocess returned error exit status 1`)
	if regexFlashKernel.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The flash-kernel package on your system encountered a problem. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "Depends: exagear.* but it is not installable"
	regexExagear := regexp.MustCompile(`Depends: exagear.* but it is not installable`)
	if regexExagear.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The exagear package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "ca-certificates-java: Depends: ca-certificates-java (>= 20190405~) but it is not going to be installed"
	regexCaCertificatesJava := regexp.MustCompile(`ca-certificates-java: Depends: ca-certificates-java (>= 20190405~) but it is not going to be installed`)
	if regexCaCertificatesJava.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The ca-certificates-java package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "E: Problem executing scripts DPkg::Post-Invoke '/home/.*/mesa_vulkan/reinstall-vulkan-driver.sh'"
	regexMesaVulkan := regexp.MustCompile(`E: Problem executing scripts DPkg::Post-Invoke '/home/.*/mesa_vulkan/reinstall-vulkan-driver.sh'`)
	if regexMesaVulkan.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The mesa_vulkan package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "dpkg: error processing archive .*steam-launcher"
	regexSteamLauncher := regexp.MustCompile(`dpkg: error processing archive .*steam-launcher`)
	if regexSteamLauncher.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The steam-launcher package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "dpkg: error processing archive .*gnome-control-center-data"
	regexGnomeControlCenterData := regexp.MustCompile(`dpkg: error processing archive .*gnome-control-center-data`)
	if regexGnomeControlCenterData.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The gnome-control-center-data package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed php7.3-fpm package post-installation script subprocess returned error exit status 1"
	regexPhp73Fpm := regexp.MustCompile(`installed php7.3-fpm package post-installation script subprocess returned error exit status 1`)
	if regexPhp73Fpm.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The php7.3-fpm package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed nulog package post-installation script subprocess returned error exit status 1"
	regexNulog := regexp.MustCompile(`installed nulog package post-installation script subprocess returned error exit status 1`)
	if regexNulog.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The nulog package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed wps-office package post-installation script subprocess returned error exit status 127"
	regexWpsOffice := regexp.MustCompile(`installed wps-office package post-installation script subprocess returned error exit status 127`)
	if regexWpsOffice.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The wps-office package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "cmake but it is not installable"
	regexCmake := regexp.MustCompile(`cmake but it is not installable`)
	if regexCmake.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The cmake package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "blockpi : Depends: python3-picamera but it is not installable"
	regexBlockpi := regexp.MustCompile(`blockpi : Depends: python3-picamera but it is not installable`)
	if regexBlockpi.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"BlockPi could not be installed because the python3-picamera package is missing. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "libgstreamer1.0-dev: Depends: libgstreamer1.0-dev-bin but it is not installable"
	regexGstreamer10Dev := regexp.MustCompile(`libgstreamer1.0-dev: Depends: libgstreamer1.0-dev-bin but it is not installable`)
	if regexGstreamer10Dev.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The libgstreamer1.0-dev package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "trying to overwrite '/usr/lib/mono/4.5/mscorlib.dll', which is also in package libmono-corlib4.5-dll"
	regexMonoCorlib45Dll := regexp.MustCompile(`trying to overwrite '/usr/lib/mono/4.5/mscorlib.dll', which is also in package libmono-corlib4.5-dll`)
	if regexMonoCorlib45Dll.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The libmono-corlib4.5-dll package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed android-androresolvd package post-installation script subprocess returned error exit status 1"
	regexAndroidAndroresolvd := regexp.MustCompile(`installed android-androresolvd package post-installation script subprocess returned error exit status 1`)
	if regexAndroidAndroresolvd.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The android-androresolvd package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "dpkg: error processing archive .*android-androresolvd"
	regexAndroidAndroresolvdArchive := regexp.MustCompile(`dpkg: error processing archive .*android-androresolvd`)
	if regexAndroidAndroresolvdArchive.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The android-androresolvd package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "installed dahdi-dkms package post-installation script subprocess returned error exit status"
	regexDahdiDkms := regexp.MustCompile(`installed dahdi-dkms package post-installation script subprocess returned error exit status`)
	if regexDahdiDkms.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The dahdi-dkms package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "ffmpeg : Depends: libsdl2-2.0-0 (>= 2.0.12) but 2.0.10+5rpi is installed"
	regexFfmpeg := regexp.MustCompile(`ffmpeg : Depends: libsdl2-2.0-0 (>= 2.0.12) but 2.0.10+5rpi is installed`)
	if regexFfmpeg.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The ffmpeg package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "freedm : Depends: prboom-plus but it is not going to be installed"
	regexFreedm := regexp.MustCompile(`freedm : Depends: prboom-plus but it is not going to be installed`)
	if regexFreedm.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The freedm package on your system is causing problems. \n\n"+
				"Maybe reinstalling this package and the prboom-plus package would help?")
		diagnosis.ErrorType = "package"
	}

	// check for "trying to overwrite '/usr/share/pixmaps/wsjtx_icon.png', which is also in package wsjtx 2.6.1"
	regexWsjtxIcon := regexp.MustCompile(`trying to overwrite '/usr/share/pixmaps/wsjtx_icon.png', which is also in package wsjtx 2.6.1`)
	if regexWsjtxIcon.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The wsjtx-data package is conflicting with the wsjtx package installed on your system. You must fix this to install additional software.\n\n"+
				"According to the forums at wsjtx.groups.io, you can fix this by uninstalling wsjtx-data with this command:\n"+
				"sudo apt purge wsjtx-data\n\n"+
				"Here is the full forum link in case it helps you: https://wsjtx.groups.io/g/main/topic/77286764")
		diagnosis.ErrorType = "package"
	}

	// check for "installed systemd package post-installation script subprocess returned error exit status"
	regexSystemd := regexp.MustCompile(`installed systemd package post-installation script subprocess returned error exit status`)
	if regexSystemd.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"What did you do to your system? The \"systemd\" package is not installing correctly. \n\n"+
				"Unless you know a lot about Linux, you may just want to reinstall your operating system. :(")
		diagnosis.ErrorType = "package"
	}

	// Check for "trying to overwrite .*, which is also in package sdl2-image"
	regexSdl2Image := regexp.MustCompile(`trying to overwrite .*, which is also in package sdl2-image`)
	if regexSdl2Image.MatchString(errors) {
		// Try to automatically remove the problematic packages
		_, err1 := runCommand("sudo", "apt", "-y", "purge", "sdl2-image")
		_, err2 := runCommand("sudo", "apt", "-y", "purge", "sdl2-mixer")
		_, err3 := runCommand("sudo", "apt", "-y", "purge", "sdl2-ttf")

		// Check if any of the commands succeeded
		packagesRemoved := err1 == nil || err2 == nil || err3 == nil

		if packagesRemoved {
			diagnosis.Captions = append(diagnosis.Captions,
				"You had some problematic SDL2 packages installed from the Doom 3 app. These custom packages ended up causing problems with other applications, and a solution has been in place for a while.\n\n"+
					"The system has automatically removed the old sdl2-image, sdl2-mixer, and sdl2-ttf packages. Please try installing your other apps again.")
		} else {
			diagnosis.Captions = append(diagnosis.Captions,
				"You have some problematic SDL2 packages installed from the Doom 3 app. These custom packages are causing problems with other applications.\n\n"+
					"Please try to remove these packages manually using:\n"+
					"sudo apt -y purge sdl2-image\n"+
					"sudo apt -y purge sdl2-mixer\n"+
					"sudo apt -y purge sdl2-ttf")
		}
		diagnosis.ErrorType = "package"
	}

	// check for "files list file for package 'libpagemaker-0.0-0:arm64' contains empty filename"
	regexLibpagemaker := regexp.MustCompile(`files list file for package 'libpagemaker-0.0-0:arm64' contains empty filename`)
	if regexLibpagemaker.MatchString(errors) {
		// Try to remove the problematic package
		cmd := exec.Command("sudo", "apt", "purge", "libpagemaker-0.0-")
		err := cmd.Run()

		if err == nil {
			diagnosis.Captions = append(diagnosis.Captions,
				"The libpagemaker-0.0-0 package was causing fatal APT issues on your system. "+
					"This package has been automatically removed to fix the issue.\n\n"+
					"You should now be able to install additional software.")
		} else {
			diagnosis.Captions = append(diagnosis.Captions,
				"The libpagemaker-0.0-0 package is causing fatal APT issues on your system. You must fix this to install additional software.\n\n"+
					"Try this: sudo apt purge libpagemaker-0.0-\n\n"+
					"Search the Internet for more help if this doesn't work.")
		}
		diagnosis.ErrorType = "package"
	}

	// check for "Package ttf-mscorefonts-installer is not available, but is referred to by another package."
	regexTtfMscorefontsInstaller := regexp.MustCompile(`Package ttf-mscorefonts-installer is not available, but is referred to by another package.`)
	if regexTtfMscorefontsInstaller.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The ttf-mscorefonts-installer package is not available, causing this app to fail to install. You must fix this to install additional software.\n\n"+
				"ttf-mscorefonts-installer is a package available in the debian contrib section of the repository. If you disabled this, you need to enable installing packages from the contrib section.\n\n"+
				"If you need more help, search the internet for 'Linux enable contrib repository'")
		diagnosis.ErrorType = "package"
	}

	// check for generic ARM64 linux kernel image on Raspberry Pi OS
	regexGenericArm64LinuxKernelImage := regexp.MustCompile(`linux-image-.*-arm64`)
	if regexGenericArm64LinuxKernelImage.MatchString(errors) {
		// Check if running on Raspberry Pi OS by checking for /etc/rpi-issue
		_, err := os.Stat("/etc/rpi-issue")
		if err == nil {
			// Try to automatically purge the problematic packages
			cmd := exec.Command("sudo", "apt", "purge", "--autoremove", "linux-image-*-arm64")
			purgeErr := cmd.Run()

			if purgeErr == nil {
				diagnosis.Captions = append(diagnosis.Captions,
					"You had a generic ARM64 linux kernel image installed on your system but were running Raspberry Pi OS. "+
						"This package designed for ARM64 servers has been automatically removed to fix the issue.\n\n"+
						"You should now be able to install additional software.")
			} else {
				diagnosis.Captions = append(diagnosis.Captions,
					"You have a generic ARM64 linux kernel image installed on your system but are running Raspberry Pi OS. This is a package designed for ARM64 servers. You must fix this to prevent apt install/upgrades from erroring.\n\n"+
						"Try this: sudo apt purge --autoremove linux-image-*-arm64")
			}
			diagnosis.ErrorType = "package"
		}
	}

	// Check for backports conflicts on Debian/Raspbian
	regexBrokenPackages := regexp.MustCompile(`E: Unable to correct problems, you have held broken packages.`)
	regexUnmetDeps := regexp.MustCompile(`The following packages have unmet dependencies:`)

	if regexBrokenPackages.MatchString(errors) && regexUnmetDeps.MatchString(errors) {
		// Get OS information
		osID, osCodename := getOSInfo()

		// Only continue for Debian or Raspbian
		if osID == "Debian" || osID == "Raspbian" {
			// Check if backports repository is enabled
			hasBackports, err := checkBackportsRepo(osCodename)
			if err == nil && hasBackports {
				// Find conflicting packages from backports
				backportsConflicts, err := findBackportsConflicts(errors)
				if err == nil && len(backportsConflicts) > 0 {
					// Create a list of conflicting packages
					packageList := strings.Join(backportsConflicts, "\n")

					diagnosis.Captions = append(diagnosis.Captions,
						"The debian "+osCodename+"-backports repo is enabled on your system and packages installed from it are causing conflicts.\n"+
							"You will need to revert to the stable version of the packages or manually upgrade all dependent packages to the "+osCodename+"-backports version.\n\n"+
							"The packages that should be reverted to the stable versions that are causing conflicts are:\n"+
							packageList+"\n\n"+
							"For more information refer to the debian documentation: https://backports.debian.org/Instructions/")
					diagnosis.ErrorType = "package"
				}
			}
		}
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
			"The git command encountered this error: \"fetch-pack: unexpected disconnect while reading sideband packet\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "fatal: did not receive expected object" aka git errors
	regexFatalObject := regexp.MustCompile(`fatal: did not receive expected object`)
	if regexFatalObject.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"fatal: did not receive expected object\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "fatal: the remote end hung up unexpectedly" aka git errors
	regexRemoteEndHungUp := regexp.MustCompile(`fatal: the remote end hung up unexpectedly`)
	if regexRemoteEndHungUp.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"fatal: the remote end hung up unexpectedly\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for SSL/TLS handshake failure, total length mismatch, failed to establish connection, timeout, connection reset by peer, name resolution failed, temporary failure in name resolution, unable to establish SSL connection, connection closed at byte, read error at byte, failed: No route to host, invalid range header, curl error, response status not successful, download snap, dial tcp, lookup api.snapcraft.io, fatal: unable to access 'https://github.com.*': Failed to connect to github.com port 443 after .* ms: Couldn't connect to server, RPC failed; curl .* transfer closed with outstanding read data remaining, RPC failed; curl .* GnuTLS recv error (-9): A TLS packet with unexpected length was received., SSL error, failure when receiving data from the peer, java.net.SocketTimeoutException: Read timed out which include git errors
	regexSslError := regexp.MustCompile(`SSL/TLS handshake failure\|total length mismatch\|failed to establish connection\|timeout\|connection reset by peer\|name resolution failed\|temporary failure in name resolution\|unable to establish SSL connection\|connection closed at byte\|read error at byte\|failed: No route to host\|invalid range header\|curl error\|response status not successful\|download snap\|dial tcp\|lookup api.snapcraft.io\|fatal: unable to access 'https://github.com.*': Failed to connect to github.com port 443 after .* ms: Couldn't connect to server\|RPC failed; curl .* transfer closed with outstanding read data remaining\|RPC failed; curl .* GnuTLS recv error (-9): A TLS packet with unexpected length was received.\|SSL error\|failure when receiving data from the peer\|java\.net\.SocketTimeoutException: Read timed out`)
	if regexSslError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The git command encountered this error: \"SSL/TLS handshake failure\" Check the stability of your Internet connection and try again.")
		diagnosis.ErrorType = "internet"
	}

	// check for "curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)"
	regexCurlError := regexp.MustCompile(`curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)`)
	if regexCurlError.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"The curl command encountered this error: \"curl: (.*) HTTP/2 stream .* was not closed cleanly: INTERNAL_ERROR (err .*)\" Check the stability of your Internet connection and try again.")
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

	// check for "E: gnupg, gnupg2 and gnupg1 do not seem to be installed, but one of them is required for this operation"
	regexGnupg := regexp.MustCompile(`E: gnupg, gnupg2 and gnupg1 do not seem to be installed, but one of them is required for this operation`)
	if regexGnupg.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Repository-signing failed because gnpug is missing. This is installed by default on most systems, but on yours it's missing for some reason. \n\n"+
				"Try installing gnupg with this command: \n"+
				"sudo apt install gnupg")
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
		diagnosis.Captions = append(diagnosis.Captions,
			"Unable to use the sudo command - the current user '$USER' is not allowed to use it. \n\n"+
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
				"sudo apt-get install xz-utils")
		diagnosis.ErrorType = "system"
	}

	// check for "aria2c: error while loading shared libraries: /lib/arm-linux-gnueabihf/libaria2.so.0: unexpected reloc type 0xc8"
	regexAria2c := regexp.MustCompile(`aria2c: error while loading shared libraries: /lib/arm-linux-gnueabihf/libaria2.so.0: unexpected reloc type 0xc8`)
	if regexAria2c.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Download failed because aria2c could not load the libaria2 library. \n\n"+
				"Try reinstalling the package: \n"+
				"sudo apt install --reinstall libaria2-0")
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

	// check for "Reinstallation of .* is not possible, it cannot be downloaded\."
	regexReinstall := regexp.MustCompile(`Reinstallation of .* is not possible, it cannot be downloaded\.`)
	if regexReinstall.MatchString(errors) {
		diagnosis.Captions = append(diagnosis.Captions,
			"Your APT setup has been corrupted somehow. \n\n"+
				"This was most likely caused by an unexpected power loss or shutdown while packages were being reinstalled or upgraded. \n\n"+
				"Fixing this will not be easy and it may not be worth your time. Reflashing the SD card may be faster. \n\n"+
				"First try running: \n"+
				"sudo dpkg --configure -a \n\n"+
				"If you still get APT errors, it *might* help to remove the apt folder and upgrade: \n"+
				"sudo rm -rf /var/lib/apt \n"+
				"sudo apt update \n\n"+
				"See: https://forums.raspberrypi.com/viewtopic.php?t=275994")
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
		diagnosis.Captions = append(diagnosis.Captions,
			"You have encountered the 'VCHI initialization failed' error. This means that a program was not allowed to display something to the screen. \n\n"+
				"You can try to fix the error by adding your user to the video group. Run this command in a terminal: \n"+
				"sudo usermod -a -G video $USER \n\n"+
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
	regexSpace := regexp.MustCompile(`No space left on device|Not enough disk space to complete this operation|You don't have enough free space in`)
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
	regexUserError := regexp.MustCompile(`^User error: `)
	if regexUserError.MatchString(errors) {
		// Extract the error message - get all lines after the "User error: " line
		scanner := bufio.NewScanner(strings.NewReader(errors))
		var errorMessage string
		found := false

		for scanner.Scan() {
			line := scanner.Text()
			if found {
				errorMessage += line + "\n"
			} else if strings.HasPrefix(line, "User error: ") {
				found = true
				errorMessage = strings.TrimPrefix(line, "User error: ") + "\n"
			}
		}

		diagnosis.Captions = append(diagnosis.Captions, errorMessage)
		diagnosis.ErrorType = "system" // Blocks error reporting
	}

	// User error with reporting allowed
	regexUserErrorAllowed := regexp.MustCompile(`^User error \(reporting allowed\): `)
	if regexUserErrorAllowed.MatchString(errors) {
		// Extract the error message - get all lines after the "User error (reporting allowed): " line
		scanner := bufio.NewScanner(strings.NewReader(errors))
		var errorMessage string
		found := false

		for scanner.Scan() {
			line := scanner.Text()
			if found {
				errorMessage += line + "\n"
			} else if strings.HasPrefix(line, "User error (reporting allowed): ") {
				found = true
				errorMessage = strings.TrimPrefix(line, "User error (reporting allowed): ") + "\n"
			}
		}

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

// FormatLogfile removes ANSI escape sequences and adds OS information to the beginning of a logfile
func FormatLogfile(filename string) error {
	if filename == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	// Read the file
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Get device info
	deviceInfo, err := GetDeviceInfo()
	if err != nil {
		deviceInfo = "Failed to get device info"
	}

	// Remove ANSI escape sequences
	cleanedContent := RemoveAnsiEscapes(string(content))

	// Create the formatted content
	formattedContent := deviceInfo + "\n\nBEGINNING OF LOG FILE:\n-----------------------\n\n" + cleanedContent

	// Write it back to the file
	return os.WriteFile(filename, []byte(formattedContent), 0644)
}

// GetDeviceInfo returns comprehensive system information about the device
func GetDeviceInfo() (string, error) {
	var info strings.Builder

	// Get OS info
	if _, err := os.Stat("/etc/os-release"); err == nil {
		osNameOutput, err := runCommand("grep", "PRETTY_NAME", "/etc/os-release")
		if err == nil {
			osName := strings.TrimSpace(osNameOutput)
			osName = strings.TrimPrefix(osName, "PRETTY_NAME=")
			osName = strings.Trim(osName, "\"")
			info.WriteString("OS: " + osName + "\n")
		} else {
			info.WriteString("OS: Unknown\n")
		}
	} else {
		info.WriteString("OS: Unknown\n")
	}

	// Get OS architecture
	archOutput, err := runCommand("getconf", "LONG_BIT")
	if err == nil {
		info.WriteString("OS architecture: " + strings.TrimSpace(archOutput) + "-bit\n")
	} else {
		info.WriteString("OS architecture: Unknown\n")
	}

	// Get Pi-Apps information
	piAppsDir := os.Getenv("DIRECTORY") // Pi-Apps directory environment variable
	if piAppsDir != "" && fileExists(piAppsDir) {
		// Get last local update date
		cmd := exec.Command("bash", "-c",
			fmt.Sprintf(`cd "%s" && git show -s --format="%%ad" --date=short | xargs date +%%x -d`, piAppsDir))
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			localUpdateDate := strings.TrimSpace(string(output))
			if localUpdateDate != "" {
				info.WriteString("Last updated Pi-Apps on: " + localUpdateDate + "\n")
			}
		}

		// Get latest Pi-Apps version
		gitURLPath := filepath.Join(piAppsDir, "etc", "git_url")
		if fileExists(gitURLPath) {
			// Use the exact command that works in the terminal
			cmd = exec.Command("bash", "-c",
				`wget -qO- https://api.github.com/repos/Botspot/pi-apps/commits/master 2>/dev/null | grep '"date"' | head -1 | awk -F'"' '{print $4}' | cut -dT -f1 | xargs -I{} date +%m/%d/%Y -d {}`)
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				dateStr := strings.TrimSpace(string(output))
				if dateStr != "" {
					info.WriteString("Latest Pi-Apps version: " + dateStr + "\n")
				}
			}
		}
	}

	// Get kernel info
	kernelArchOutput, err := runCommand("uname", "-m")
	kernelVersionOutput, err2 := runCommand("uname", "-r")
	if err == nil && err2 == nil {
		info.WriteString("Kernel: " + strings.TrimSpace(kernelArchOutput) + " " + strings.TrimSpace(kernelVersionOutput) + "\n")
	} else {
		info.WriteString("Kernel: Unknown\n")
	}

	// Get device model and SoC information
	model, socID := GetDeviceModel()
	info.WriteString("Device model: " + model + "\n")

	// Add SoC information if available
	if socID != "" {
		info.WriteString("SOC identifier: " + socID + "\n")
	}

	// Get hashed machine-id
	if _, err := os.Stat("/etc/machine-id"); err == nil {
		machineIDOutput, err := runCommand("sh", "-c", "cat /etc/machine-id | sha1sum | awk '{print $1}' | head -1")
		if err == nil && machineIDOutput != "" {
			info.WriteString("Machine-id (hashed): " + strings.TrimSpace(machineIDOutput) + "\n")
		}
	}

	// Get hashed serial-number
	if _, err := os.Stat("/sys/firmware/devicetree/base/serial-number"); err == nil {
		serialOutput, err := runCommand("sh", "-c", "cat /sys/firmware/devicetree/base/serial-number | sha1sum | awk '{print $1}' | head -1")
		if err == nil && serialOutput != "" {
			info.WriteString("Serial-number (hashed): " + strings.TrimSpace(serialOutput) + "\n")
		}
	}

	// Get CPU name
	cpuOutput, err := runCommand("sh", "-c", "lscpu | awk '/Model name:/ {print $3}'")
	if err == nil && cpuOutput != "" {
		info.WriteString("CPU name: " + strings.TrimSpace(cpuOutput) + "\n")
	}

	// Get RAM size
	ramOutput, err := runCommand("sh", "-c", "echo \"scale=2 ; $(awk '/MemTotal/ {print $2}' /proc/meminfo) / 1024000\" | bc")
	if err == nil && ramOutput != "" {
		info.WriteString("RAM size: " + strings.TrimSpace(ramOutput) + " GB\n")
	}

	// Get Raspberry Pi OS image version
	if _, err := os.Stat("/etc/rpi-issue"); err == nil {
		rpiOutput, err := runCommand("sh", "-c", "cat /etc/rpi-issue | grep 'Raspberry Pi reference' | sed 's/Raspberry Pi reference //g'")
		if err == nil && rpiOutput != "" {
			info.WriteString("Raspberry Pi OS image version: " + strings.TrimSpace(rpiOutput) + "\n")
		}
	}

	// Get language settings
	lang := os.Getenv("LANG")
	if lang != "" {
		info.WriteString("Language: " + lang + "\n")
	} else {
		lcAll := os.Getenv("LC_ALL")
		if lcAll != "" {
			info.WriteString("Language: " + lcAll + "\n")
		}
	}

	return info.String(), nil
}

// RemoveAnsiEscapes removes ANSI escape sequences from a string
func RemoveAnsiEscapes(input string) string {
	// Replace \r with \n
	input = strings.ReplaceAll(input, "\r", "\n")

	// Remove various ANSI escape sequences
	regexes := []*regexp.Regexp{
		regexp.MustCompile(`\x1b\[?[0-9;]*[a-zA-Z]`),
		regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`),
		regexp.MustCompile(`\x1b\[[0-9;]*`),
	}

	for _, regex := range regexes {
		input = regex.ReplaceAllString(input, "")
	}

	// Remove progress bar lines
	progressBarRegex := regexp.MustCompile(`\.{10} \.{10} \.{10} \.{10} \.{9}`)
	lines := strings.Split(input, "\n")
	var filteredLines []string

	for _, line := range lines {
		if !progressBarRegex.MatchString(line) {
			filteredLines = append(filteredLines, line)
		}
	}

	return strings.Join(filteredLines, "\n")
}

// Helper function to check if a string contains any of the given patterns
func containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// SendErrorReport sends an error report to the Pi-Apps team
// This is a placeholder implementation
func SendErrorReport(logfilePath string) (string, error) {
	// Validate arguments
	if logfilePath == "" {
		return "", fmt.Errorf("send_error_report(): requires an argument")
	}

	// Check if file exists
	if !FileExists(logfilePath) {
		return "", fmt.Errorf("send_error_report(): '%s' is not a valid file", logfilePath)
	}

	// Check if curl is available
	_, err := exec.LookPath("curl")
	if err != nil {
		return "", fmt.Errorf("send_error_report(): Cannot send report: curl command not found")
	}

	// Format the log file before sending to ensure it's readable
	if err := FormatLogfile(logfilePath); err != nil {
		return "", fmt.Errorf("send_error_report(): error formatting log file: %w", err)
	}

	// Check if the log file contains the required header before sending
	containsHeader, err := fileContainsPattern(logfilePath, "^Last updated Pi-Apps on:")
	if err != nil {
		return "", fmt.Errorf("error checking log file contents: %w", err)
	}

	if !containsHeader {
		// If header is not found, just return success but don't actually send the report
		// This is to maintain compatibility with the original bash implementation
		return "Log file not sent - missing required header", nil
	}

	// The webhook URL is obfuscated to prevent abuse
	// This is similar to how the original bash script handled it
	webhookURLData := "aHR0cHM6Ly9yYXcuZ2l0aHVidXNlcmNvbnRlbnQuY29tL0JvdHNwb3QvcGktYXBwcy1hbmFseXRpY3MvbWFpbi9lcnJvci1sb2ctd2ViaG9vay1uZXcK"
	decodedURL, err := getDeobfuscatedWebhookURL(webhookURLData)
	if err != nil {
		return "", fmt.Errorf("error processing webhook URL: %w", err)
	}

	// Create a filename for the upload that removes the .log extension
	filename := filepath.Base(logfilePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".txt"

	// Use curl to upload the file to maintain compatibility with the original implementation
	// This approach is more resistant to simple reverse engineering
	cmd := exec.Command("curl", "-F", fmt.Sprintf("file=@%s;filename=%s", logfilePath, filename), decodedURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("curl failed to upload log file: %s", stderr.String())
	}

	return "Error report sent successfully!", nil
}

// getDeobfuscatedWebhookURL handles the deobfuscation of the webhook URL
// This function processes the obfuscated data to obtain the actual webhook URL
// The implementation is deliberately kept minimal to maintain security
func getDeobfuscatedWebhookURL(encodedData string) (string, error) {
	// First level of deobfuscation - base64 decode
	// This actually contains the GitHub URL where the webhook is stored
	githubURL, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", err
	}

	// Fetch the webhook URL from the obfuscated location
	resp, err := http.Get(string(githubURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	webhookData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Final layer of deobfuscation
	finalURL, err := base64.StdEncoding.DecodeString(string(webhookData))
	if err != nil {
		return "", err
	}

	return string(finalURL), nil
}

// fileContainsPattern checks if a file contains a given pattern
func fileContainsPattern(filePath, pattern string) (bool, error) {
	// Use grep to check if the file contains the pattern
	cmd := exec.Command("grep", pattern, filePath)
	if err := cmd.Run(); err != nil {
		// grep returns exit code 1 if the pattern is not found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Helper function to run shell commands
func runCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Helper function to get unique strings from a slice
func uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	sort.Strings(list)
	return list
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Helper function to check if raspi.list contains the required repository entries
func containsRaspiRepo(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	// Check for the required repository patterns
	repoPatterns := []string{
		"^deb http://archive.raspberrypi.org/debian",
		"^deb https://archive.raspberrypi.org/debian",
		"^deb http://archive.raspberrypi.com/debian",
		"^deb https://archive.raspberrypi.com/debian",
	}

	for _, pattern := range repoPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(string(content)) {
			return true
		}
	}

	return false
}

// getCodename returns the OS codename
func getCodename() string {
	// Try to get codename from /etc/os-release
	if _, err := os.Stat("/etc/os-release"); err == nil {
		output, err := runCommand("grep", "VERSION_CODENAME", "/etc/os-release")
		if err == nil && output != "" {
			parts := strings.Split(output, "=")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	// Try alternate approach for older systems
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		output, err := os.ReadFile("/etc/debian_version")
		if err == nil {
			codename := strings.TrimSpace(string(output))
			if strings.HasPrefix(codename, "11") {
				return "bullseye"
			} else if strings.HasPrefix(codename, "10") {
				return "buster"
			} else if strings.HasPrefix(codename, "9") {
				return "stretch"
			}
		}
	}

	// Fallback to a reasonable default
	return "bullseye"
}

// getArchitecture returns "32" or "64" based on system architecture
func getArchitecture() string {
	output, err := runCommand("getconf", "LONG_BIT")
	if err == nil {
		return strings.TrimSpace(output)
	}

	// Try alternative method
	output, err = runCommand("uname", "-m")
	if err == nil {
		if strings.Contains(output, "64") {
			return "64"
		} else {
			return "32"
		}
	}

	// Default fallback
	return "64"
}

// getOSInfo returns the OS ID and codename
func getOSInfo() (string, string) {
	osID := "Unknown"
	osCodename := "Unknown"

	// Check if /etc/os-release exists
	if _, err := os.Stat("/etc/os-release"); err == nil {
		// Get OS ID
		idOutput, err := runCommand("grep", "^ID=", "/etc/os-release")
		if err == nil && idOutput != "" {
			parts := strings.Split(idOutput, "=")
			if len(parts) >= 2 {
				osID = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}

		// Get OS codename
		codenameOutput, err := runCommand("grep", "^VERSION_CODENAME=", "/etc/os-release")
		if err == nil && codenameOutput != "" {
			parts := strings.Split(codenameOutput, "=")
			if len(parts) >= 2 {
				osCodename = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
	}

	// Additional check for Raspbian (may identify as Debian)
	if fileExists("/etc/rpi-issue") && osID == "Debian" {
		osID = "Raspbian"
	}

	return osID, osCodename
}

// checkBackportsRepo checks if the Debian backports repository is enabled
func checkBackportsRepo(codename string) (bool, error) {
	// Run apt-get indextargets command to list repositories
	output, err := runCommand("apt-get", "indextargets", "--no-release-info", "--format", "$(SITE) $(RELEASE) $(COMPONENT) $(TARGET_OF)")
	if err != nil {
		return false, err
	}

	// Process output to check for backports
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Check for fields matching backports repository
		if len(fields) >= 4 &&
			fields[3] == "deb" &&
			strings.Contains(fields[0], "debian.org/debian") &&
			strings.Contains(fields[1], codename+"-backports") &&
			fields[2] == "main" {
			return true, nil
		}
	}

	return false, nil
}

// findBackportsConflicts extracts package names from conflict errors
// and checks if they are from backports
func findBackportsConflicts(errors string) ([]string, error) {
	var conflicts []string

	// Extract package candidates from unmet dependencies cases
	// Case 1: Lines matching "^ .* : Depends:"
	dependsCase1 := regexp.MustCompile(`^ .* : Depends:`)
	scanner := bufio.NewScanner(strings.NewReader(errors))
	var candidates []string

	for scanner.Scan() {
		line := scanner.Text()
		if dependsCase1.MatchString(line) {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// Add both the package and its dependency
				candidates = append(candidates, parts[0], parts[3])
			}
		}
	}

	// Case 2: Lines matching "^ +Depends:"
	dependsCase2 := regexp.MustCompile(`^ +Depends:`)
	scanner = bufio.NewScanner(strings.NewReader(errors))
	for scanner.Scan() {
		line := scanner.Text()
		if dependsCase2.MatchString(line) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				candidates = append(candidates, parts[1])
			}
		}
	}

	// Case 3: Lines matching "^Depends:"
	dependsCase3 := regexp.MustCompile(`^Depends:`)
	scanner = bufio.NewScanner(strings.NewReader(errors))
	for scanner.Scan() {
		line := scanner.Text()
		if dependsCase3.MatchString(line) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				// Split dependencies by commas
				deps := strings.Split(parts[1], ", ")
				for _, dep := range deps {
					// Remove version requirements
					dep = regexp.MustCompile(`\([^)]*\)`).ReplaceAllString(dep, "")
					dep = strings.TrimSpace(dep)
					if dep != "" {
						candidates = append(candidates, dep)
					}
				}
			}
		}
	}

	// Clean package names (remove architecture suffixes)
	var cleanCandidates []string
	for _, pkg := range candidates {
		cleanPkg := regexp.MustCompile(`:(armhf|arm64|all)`).ReplaceAllString(pkg, "")
		cleanCandidates = append(cleanCandidates, cleanPkg)
	}

	// Remove duplicates
	cleanCandidates = uniqueStrings(cleanCandidates)

	// For each candidate, check if it's installed from backports
	for _, pkg := range cleanCandidates {
		output, err := runCommand("apt", "list", "--installed", pkg)
		if err == nil && strings.Contains(output, "-backports,now") {
			conflicts = append(conflicts, pkg)
		}
	}

	return conflicts, nil
}

// GetDeviceModel returns detailed information about the hardware model and SoC
func GetDeviceModel() (string, string) {
	// Initialize variables
	model := ""
	socID := ""

	// Check all possible model locations from most to least common
	modelPaths := []string{
		"/sys/firmware/devicetree/base/model",
		"/sys/firmware/devicetree/base/banner-name",
		"/tmp/sysinfo/model",
		"/sys/devices/virtual/dmi/id/product_name",
		"/sys/class/dmi/id/product_name",
	}

	// Try each path until we find one that works
	for _, path := range modelPaths {
		if _, err := os.Stat(path); err == nil {
			modelBytes, err := os.ReadFile(path)
			if err == nil && len(modelBytes) > 0 {
				// Remove null terminator and trim whitespace
				model = strings.TrimSpace(strings.ReplaceAll(string(modelBytes), "\x00", ""))
				if model != "" {
					break
				}
			}
		}
	}

	// Check for Android environment
	// Android typically has /system/app/ and /system/priv-app directories
	if model == "" && fileExists("/system/app/") && fileExists("/system/priv-app") {
		// Try multiple Android property commands in order of preference
		androidProps := []string{
			"ro.product.marketname",
			"ro.vendor.product.display",
			"ro.config.devicename",
			"ro.config.marketing_name",
			"ro.product.vendor.model",
			"ro.product.oppo_model",
			"ro.oppo.market.name",
			"ro.product.model",
			"ro.product.product.model",
			"ro.product.odm.model",
		}

		for _, prop := range androidProps {
			cmd := exec.Command("getprop", prop)
			output, err := cmd.Output()
			if err == nil {
				propValue := strings.TrimSpace(string(output))
				if propValue != "" {
					model = propValue
					break
				}
			}
		}
	}

	// SoC (System on Chip) detection
	// Check for Tegra, Rockchip, Broadcom (RPi), etc.
	compatiblePath := "/proc/device-tree/compatible"
	if fileExists(compatiblePath) {
		compatibleBytes, err := os.ReadFile(compatiblePath)
		if err == nil {
			chip := strings.ReplaceAll(string(compatibleBytes), "\x00", "")

			// Nvidia Tegra detection
			tegraMapping := map[string]string{
				"tegra20":  "tegra-2",
				"tegra30":  "tegra-3",
				"tegra114": "tegra-4",
				"tegra124": "tegra-k1-32",
				"tegra132": "tegra-k1-64",
				"tegra210": "tegra-x1",
				"tegra186": "tegra-x2",
				"tegra194": "xavier",
				"tegra234": "orin",
				"tegra239": "switch-pro-chip",
			}

			for key, value := range tegraMapping {
				if strings.Contains(chip, key) {
					socID = value
					break
				}
			}

			// Generic Tegra detection
			if socID == "" && strings.Contains(chip, "tegra") {
				socID = "jetson-unknown"
			}

			// Rockchip SoC detection
			rockchipIDs := []string{
				"rk3399", "rk3308", "rk3326", "rk3328",
				"rk3368", "rk3566", "rk3568",
			}

			for _, id := range rockchipIDs {
				if strings.Contains(chip, id) {
					socID = id
					break
				}
			}

			// Amlogic SoC detection
			if strings.Contains(chip, "g12b") {
				socID = "g12b"
			}

			// Broadcom (Raspberry Pi) detection
			bcmIDs := []string{
				"bcm2712", "bcm2711", "bcm2837", "bcm2836", "bcm2835",
			}

			for _, id := range bcmIDs {
				if strings.Contains(chip, id) {
					socID = id
					break
				}
			}
		}
	}

	// Older Tegra detection method
	if socID == "" && fileExists("/sys/devices/soc0/family") {
		familyBytes, err := os.ReadFile("/sys/devices/soc0/family")
		if err == nil {
			chip := strings.ReplaceAll(string(familyBytes), "\x00", "")

			tegraMapping := map[string]string{
				"tegra20":  "tegra-2",
				"tegra30":  "tegra-3",
				"tegra114": "tegra-4",
				"tegra124": "tegra-k1-32",
				"tegra132": "tegra-k1-64",
				"tegra210": "tegra-x1",
			}

			for key, value := range tegraMapping {
				if strings.Contains(chip, key) {
					socID = value
					break
				}
			}
		}
	}

	// If still no model, use hostname as last resort (for RPi)
	if model == "" {
		hostname, err := os.Hostname()
		if err == nil && hostname != "" {
			// Raspberry Pi hostnames often contain model info
			if strings.Contains(strings.ToLower(hostname), "raspberry") ||
				strings.Contains(strings.ToLower(hostname), "rpi") {
				model = hostname
			}
		}
	}

	// If model is still empty, set to Unknown
	if model == "" {
		model = "Unknown"
	}

	return model, socID
}
