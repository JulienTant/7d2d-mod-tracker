package tracker

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var (
	versionPattern       = regexp.MustCompile(`(?i)(?:^|[^0-9])(?:v(?:ersion)?[\s._-]*)?([0-9]+(?:\.[0-9]+){1,3})(?:[^0-9]|$)`)
	taggedVersionPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])v(?:ersion)?[\s._-]*([0-9]+)(?:[^0-9]|$)`)
	identifierPattern    = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,3}$`)
)

type Mod struct {
	Folder      string
	Name        string
	Description string
	Author      string
	Version     string
	Website     string
	Path        string
	Sources     []string
}

type UpdateStatus string

const (
	StatusNeedsSource     UpdateStatus = "Needs source"
	StatusNotChecked      UpdateStatus = "Not checked"
	StatusCheckFailed     UpdateStatus = "Check failed"
	StatusUnknown         UpdateStatus = "Unknown"
	StatusLatestFound     UpdateStatus = "Latest found"
	StatusUpdateAvailable UpdateStatus = "Update available"
	StatusAhead           UpdateStatus = "Ahead"
	StatusUpToDate        UpdateStatus = "Up to date"
)

func (status UpdateStatus) String() string {
	return string(status)
}

type modInfo struct {
	Name        valueElement
	DisplayName valueElement
	Description valueElement
	Author      valueElement
	Version     valueElement
	Website     valueElement
	Nested      *modInfo `xml:"ModInfo"`
}

type valueElement struct {
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

func (e valueElement) String() string {
	if strings.TrimSpace(e.Value) != "" {
		return strings.TrimSpace(e.Value)
	}
	return strings.TrimSpace(e.Text)
}

func DefaultModsPaths() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		programFiles := os.Getenv("PROGRAMFILES(X86)")
		if programFiles == "" {
			programFiles = `C:\Program Files (x86)`
		}
		return []string{
			filepath.Join(appData, "7DaysToDie", "Mods"),
			filepath.Join(programFiles, "Steam", "steamapps", "common", "7 Days To Die", "Mods"),
		}
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "7DaysToDie", "Mods"),
			filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "7 Days To Die", "Mods"),
		}
	default:
		return []string{
			filepath.Join(
				home, ".local", "share", "Steam", "steamapps", "compatdata", "251570",
				"pfx", "drive_c", "users", "steamuser", "AppData", "Roaming", "7DaysToDie", "Mods",
			),
			filepath.Join(home, ".local", "share", "7DaysToDie", "Mods"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "7 Days To Die", "Mods"),
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "7 Days To Die", "Mods"),
		}
	}
}

func ExistingDefaultPath() string {
	return bestExistingModsPath(DefaultModsPaths())
}

func DetectModsPath() string {
	return bestExistingModsPath(DefaultModsPaths())
}

func bestExistingModsPath(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	bestPath, bestScore := candidates[0], -1
	for index, candidate := range candidates {
		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		communityMods := 0
		for _, entry := range entries {
			if entry.IsDir() && !isManagedGameMod(entry.Name()) {
				if _, err := os.Stat(filepath.Join(candidate, entry.Name(), "ModInfo.xml")); err == nil {
					communityMods++
				}
			}
		}
		score := 1 + len(candidates) - index
		if communityMods > 0 {
			score += communityMods * 1000
			if !IsSteamInstallModsPath(candidate) {
				score += 100
			}
		}
		if score > bestScore {
			bestPath, bestScore = candidate, score
		}
	}
	return bestPath
}

func RecommendedUserModsPath() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "7DaysToDie", "Mods")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "7DaysToDie", "Mods")
	default:
		proton := filepath.Join(
			home, ".local", "share", "Steam", "steamapps", "compatdata", "251570",
			"pfx", "drive_c", "users", "steamuser", "AppData", "Roaming", "7DaysToDie", "Mods",
		)
		if _, err := os.Stat(filepath.Dir(proton)); err == nil {
			return proton
		}
		return filepath.Join(home, ".local", "share", "7DaysToDie", "Mods")
	}
}

