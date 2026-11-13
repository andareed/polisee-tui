package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-lib"
)

type exportResult struct {
	path string
	err  error
}

func (m *Model) canExport() bool {
	n := m.doc.Nodes[m.selected]
	return n != nil && n.Kind == "policy" && !m.loading && !m.exporting
}

func (m *Model) openExport() tea.Cmd {
	if !m.canExport() {
		m.message = "Select a main policy to export"
		return nil
	}
	n := m.doc.Nodes[m.selected]
	text, err := m.doc.Engine.ExportPolicy(n.ID)
	if err != nil {
		m.message = "Export failed: " + err.Error()
		m.failed = true
		return nil
	}
	m.exportXML = text
	m.overlay = exportOverlay
	m.overlayOffset = 0
	m.browseEditing = false
	m.suggestion = -1
	m.fileOptions = nil
	m.dialog = newInput("Save policy XML as")
	m.dialog.SetValue(polisee.PolicyFilename(n.Name))
	m.dialog.CursorEnd()
	m.dialog.SetWidth(max(1, m.width-10))
	m.failed = false
	m.message = "Export " + n.Name + " with all subrules"
	return tea.Batch(m.dialog.Focus(), m.pathCommand())
}

func (m *Model) saveExport(path string, replace bool) tea.Cmd {
	if !strings.EqualFold(filepath.Ext(path), ".xml") {
		path += ".xml"
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		m.message = err.Error()
		m.failed = true
		return nil
	}
	target, targetErr := os.Stat(absolute)
	if m.path != "" {
		source, _ := os.Stat(m.path)
		loadedPath, _ := filepath.Abs(m.path)
		if absolute == loadedPath || (source != nil && target != nil && os.SameFile(source, target)) {
			m.message = "Choose a different file to preserve the loaded policy export"
			m.failed = true
			return nil
		}
	}
	if targetErr == nil && !replace {
		m.exportPath = absolute
		m.overlay = overwriteOverlay
		m.dialog.Blur()
		return nil
	}
	text := m.exportXML
	m.closeOverlay()
	m.message = "Exporting policy…"
	m.exporting = true
	return func() tea.Msg {
		// Exclusive creation prevents replacing a file that appeared after the
		// dialog checked. Replacement is only used after explicit confirmation.
		if !replace {
			f, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return exportResult{absolute, err}
			}
			_, err = f.WriteString(text)
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.Remove(absolute)
			}
			return exportResult{absolute, err}
		}
		f, err := os.CreateTemp(filepath.Dir(absolute), ".polisee-export-*")
		if err != nil {
			return exportResult{absolute, err}
		}
		defer os.Remove(f.Name())
		_, err = f.WriteString(text)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(f.Name(), absolute)
		}
		return exportResult{absolute, err}
	}
}

func (m *Model) exportDone(result exportResult) {
	m.exporting = false
	m.failed = result.err != nil
	if result.err != nil {
		m.message = "Export failed: " + result.err.Error()
	} else {
		m.message = fmt.Sprintf("Exported policy to %s", result.path)
	}
}
