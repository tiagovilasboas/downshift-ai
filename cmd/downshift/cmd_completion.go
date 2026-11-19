// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/clispec"
)

type completionModel struct {
	Top         []string
	Subs        map[string][]string
	Flags       map[string][]string
	GlobalFlags []string
}

var completionWriters = map[string]func(io.Writer, completionModel){
	"bash":       writeBashCompletion,
	"zsh":        writeZshCompletion,
	"fish":       writeFishCompletion,
	"powershell": writePowerShellCompletion,
	"pwsh":       writePowerShellCompletion,
}

func runCompletion(args []string, out, errw io.Writer) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(errw, "usage: downshift completion <bash|zsh|fish|powershell>")
		return 2
	}
	shell := strings.ToLower(strings.TrimSpace(args[0]))
	writer, ok := completionWriters[shell]
	if !ok {
		fmt.Fprintf(errw, "unknown shell %q (expected bash, zsh, fish, or powershell)\n", args[0])
		return 2
	}
	writer(out, buildCompletionModel())
	return 0
}

func buildCompletionModel() completionModel {
	m := completionModel{
		Subs:        map[string][]string{"completion": {"bash", "fish", "powershell", "zsh"}},
		Flags:       map[string][]string{},
		GlobalFlags: []string{"--help", "--plain", "--version"},
	}
	for _, c := range clispec.Commands() {
		if c.Hidden || c.Legacy || c.AliasOf != "" {
			continue
		}
		var flags []string
		for _, fl := range c.Flags {
			if !fl.Deprecated {
				flags = append(flags, fl.Name)
			}
		}
		sort.Strings(flags)
		if parent, sub, hasSub := strings.Cut(c.Name, " "); hasSub {
			m.Subs[parent] = append(m.Subs[parent], sub)
			m.Flags[c.Name] = flags
			continue
		}
		m.Top = append(m.Top, c.Name)
		m.Flags[c.Name] = flags
	}
	sort.Strings(m.Top)
	for k := range m.Subs {
		sort.Strings(m.Subs[k])
	}
	return m
}

func writeBashCompletion(w io.Writer, m completionModel) {
	fmt.Fprintf(w, "# bash completion for downshift\n")
	fmt.Fprintf(w, "_downshift_completions() {\n")
	fmt.Fprintf(w, "  local cur prev cmd sub\n")
	fmt.Fprintf(w, "  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(w, "  cmd=\"${COMP_WORDS[1]:-}\"\n")
	fmt.Fprintf(w, "  sub=\"${COMP_WORDS[2]:-}\"\n")
	fmt.Fprintf(w, "  if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	fmt.Fprintf(w, "    COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") )\n", strings.Join(append(append([]string{}, m.Top...), m.GlobalFlags...), " "))
	fmt.Fprintf(w, "    return 0\n  fi\n")
	fmt.Fprintf(w, "  case \"${cmd}\" in\n")
	for _, cmd := range m.Top {
		words := append([]string{}, m.Subs[cmd]...)
		words = append(words, m.Flags[cmd]...)
		words = append(words, m.GlobalFlags...)
		fmt.Fprintf(w, "    %s)\n", cmd)
		if len(m.Subs[cmd]) > 0 {
			fmt.Fprintf(w, "      if [[ ${COMP_CWORD} -ge 3 ]]; then\n")
			fmt.Fprintf(w, "        case \"${sub}\" in\n")
			for _, sub := range m.Subs[cmd] {
				subFlags := append(append([]string{}, m.Flags[cmd+" "+sub]...), m.GlobalFlags...)
				fmt.Fprintf(w, "          %s) COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") ); return 0 ;;\n", sub, strings.Join(subFlags, " "))
			}
			fmt.Fprintf(w, "        esac\n      fi\n")
		}
		fmt.Fprintf(w, "      COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") );;\n", strings.Join(words, " "))
	}
	fmt.Fprintf(w, "  esac\n}\ncomplete -F _downshift_completions downshift\n")
}

