// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"path/filepath"
	"strings"
)

// ProducerBashScript labels an inline or heredoc shell script (bash -c, sh -).
const ProducerBashScript = "bash-script"

// pureFilters read a stream and never originate the bytes worth compressing:
// in `a | b` the label names a, not b.
var pureFilters = map[string]bool{
	"head": true, "tail": true, "grep": true, "sort": true, "uniq": true, "wc": true,
	"cut": true, "awk": true, "sed": true, "less": true, "cat": true, "tee": true,
	"jq": true, "tr": true, "rg": true,
}

// shellInterpreters label as ProducerBashScript.
var shellInterpreters = map[string]bool{"bash": true, "sh": true, "zsh": true}

// ProducerProgram returns a bounded code for the program that most likely
// produced a shell command's output. Rules, in order:
//
//  1. A heredoc body is dropped (everything from the first unquoted `<<`).
//  2. `cmd /c X`, `powershell -Command X` and `pwsh -c X` are unwrapped to X.
//  3. The command is split at &&, ||, ; and newlines; the first command whose
//     program is not trivial setup (cd, export, source ...) is used. Leading
//     NAME=value assignments, env, time, sudo and similar wrappers are skipped.
//  4. In a pipeline the output comes from the first stage that is not a pure
//     filter (head, tail, grep, sort, uniq, wc, cut, awk, sed, less, cat, tee,
//     jq, tr, rg); a pipeline of filters only is named by its first stage.
//  5. python, python3 and node are named as such; bash, sh and zsh are
//     "bash-script"; a ./script or *.sh is "script".
//
// A program outside the CommandHead vocabulary is "unknown". The result is
// one of that vocabulary, "script", "unknown" or "bash-script"; never a path,
// argument or other command text.
func ProducerProgram(command string) string {
	command = stripHeredoc(command)
	for range 3 { // cmd /c "powershell -c ..." nesting stays bounded
		inner, ok := unwrapWinShell(command)
		if !ok {
			break
		}
		command = stripHeredoc(inner)
	}
	first := ""
	for _, cmd := range splitCommands(command) {
		code, base := pipelineProducer(cmd)
		if base == "" {
			continue
		}
		if first == "" {
			first = code
		}
		if !trivialHeads[base] {
			return code
		}
	}
	if first == "" {
		return HeadUnknown
	}
	return first
}

// stripHeredoc cuts command at the first `<<` outside quotes.
func stripHeredoc(command string) string {
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '<' && strings.HasPrefix(command[i:], "<<"):
			return command[:i]
		}
	}
	return command
}

// unwrapWinShell returns the script of `cmd /c`, `powershell -Command` or
// `pwsh -c`, without surrounding quotes.
func unwrapWinShell(command string) (string, bool) {
	f := strings.Fields(command)
	if len(f) < 3 {
		return "", false
	}
	prog := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(f[0], `\`, "/"))), ".exe")
	flag := strings.ToLower(f[1])
	switch prog {
	case "cmd":
		if flag != "/c" && flag != "/k" {
			return "", false
		}
	case "powershell", "pwsh":
		if flag != "-command" && flag != "-c" {
			return "", false
		}
	default:
		return "", false
	}
	rest := strings.TrimSpace(command[strings.Index(command, f[1])+len(f[1]):])
	return strings.Trim(rest, `"'`), true
}

// splitCommands splits at &&, ||, ; and newlines outside quotes. A single `|`
// stays: it belongs to a pipeline.
func splitCommands(command string) []string {
	var out []string
	var quote byte
	start := 0
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ';' || c == '\n':
			out = append(out, command[start:i])
			start = i + 1
		case (c == '&' || c == '|') && i+1 < len(command) && command[i+1] == c:
			out = append(out, command[start:i])
			start = i + 2
			i++
		}
	}
	return append(out, command[start:])
}

// pipelineProducer names the producing stage of one pipeline and returns its
// code and program basename ("" base when no stage has a program).
func pipelineProducer(cmd string) (code, base string) {
	var stages []string
	var quote byte
	start := 0
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '|':
			stages = append(stages, cmd[start:i])
			start = i + 1
		}
	}
	stages = append(stages, cmd[start:])
	for _, st := range stages {
		c, b := segmentHead(strings.Fields(st))
		if b == "" {
			continue
		}
		if shellInterpreters[b] {
			c = ProducerBashScript
		}
		if base == "" {
			code, base = c, b
		}
		if !pureFilters[b] {
			return c, b
		}
	}
	return code, base
}
