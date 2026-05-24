// Copyright (C) 2026 pi-apps-go contributors
// This file is part of Pi-Apps Go - a modern, cross-architecture/cross-platform, and modular Pi-Apps implementation in Go.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Module: tui.go
// Description: Command-line interface for settings application
// SPDX-License-Identifier: GPL-3.0-or-later

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RunSettingsTUI runs the experimental Charm stack terminal UI (Bubble Tea, Lip Gloss, bubbles list) for Pi-Apps settings.
// It reads the same data/settings files as the GTK window. Invoke from the CLI as: settings tui
//
// If PI_APPS_DIR is unset, it reports via ErrorNoExit and returns nil (matching other settings entry points).
func RunSettingsTUI() error {
	directory := GetPiAppsDir()
	if directory == "" {
		ErrorNoExit(T("PI_APPS_DIR environment variable not set"))
		return nil
	}

	settings, err := loadSettingsState(directory)
	if err != nil {
		return err
	}

	m, err := newSettingsTUIModel(directory, settings)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

type actionListItem struct {
	title, description, actionID string
}

func (a actionListItem) FilterValue() string { return a.title }
func (a actionListItem) Title() string       { return a.title }
func (a actionListItem) Description() string {
	return a.description
}

type settingsTUIModel struct {
	directory    string
	settings     map[string]*Setting
	settingNames []string
	width        int
	height       int
	tab          int
	list         list.Model
	fieldPtrs    map[string]*string
	cursor       int
	editing      bool
	confirmReset bool
	lastErr      string
}

// isYesNoSetting reports whether the setting is exactly Yes/No (any order).
func isYesNoSetting(s *Setting) bool {
	if s == nil || len(s.Values) != 2 {
		return false
	}
	hasY, hasN := false, false
	for _, v := range s.Values {
		if v == "Yes" {
			hasY = true
		}
		if v == "No" {
			hasN = true
		}
	}
	return hasY && hasN
}

func newSettingsTUIModel(directory string, settings map[string]*Setting) (*settingsTUIModel, error) {
	fieldPtrs := make(map[string]*string)
	var names []string
	for _, name := range sortedSettingNames(settings) {
		setting := settings[name]
		if setting == nil || len(setting.Values) == 0 {
			continue
		}
		p := new(string)
		*p = setting.Current
		fieldPtrs[name] = p
		names = append(names, name)
	}

	if len(fieldPtrs) == 0 {
		return nil, fmt.Errorf("no settings fields to display")
	}

	items := []list.Item{
		actionListItem{
			title:       T("Categories"),
			description: T("Does an App belong in Editors instead of Tools? This lets you move it."),
			actionID:    "category_editor",
		},
		actionListItem{
			title:       T("Log files"),
			description: T("View past installation logs. Useful for debugging, or to see what you installed yesterday."),
			actionID:    "log_viewer",
		},
		actionListItem{
			title:       T("Multi-Install"),
			description: T("Install multiple apps at the same time."),
			actionID:    "multi_install",
		},
		actionListItem{
			title:       T("New App"),
			description: T("Make your own app! It's pretty easy if you follow the instructions."),
			actionID:    "create_app",
		},
		actionListItem{
			title:       T("Import App"),
			description: T("Did someone else make an app but it's not on Pi-Apps yet? Import it here."),
			actionID:    "import_app",
		},
		actionListItem{
			title:       T("Multi-Uninstall"),
			description: T("Uninstall multiple apps at the same time."),
			actionID:    "multi_uninstall",
		},
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3)
	lst := list.New(items, delegate, 0, 0)
	lst.SetShowTitle(false)
	lst.SetFilteringEnabled(false)
	lst.SetShowPagination(false)

	return &settingsTUIModel{
		directory:    directory,
		settings:     settings,
		settingNames: names,
		tab:          0,
		list:         lst,
		fieldPtrs:    fieldPtrs,
	}, nil
}

func (m *settingsTUIModel) appListTheme() string {
	if p, ok := m.fieldPtrs["App List Style"]; ok {
		return *p
	}
	return "default"
}

func (m *settingsTUIModel) applyListLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	const headerFooter = 6
	contentH := m.height - headerFooter
	if contentH < 8 {
		contentH = 8
	}
	w := m.width
	if w < 20 {
		w = 20
	}
	m.list.SetWidth(w)
	m.list.SetHeight(contentH)
}

