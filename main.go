package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"7d2d-mod-tracker/internal/tracker"
)

var exampleSources = []string{
	"https://7daystodiemods.com/mods/agf-v3-hudplus-1main/",
	"https://www.nexusmods.com/7daystodie/mods/870",
}

type rowState struct {
	latest   string
	status   tracker.UpdateStatus
	failures []tracker.CheckResult
	checked  time.Time
}

type modSelectionItem struct {
	key   string
	label string
}

type uiState struct {
	app               fyne.App
	window            fyne.Window
	config            tracker.Config
	mods              []tracker.Mod
	results           map[string]rowState
	folder            *widget.Entry
	cards             *fyne.Container
	status            *widget.Label
	checkButton       *widget.Button
	logger            *log.Logger
	logDir            string
	migrationPrompted bool
	checking          bool
}

func main() {
	logger, logDir, logFile, logErr := newAppLogger()
	if logFile != nil {
		defer logFile.Close()
	}
	if logErr != nil {
		logger.Printf("could not initialize file logging: %v", logErr)
	}
	logger.Printf("starting 7D2D Mod Update Tracker")

	a := app.NewWithID("com.7d2d.modtracker")
	w := a.NewWindow("7D2D Mod Update Tracker")
	state := &uiState{
		app: a, window: w, config: tracker.LoadConfig(), results: make(map[string]rowState),
		logger: logger, logDir: logDir,
	}
	state.build()
	w.Resize(fyne.NewSize(1000, 620))
	w.ShowAndRun()
}

func (s *uiState) build() {
	s.folder = widget.NewEntry()
	if s.config.Folder == "" {
		s.config.Folder = tracker.ExistingDefaultPath()
	}
	s.folder.SetText(s.config.Folder)

	scan := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), s.scan)
	s.checkButton = widget.NewButton("Check updates", s.checkAll)
	openMods := widget.NewButtonWithIcon("Open Mods", theme.FolderOpenIcon(), s.openModsFolder)
	settings := widget.NewButtonWithIcon("", theme.SettingsIcon(), s.showSettings)
	settings.Importance = widget.LowImportance

	s.cards = container.New(newResponsiveGridLayout(482, 217))
	cardScroll := container.NewVScroll(s.cards)

	s.status = widget.NewLabel("Choose a Mods folder to begin.")

	top := container.NewBorder(
		nil,
		nil,
		widget.NewLabelWithStyle("Installed mods", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(scan, s.checkButton, openMods, settings),
	)
	s.window.SetContent(container.NewBorder(top, s.status, nil, nil, cardScroll))
	if info, err := os.Stat(s.config.Folder); err == nil && info.IsDir() {
		s.scan()
	} else {
		s.logger.Printf("configured Mods folder is unavailable: %s", s.config.Folder)
		s.status.SetText("Choose your 7 Days To Die Mods folder to begin.")
	}
}

