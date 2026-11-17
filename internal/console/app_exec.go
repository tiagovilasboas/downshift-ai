// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/output"
	"github.com/tiagovilasboas/downshift-ai/internal/console/palette"
)

// cancelRun stops the running command; its result then arrives as a ranMsg.
func (a app) cancelRun() { a.cancel() }

func (a app) start(c palette.ChosenMsg) (tea.Model, tea.Cmd) {
	if c.Spec.Terminal {
		a.mode = modeScreen
		return a, tea.ExecProcess(a.run.Interactive(c.Args), func(error) tea.Msg { return terminalDoneMsg{} })
	}
	return a.capture(c.Args)
}

func (a app) capture(args []string) (tea.Model, tea.Cmd) {
	a.mode, a.out = modeOutput, output.Running(args)
	a.ctx, a.cancel = context.WithCancel(context.Background())
	return a, a.captureCmd(a.ctx, args)
}

func (a app) captureCmd(ctx context.Context, args []string) tea.Cmd {
	run := a.run
	return func() tea.Msg { return ranMsg{res: run.Capture(ctx, args)} }
}
