package api

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ListApps lists apps based on the specified filter
// Filters include: installed, uninstalled, corrupted, cpu_installable, hidden, visible,
// online, online_only, local, local_only, all, package, standard, have_status, missing_status, disabled
func ListApps(filter string) ([]string, error) {
	// Get the directory from environment variable
	directory := os.Getenv("PI_APPS_DIR")
	if directory == "" {
		return nil, fmt.Errorf("PI_APPS_DIR environment variable not set")
	}

	// Default case: local apps (all local apps)
	if filter == "" || filter == "local" {
		apps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to list local apps: %w", err)
		}
		sort.Strings(apps)
		return apps, nil
	}

	switch filter {
	case "all":
		// Combined list of apps, both online and local with duplicates removed
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		// Combine and remove duplicates
		return ListUnion(localApps, onlineApps), nil

	case "installed":
		// List apps that are installed
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get installed apps: %w", err)
		}
		return installedApps, nil

	case "uninstalled":
		// List apps that are uninstalled
		uninstalledApps, err := getAppsWithStatus(directory, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get uninstalled apps: %w", err)
		}
		return uninstalledApps, nil

	case "corrupted":
		// List apps that are corrupted
		corruptedApps, err := getCorruptedApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get corrupted apps: %w", err)
		}
		return corruptedApps, nil

	case "disabled":
		// List apps that are disabled
		disabledApps, err := getAppsWithStatusContent(directory, "disabled")
		if err != nil {
			return nil, fmt.Errorf("failed to get disabled apps: %w", err)
		}
		return disabledApps, nil

	case "have_status":
		// List apps that have a status file
		statusApps, err := getAppsWithStatusFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get apps with status files: %w", err)
		}
		return statusApps, nil

	case "missing_status":
		// List apps that don't have a status file
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		statusApps, err := getAppsWithStatusFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get apps with status files: %w", err)
		}

		return ListSubtract(localApps, statusApps), nil

	case "cpu_installable":
		// List apps that can be installed on the device's OS architecture (32-bit or 64-bit)
		return getCPUInstallableApps(directory)

	case "package":
		// List apps that have a "packages" file
		return getAppsWithFile(directory, "packages")

	case "standard":
		// List apps that have scripts
		return getStandardApps(directory)

	case "hidden":
		// List apps that are in the hidden category
		hiddenApps, err := getCategoryApps(directory, "hidden")
		if err != nil {
			return nil, fmt.Errorf("failed to get hidden apps: %w", err)
		}
		return hiddenApps, nil

	case "visible":
		// List apps that are in any category but 'hidden'
		allCategories, err := readCategoryFiles(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to read category files: %w", err)
		}

		var visibleApps []string
		for app, category := range allCategories {
			if category != "hidden" {
				visibleApps = append(visibleApps, app)
			}
		}

		// Subtract disabled apps if needed
		// disabled, err := getAppsWithStatusContent(directory, "disabled")
		// if err != nil {
		//     return nil, fmt.Errorf("failed to get disabled apps: %w", err)
		// }
		// return ListSubtract(visibleApps, disabled), nil

		sort.Strings(visibleApps)
		return visibleApps, nil

	case "online":
		// List apps that exist on the online git repo
		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}
		return onlineApps, nil

	case "online_only":
		// List apps that exist only on the git repo, and not locally
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		return ListSubtract(onlineApps, localApps), nil

	case "local_only":
		// List apps that exist only locally, and not on the git repo
		localApps, err := listLocalApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get local apps: %w", err)
		}

		onlineApps, err := listOnlineApps(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to get online apps: %w", err)
		}

		return ListSubtract(localApps, onlineApps), nil

	default:
		// Check if the filter is a category name
		categoryApps, err := getCategoryApps(directory, filter)
		if err == nil {
			// Successfully found a category
			return categoryApps, nil
		}

		return nil, fmt.Errorf("unknown filter: %s", filter)
	}
}