func (s *uiState) showSettings() {
	settingsWindow := s.app.NewWindow("Settings")
	folderEntry := widget.NewEntry()
	folderEntry.SetText(s.folder.Text)
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(s.config.NexusAPIKey)
	apiKeysURL, _ := url.Parse("https://www.nexusmods.com/settings/api-keys")
	apiKeysLink := widget.NewHyperlink("Generate or manage a Nexus API key ↗", apiKeysURL)

	browse := widget.NewButton("Browse…", func() {
		selectFolder(settingsWindow, func(path string, err error) {
			if err != nil {
				dialog.ShowError(err, settingsWindow)
			} else if path != "" {
				folderEntry.SetText(path)
			}
		})
	})
	detect := widget.NewButton("Detect", func() {
		detected := tracker.DetectModsPath()
		folderEntry.SetText(detected)
		s.logger.Printf("detected Mods folder: %s", detected)
	})
	folderRow := container.NewBorder(nil, nil, nil, container.NewHBox(detect, browse), folderEntry)
	sectionSpacing := canvas.NewRectangle(color.Transparent)
	sectionSpacing.SetMinSize(fyne.NewSize(1, 16))
	settingsFields := container.NewVBox(
		widget.NewLabelWithStyle("Mods folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		folderRow,
		sectionSpacing,
		widget.NewLabelWithStyle("Nexus API key", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		apiKeyEntry,
		apiKeysLink,
	)
	save := widget.NewButton("Save", func() {
		s.folder.SetText(strings.TrimSpace(folderEntry.Text))
		s.config.Folder = s.folder.Text
		s.config.NexusAPIKey = strings.TrimSpace(apiKeyEntry.Text)
		if err := tracker.SaveConfig(s.config); err != nil {
			s.logger.Printf("could not save settings: %v", err)
			dialog.ShowError(err, settingsWindow)
			return
		}
		s.logger.Printf("settings saved; Mods folder: %s", s.config.Folder)
		settingsWindow.Close()
		s.scan()
	})
	cancel := widget.NewButton("Cancel", settingsWindow.Close)
	openLogs := widget.NewButtonWithIcon("Open logs", theme.FolderOpenIcon(), s.openLogs)
	exportMods := widget.NewButtonWithIcon("Export Mods…", theme.DownloadIcon(), func() {
		s.exportMods(settingsWindow)
	})
	importMods := widget.NewButtonWithIcon("Import Mods…", theme.UploadIcon(), func() {
		s.importMods(settingsWindow, strings.TrimSpace(folderEntry.Text))
	})
	settingsWindow.SetContent(container.NewBorder(
		nil,
		container.NewBorder(
			nil,
			nil,
			container.NewHBox(openLogs, exportMods, importMods),
			container.NewHBox(cancel, save),
		),
		nil,
		nil,
		settingsFields,
	))
	width := s.window.Canvas().Size().Width * 0.7
	if width < 600 {
		width = 600
	}
	settingsWindow.Resize(fyne.NewSize(width, 300))
	settingsWindow.CenterOnScreen()
	settingsWindow.Show()
}

func (s *uiState) exportMods(parent fyne.Window) {
	if len(s.mods) == 0 {
		dialog.ShowInformation("Nothing to export", "No community mods are currently loaded.", parent)
		return
	}
	mods := append([]tracker.Mod(nil), s.mods...)
	sort.SliceStable(mods, func(i, j int) bool {
		return strings.ToLower(mods[i].Name) < strings.ToLower(mods[j].Name)
	})
	items := make([]modSelectionItem, 0, len(mods))
	modsByFolder := make(map[string]tracker.Mod, len(mods))
	for _, mod := range mods {
		label := mod.Name
		if mod.Version != "" {
			label += " — " + mod.Version
		}
		items = append(items, modSelectionItem{key: mod.Folder, label: label})
		modsByFolder[mod.Folder] = mod
	}
	s.showModSelection(
		"Select mods to export",
		"Choose the mod folders and source mappings to include.",
		"Export selected…",
		items,
		func(selected map[string]bool) {
			var selectedMods []tracker.Mod
			for _, item := range items {
				if selected[item.key] {
					selectedMods = append(selectedMods, modsByFolder[item.key])
				}
			}
			s.saveModsArchive(parent, selectedMods)
		},
	)
}

func (s *uiState) saveModsArchive(parent fyne.Window, mods []tracker.Mod) {
	selectArchiveSave(parent, "7d2d-mods"+tracker.ModArchiveExtension, func(archivePath string, err error) {
		if err != nil {
			dialog.ShowError(err, parent)
			return
		}
		if archivePath == "" {
			return
		}
		s.status.SetText(fmt.Sprintf("Exporting %d mod(s)…", len(mods)))
		go func() {
			writer, createErr := os.Create(archivePath)
			if createErr != nil {
				fyne.Do(func() {
					s.logger.Printf("could not create mod archive %s: %v", archivePath, createErr)
					s.status.SetText("Mod export failed.")
					dialog.ShowError(createErr, parent)
				})
				return
			}
			exportErr := tracker.ExportModsArchive(writer, mods)
			closeErr := writer.Close()
			if exportErr == nil {
				exportErr = closeErr
			}
			fyne.Do(func() {
				if exportErr != nil {
					s.logger.Printf("mod export failed: %v", exportErr)
					s.status.SetText("Mod export failed.")
					dialog.ShowError(exportErr, parent)
					return
				}
				s.logger.Printf("exported %d mods to %s", len(mods), archivePath)
				s.status.SetText(fmt.Sprintf("Exported %d mod(s).", len(mods)))
				dialog.ShowInformation(
					"Mods exported",
					fmt.Sprintf("Exported %d mod(s) with their update sources.\n\nThe Nexus API key was not included.", len(mods)),
					parent,
				)
			})
		}()
	})
}

func (s *uiState) importMods(parent fyne.Window, destination string) {
	if destination == "" {
		dialog.ShowInformation("Choose a Mods folder", "Set the destination Mods folder before importing.", parent)
		return
	}
	selectArchiveOpen(parent, func(archivePath string, err error) {
		if err != nil {
			dialog.ShowError(err, parent)
			return
		}
		if archivePath == "" {
			return
		}
		s.status.SetText("Reading mod archive…")
		go func() {
			file, openErr := os.Open(archivePath)
			if openErr != nil {
				fyne.Do(func() {
					s.status.SetText("Mod import failed.")
					dialog.ShowError(openErr, parent)
				})
				return
			}
			info, statErr := file.Stat()
			var manifest tracker.ModArchiveManifest
			if statErr == nil {
				manifest, statErr = tracker.InspectModsArchive(file, info.Size())
			}
			closeErr := file.Close()
			if statErr == nil {
				statErr = closeErr
			}
			fyne.Do(func() {
				if statErr != nil {
					s.logger.Printf("could not inspect mod archive %s: %v", archivePath, statErr)
					s.status.SetText("Could not read mod archive.")
					dialog.ShowError(statErr, parent)
					return
				}
				s.status.SetText(fmt.Sprintf("Archive contains %d mod(s).", len(manifest.Mods)))
				items := make([]modSelectionItem, 0, len(manifest.Mods))
				for _, mod := range manifest.Mods {
					label := mod.Name
					if mod.Version != "" {
						label += " — " + mod.Version
					}
					label += "  [" + mod.Folder + "]"
					items = append(items, modSelectionItem{key: mod.Folder, label: label})
				}
				s.showModSelection(
					"Select mods to import",
					"Selected mods are installed into the configured Mods folder. "+
						"If a selected folder already exists, it is replaced as a whole; files are never merged.",
					"Import selected",
					items,
					func(selected map[string]bool) {
						s.installModsArchive(parent, archivePath, destination, selected)
					},
				)
			})
		}()
	})
}

func (s *uiState) installModsArchive(
	parent fyne.Window,
	archivePath string,
	destination string,
	selected map[string]bool,
) {
	s.status.SetText("Importing selected mods…")
	go func() {
		file, importErr := os.Open(archivePath)
		if importErr != nil {
			fyne.Do(func() {
				s.status.SetText("Mod import failed.")
				dialog.ShowError(importErr, parent)
			})
			return
		}
		info, statErr := file.Stat()
		var result tracker.ModArchiveImportResult
		if statErr == nil {
			result, statErr = tracker.ImportModsArchive(
				file,
				info.Size(),
				destination,
				tracker.ModArchiveImportOptions{
					SelectedFolders: selected,
					ReplaceExisting: true,
				},
			)
		}
		closeErr := file.Close()
		if statErr == nil {
			statErr = closeErr
		}
		fyne.Do(func() {
			if statErr != nil {
				s.logger.Printf("mod import failed from %s: %v", archivePath, statErr)
				s.status.SetText("Mod import failed.")
				dialog.ShowError(statErr, parent)
				return
			}
			for name, sources := range result.Sources {
				s.config.Sources[name] = sources
			}
			s.folder.SetText(destination)
			s.config.Folder = destination
			if saveErr := tracker.SaveConfig(s.config); saveErr != nil {
				s.logger.Printf("could not save imported sources: %v", saveErr)
				s.status.SetText("Mods imported, but sources could not be saved.")
				dialog.ShowError(saveErr, parent)
				return
			}
			s.logger.Printf("imported %d new and replaced %d mods from %s",
				len(result.Imported), len(result.Replaced), archivePath)
			s.scan()
			message := fmt.Sprintf(
				"Imported %d new mod folder(s), replaced %d existing folder(s), and merged %d source mapping(s).",
				len(result.Imported),
				len(result.Replaced),
				len(result.Sources),
			)
			dialog.ShowInformation("Mods imported", message, parent)
		})
	}()
}

func (s *uiState) showModSelection(
	title string,
	description string,
	confirmLabel string,
	items []modSelectionItem,
	onConfirm func(map[string]bool),
) {
	selectionWindow := s.app.NewWindow(title)
	checks := make(map[string]*widget.Check, len(items))
	list := container.NewVBox()
	for _, item := range items {
		check := widget.NewCheck(item.label, nil)
		check.SetChecked(true)
		checks[item.key] = check
		list.Add(check)
	}
	descriptionLabel := widget.NewLabel(description)
	descriptionLabel.Wrapping = fyne.TextWrapWord
	selectAll := widget.NewButton("Select all", func() {
		for _, check := range checks {
			check.SetChecked(true)
		}
	})
	selectNone := widget.NewButton("Select none", func() {
		for _, check := range checks {
			check.SetChecked(false)
		}
	})
	cancel := widget.NewButton("Cancel", selectionWindow.Close)
	confirm := widget.NewButton(confirmLabel, func() {
		selected := make(map[string]bool)
		for key, check := range checks {
			if check.Checked {
				selected[key] = true
			}
		}
		if len(selected) == 0 {
			dialog.ShowInformation("Nothing selected", "Select at least one mod to continue.", selectionWindow)
			return
		}
		selectionWindow.Close()
		onConfirm(selected)
	})
	selectionWindow.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, container.NewHBox(selectAll, selectNone), descriptionLabel),
		container.NewHBox(cancel, confirm),
		nil,
		nil,
		container.NewVScroll(list),
	))
	selectionWindow.Resize(fyne.NewSize(680, 500))
	selectionWindow.CenterOnScreen()
	selectionWindow.Show()
}