func (m *settingsTUIModel) Init() tea.Cmd {
	return nil
}

func (m *settingsTUIModel) applyResetDefaults() {
	for name, setting := range m.settings {
		if len(setting.Values) == 0 {
			continue
		}
		def := setting.Values[0]
		setting.Current = def
		if ptr, ok := m.fieldPtrs[name]; ok {
			*ptr = def
		}
		sp := filepath.Join(m.directory, "data", "settings", name)
		if err := os.WriteFile(sp, []byte(def), 0644); err != nil {
			m.lastErr = fmt.Sprintf("%s: %v", name, err)
		}
	}
}

// valueIndex returns the index of canonical value in setting.Values, or 0.
func valueIndex(s *Setting, canonical string) int {
	for i, v := range s.Values {
		if v == canonical {
			return i
		}
	}
	return 0
}

// nudgeCanonical moves the current value by delta in the cyclic Values list.
func (m *settingsTUIModel) nudgeCanonical(delta int) {
	if m.cursor < 0 || m.cursor >= len(m.settingNames) {
		return
	}
	name := m.settingNames[m.cursor]
	s := m.settings[name]
	p := m.fieldPtrs[name]
	if len(s.Values) == 0 {
		return
	}
	i := valueIndex(s, *p)
	i += delta
	for i < 0 {
		i += len(s.Values)
	}
	i %= len(s.Values)
	*p = s.Values[i]
}

func (m *settingsTUIModel) toggleYesNo() {
	name := m.settingNames[m.cursor]
	p := m.fieldPtrs[name]
	if *p == "Yes" {
		*p = "No"
	} else {
		*p = "Yes"
	}
}

// switchSettingsTab switches between Settings (0) and Actions (1). Many terminals do not send
// distinct Ctrl+1/Ctrl+2 sequences (they overlap other Ctrl keys), so we also accept F1/F2 and alt+1/2.
func (m *settingsTUIModel) switchSettingsTab(which int) {
	m.tab = which
	if which == 1 {
		m.editing = false
	}
}

func (m *settingsTUIModel) matchesTabSwitch(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyF1:
		m.switchSettingsTab(0)
		return true
	case tea.KeyF2:
		m.switchSettingsTab(1)
		return true
	}
	switch msg.String() {
	case "alt+1":
		m.switchSettingsTab(0)
		return true
	case "alt+2":
		m.switchSettingsTab(1)
		return true
	}
	return false
}

