package tracker

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ModArchiveExtension = ".7d2dml"
	modArchiveVersion   = 1
	maxArchiveFiles     = 100_000
	maxArchiveBytes     = uint64(20 << 30)
)

type ModArchiveManifest struct {
	Version    int                  `json:"version"`
	ExportedAt time.Time            `json:"exported_at"`
	Mods       []ArchivedMod        `json:"mods"`
	Sources    map[string]SourceIDs `json:"sources"`
}

type ArchivedMod struct {
	Folder  string `json:"folder"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ModArchiveImportOptions struct {
	SelectedFolders map[string]bool
	ReplaceExisting bool
}

type ModArchiveImportResult struct {
	Imported []string
	Replaced []string
	Skipped  []string
	Sources  map[string]SourceIDs
}

func ExportModsArchive(writer io.Writer, mods []Mod) error {
	archive := zip.NewWriter(writer)
	manifest := ModArchiveManifest{
		Version:    modArchiveVersion,
		ExportedAt: time.Now().UTC(),
		Sources:    make(map[string]SourceIDs),
	}
	for _, mod := range mods {
		manifest.Mods = append(manifest.Mods, ArchivedMod{
			Folder:  filepath.Base(filepath.Clean(mod.Folder)),
			Name:    mod.Name,
			Version: mod.Version,
		})
		if ids := SourceIDsFromURLs(mod.Sources); ids.Nexus != nil || ids.SevenD2D != nil {
			manifest.Sources[mod.Name] = ids
		}
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = archive.Close()
		return err
	}
	manifestFile, err := archive.Create("manifest.json")
	if err != nil {
		_ = archive.Close()
		return err
	}
	if _, err := manifestFile.Write(append(manifestData, '\n')); err != nil {
		_ = archive.Close()
		return err
	}

	seenFolders := make(map[string]bool)
	for _, mod := range mods {
		folder := filepath.Base(filepath.Clean(mod.Folder))
		if folder == "." || folder == string(filepath.Separator) || seenFolders[folder] {
			_ = archive.Close()
			return fmt.Errorf("invalid or duplicate mod folder %q", mod.Folder)
		}
		seenFolders[folder] = true
		if err := addModFolderToArchive(archive, folder, mod.Path); err != nil {
			_ = archive.Close()
			return fmt.Errorf("export %s: %w", mod.Name, err)
		}
	}
	return archive.Close()
}

func addModFolderToArchive(archive *zip.Writer, folder, root string) error {
	return filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported: %s", filePath)
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		name := path.Join("Mods", folder)
		if relative != "." {
			name = path.Join(name, filepath.ToSlash(relative))
		}
		if entry.IsDir() {
			_, err = archive.CreateHeader(&zip.FileHeader{Name: name + "/", Method: zip.Store})
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", filePath)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		target, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(target, source)
		closeErr := source.Close()
		return errors.Join(copyErr, closeErr)
	})
}

func InspectModsArchive(reader io.ReaderAt, size int64) (ModArchiveManifest, error) {
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return ModArchiveManifest{}, fmt.Errorf("open mod archive: %w", err)
	}
	return inspectModsArchive(archive)
}

func ImportModsArchive(
	reader io.ReaderAt,
	size int64,
	destination string,
	options ModArchiveImportOptions,
) (ModArchiveImportResult, error) {
	result := ModArchiveImportResult{Sources: make(map[string]SourceIDs)}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return result, fmt.Errorf("open mod archive: %w", err)
	}
	manifest, err := inspectModsArchive(archive)
	if err != nil {
		return result, err
	}

	selected := options.SelectedFolders
	if selected == nil {
		selected = make(map[string]bool, len(manifest.Mods))
		for _, mod := range manifest.Mods {
			selected[mod.Folder] = true
		}
	}
	selectedMods := make(map[string]ArchivedMod)
	for _, mod := range manifest.Mods {
		if selected[mod.Folder] {
			selectedMods[mod.Folder] = mod
			if ids, ok := manifest.Sources[mod.Name]; ok {
				result.Sources[mod.Name] = ids
			}
		}
	}
	if len(selectedMods) == 0 {
		return result, fmt.Errorf("no mods were selected")
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return result, err
	}
	staging, err := os.MkdirTemp(destination, ".7d2d-mod-import-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(staging)
	newRoot := filepath.Join(staging, "new")
	backupRoot := filepath.Join(staging, "backup")
	if err := os.MkdirAll(newRoot, 0o755); err != nil {
		return result, err
	}

	for _, file := range archive.File {
		clean, _ := validateArchivePath(file.Name)
		if clean == "manifest.json" {
			continue
		}
		parts := strings.Split(clean, "/")
		if !selected[parts[1]] {
			continue
		}
		relative := path.Join(parts[1:]...)
		target := filepath.Join(newRoot, filepath.FromSlash(relative))
		if err := extractArchiveFile(file, target); err != nil {
			return result, err
		}
	}

	var folders []string
	for folder := range selectedMods {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	var installed []string
	backedUp := make(map[string]bool)
	rollback := func() {
		for _, folder := range installed {
			_ = os.RemoveAll(filepath.Join(destination, folder))
		}
		for folder := range backedUp {
			_ = os.Rename(filepath.Join(backupRoot, folder), filepath.Join(destination, folder))
		}
	}
	for _, folder := range folders {
		target := filepath.Join(destination, folder)
		if _, statErr := os.Stat(target); statErr == nil {
			if !options.ReplaceExisting {
				result.Skipped = append(result.Skipped, folder)
				continue
			}
			if err := os.MkdirAll(backupRoot, 0o755); err != nil {
				rollback()
				return result, err
			}
			if err := os.Rename(target, filepath.Join(backupRoot, folder)); err != nil {
				rollback()
				return result, fmt.Errorf("back up existing mod folder %s: %w", folder, err)
			}
			backedUp[folder] = true
		} else if !errors.Is(statErr, os.ErrNotExist) {
			rollback()
			return result, statErr
		}
		if err := os.Rename(filepath.Join(newRoot, folder), target); err != nil {
			rollback()
			return result, fmt.Errorf("install mod folder %s: %w", folder, err)
		}
		installed = append(installed, folder)
		if backedUp[folder] {
			result.Replaced = append(result.Replaced, folder)
		} else {
			result.Imported = append(result.Imported, folder)
		}
	}
	sort.Strings(result.Imported)
	sort.Strings(result.Replaced)
	sort.Strings(result.Skipped)
	return result, nil
}

func inspectModsArchive(archive *zip.Reader) (ModArchiveManifest, error) {
	if len(archive.File) > maxArchiveFiles {
		return ModArchiveManifest{}, fmt.Errorf("archive contains too many files")
	}

	var totalBytes uint64
	var manifestFile *zip.File
	folders := make(map[string]bool)
	for _, file := range archive.File {
		totalBytes += file.UncompressedSize64
		if totalBytes > maxArchiveBytes {
			return ModArchiveManifest{}, fmt.Errorf("archive expands beyond the %d GiB safety limit", maxArchiveBytes>>30)
		}
		clean, err := validateArchivePath(file.Name)
		if err != nil {
			return ModArchiveManifest{}, err
		}
		if clean == "manifest.json" {
			manifestFile = file
			continue
		}
		parts := strings.Split(clean, "/")
		if len(parts) < 2 || parts[0] != "Mods" || parts[1] == "" {
			return ModArchiveManifest{}, fmt.Errorf("unexpected archive entry %q", file.Name)
		}
		folders[parts[1]] = true
		if file.Mode()&os.ModeSymlink != 0 {
			return ModArchiveManifest{}, fmt.Errorf("archive contains unsupported symbolic link %q", file.Name)
		}
	}
	if manifestFile == nil {
		return ModArchiveManifest{}, fmt.Errorf("archive is missing manifest.json")
	}
	manifest, err := readArchiveManifest(manifestFile)
	if err != nil {
		return ModArchiveManifest{}, err
	}
	if len(manifest.Mods) == 0 {
		for folder := range folders {
			manifest.Mods = append(manifest.Mods, ArchivedMod{Folder: folder, Name: folder})
		}
	}
	manifestFolders := make(map[string]bool)
	for _, mod := range manifest.Mods {
		if mod.Folder == "" || strings.ContainsAny(mod.Folder, `/\`) || mod.Folder == "." || mod.Folder == ".." {
			return ModArchiveManifest{}, fmt.Errorf("manifest contains invalid mod folder %q", mod.Folder)
		}
		if strings.TrimSpace(mod.Name) == "" {
			return ModArchiveManifest{}, fmt.Errorf("manifest contains an empty mod name")
		}
		if manifestFolders[mod.Folder] {
			return ModArchiveManifest{}, fmt.Errorf("manifest contains duplicate mod folder %q", mod.Folder)
		}
		if !folders[mod.Folder] {
			return ModArchiveManifest{}, fmt.Errorf("manifest references missing mod folder %q", mod.Folder)
		}
		manifestFolders[mod.Folder] = true
	}
	for folder := range folders {
		if !manifestFolders[folder] {
			return ModArchiveManifest{}, fmt.Errorf("archive contains unlisted mod folder %q", folder)
		}
	}
	sort.Slice(manifest.Mods, func(i, j int) bool {
		return strings.ToLower(manifest.Mods[i].Name) < strings.ToLower(manifest.Mods[j].Name)
	})
	return manifest, nil
}

func validateArchivePath(name string) (string, error) {
	if name == "" || strings.Contains(name, `\`) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	clean := path.Clean(strings.TrimSuffix(name, "/"))
	if clean == "." || clean != strings.TrimSuffix(name, "/") || path.IsAbs(clean) ||
		clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return clean, nil
}

func readArchiveManifest(file *zip.File) (ModArchiveManifest, error) {
	var manifest ModArchiveManifest
	if file.UncompressedSize64 > 4<<20 {
		return manifest, fmt.Errorf("manifest.json is too large")
	}
	reader, err := file.Open()
	if err != nil {
		return manifest, err
	}
	defer reader.Close()
	if err := json.NewDecoder(io.LimitReader(reader, 4<<20)).Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("read manifest.json: %w", err)
	}
	if manifest.Version != modArchiveVersion {
		return manifest, fmt.Errorf("unsupported mod archive version %d", manifest.Version)
	}
	if manifest.Sources == nil {
		manifest.Sources = make(map[string]SourceIDs)
	}
	for name, ids := range manifest.Sources {
		if strings.TrimSpace(name) == "" {
			return manifest, fmt.Errorf("manifest contains an empty mod name")
		}
		if ids.Nexus != nil && !nexusIDPattern.MatchString(*ids.Nexus) {
			return manifest, fmt.Errorf("manifest contains an invalid Nexus ID for %q", name)
		}
		if ids.SevenD2D != nil && !slugPattern.MatchString(*ids.SevenD2D) {
			return manifest, fmt.Errorf("manifest contains an invalid 7D2D Mods slug for %q", name)
		}
	}
	return manifest, nil
}

func extractArchiveFile(file *zip.File, target string) error {
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	mode := file.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	mode &= 0o755
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(targetFile, source)
	closeErr := targetFile.Close()
	return errors.Join(copyErr, closeErr)
}