func (s *uiState) scan() {
	mods, err := tracker.Scan(s.folder.Text, s.config.Sources, s.config.LegacySources)
	if err != nil {
		s.logger.Printf("scan failed for %s: %v", s.folder.Text, err)
		dialog.ShowError(err, s.window)
		return
	}
	s.mods = mods
	s.results = make(map[string]rowState)
	s.config.Folder = s.folder.Text
	if err := tracker.SaveConfig(s.config); err != nil {
		s.logger.Printf("could not save configuration after scan: %v", err)
		dialog.ShowError(err, s.window)
	}
	s.refreshCards()
	s.status.SetText(fmt.Sprintf("Found %d mod folder(s).", len(mods)))
	s.logger.Printf("scan completed: %s (%d visible mods)", s.folder.Text, len(mods))
	s.offerUserModsMigration()
	if hasCheckableMods(mods) {
		s.checkAll()
	}
}

func hasCheckableMods(mods []tracker.Mod) bool {
	for _, mod := range mods {
		if len(mod.Sources) > 0 {
			return true
		}
	}
	return false
}

func (s *uiState) offerUserModsMigration() {
	if s.migrationPrompted || len(s.mods) == 0 || !tracker.IsSteamInstallModsPath(s.folder.Text) {
		return
	}
	destination := tracker.RecommendedUserModsPath()
	if filepath.Clean(destination) == filepath.Clean(s.folder.Text) {
		return
	}
	s.migrationPrompted = true
	message := fmt.Sprintf(
		"These community mods are installed beside the game files. "+
			"Keeping them in the per-user Mods folder avoids Steam updates or file verification affecting them.\n\n"+
			"Move %d community mod(s) to:\n%s\n\n"+
			"0_TFP_Harmony is managed by the game and will remain here.",
		len(s.mods), destination,
	)
	dialog.ShowConfirm("Use the per-user Mods folder?", message, func(move bool) {
		if !move {
			s.logger.Printf("community mod migration declined")
			return
		}
		if err := tracker.MoveCommunityMods(s.mods, destination); err != nil {
			s.logger.Printf("community mod migration failed: %v", err)
			dialog.ShowError(err, s.window)
			return
		}
		s.logger.Printf("moved %d community mods to %s", len(s.mods), destination)
		s.folder.SetText(destination)
		s.config.Folder = destination
		if err := tracker.SaveConfig(s.config); err != nil {
			s.logger.Printf("could not save migrated Mods folder: %v", err)
			dialog.ShowError(err, s.window)
			return
		}
		s.scan()
		dialog.ShowInformation(
			"Mods moved",
			"Community mods were moved successfully. 0_TFP_Harmony remains in the Steam game folder.",
			s.window,
		)
	}, s.window)
}

