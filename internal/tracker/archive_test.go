package tracker

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExportAndImportModsArchive(t *testing.T) {
	sourceRoot := t.TempDir()
	modPath := filepath.Join(sourceRoot, "Example-v1")
	if err := os.MkdirAll(filepath.Join(modPath, "Config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "ModInfo.xml"), []byte("<xml/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "Config", "items.xml"), []byte("<items/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	nexusID := "123"
	mod := Mod{
		Folder:  "Example-v1",
		Name:    "Example Mod",
		Path:    modPath,
		Sources: SourceIDs{Nexus: &nexusID}.URLs(),
	}

	var data bytes.Buffer
	if err := ExportModsArchive(&data, []Mod{mod}); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	result, err := ImportModsArchive(
		bytes.NewReader(data.Bytes()),
		int64(data.Len()),
		destination,
		ModArchiveImportOptions{ReplaceExisting: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Imported) != 1 || result.Imported[0] != "Example-v1" {
		t.Fatalf("unexpected imported folders: %#v", result.Imported)
	}
	if result.Sources["Example Mod"].NexusValue() != nexusID {
		t.Fatalf("source mapping was not imported: %#v", result.Sources)
	}
	if _, err := os.Stat(filepath.Join(destination, "Example-v1", "Config", "items.xml")); err != nil {
		t.Fatalf("mod file was not imported: %v", err)
	}
}

func TestImportModsArchiveReplacesSelectedExistingFolders(t *testing.T) {
	sourceRoot := t.TempDir()
	modPath := filepath.Join(sourceRoot, "Existing")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := ExportModsArchive(&data, []Mod{{Folder: "Existing", Name: "Existing", Path: modPath}}); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	existing := filepath.Join(destination, "Existing")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ImportModsArchive(
		bytes.NewReader(data.Bytes()),
		int64(data.Len()),
		destination,
		ModArchiveImportOptions{
			SelectedFolders: map[string]bool{"Existing": true},
			ReplaceExisting: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Replaced) != 1 || result.Replaced[0] != "Existing" {
		t.Fatalf("unexpected replaced folders: %#v", result.Replaced)
	}
	if _, err := os.Stat(filepath.Join(existing, "new.txt")); err != nil {
		t.Fatalf("replacement file was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(existing, "keep.txt")); !os.IsNotExist(err) {
		t.Fatalf("old folder contents were merged instead of replaced")
	}
}

func TestImportModsArchiveOnlyInstallsSelectedModsAndSources(t *testing.T) {
	sourceRoot := t.TempDir()
	var mods []Mod
	for _, item := range []struct {
		folder string
		name   string
		id     string
	}{
		{folder: "Selected", name: "Selected Mod", id: "101"},
		{folder: "Ignored", name: "Ignored Mod", id: "202"},
	} {
		modPath := filepath.Join(sourceRoot, item.folder)
		if err := os.MkdirAll(modPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modPath, "ModInfo.xml"), []byte(item.name), 0o644); err != nil {
			t.Fatal(err)
		}
		id := item.id
		mods = append(mods, Mod{
			Folder:  item.folder,
			Name:    item.name,
			Path:    modPath,
			Sources: SourceIDs{Nexus: &id}.URLs(),
		})
	}
	var data bytes.Buffer
	if err := ExportModsArchive(&data, mods); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	result, err := ImportModsArchive(
		bytes.NewReader(data.Bytes()),
		int64(data.Len()),
		destination,
		ModArchiveImportOptions{
			SelectedFolders: map[string]bool{"Selected": true},
			ReplaceExisting: true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "Selected", "ModInfo.xml")); err != nil {
		t.Fatalf("selected mod was not imported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "Ignored")); !os.IsNotExist(err) {
		t.Fatalf("unselected mod was imported")
	}
	if _, ok := result.Sources["Selected Mod"]; !ok {
		t.Fatalf("selected source was not imported")
	}
	if _, ok := result.Sources["Ignored Mod"]; ok {
		t.Fatalf("unselected source was imported")
	}
}

func TestImportModsArchiveRejectsZipSlip(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	manifest, _ := writer.Create("manifest.json")
	_, _ = manifest.Write([]byte(`{"version":1,"sources":{}}`))
	unsafe, _ := writer.Create("../outside.txt")
	_, _ = unsafe.Write([]byte("unsafe"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := ImportModsArchive(
		bytes.NewReader(data.Bytes()),
		int64(data.Len()),
		t.TempDir(),
		ModArchiveImportOptions{ReplaceExisting: true},
	)
	if err == nil {
		t.Fatal("expected unsafe archive path to be rejected")
	}
}
