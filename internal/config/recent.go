package config

import "slices"

const maxRecentFolders = 20

func RecentFolders() ([]string, error) {
	path, err := statePath("recent.json")
	if err != nil {
		return nil, err
	}
	return loadRecentFolders(path)
}

func loadRecentFolders(path string) ([]string, error) {
	var folders []string
	err := loadJSON(path, &folders)
	return folders, err
}

// RememberFolder moves a successfully opened folder to the front of the list.
func RememberFolder(folder string) error {
	path, err := statePath("recent.json")
	if err != nil {
		return err
	}
	return rememberFolder(path, folder)
}

func rememberFolder(path, folder string) error {
	folders, err := loadRecentFolders(path)
	if err != nil {
		return err
	}
	recent := []string{folder}
	seen := map[string]bool{folder: true, "": true}
	for _, previous := range folders {
		if len(recent) == maxRecentFolders {
			break
		}
		if !seen[previous] {
			recent = append(recent, previous)
			seen[previous] = true
		}
	}
	return saveJSON(path, recent)
}

// ForgetFolder removes a recent entry without changing its directory or session.
func ForgetFolder(folder string) ([]string, error) {
	path, err := statePath("recent.json")
	if err != nil {
		return nil, err
	}
	return forgetFolder(path, folder)
}

func forgetFolder(path, folder string) ([]string, error) {
	folders, err := loadRecentFolders(path)
	if err != nil {
		return nil, err
	}
	folders = slices.DeleteFunc(folders, func(previous string) bool { return previous == folder })
	if folders == nil {
		folders = []string{}
	}
	return folders, saveJSON(path, folders)
}