func (s *uiState) refreshCards() {
	s.cards.RemoveAll()
	ordered := make([]*tracker.Mod, 0, len(s.mods))
	for i := range s.mods {
		ordered = append(ordered, &s.mods[i])
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		leftUpdate := s.results[ordered[i].Folder].status == tracker.StatusUpdateAvailable
		rightUpdate := s.results[ordered[j].Folder].status == tracker.StatusUpdateAvailable
		if leftUpdate != rightUpdate {
			return leftUpdate
		}
		return strings.ToLower(ordered[i].Name) < strings.ToLower(ordered[j].Name)
	})
	for _, mod := range ordered {
		s.cards.Add(s.buildModCard(mod))
	}
	s.cards.Refresh()
}

func (s *uiState) buildModCard(mod *tracker.Mod) fyne.CanvasObject {
	result, checked := s.results[mod.Folder]
	latest, status := "—", tracker.StatusNeedsSource
	if len(mod.Sources) > 0 {
		status = tracker.StatusNotChecked
	}
	if checked {
		if result.latest != "" {
			latest = result.latest
		}
		status = result.status
	}
	installed := mod.Version
	if installed == "" {
		installed = "—"
	}

	title := widget.NewLabelWithStyle(mod.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	versionInfo := container.NewHBox(
		widget.NewLabel("Installed: "+installed),
		widget.NewSeparator(),
		widget.NewLabel("Latest: "+latest),
	)

	ids := tracker.SourceIDsFromURLs(mod.Sources)
	var actions []fyne.CanvasObject
	if ids.Nexus != nil {
		nexusURL := "https://www.nexusmods.com/7daystodie/mods/" + ids.NexusValue()
		actions = append(actions, widget.NewButtonWithIcon(
			"Nexus", theme.NavigateNextIcon(),
			func() { s.openURL(nexusURL) },
		))
	}
	if ids.SevenD2D != nil {
		sevenURL := "https://7daystodiemods.com/mods/" + ids.SevenD2DValue() + "/"
		actions = append(actions, widget.NewButtonWithIcon(
			"7D2D Mods", theme.NavigateNextIcon(),
			func() { s.openURL(sevenURL) },
		))
	}
	editButton := widget.NewButtonWithIcon(
		"Edit sources", theme.DocumentCreateIcon(),
		func() { s.setSources(mod) },
	)
	actions = append(actions, editButton)
	var failureAction fyne.CanvasObject
	if len(result.failures) > 0 {
		failureLabel := tracker.SourceName(result.failures[0].URL) + " failed — details"
		if len(result.failures) > 1 {
			failureLabel = fmt.Sprintf("%d sources failed — details", len(result.failures))
		}
		failureAction = widget.NewButtonWithIcon(
			failureLabel,
			theme.WarningIcon(),
			func() { s.showCheckFailures(mod, result) },
		)
	}

	updateAvailable := status == tracker.StatusUpdateAvailable
	statusText := status.String()
	if updateAvailable {
		statusText = "UPDATE AVAILABLE"
	}
	statusLabel := widget.NewLabelWithStyle(
		statusText,
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: updateAvailable},
	)
	contentItems := []fyne.CanvasObject{
		container.NewBorder(nil, nil, title, statusLabel),
		versionInfo,
	}
	if failureAction != nil {
		contentItems = append(contentItems, failureAction)
	}
	contentItems = append(contentItems, container.NewHBox(actions...))
	content := container.NewVBox(contentItems...)

	borderColor := color.NRGBA{R: 0x78, G: 0x78, B: 0x78, A: 0x80}
	borderWidth := float32(1)
	if updateAvailable {
		borderColor = color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff}
		borderWidth = 3
	}
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = borderColor
	border.StrokeWidth = borderWidth
	border.CornerRadius = 8
	card := container.NewStack(border, container.NewPadded(content))
	return container.New(layout.NewCustomPaddedLayout(16, 16, 16, 16), card)
}

