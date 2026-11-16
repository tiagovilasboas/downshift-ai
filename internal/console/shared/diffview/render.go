// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package diffview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
)

// View draws header, the visible diff lines and the key footer.
func (m Model) View() string {
	if len(m.files) == 0 {
		return frame.Header("No changes", "", m.width)
	}
	ls := m.lines()
	end := min(m.top+m.bodyHeight(), len(ls))
	first := min(m.top+1, len(ls))
	title := fmt.Sprintf("file %d/%d  %s", m.cur+1, len(m.files), m.files[m.cur].Path)
	pos := fmt.Sprintf("line %d–%d of %d", first, end, len(ls))
	head := frame.Header(title, pos, m.width)
	if !m.th.NoColor {
		head = m.th.Bold.Render(head)
	}
	rows := []string{head}
	for _, l := range ls[m.top:end] {
		rows = append(rows, m.style(l).MaxWidth(m.width).Render(l))
	}
	for len(rows) < m.bodyHeight()+1 {
		rows = append(rows, "")
	}
	rows = append(rows, frame.Footer([]frame.Key{
		{Key: "↑/↓", Help: "scroll"}, {Key: "pgup/pgdn", Help: "page"}, {Key: "g/G", Help: "top/bottom"},
		{Key: "tab", Help: "next file"}, {Key: "enter", Help: "done"},
	}, m.width))
	return strings.Join(rows, "\n")
}

func (m Model) style(l string) lipgloss.Style {
	switch {
	case strings.HasPrefix(l, "@@"):
		return m.th.Info
	case strings.HasPrefix(l, "+"):
		return m.th.OK
	case strings.HasPrefix(l, "-"):
		return m.th.Fail
	}
	return m.th.Muted
}