// ListIntersect returns a list of items that appear in both list1 and list2 (exact matches only)
func ListIntersect(list1, list2 []string) []string {
	// Create a map from list2 for faster lookups
	list2Map := make(map[string]bool)
	for _, item := range list2 {
		list2Map[item] = true
	}

	// Find items in list1 that are also in list2
	var result []string
	for _, item := range list1 {
		if list2Map[item] {
			result = append(result, item)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListIntersectPartial returns a list of items from list1 that have a partial match in list2
func ListIntersectPartial(list1, list2 []string) []string {
	var result []string
	for _, item1 := range list1 {
		for _, item2 := range list2 {
			if strings.Contains(item1, item2) {
				result = append(result, item1)
				break
			}
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListSubtract returns a list of items from list1 that don't appear in list2 (exact matches only)
func ListSubtract(list1, list2 []string) []string {
	// Create a map from list2 for faster lookups
	list2Map := make(map[string]bool)
	for _, item := range list2 {
		list2Map[item] = true
	}

	// Find items in list1 that are not in list2
	var result []string
	for _, item := range list1 {
		if !list2Map[item] {
			result = append(result, item)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListSubtractPartial returns a list of items from list1 that don't have a partial match in list2
func ListSubtractPartial(list1, list2 []string) []string {
	var result []string
	for _, item1 := range list1 {
		hasMatch := false
		for _, item2 := range list2 {
			if strings.Contains(item1, item2) {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			result = append(result, item1)
		}
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// ListUnion returns a combined list with duplicates removed
func ListUnion(list1, list2 []string) []string {
	// Create a map to track seen items
	seen := make(map[string]bool)
	for _, item := range list1 {
		seen[item] = true
	}
	for _, item := range list2 {
		seen[item] = true
	}

	// Create a result list from the map keys
	var result []string
	for item := range seen {
		result = append(result, item)
	}

	// Sort the result for consistent output
	sort.Strings(result)
	return result
}

// Helper functions

// listLocalApps lists all locally available apps
func listLocalApps(directory string) ([]string, error) {
	appsDir := filepath.Join(directory, "apps")

	var apps []string
	err := filepath.WalkDir(appsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root apps directory
		if path == appsDir {
			return nil
		}

		// Only process directories directly under apps/
		if d.IsDir() && filepath.Dir(path) == appsDir {
			apps = append(apps, filepath.Base(path))
			// Don't descend into subdirectories
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return apps, nil
}

// listOnlineApps lists all apps available in the remote repository
func listOnlineApps(directory string) ([]string, error) {
	updateDir := filepath.Join(directory, "update", "pi-apps", "apps")

	// If update directory exists, use it
	if checkFileExists(updateDir) {
		var apps []string
		err := filepath.WalkDir(updateDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip the root apps directory
			if path == updateDir {
				return nil
			}

			// Only process directories directly under apps/
			if d.IsDir() && filepath.Dir(path) == updateDir {
				apps = append(apps, filepath.Base(path))
				// Don't descend into subdirectories
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil {
			return nil, err
		}

		return apps, nil
	}

	// For now, return local apps as a fallback
	// In a real implementation, this would fetch from the remote repository
	return listLocalApps(directory)
}

// getAppsWithStatus returns a list of apps with the specified status (installed or uninstalled)
func getAppsWithStatus(directory string, wantInstalled bool) ([]string, error) {
	// Get all local apps
	allApps, err := listLocalApps(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
	}

	var filteredApps []string
	for _, app := range allApps {
		// Check if the app has the expected status
		isInstalled := checkAppInstalled(directory, app)
		if (wantInstalled && isInstalled) || (!wantInstalled && !isInstalled) {
			filteredApps = append(filteredApps, app)
		}
	}

	sort.Strings(filteredApps)
	return filteredApps, nil
}

// getAppsWithStatusContent returns a list of apps with the specified status content
func getAppsWithStatusContent(directory string, statusContent string) ([]string, error) {
	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
		return []string{}, nil
	}

	// For each status file, check its content
	var matchingApps []string
	err := filepath.WalkDir(statusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root status directory
		if path == statusDir {
			return nil
		}

		// Only process files directly under status/
		if !d.IsDir() && filepath.Dir(path) == statusDir {
			// Read the file content
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files with errors
			}

			// Check if the content matches
			if strings.TrimSpace(string(content)) == statusContent {
				matchingApps = append(matchingApps, filepath.Base(path))
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(matchingApps)
	return matchingApps, nil
}

// getCorruptedApps returns a list of corrupted apps
func getCorruptedApps(directory string) ([]string, error) {
	// Get all local apps
	allApps, err := listLocalApps(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var corruptedApps []string
	for _, app := range allApps {
		appDir := filepath.Join(directory, "apps", app)

		// Check if install and uninstall scripts exist
		installScript := filepath.Join(appDir, "install")
		uninstallScript := filepath.Join(appDir, "uninstall")

		// App is corrupted if either script is missing
		if !checkFileExists(installScript) || !checkFileExists(uninstallScript) {
			corruptedApps = append(corruptedApps, app)
			continue
		}

		// App is corrupted if the icon is missing
		iconFile := filepath.Join(appDir, "icon.png")
		if !checkFileExists(iconFile) {
			corruptedApps = append(corruptedApps, app)
			continue
		}

		// App is corrupted if the description is missing
		descFile := filepath.Join(appDir, "description")
		if !checkFileExists(descFile) {
			corruptedApps = append(corruptedApps, app)
			continue
		}
	}

	sort.Strings(corruptedApps)
	return corruptedApps, nil
}

// getAppsWithStatusFiles returns a list of apps that have status files
func getAppsWithStatusFiles(directory string) ([]string, error) {
	statusDir := filepath.Join(directory, "data", "status")
	if _, err := os.Stat(statusDir); os.IsNotExist(err) {
		// If status directory doesn't exist, create it
		if err := os.MkdirAll(statusDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create status directory: %w", err)
		}
		return []string{}, nil
	}

	var apps []string
	err := filepath.WalkDir(statusDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root status directory
		if path == statusDir {
			return nil
		}

		// Only process files directly under status/
		if !d.IsDir() && filepath.Dir(path) == statusDir {
			apps = append(apps, filepath.Base(path))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(apps)
	return apps, nil
}

// getCPUInstallableApps returns a list of apps that can be installed on the current CPU
func getCPUInstallableApps(directory string) ([]string, error) {
	// Get system architecture
	cmd := exec.Command("getconf", "LONG_BIT")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to determine system architecture: %w", err)
	}

	arch := strings.TrimSpace(string(output))

	var appNames []string
	appPath := filepath.Join(directory, "apps")

	// Find apps with install script, install-XX script, or packages file
	err = filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			fileName := d.Name()
			if fileName == "install" || fileName == fmt.Sprintf("install-%s", arch) || fileName == "packages" {
				// Get app name (parent directory name)
				appName := filepath.Base(filepath.Dir(path))
				appNames = append(appNames, appName)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	// Remove duplicates
	uniqueApps := make(map[string]bool)
	var result []string
	for _, app := range appNames {
		if !uniqueApps[app] {
			uniqueApps[app] = true
			result = append(result, app)
		}
	}

	sort.Strings(result)
	return result, nil
}

// getAppsWithFile returns a list of apps that have the specified file
func getAppsWithFile(directory string, fileName string) ([]string, error) {
	var appNames []string
	appPath := filepath.Join(directory, "apps")

	err := filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && d.Name() == fileName {
			// Get app name (parent directory name)
			appName := filepath.Base(filepath.Dir(path))
			appNames = append(appNames, appName)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	sort.Strings(appNames)
	return appNames, nil
}

// getStandardApps returns a list of apps that have scripts
func getStandardApps(directory string) ([]string, error) {
	var appNames []string
	appPath := filepath.Join(directory, "apps")

	err := filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			fileName := d.Name()
			if fileName == "install" || fileName == "install-32" || fileName == "install-64" || fileName == "uninstall" {
				// Get app name (parent directory name)
				appName := filepath.Base(filepath.Dir(path))
				appNames = append(appNames, appName)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk app directory: %w", err)
	}

	// Remove duplicates
	uniqueApps := make(map[string]bool)
	var result []string
	for _, app := range appNames {
		if !uniqueApps[app] {
			uniqueApps[app] = true
			result = append(result, app)
		}
	}

	sort.Strings(result)
	return result, nil
}

// getCategoryApps returns a list of apps in the specified category
func getCategoryApps(directory string, category string) ([]string, error) {
	categoryFile := filepath.Join(directory, "data", "categories", category)

	// Make sure the category file exists
	if !checkFileExists(categoryFile) {
		// For categories that don't exist yet, create a directory
		categoryDir := filepath.Join(directory, "data", "categories")
		if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
			if err := os.MkdirAll(categoryDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create categories directory: %w", err)
			}
		}

		// Create an empty category file
		if err := os.WriteFile(categoryFile, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("failed to create category file: %w", err)
		}

		// Return an empty list
		return []string{}, nil
	}

	// Read app names from category file
	data, err := os.ReadFile(categoryFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read category file: %w", err)
	}

	var apps []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		appName := scanner.Text()
		if appName != "" {
			apps = append(apps, appName)
		}
	}

	sort.Strings(apps)
	return apps, nil
}

// readCategoryFiles reads all app category assignments
func readCategoryFiles(directory string) (map[string]string, error) {
	categoryDir := filepath.Join(directory, "data", "categories")

	// Make sure the category directory exists
	if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
		// Create the categories directory
		if err := os.MkdirAll(categoryDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create categories directory: %w", err)
		}

		// Return an empty map
		return make(map[string]string), nil
	}

	// Read all category files
	entries, err := os.ReadDir(categoryDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read category directory: %w", err)
	}

	result := make(map[string]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			categoryFilePath := filepath.Join(categoryDir, entry.Name())
			categoryName := entry.Name()

			// Read app names from category file
			data, err := os.ReadFile(categoryFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read category file %s: %w", categoryName, err)
			}

			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				appName := scanner.Text()
				if appName != "" {
					result[appName] = categoryName
				}
			}
		}
	}

	return result, nil
}

// checkAppInstalled checks if an app is installed
func checkAppInstalled(directory, app string) bool {
	statusFile := filepath.Join(directory, "data", "status", app)
	return checkFileExists(statusFile)
}

// checkFileExists checks if a file exists
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadCategoryFiles generates a combined categories-list from several sources:
// category-overrides, global categories file, and unlisted apps. Format: "app|category"
func ReadCategoryFiles(directory string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	// Read all categories from the categories directory
	categoriesDir := filepath.Join(directory, "data", "categories")
	if checkFileExists(categoriesDir) {
		entries, err := os.ReadDir(categoriesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				categoryName := entry.Name()
				categoryFile := filepath.Join(categoriesDir, categoryName)

				// Read the apps in this category
				data, err := os.ReadFile(categoryFile)
				if err != nil {
					continue
				}

				scanner := bufio.NewScanner(strings.NewReader(string(data)))
				for scanner.Scan() {
					appName := scanner.Text()
					if appName == "" {
						continue
					}

					if !seen[appName] {
						result = append(result, appName+"|"+categoryName)
						seen[appName] = true
					}
				}
			}
		}
	}

	// Check for category-overrides file (if we were to implement it)
	userOverridesFile := filepath.Join(directory, "data", "category-overrides")
	if checkFileExists(userOverridesFile) {
		// This would override the categories above
	}

	// Add all local apps that don't have a category yet
	localApps, err := listLocalApps(directory)
	if err == nil {
		for _, app := range localApps {
			if !seen[app] {
				result = append(result, app+"|")
				seen[app] = true
			}
		}
	}

	// Filter empty entries
	var filteredResult []string
	for _, line := range result {
		if line != "" {
			filteredResult = append(filteredResult, line)
		}
	}

	return filteredResult, nil
}

// AppPrefixCategory lists all apps in a category with format "category/app",
// or if category is left blank, then list the full structure of all categories
func AppPrefixCategory(directory, category string) ([]string, error) {
	var result []string

	// Get the "Show apps" setting
	showAppsSetting := ""
	settingsFile := filepath.Join(directory, "data", "settings", "Show apps")
	if checkFileExists(settingsFile) {
		data, err := os.ReadFile(settingsFile)
		if err == nil {
			showAppsSetting = strings.TrimSpace(string(data))
		}
	}

	// Prepare filter function based on settings
	filterApps := func(apps []string) ([]string, error) {
		if showAppsSetting == "standard" {
			// If only showing standard apps, hide package apps
			packageApps, err := getAppsWithFile(directory, "packages")
			if err != nil {
				return nil, err
			}

			// Format package apps with wildcard prefix for partial matching
			var formattedPackageApps []string
			for _, app := range packageApps {
				formattedPackageApps = append(formattedPackageApps, ".*/"+app)
			}

			return ListSubtractPartial(apps, formattedPackageApps), nil
		} else if showAppsSetting == "packages" {
			// If only showing package apps, hide standard apps
			standardApps, err := getStandardApps(directory)
			if err != nil {
				return nil, err
			}

			// Format standard apps with wildcard prefix for partial matching
			var formattedStandardApps []string
			for _, app := range standardApps {
				formattedStandardApps = append(formattedStandardApps, ".*/"+app)
			}

			return ListSubtractPartial(apps, formattedStandardApps), nil
		}

		// Default case: don't filter
		return apps, nil
	}

	// Get hidden apps
	hiddenApps, err := getCategoryApps(directory, "hidden")
	if err != nil {
		return nil, err
	}

	if category == "Installed" {
		// Show special "Installed" category - don't filter it
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(installedApps, hiddenApps)

		// Format installed apps with category prefix
		for _, app := range filteredApps {
			result = append(result, "Installed/"+app)
		}
	} else if category == "Packages" {
		// Show special "Packages" category
		packageApps, err := getAppsWithFile(directory, "packages")
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(packageApps, hiddenApps)

		// Format package apps with category prefix
		for _, app := range filteredApps {
			result = append(result, "Packages/"+app)
		}
	} else if category == "All Apps" {
		// Show special "All Apps" category
		cpuInstallableApps, err := getCPUInstallableApps(directory)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredApps := ListSubtract(cpuInstallableApps, hiddenApps)

		// Format apps with category prefix
		var formattedApps []string
		for _, app := range filteredApps {
			formattedApps = append(formattedApps, "All Apps/"+app)
		}

		// Apply filter based on settings
		filteredResult, err := filterApps(formattedApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredResult...)
	} else if category == "" {
		// Show all categories

		// First, get regular categories
		categoryEntries, err := ReadCategoryFiles(directory)
		if err != nil {
			return nil, err
		}

		// Map of categories to apps
		categories := make(map[string][]string)
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			appName := parts[0]
			categoryName := ""
			if len(parts) > 1 {
				categoryName = parts[1]
			}

			// Skip hidden apps
			if containsApp(hiddenApps, appName) {
				continue
			}

			if categoryName != "" && categoryName != "hidden" {
				categories[categoryName] = append(categories[categoryName], appName)
			}
		}

		// Format apps with category prefix
		var formattedCategoryApps []string
		for categoryName, apps := range categories {
			for _, appName := range apps {
				formattedCategoryApps = append(formattedCategoryApps, categoryName+"/"+appName)
			}
		}

		// Apply filter based on settings
		filteredCategoryApps, err := filterApps(formattedCategoryApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredCategoryApps...)

		// Add special "Installed" category - don't filter it
		installedApps, err := getAppsWithStatus(directory, true)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredInstalledApps := ListSubtract(installedApps, hiddenApps)

		for _, app := range filteredInstalledApps {
			result = append(result, "Installed/"+app)
		}

		// Add special "Packages" category if not standard mode
		if showAppsSetting != "standard" {
			packageApps, err := getAppsWithFile(directory, "packages")
			if err != nil {
				return nil, err
			}

			// Filter out hidden apps
			filteredPackageApps := ListSubtract(packageApps, hiddenApps)

			for _, app := range filteredPackageApps {
				result = append(result, "Packages/"+app)
			}
		}

		// Add special "All Apps" category
		cpuInstallableApps, err := getCPUInstallableApps(directory)
		if err != nil {
			return nil, err
		}

		// Filter out hidden apps
		filteredAllApps := ListSubtract(cpuInstallableApps, hiddenApps)

		// Format apps with category prefix
		var formattedAllApps []string
		for _, app := range filteredAllApps {
			formattedAllApps = append(formattedAllApps, "All Apps/"+app)
		}

		// Apply filter based on settings
		filteredAllAppsResult, err := filterApps(formattedAllApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredAllAppsResult...)
	} else {
		// Show apps in specific category
		categoryEntries, err := ReadCategoryFiles(directory)
		if err != nil {
			return nil, err
		}

		// Get apps in the specified category
		var appsInCategory []string
		for _, entry := range categoryEntries {
			parts := strings.Split(entry, "|")
			appName := parts[0]
			categoryName := ""
			if len(parts) > 1 {
				categoryName = parts[1]
			}

			// Skip hidden apps
			if containsApp(hiddenApps, appName) {
				continue
			}

			if categoryName == category {
				appsInCategory = append(appsInCategory, appName)
			}
		}

		// Format apps with category prefix
		var formattedCategoryApps []string
		for _, appName := range appsInCategory {
			formattedCategoryApps = append(formattedCategoryApps, category+"/"+appName)
		}

		// Apply filter based on settings
		filteredResult, err := filterApps(formattedCategoryApps)
		if err != nil {
			return nil, err
		}

		result = append(result, filteredResult...)
	}

	sort.Strings(result)
	return result, nil
}

// Helper function to check if a string is in a slice
func containsApp(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