func (s *uiState) showCheckFailures(mod *tracker.Mod, result rowState) {
	var details strings.Builder
	fmt.Fprintf(&details, "Mod: %s\nInstalled version: %s\n", mod.Name, mod.Version)
	if !result.checked.IsZero() {
		fmt.Fprintf(&details, "Checked: %s\n", result.checked.Format(time.RFC3339))
	}
	metadata := s.app.Metadata()
	appName := metadata.Name
	if appName == "" {
		appName = "7D2D Mod Update Tracker"
	}
	appVersion := metadata.Version
	if appVersion == "" {
		appVersion = "development"
	}
	fmt.Fprintf(&details, "App: %s %s (build %d)\nRuntime: %s, %s/%s\n",
		appName,
		appVersion,
		metadata.Build,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	for i, failure := range result.failures {
		fmt.Fprintf(&details, "\nFailure %d\n%s\nSource: %s\n",
			i+1,
			tracker.CheckFailureDetail(failure, s.config.NexusAPIKey != ""),
			failure.URL,
		)
		if failure.Endpoint != "" && failure.Endpoint != failure.URL {
			fmt.Fprintf(&details, "Request endpoint: %s\n", failure.Endpoint)
		}
	}

	detailText := details.String()
	detailsEntry := widget.NewMultiLineEntry()
	detailsEntry.Wrapping = fyne.TextWrapWord
	detailsEntry.SetText(detailText)

	detailWindow := s.app.NewWindow("Update check details — " + mod.Name)
	copyDetails := widget.NewButtonWithIcon("Copy details", theme.ContentCopyIcon(), func() {
		s.app.Clipboard().SetContent(detailsEntry.Text)
	})
	openLogs := widget.NewButtonWithIcon("Open logs", theme.FolderOpenIcon(), s.openLogs)
	settings := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() {
		detailWindow.Close()
		s.showSettings()
	})
	closeButton := widget.NewButton("Close", detailWindow.Close)
	detailWindow.SetContent(container.NewBorder(
		nil,
		container.NewHBox(copyDetails, openLogs, settings, closeButton),
		nil,
		nil,
		detailsEntry,
	))
	detailWindow.Resize(fyne.NewSize(720, 430))
	detailWindow.CenterOnScreen()
	detailWindow.Show()
}

