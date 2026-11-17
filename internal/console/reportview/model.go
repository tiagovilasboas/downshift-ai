// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	chrome        = 3 // header, tab bar and footer lines
)

// Model is the screen state. Tab 0 is the summary; the rest are Data.Tabs.
type Model struct {
	th            theme.Theme
	data          Data
	tab, top      int
	width, height int
	done          bool
}

// New builds the screen over data.
func New(th theme.Theme, data Data) Model {
	return Model{th: th, data: data, width: defaultWidth, height: defaultHeight}
}

// Done reports whether the user closed the screen.
func (m Model) Done() bool { return m.done }

// Init has nothing to start.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) tabCount() int { return 1 + len(m.data.Tabs) }

func (m Model) maxTop() int { return max(0, len(m.body())-m.bodyHeight()) }

func (m *Model) scroll(to int) { m.top = min(max(to, 0), m.maxTop()) }

func (m *Model) goTab(i int) {
	n := m.tabCount()
	m.tab = ((i % n) + n) % n
	m.top = 0
}

// Update handles keys and window size.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll(m.top)
	case tea.KeyPressMsg:
		m.key(msg.String())
	}
	return m, nil
}

func (m *Model) key(k string) {
	page := m.bodyHeight()
	switch k {
	case "right", "l", "tab":
		m.goTab(m.tab + 1)
	case "left", "h", "shift+tab":
		m.goTab(m.tab - 1)
	case "1", "2", "3", "4", "5":
		if i := int(k[0] - '1'); i < m.tabCount() {
			m.goTab(i)
		}
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
	case "q", "esc":
		m.done = true
	}
}
