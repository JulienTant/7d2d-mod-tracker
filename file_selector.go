package main

import (
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"

	"7d2d-mod-tracker/internal/tracker"
)

var modArchiveFilter = zenity.FileFilter{
	Name:     "7D2D mod libraries",
	Patterns: []string{"*" + tracker.ModArchiveExtension},
	CaseFold: true,
}

func selectFolder(_ fyne.Window, callback func(string, error)) {
	runNativeSelector(callback, func() (string, error) {
		return zenity.SelectFile(
			zenity.Title("Choose Mods folder"),
			zenity.Directory(),
		)
	})
}

func selectArchiveSave(_ fyne.Window, defaultName string, callback func(string, error)) {
	runNativeSelector(func(path string, err error) {
		if err == nil && path != "" && !strings.EqualFold(filepath.Ext(path), tracker.ModArchiveExtension) {
			path += tracker.ModArchiveExtension
		}
		callback(path, err)
	}, func() (string, error) {
		return zenity.SelectFileSave(
			zenity.Title("Export Mods"),
			zenity.Filename(defaultName),
			modArchiveFilter,
			zenity.ConfirmOverwrite(),
		)
	})
}

func selectArchiveOpen(_ fyne.Window, callback func(string, error)) {
	runNativeSelector(callback, func() (string, error) {
		return zenity.SelectFile(
			zenity.Title("Import Mods"),
			modArchiveFilter,
		)
	})
}

func runNativeSelector(callback func(string, error), selectPath func() (string, error)) {
	go func() {
		path, err := selectPath()
		if errors.Is(err, zenity.ErrCanceled) {
			path, err = "", nil
		}
		fyne.Do(func() {
			callback(path, err)
		})
	}()
}