func (s *uiState) setSources(mod *tracker.Mod) {
	initialSources := mod.Sources
	lower := strings.ToLower(mod.Folder)
	if len(initialSources) == 0 && strings.Contains(lower, "agf") && strings.Contains(lower, "hudplus") {
		initialSources = exampleSources
	}
	ids := tracker.SourceIDsFromURLs(initialSources)
	nexusEntry := widget.NewEntry()
	nexusEntry.SetPlaceHolder("e.g. 870")
	nexusEntry.SetText(ids.NexusValue())
	sevenD2DEntry := widget.NewEntry()
	sevenD2DEntry.SetPlaceHolder("e.g. agf-v3-hudplus-1main")
	sevenD2DEntry.SetText(ids.SevenD2DValue())

	sourceWindow := s.app.NewWindow("Update sources — " + mod.Name)
	save := widget.NewButton("Save", func() {
		ids, err := tracker.SourceIDsFromInputs(nexusEntry.Text, sevenD2DEntry.Text)
		if err != nil {
			dialog.ShowError(err, sourceWindow)
			return
		}
		s.config.Sources[mod.Name] = ids
		delete(s.config.LegacySources, mod.Folder)
		mod.Sources = ids.URLs()
		if err := tracker.SaveConfig(s.config); err != nil {
			s.logger.Printf("could not save sources for %q: %v", mod.Name, err)
			dialog.ShowError(err, sourceWindow)
			return
		}
		s.logger.Printf("sources updated for %q: nexus=%t, 7d2dmods=%t",
			mod.Name, ids.Nexus != nil, ids.SevenD2D != nil)
		s.refreshCards()
		sourceWindow.Close()
		if len(mod.Sources) > 0 {
			s.checkAll()
		}
	})
	cancel := widget.NewButton("Cancel", sourceWindow.Close)
	instructions := widget.NewLabel("Enter a short identifier, or paste the full mod-page URL.")
	form := widget.NewForm(
		widget.NewFormItem("Nexus Mod ID", nexusEntry),
		widget.NewFormItem("7D2D Mods slug", sevenD2DEntry),
	)
	sourceWindow.SetContent(container.NewBorder(
		instructions,
		container.NewHBox(cancel, save),
		nil,
		nil,
		form,
	))

	mainSize := s.window.Canvas().Size()
	width := mainSize.Width * 0.75
	if width < 560 {
		width = 560
	}
	height := mainSize.Height * 0.65
	if height < 320 {
		height = 320
	}
	sourceWindow.Resize(fyne.NewSize(width, height))
	sourceWindow.CenterOnScreen()
	sourceWindow.Show()
}

