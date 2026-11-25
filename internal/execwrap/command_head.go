// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"path/filepath"
	"strings"
)

// Head codes that are not a program name.
const (
	HeadScript  = "script"
	HeadUnknown = "unknown"
)

// headVocabulary maps a well-known program basename to the code logged for it.
// Only these values ever leave CommandHead: a program outside the list is
// "script" or "unknown", never its own name.
var headVocabulary = map[string]string{
	"python": "python", "python3": "python", "node": "node", "npm": "npm", "npx": "npx",
	"pnpm": "pnpm", "yarn": "yarn", "bun": "bun", "deno": "deno", "curl": "curl",
	"wget": "wget", "jq": "jq", "yq": "yq", "docker": "docker", "kubectl": "kubectl",
	"helm": "helm", "terraform": "terraform", "aws": "aws", "gcloud": "gcloud", "gh": "gh",
	"sed": "sed", "awk": "awk", "cat": "cat", "echo": "echo", "printf": "printf",
	"head": "head", "tail": "tail", "find": "find", "xargs": "xargs", "sort": "sort",
	"uniq": "uniq", "tr": "tr", "cut": "cut", "diff": "diff", "make": "make", "go": "go",
	"cargo": "cargo", "rustc": "rustc", "java": "java", "mvn": "mvn", "gradle": "gradle",
	"php": "php", "composer": "composer", "ruby": "ruby", "bundle": "bundle", "pip": "pip",
	"pip3": "pip", "uv": "uv", "sqlite3": "sqlite3", "mysql": "mysql", "psql": "psql",
	"redis-cli": "redis-cli", "ps": "ps", "lsof": "lsof", "du": "du", "df": "df",
	"which": "which", "tree": "tree", "rg": "rg", "fd": "fd", "bat": "bat",
	"open": "open", "osascript": "osascript", "swift": "swift", "xcodebuild": "xcodebuild",
	"grep": "grep", "ls": "ls", "git": "git", "cd": "cd", "mkdir": "mkdir", "cp": "cp",
	"mv": "mv", "rm": "rm", "touch": "touch", "chmod": "chmod", "wc": "wc", "tee": "tee",
	"ssh": "ssh", "scp": "scp", "rsync": "rsync", "tar": "tar", "unzip": "unzip",
	"zip": "zip", "kill": "kill", "sleep": "sleep", "test": "test", "export": "export",
	"source": "source", "bash": "bash", "sh": "sh", "zsh": "zsh", "brew": "brew",
}

// headWrappers are programs skipped to reach the one they run.
var headWrappers = map[string]bool{
	"env": true, "sudo": true, "time": true, "nice": true, "timeout": true,
	"nohup": true, "command": true, "exec": true,
}

// CommandHead returns a code for the program a shell command runs: the
// basename of the first program after leading NAME=value assignments and
// wrappers, only when it is in the fixed vocabulary. A script (./x, *.sh) is
// "script"; anything else is "unknown". It never returns a path, script name
// or argument.
//
// A compound command is split on top-level &&, ||, ; and |, and the first
// segment whose program is not a trivial setup (cd, export, source ...) names
// it. When every segment is trivial, the first segment's head is returned.
func CommandHead(command string) string {
	first := ""
	for _, seg := range splitSegments(command) {
		code, base := segmentHead(strings.Fields(stripFuncHeader(seg)))
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

// trivialHeads are shell setup or condition programs that never name what a compound runs.
var trivialHeads = map[string]bool{
	"cd": true, "pushd": true, "popd": true, "export": true, "set": true,
	"source": true, ".": true, "unset": true, "true": true, "alias": true,
	"test": true, "[": true, "[[": true, "read": true,
}

// shellControlKeywords are shell syntax keywords skipped when finding the
// inner command of a control-flow construct.
var shellControlKeywords = map[string]bool{
	"do": true, "done": true, "then": true, "else": true, "elif": true,
	"fi": true, "if": true, "while": true, "until": true, "in": true,
	"case": true, "esac": true, "function": true,
}

// stripFuncHeader removes a leading shell function definition header such as
// `k(){`, `k() {`, `k () {`, `function k {`, or `function k() {`.
func stripFuncHeader(seg string) string {
	s := strings.TrimSpace(seg)
	hasFuncKW := false
	if rest, ok := cutKeyword(s, "function"); ok {
		hasFuncKW = true
		s = strings.TrimSpace(rest)
	}
	n := 0
	for n < len(s) {
		c := s[n]
		isAlpha := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
		isRest := isAlpha || (n > 0 && (c >= '0' && c <= '9' || c == '-'))
		if (n == 0 && !isAlpha) || (n > 0 && !isRest) {
			break
		}
		n++
	}
	if n == 0 {
		return seg
	}
	rest := strings.TrimSpace(s[n:])
	if strings.HasPrefix(rest, "()") {
		rest = strings.TrimSpace(rest[2:])
		return strings.TrimPrefix(rest, "{")
	}
	if hasFuncKW {
		if strings.HasPrefix(rest, "{") {
			return rest[1:]
		}
		if rest == "" {
			return ""
		}
	}
	return seg
}

func cutKeyword(s, kw string) (string, bool) {
	if !strings.HasPrefix(s, kw) || len(s) <= len(kw) {
		return "", false
	}
	if s[len(kw)] != ' ' && s[len(kw)] != '\t' {
		return "", false
	}
	return s[len(kw)+1:], true
}

// splitSegments splits command at top-level &&, ||, ;, | and newlines,
// ignoring separators inside single or double quotes.
func splitSegments(command string) []string {
	var segs []string
	var quote rune
	start := 0
	for i, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ';' || r == '|' || r == '&' || r == '\n':
			segs = append(segs, command[start:i])
			start = i + 1
		}
	}
	return append(segs, command[start:])
}

// segmentHead returns the head code of one simple command and the basename
// of its program ("" when it has none).
func segmentHead(fields []string) (code, base string) {
	for i := 0; i < len(fields); i++ {
		tok := strings.Trim(fields[i], `"'`+"`(){}")
		if tok == "" || tok == "!" || isAssignment(tok) {
			continue
		}
		if tok == "for" {
			return HeadUnknown, ""
		}
		if shellControlKeywords[tok] {
			continue
		}
		base := filepath.Base(strings.ReplaceAll(tok, `\`, "/"))
		if headWrappers[base] {
			// Skip the wrapper's own flags and numeric arguments.
			for i+1 < len(fields) && (strings.HasPrefix(fields[i+1], "-") || isNumeric(fields[i+1])) {
				i++
			}
			continue
		}
		if code, ok := headVocabulary[base]; ok {
			return code, base
		}
		if strings.HasPrefix(tok, "./") || strings.HasPrefix(tok, "../") || strings.HasSuffix(base, ".sh") {
			return HeadScript, base
		}
		return HeadUnknown, base
	}
	return HeadUnknown, ""
}

func isAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range tok[:eq] {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// LineBucket buckets an output line count into a fixed code.
func LineBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n <= 20:
		return "1-20"
	case n <= 140:
		return "21-140"
	case n <= 1000:
		return "141-1000"
	}
	return ">1000"
}

// CountLines counts the lines of text: a final line without a newline counts.
func CountLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}
