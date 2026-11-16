// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package diffview is a scrollable unified-diff viewer, one file at a time.
// It is a Bubble Tea model with no program inside.
package diffview

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// File is one file's unified-diff lines (+, -, space, @@).
type File struct {
	Path  string
	Lines []string
}

const (
	defaultWidth  = 80
	defaultHeight = 24
	chrome        = 2 // header and footer lines
)

// Model is the viewer state.
type Model struct {
	th            theme.Theme
	files         []File
	cur, top      int
	width, height int
	done          bool
}

// New builds a viewer over files.
func New(th theme.Theme, files []File) Model {
	return Model{th: th, files: files, width: defaultWidth, height: defaultHeight}
}

// Done reports whether the user closed the viewer.
func (m Model) Done() bool { return m.done }

// Init has nothing to start.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) bodyHeight() int { return max(1, m.height-chrome) }

func (m Model) lines() []string {
	if len(m.files) == 0 {
		return nil
	}
	return m.files[m.cur].Lines
}

// maxTop is the largest scroll offset: the last line sits on the last row.
func (m Model) maxTop() int { return max(0, len(m.lines())-m.bodyHeight()) }

func (m *Model) scroll(to int) { m.top = min(max(to, 0), m.maxTop()) }

func (m *Model) switchFile(delta int) {
	if n := len(m.files); n > 0 {
		m.cur = ((m.cur+delta)%n + n) % n
		m.top = 0
	}
}

// Update handles keys and window size.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll(m.top)
	case tea.KeyPressMsg:
		page := m.bodyHeight()
		switch msg.String() {
		case "up", "k":
			m.scroll(m.top - 1)
		case "down", "j":
			m.scroll(m.top + 1)
		case "pgup":
			m.scroll(m.top - page)
		case "pgdown":
			m.scroll(m.top + page)
		case "g", "home":
			m.scroll(0)
		case "G", "end":
			m.scroll(m.maxTop())
		case "tab", "]":
			m.switchFile(1)
		case "shift+tab", "[":
			m.switchFile(-1)
		case "enter", "q":
			m.done = true
		}
	}
	return m, nil
}