func (s *uiState) openURL(value string) {
	parsed, err := url.Parse(value)
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}
	_ = s.app.OpenURL(parsed)
}

func (s *uiState) openLogs() {
	if s.logDir == "" {
		dialog.ShowError(fmt.Errorf("the log directory is unavailable"), s.window)
		return
	}
	s.openDirectory(s.logDir, "log")
}

func (s *uiState) openModsFolder() {
	path := strings.TrimSpace(s.folder.Text)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		dialog.ShowError(fmt.Errorf("Mods folder is unavailable: %w", err), s.window)
		return
	}
	s.openDirectory(path, "Mods")
}

func (s *uiState) openDirectory(path, label string) {
	filePath := filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	fileURL := &url.URL{Scheme: "file", Path: filePath}
	if err := s.app.OpenURL(fileURL); err != nil {
		s.logger.Printf("could not open %s directory %s: %v", label, path, err)
		dialog.ShowError(err, s.window)
	}
}

func (s *uiState) checkAll() {
	if s.checking {
		return
	}
	var targets []tracker.Mod
	for _, mod := range s.mods {
		if len(mod.Sources) > 0 {
			targets = append(targets, mod)
		}
	}
	if len(targets) == 0 {
		dialog.ShowInformation("No sources", "Select a mod and use “Set sources…” first.", s.window)
		return
	}
	s.checking = true
	s.checkButton.Disable()
	s.status.SetText(fmt.Sprintf("Checking %d mod(s)…", len(targets)))
	s.logger.Printf("checking updates for %d mods", len(targets))
	go func() {
		checker := tracker.NewChecker(s.config.NexusAPIKey)
		for _, mod := range targets {
			var checks []tracker.CheckResult
			for _, source := range mod.Sources {
				checks = append(checks, checker.Check(context.Background(), source))
			}
			latest := tracker.Latest(checks)
			result := rowState{status: tracker.StatusCheckFailed, checked: time.Now()}
			for _, check := range checks {
				if check.Err != nil {
					result.failures = append(result.failures, check)
				}
			}
			for _, check := range result.failures {
				s.logger.Printf("update source failed for %q: source=%s endpoint=%s error=%T: %v",
					mod.Name, check.URL, check.Endpoint, check.Err, check.Err)
			}
			if latest.Err == nil {
				result.latest = latest.Version
				result.status = tracker.CompareVersions(mod.Version, latest.Version)
				s.logger.Printf("update check %q: installed=%q latest=%q status=%s",
					mod.Name, mod.Version, latest.Version, result.status)
			}
			fyne.Do(func() {
				s.results[mod.Folder] = result
				s.refreshCards()
			})
		}
		s.logger.Printf("update checks completed for %d mods", len(targets))
		fyne.Do(func() {
			s.checking = false
			s.checkButton.Enable()
			s.status.SetText(fmt.Sprintf("Finished checking %d mod(s).", len(targets)))
		})
	}()
}
