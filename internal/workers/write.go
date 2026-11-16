// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "errors"

// ErrWriteOff refuses a write job when the write route is not enabled.
var ErrWriteOff = errors.New("write mode is off; the owner enables it with DOWNSHIFT_WORKERS=bypass")

// WriteLaunchers returns the write route: Claude in print mode with the bypass
// permission mode, Codex in exec mode with workspace-write and approval never.
// Both run through writeIsolated, so they start inside their own worktree only.
func WriteLaunchers() map[string]Launcher {
	return map[string]Launcher{
		"claude": writeIsolated{inner: cliLauncher{plan: claudeWrite}},
		"codex":  writeIsolated{inner: cliLauncher{plan: codexWrite}},
	}
}

// Routes sends read jobs to the read launcher and write jobs to the write one.
// A nil write launcher refuses every write job before anything starts.
func Routes(read, write Launcher) Launcher { return routes{read: read, write: write} }

type routes struct {
	read  Launcher
	write Launcher
}

func (r routes) Launch(spec Spec) (Process, error) {
	if !spec.Write {
		return r.read.Launch(spec)
	}
	if r.write == nil {
		return nil, ErrWriteOff
	}
	return r.write.Launch(spec)
}

// LaunchResume routes a resume the same way a launch is routed: a read worker's
// next turn through the read launcher, a write worker's through the write one.
// Without this forward, a queue built through Routes (every real MCP server,
// cmd/downshift workerLaunchers) is never a resumeLauncher, so canResume always
// refuses a settled worker and send never resumes a finished worker, even when
// its inner launcher could. A route whose launcher cannot resume (or a nil
// write launcher) is refused the same way its launch is, so the queue never
// starts a turn it cannot continue.
func (r routes) LaunchResume(spec Spec, sessionID string) (Process, error) {
	l := r.read
	if spec.Write {
		if r.write == nil {
			return nil, ErrWriteOff
		}
		l = r.write
	}
	rl, ok := l.(resumeLauncher)
	if !ok {
		return nil, errors.New(spec.Agent + " cannot resume a session")
	}
	return rl.LaunchResume(spec, sessionID)
}

// CleanupSession routes the session deletion the same way a launch is routed:
// a read worker's session through the read launcher, a write worker's through
// the write one. A route whose launcher cannot clean up (or is nil) is a no-op,
// so the queue's cleanup stays agnostic: it calls the route, the route calls the
// launcher of the worker's mode, and the per-CLI deletion lives inside that
// launcher (cleanup.go).
func (r routes) CleanupSession(spec Spec, sessionID string) error {
	l := r.read
	if spec.Write {
		l = r.write
	}
	sc, ok := l.(sessionCleaner)
	if !ok {
		return nil
	}
	return sc.CleanupSession(spec, sessionID)
}
