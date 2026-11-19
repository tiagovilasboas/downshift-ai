// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// completionHelp documents the shell-completion generator command.
var completionHelp = map[string]string{
	"completion": `downshift completion — Print shell completion script for bash, zsh, fish or powershell

Usage:
  downshift completion <bash|zsh|fish|powershell>

Flags:
  (none)

Examples:
  downshift completion zsh > "${fpath[1]}/_downshift"
  source <(downshift completion bash)
  downshift completion fish > ~/.config/fish/completions/downshift.fish

Next: downshift help, to list available commands
`,
}
