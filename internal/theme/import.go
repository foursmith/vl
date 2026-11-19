package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type themeContribution struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	UITheme string `json:"uiTheme"`
	Path    string `json:"path"`
}

type extensionManifest struct {
	Publisher   string `json:"publisher"`
	Name        string `json:"name"`
	Contributes struct {
		Themes []themeContribution `json:"themes"`
	} `json:"contributes"`
}

func VSCodeExtensionDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	candidates := []string{filepath.Join(home, ".vscode", "extensions")}
	var windowsInstalls []string
	if code, err := exec.LookPath("code"); err == nil {
		if resolved, err := filepath.EvalSymlinks(code); err == nil {
			install := filepath.Join(filepath.Dir(resolved), "..")
			if runtime.GOOS == "windows" {
				windowsInstalls = append(windowsInstalls, install)
			} else {
				candidates = append(candidates, filepath.Join(install, "extensions"))
			}
		}
	}
	switch runtime.GOOS {
	case "darwin":
		for _, apps := range []string{"/Applications", filepath.Join(home, "Applications")} {
			candidates = append(candidates, filepath.Join(apps, "Visual Studio Code.app", "Contents", "Resources", "app", "extensions"))
		}
	case "linux":
		candidates = append(candidates, "/usr/share/code/resources/app/extensions",
			"/usr/lib/code/extensions", "/usr/lib/code/resources/app/extensions",
			"/opt/visual-studio-code/resources/app/extensions", "/snap/code/current/usr/share/code/resources/app/extensions")
	case "windows":
		for _, programs := range []string{filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			if filepath.IsAbs(programs) {
				windowsInstalls = append(windowsInstalls, filepath.Join(programs, "Microsoft VS Code"))
			}
		}
	}
	for _, install := range windowsInstalls {
		dirs, err := windowsVSCodeExtensionCandidates(install)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, dirs...)
	}

	var dirs []string
	seen := map[string]bool{}
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			real, err := filepath.EvalSymlinks(dir)
			if err != nil {
				return nil, err
			}
			if !seen[real] {
				dirs = append(dirs, real)
				seen[real] = true
			}
		}
	}
	if len(dirs) == 0 {
		return nil, errors.New("No VS Code extension folders found")
	}
	return dirs, nil
}

var windowsVersionDirPattern = regexp.MustCompile(`^[0-9a-f]{10}$`)

func windowsVSCodeExtensionCandidates(install string) ([]string, error) {
	dirs := []string{filepath.Join(install, "resources", "app", "extensions")}
	entries, err := os.ReadDir(install)
	if errors.Is(err, os.ErrNotExist) {
		return dirs, nil
	}
	if err != nil {
		return nil, err
	}
	// New Windows installs put resources under the first ten commit hash digits.
	for _, entry := range entries {
		if entry.IsDir() && windowsVersionDirPattern.MatchString(entry.Name()) {
			dirs = append(dirs, filepath.Join(install, entry.Name(), "resources", "app", "extensions"))
		}
	}
	return dirs, nil
}

// ImportVSCode copies the color themes declared by installed extensions.
// Successful imports are kept when other contributions fail to load.
func ImportVSCode(dest string, dirs []string) (int, error) {
	var problems []error
	count := 0
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		var obsolete map[string]bool
		if err := readJSONC(filepath.Join(dir, ".obsolete"), &obsolete); err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = append(problems, fmt.Errorf("%s: %w", filepath.Join(dir, ".obsolete"), err))
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || obsolete[name] ||
				(!entry.IsDir() && entry.Type()&os.ModeSymlink == 0) {
				continue
			}
			added, err := importExtension(dest, filepath.Join(dir, name))
			count += added
			if err != nil {
				problems = append(problems, err)
			}
		}
	}
	return count, errors.Join(problems...)
}

func importExtension(dest, dir string) (int, error) {
	var manifest extensionManifest
	if err := readJSONC(filepath.Join(dir, "package.json"), &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("%s: %w", dir, err)
	}
	if len(manifest.Contributes.Themes) == 0 {
		return 0, nil
	}
	extensionID := strings.ToLower(manifest.Publisher + "." + manifest.Name)
	if !extensionIDPattern.MatchString(extensionID) {
		return 0, fmt.Errorf("%s: invalid extension ID %q", dir, extensionID)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	read := func(name string) ([]byte, error) {
		file, err := root.Open(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return readThemeData(file, name)
	}
	var labels map[string]string
	if data, err := read("package.nls.json"); err == nil {
		_ = decodeJSONC(data, &labels)
	}

	count := 0
	var problems []error
	for _, contribution := range manifest.Contributes.Themes {
		source := path.Clean(strings.ReplaceAll(contribution.Path, "\\", "/"))
		label := contribution.Label
		if strings.HasPrefix(label, "%") && strings.HasSuffix(label, "%") {
			label = labels[strings.Trim(label, "%")]
		}
		id := contribution.ID
		if id == "" {
			id = label
		}
		if id == "" {
			id = path.Base(source)
		}
		if isBundledTheme(extensionID, id) {
			continue
		}
		filename := themeFilename(id)
		added, err := importContribution(filepath.Join(dest, extensionID, filename), source, id, label, contribution.UITheme, read)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s (%q): %w", extensionID, id, err))
			continue
		}
		if added {
			count++
		}
	}
	return count, errors.Join(problems...)
}

func importContribution(dest, source, id, label, kind string, read func(string) ([]byte, error)) (bool, error) {
	if _, err := os.Lstat(dest); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if source == "." || !filepath.IsLocal(filepath.FromSlash(source)) {
		return false, fmt.Errorf("invalid theme path %q", source)
	}
	resolved, err := loadSource(source, kind, read)
	if err != nil {
		return false, err
	}
	if label != "" {
		resolved.Name = label
	}
	resolved.ID = id
	data, err := json.Marshal(resolved)
	if err != nil {
		return false, err
	}
	if len(data)+1 > maxThemeBytes {
		return false, errors.New("resolved theme exceeds 8 MiB")
	}

	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, err
	}
	file, err := os.CreateTemp(dir, ".import-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(append(data, '\n'))
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return false, err
	}
	// Link publishes the complete file without replacing a concurrent import.
	if err := os.Link(file.Name(), dest); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