func writeZshCompletion(w io.Writer, m completionModel) {
	fmt.Fprintf(w, "#compdef downshift\n# zsh completion for downshift\n\n")
	fmt.Fprintf(w, "_downshift() {\n")
	fmt.Fprintf(w, "  local -a commands\n")
	fmt.Fprintf(w, "  if (( CURRENT == 2 )); then\n")
	fmt.Fprintf(w, "    commands=(%s)\n", joinSingleQuoted(m.Top))
	fmt.Fprintf(w, "    compadd -a commands\n    return\n  fi\n")
	fmt.Fprintf(w, "  case \"${words[2]}\" in\n")
	for _, cmd := range m.Top {
		words := append([]string{}, m.Subs[cmd]...)
		words = append(words, m.Flags[cmd]...)
		words = append(words, m.GlobalFlags...)
		fmt.Fprintf(w, "    %s)\n", cmd)
		if len(m.Subs[cmd]) > 0 {
			fmt.Fprintf(w, "      if (( CURRENT >= 4 )); then\n        case \"${words[3]}\" in\n")
			for _, sub := range m.Subs[cmd] {
				subFlags := append(append([]string{}, m.Flags[cmd+" "+sub]...), m.GlobalFlags...)
				fmt.Fprintf(w, "          %s) compadd -- %s; return ;;\n", sub, joinSingleQuoted(subFlags))
			}
			fmt.Fprintf(w, "        esac\n      fi\n")
		}
		fmt.Fprintf(w, "      compadd -- %s ;;\n", joinSingleQuoted(words))
	}
	fmt.Fprintf(w, "  esac\n}\n\n")
	fmt.Fprintf(w, "if [[ \"${funcstack[1]:-}\" == _downshift ]]; then\n  _downshift \"$@\"\nelse\n  compdef _downshift downshift\nfi\n")
}

func writeFishCompletion(w io.Writer, m completionModel) {
	fmt.Fprintf(w, "# fish completion for downshift\n")
	fmt.Fprintf(w, "complete -c downshift -f\n")
	for _, gf := range m.GlobalFlags {
		fmt.Fprintf(w, "complete -c downshift -n '__fish_use_subcommand' -l %s\n", strings.TrimPrefix(gf, "--"))
	}
	for _, cmd := range m.Top {
		fmt.Fprintf(w, "complete -c downshift -n '__fish_use_subcommand' -a '%s'\n", cmd)
		for _, fl := range m.Flags[cmd] {
			fmt.Fprintf(w, "complete -c downshift -n '__fish_seen_subcommand_from %s' -l %s\n", cmd, strings.TrimPrefix(fl, "--"))
		}
		for _, sub := range m.Subs[cmd] {
			fmt.Fprintf(w, "complete -c downshift -n '__fish_seen_subcommand_from %s; and not __fish_seen_subcommand_from %s' -a '%s'\n", cmd, strings.Join(m.Subs[cmd], " "), sub)
			for _, fl := range m.Flags[cmd+" "+sub] {
				fmt.Fprintf(w, "complete -c downshift -n '__fish_seen_subcommand_from %s; and __fish_seen_subcommand_from %s' -l %s\n", cmd, sub, strings.TrimPrefix(fl, "--"))
			}
		}
	}
}

func writePowerShellCompletion(w io.Writer, m completionModel) {
	fmt.Fprintf(w, "# PowerShell completion for downshift\n")
	fmt.Fprintf(w, "Register-ArgumentCompleter -Native -CommandName downshift -ScriptBlock {\n")
	fmt.Fprintf(w, "  param($wordToComplete, $commandAst, $cursorPosition)\n")
	fmt.Fprintf(w, "  $tokens = $commandAst.CommandElements | ForEach-Object { $_.ToString() }\n")
	fmt.Fprintf(w, "  $top = @(%s)\n", strings.Join(singleQuoteSlice(m.Top), ", "))
	fmt.Fprintf(w, "  $globalFlags = @(%s)\n", strings.Join(singleQuoteSlice(m.GlobalFlags), ", "))
	fmt.Fprintf(w, "  $subMap = @{\n")
	for _, cmd := range m.Top {
		words := append([]string{}, m.Subs[cmd]...)
		words = append(words, m.Flags[cmd]...)
		if len(words) > 0 {
			fmt.Fprintf(w, "    '%s' = @(%s)\n", cmd, strings.Join(singleQuoteSlice(words), ", "))
		}
		for _, sub := range m.Subs[cmd] {
			key := cmd + " " + sub
			if len(m.Flags[key]) > 0 {
				fmt.Fprintf(w, "    '%s' = @(%s)\n", key, strings.Join(singleQuoteSlice(m.Flags[key]), ", "))
			}
		}
	}
	fmt.Fprintf(w, "  }\n")
	fmt.Fprintf(w, "  $candidates = if ($tokens.Count -le 2 -and $wordToComplete -notlike '-*') { $top } elseif ($tokens.Count -ge 3 -and $subMap.ContainsKey(\"$($tokens[1]) $($tokens[2])\")) { $subMap[\"$($tokens[1]) $($tokens[2])\"] + $globalFlags } elseif ($tokens.Count -ge 2 -and $subMap.ContainsKey($tokens[1])) { $subMap[$tokens[1]] + $globalFlags } else { $globalFlags }\n")
	fmt.Fprintf(w, "  $candidates | Where-Object { $_ -like \"$wordToComplete*\" } | ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }\n")
	fmt.Fprintf(w, "}\n")
}

func singleQuoteSlice(items []string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "'" + strings.ReplaceAll(it, "'", "''") + "'"
	}
	return out
}

func joinSingleQuoted(items []string) string {
	return strings.Join(singleQuoteSlice(items), " ")
}
