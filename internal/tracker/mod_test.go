package tracker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionFromText(t *testing.T) {
	if got := VersionFromText("AGF-HUDPlus-1Main-v6.5.5"); got != "6.5.5" {
		t.Fatalf("got %q", got)
	}
	if got := VersionFromText("Example-v2"); got != "2" {
		t.Fatalf("single-number version: got %q", got)
	}
	if got := VersionIdentifier("2"); got != "2" {
		t.Fatalf("identifier: got %q", got)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := map[[2]string]UpdateStatus{
		{"6.5.4", "6.5.5"}: StatusUpdateAvailable,
		{"6.5.5", "6.5.5"}: StatusUpToDate,
		{"6.6", "6.5.5"}:   StatusAhead,
	}
	for versions, want := range cases {
		if got := CompareVersions(versions[0], versions[1]); got != want {
			t.Errorf("%v: got %q, want %q", versions, got, want)
		}
	}
}

func TestScanModInfo(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "Example-v1.0")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0" encoding="UTF-8" ?>
<xml><ModInfo><Name value="Example Mod"/><Description value="An example"/>
<Author value="Test Author"/><Version value="2"/><Website value="https://7daystodiemods.com/mods/example"/></ModInfo></xml>`
	if err := os.WriteFile(filepath.Join(folder, "ModInfo.xml"), []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	mods, err := Scan(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].Name != "Example Mod" || mods[0].Version != "2" {
		t.Fatalf("unexpected result: %#v", mods)
	}
	if mods[0].Author != "Test Author" || mods[0].Description != "An example" {
		t.Fatalf("metadata not parsed: %#v", mods[0])
	}
	if len(mods[0].Sources) != 1 || mods[0].Sources[0] != "https://7daystodiemods.com/mods/example" {
		t.Fatalf("website not used as source: %#v", mods[0].Sources)
	}
}

func TestScanFlatLegacyModInfo(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "Legacy")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `<xml><Name value="Legacy Mod"/><Version value="1.2"/></xml>`
	if err := os.WriteFile(filepath.Join(folder, "ModInfo.xml"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	mods, err := Scan(root, nil, nil)
	if err != nil || len(mods) != 1 || mods[0].Version != "1.2" {
		t.Fatalf("flat format failed: %#v, %v", mods, err)
	}
}

func TestScanHidesGameManagedHarmony(t *testing.T) {
	root := t.TempDir()
	for _, folder := range []string{"0_TFP_Harmony", "0_TPF_Harmony", "CommunityMod"} {
		if err := os.Mkdir(filepath.Join(root, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mods, err := Scan(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].Folder != "CommunityMod" {
		t.Fatalf("unexpected visible mods: %#v", mods)
	}
}

func TestIsSteamInstallModsPath(t *testing.T) {
	if !IsSteamInstallModsPath("/home/user/.local/share/Steam/steamapps/common/7 Days To Die/Mods") {
		t.Fatal("expected Steam Mods path to be detected")
	}
	if IsSteamInstallModsPath("/home/user/.local/share/7DaysToDie/Mods") {
		t.Fatal("per-user path must not be detected as a Steam install path")
	}
}

func TestMoveCommunityMods(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "game", "Mods")
	destination := filepath.Join(root, "user", "Mods")
	modPath := filepath.Join(source, "CommunityMod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	mods := []Mod{{Folder: "CommunityMod", Path: modPath}}
	if err := MoveCommunityMods(mods, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "CommunityMod")); err != nil {
		t.Fatalf("mod was not moved: %v", err)
	}
}

func TestBestExistingModsPathPrefersPopulatedUserFolder(t *testing.T) {
	root := t.TempDir()
	emptyUser := filepath.Join(root, "empty-user", "Mods")
	steam := filepath.Join(root, "steamapps", "common", "7 Days To Die", "Mods")
	populatedUser := filepath.Join(root, "AppData", "Roaming", "7DaysToDie", "Mods")
	for _, path := range []string{emptyUser, steam, populatedUser} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{steam, populatedUser} {
		mod := filepath.Join(path, "CommunityMod")
		if err := os.Mkdir(mod, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mod, "ModInfo.xml"), []byte("<xml/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := bestExistingModsPath([]string{emptyUser, populatedUser, steam})
	if got != populatedUser {
		t.Fatalf("got %q, want %q", got, populatedUser)
	}
}

func TestBestExistingModsPathFindsPopulatedSteamFolder(t *testing.T) {
	root := t.TempDir()
	user := filepath.Join(root, "user", "Mods")
	steam := filepath.Join(root, "steamapps", "common", "7 Days To Die", "Mods")
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(steam, "CommunityMod")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "ModInfo.xml"), []byte("<xml/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := bestExistingModsPath([]string{user, steam})
	if got != steam {
		t.Fatalf("got %q, want %q", got, steam)
	}
}

func TestExtractPageVersion(t *testing.T) {
	page := `<dt>Version</dt><dd>6.5.5</dd><p>Version 6.5.4</p>`
	if got := extractPageVersion(page); got != "6.5.5" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractSevenD2DVersion(t *testing.T) {
	body := []byte(`{
		"version": "2024.11.0",
		"mod_files": [{"mod_version": "1.5.1"}],
		"current_version": {"version": "1.5.1"}
	}`)
	if got := extractSevenD2DVersion(body); got != "1.5.1" {
		t.Fatalf("got %q", got)
	}
}

func TestSevenD2DAPIURL(t *testing.T) {
	got := sevenD2DAPIURL("https://7daystodiemods.com/mods/smart-doors/")
	want := "https://api.7daystodiemods.com/v1/mods/smart-doors"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLatestChoosesMostAdvancedProviderVersion(t *testing.T) {
	results := []CheckResult{
		{URL: "https://www.nexusmods.com/7daystodie/mods/123", Version: "1.2.3"},
		{URL: "https://7daystodiemods.com/mods/example/", Version: "1.4.2"},
	}
	latest := Latest(results)
	if latest.Version != "1.4.2" {
		t.Fatalf("got %q from %q", latest.Version, latest.URL)
	}
}

func TestSupportedSources(t *testing.T) {
	for _, source := range []string{
		"https://www.nexusmods.com/7daystodie/mods/870",
		"https://7daystodiemods.com/mods/example/",
	} {
		if !IsSupportedSource(source) {
			t.Errorf("expected supported source: %s", source)
		}
	}
	for _, source := range []string{
		"https://example.com/mod",
		"not a URL",
		"https://nexusmods.com.example.org/mod",
	} {
		if IsSupportedSource(source) {
			t.Errorf("expected unsupported source: %s", source)
		}
	}
}
