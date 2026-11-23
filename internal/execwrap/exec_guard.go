// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// TimeoutEnv sets an optional execution timeout for wrapped commands (for
// example "30s" or "2m"). Empty, invalid, or non-positive values fail open
// with no timeout.
const TimeoutEnv = "DOWNSHIFT_EXEC_TIMEOUT"

// ExitTimeout matches GNU timeout(1): the command was terminated because it
// exceeded its execution timeout.
const ExitTimeout = 124

const timeoutGrace = 200 * time.Millisecond

var nonTTYPromptEnv = [][2]string{
	{"GIT_TERMINAL_PROMPT", "0"},
	{"GH_PROMPT_DISABLED", "1"},
}

// nonTTYEnv builds the child environment from base, always appending
// DepthEnv=1. When stdin is not an interactive terminal, it also injects
// non-interactive prompt-disabling variables unless the caller already set them.
func nonTTYEnv(base []string, interactive bool) []string {
	env := make([]string, len(base), len(base)+1+len(nonTTYPromptEnv))
	copy(env, base)
	env = append(env, DepthEnv+"=1")
	if interactive {
		return env
	}
	for _, kv := range nonTTYPromptEnv {
		if !hasEnvKey(base, kv[0]) {
			env = append(env, kv[0]+"="+kv[1])
		}
	}
	return env
}

func hasEnvKey(env []string, key string) bool {
	for _, entry := range env {
		k, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if runtime.GOOS == "windows" {
			if strings.EqualFold(k, key) {
				return true
			}
			continue
		}
		if k == key {
			return true
		}
	}
	return false
}

// parseTimeout parses a duration string, failing open (0) on empty, invalid,
// or non-positive values so normal commands are never broken by bad config.
func parseTimeout(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

func resolveTimeout(explicit time.Duration) time.Duration {
	if explicit > 0 {
		return explicit
	}
	return parseTimeout(os.Getenv(TimeoutEnv))
}

// waitWithTimeout waits for cmd to exit, or terminates its process group when
// timeout > 0 expires before Wait returns.
func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) (error, bool) {
	if timeout <= 0 {
		return cmd.Wait(), false
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-waitCh:
		return err, false
	case <-timer.C:
		signalChild(cmd, syscall.SIGTERM)
		grace := time.NewTimer(timeoutGrace)
		defer grace.Stop()
		select {
		case err := <-waitCh:
			return err, true
		case <-grace.C:
			signalChild(cmd, syscall.SIGKILL)
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return <-waitCh, true
		}
	}
}
