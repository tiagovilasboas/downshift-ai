// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
)

func (m Model) bodyHeight() int { return max(1, m.height-chrome) }

func (m Model) body() []string {
	if m.tab == 0 {
		return m.summaryLines()
	}
	return m.data.Tabs[m.tab-1].Lines
}

// View draws the headline, tab bar, body and key footer.
func (m Model) View() string {
	lines := m.body()
	end := min(m.top+m.bodyHeight(), len(lines))
	title, step := m.summaryHeader(), ""
	if m.tab > 0 {
		title = m.data.Tabs[m.tab-1].Title
		step = fmt.Sprintf("%d–%d of %d", min(m.top+1, len(lines)), end, len(lines))
	}
	head := frame.Header(title, step, m.width)
	if !m.th.NoColor {
		head = m.th.Bold.Render(head)
	}
	rows := []string{head, m.tabBar()}
	for _, l := range lines[m.top:end] {
		rows = append(rows, m.rowLine(l))
	}
	for len(rows) < m.bodyHeight()+2 {
		rows = append(rows, "")
	}
	keys := []frame.Key{{Key: "←/→", Help: "tab"}, {Key: "1-5", Help: "jump"}, {Key: "q", Help: "quit"}}
	if m.tab > 0 {
		keys = []frame.Key{{Key: "←/→", Help: "tab"}, {Key: "↑/↓", Help: "scroll"}, {Key: "g/G", Help: "top/bottom"}, {Key: "q", Help: "quit"}}
	}
	rows = append(rows, m.th.Muted.Render(frame.Footer(keys, m.width)))
	return strings.Join(rows, "\n")
}

// rowLine clips a body line. Summary lines are styled and already fit; detail
// lines are plain.
func (m Model) rowLine(l string) string {
	if m.tab == 0 {
		return l
	}
	return frame.Clip(l, m.width)
}

func (m Model) tabBar() string {
	names := []string{"Summary"}
	for _, t := range m.data.Tabs {
		names = append(names, t.Title)
	}
	for i, n := range names {
		if i == m.tab {
			names[i] = "[" + n + "]"
		}
	}
	line := frame.Clip(strings.Join(names, " "), m.width)
	if m.th.NoColor {
		return line
	}
	return m.th.Accent.Render(line)
}