//nolint:gocyclo // TUI message routing is inherently branchy.
func (m *settingsTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyListLayout()
		var lc tea.Cmd
		m.list, lc = m.list.Update(msg)
		return m, lc

	case tea.KeyMsg:
		if m.confirmReset {
			switch strings.ToLower(msg.String()) {
			case "y":
				m.applyResetDefaults()
				m.confirmReset = false
				m.lastErr = ""
				m.editing = false
			case "n", "esc":
				m.confirmReset = false
			}
			return m, nil
		}

		// Esc: leave edit mode first; otherwise quit (same as GTK cancel).
		if msg.Type == tea.KeyEscape || msg.String() == "esc" {
			if m.editing {
				m.editing = false
				return m, nil
			}
			return m, tea.Quit
		}

		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			return m, tea.Quit
		case "ctrl+s":
			vals := make(map[string]string)
			for k, p := range m.fieldPtrs {
				vals[k] = *p
			}
			if err := writeCanonicalSettings(m.directory, vals); err != nil {
				m.lastErr = err.Error()
				return m, nil
			}
			for k, v := range vals {
				if s, ok := m.settings[k]; ok {
					s.Current = v
				}
			}
			m.lastErr = ""
			return m, tea.Quit
		case "ctrl+r":
			m.confirmReset = true
			m.lastErr = ""
			return m, nil
		}

		if m.matchesTabSwitch(msg) {
			return m, nil
		}

		if m.tab == 1 {
			if msg.String() == "enter" {
				if it, ok := m.list.SelectedItem().(actionListItem); ok {
					runSettingsAction(m.directory, it.actionID, m.appListTheme())
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		// Settings tab: browse vs edit.
		if m.cursor < 0 || m.cursor >= len(m.settingNames) {
			return m, nil
		}
		curName := m.settingNames[m.cursor]
		s := m.settings[curName]
		if m.editing {
			switch msg.Type {
			case tea.KeyLeft:
				m.nudgeCanonical(-1)
				return m, nil
			case tea.KeyRight:
				m.nudgeCanonical(1)
				return m, nil
			case tea.KeyEnter:
				m.editing = false
				return m, nil
			}
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == ' ' && s != nil && isYesNoSetting(s) {
				m.toggleYesNo()
				return m, nil
			}
			return m, nil
		}

		// Browse: move between settings with arrow keys.
		switch msg.Type {
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.settingNames)-1 {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			m.editing = true
			return m, nil
		}
	}

	if m.tab == 1 {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *settingsTUIModel) maxLabelWidth() int {
	maxW := 0
	for _, name := range m.settingNames {
		if m.settings[name] == nil || len(m.settings[name].Values) == 0 {
			continue
		}
		w := lipgloss.Width(TranslateSettingName(name))
		if w > maxW {
			maxW = w
		}
	}
	if maxW < 12 {
		maxW = 12
	}
	return maxW
}

func (m *settingsTUIModel) renderSettingsPane() string {
	lw := m.maxLabelWidth()
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	editHint := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	var lines []string
	for i, name := range m.settingNames {
		setting := m.settings[name]
		if setting == nil || len(setting.Values) == 0 {
			continue
		}
		p := m.fieldPtrs[name]
		label := TranslateSettingName(name)
		rowStyle := normalStyle
		if i == m.cursor {
			rowStyle = selStyle
		}

		labelCell := lipgloss.NewStyle().Width(lw).Align(lipgloss.Right).Render(label)
		var valueCell string
		if isYesNoSetting(setting) {
			box := "☐"
			if *p == "Yes" {
				box = "☑"
			}
			valueCell = box + " " + TranslateSettingValue(*p)
		} else {
			valueCell = TranslateSettingValue(*p)
		}
		line := rowStyle.Render(labelCell + "  " + valueCell)
		if i == m.cursor && m.editing {
			h := "←" + T("/") + "→  " + T("Enter")
			if isYesNoSetting(setting) {
				h = "←" + T("/") + "→  " + T("Space") + "  " + T("Enter")
			}
			line += "  " + editHint.Render(h)
		} else if i == m.cursor && !m.editing {
			line += "  " + editHint.Render(T("Enter")+" "+T("edit"))
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m *settingsTUIModel) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render(T("Pi-Apps Settings"))

	tab1Style := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tab2Style := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	if m.tab == 0 {
		tab1Style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Underline(true)
	} else {
		tab2Style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Underline(true)
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Top,
		tab1Style.Render(" "+T("Settings")+" "),
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" │ "),
		tab2Style.Render(" "+T("Actions")+" "),
	)

	var body string
	if m.tab == 0 {
		body = m.renderSettingsPane()
	} else {
		body = m.list.View()
	}

	help := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		T("ctrl+s") + " " + T("Save") + "  " +
			T("esc") + "/" + T("ctrl+q") + " " + T("Cancel") + "  " +
			T("ctrl+r") + " " + T("Reset") + "  " +
			"F1/F2 " + T("Tab") + "  " +
			T("alt+1") + "/" + T("alt+2"),
	)

	var confirm string
	if m.confirmReset {
		confirm = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(
			T("Are you sure you want to reset all settings to their defaults?") + " (y/n)",
		)
	}

	errLine := ""
	if m.lastErr != "" {
		errLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.lastErr)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabs,
		"",
		lipgloss.NewStyle().MaxWidth(m.width).Render(body),
		"",
		confirm,
		errLine,
		help,
	)
}