func IsSteamInstallModsPath(path string) bool {
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	return strings.HasSuffix(clean, "/steamapps/common/7 days to die/mods")
}

func MoveCommunityMods(mods []Mod, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	for _, mod := range mods {
		target := filepath.Join(destination, mod.Folder)
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("destination already contains %q", mod.Folder)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, mod := range mods {
		if err := os.Rename(mod.Path, filepath.Join(destination, mod.Folder)); err != nil {
			return fmt.Errorf("moving %q: %w", mod.Folder, err)
		}
	}
	return nil
}

func VersionFromText(text string) string {
	matches := versionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1]
	}
	match := taggedVersionPattern.FindStringSubmatch(text)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func VersionIdentifier(text string) string {
	value := strings.TrimSpace(text)
	if identifierPattern.MatchString(value) {
		return value
	}
	return VersionFromText(value)
}

func Scan(path string, sourceMap map[string]SourceIDs, legacySources map[string][]string) ([]Mod, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var mods []Mod
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := entry.Name()
		if isManagedGameMod(folder) {
			continue
		}
		mod := Mod{
			Folder: folder, Name: folder, Version: VersionFromText(folder),
			Path: filepath.Join(path, folder),
		}
		data, readErr := os.ReadFile(filepath.Join(mod.Path, "ModInfo.xml"))
		if readErr == nil {
			var info modInfo
			if xml.Unmarshal(data, &info) == nil {
				if info.Nested != nil {
					info = *info.Nested
				}
				if info.DisplayName.String() != "" {
					mod.Name = info.DisplayName.String()
				} else if info.Name.String() != "" {
					mod.Name = info.Name.String()
				}
				mod.Description = info.Description.String()
				mod.Author = info.Author.String()
				mod.Website = info.Website.String()
				if version := VersionIdentifier(info.Version.String()); version != "" {
					mod.Version = version
				}
			}
		}
		mod.Sources = sourceMap[mod.Name].URLs()
		if len(mod.Sources) == 0 {
			mod.Sources = supportedSources(legacySources[folder])
			if len(mod.Sources) > 0 {
				sourceMap[mod.Name] = SourceIDsFromURLs(mod.Sources)
				delete(legacySources, folder)
			}
		}
		if len(mod.Sources) == 0 && IsSupportedSource(mod.Website) {
			mod.Sources = []string{mod.Website}
		}
		mods = append(mods, mod)
	}
	sort.Slice(mods, func(i, j int) bool {
		return strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
	})
	return mods, nil
}

func isManagedGameMod(folder string) bool {
	switch strings.ToLower(strings.TrimSpace(folder)) {
	case "0_tfp_harmony", "0_tpf_harmony":
		return true
	default:
		return false
	}
}

func supportedSources(values []string) []string {
	var result []string
	for _, value := range values {
		if IsSupportedSource(value) {
			result = append(result, value)
		}
	}
	return result
}

func VersionParts(version string) []int {
	identifier := VersionIdentifier(version)
	if identifier == "" {
		return nil
	}
	parts := strings.Split(identifier, ".")
	result := make([]int, len(parts))
	for i, part := range parts {
		result[i], _ = strconv.Atoi(part)
	}
	return result
}

func CompareVersions(installed, latest string) UpdateStatus {
	left, right := VersionParts(installed), VersionParts(latest)
	if len(right) == 0 {
		return StatusUnknown
	}
	if len(left) == 0 {
		return StatusLatestFound
	}
	width := max(len(left), len(right))
	for i := 0; i < width; i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if r > l {
			return StatusUpdateAvailable
		}
		if l > r {
			return StatusAhead
		}
	}
	return StatusUpToDate
}

var ErrNoVersion = errors.New("could not find a version on page")
