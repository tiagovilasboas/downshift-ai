// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

func TestProducerProgram(t *testing.T) {
	cases := []struct{ in, want string }{
		{"git log | wc -l", "git"},
		{"git log --oneline | head -5 | grep fix", "git"},
		{"cd x && gh auth status", "gh"},
		{"cd x && git status | tail -3", "git"},
		{"FOO=1 BAR=2 node a.js", "node"},
		{"VAR=1 cmd_not_known", "unknown"},
		{"time go test ./... | tail", "go"},
		{"sudo docker ps | sort", "docker"},
		{"python3 - <<EOF\nprint(1)\nEOF", "python"},
		{"python3 - <<'EOF'\nimport os\nEOF", "python"},
		{"node - <<EOF\nconsole.log(1)\nEOF", "node"},
		{"bash -c 'git log | wc -l'", ProducerBashScript},
		{"bash <<EOF\nls\nEOF", ProducerBashScript},
		{"/bin/sh -c 'echo hi'", ProducerBashScript},
		{"./run.sh | tail", "script"},
		{"cat file | grep x", "cat"},
		{"cat a | wc -l", "cat"},
		{"cd a && cd b", "cd"},
		{"cd x; export A=1; make build 2>&1 | tail -20", "make"},
		{"mysecrettool --flag | head", "unknown"},
		{"mysecrettool | grep x", "unknown"},
		{"", "unknown"},
		{"A=1", "unknown"},
		{"curl -s 'http://x|y' | jq .", "curl"},
		{`cmd /c git status`, "git"},
		{`cmd /c "cd x && npm test | findstr ok"`, "npm"},
		{`cmd.exe /c python3 x.py`, "python"},
		{`powershell -Command "git log | Select-Object -First 5"`, "git"},
		{`pwsh -c 'docker ps | Sort-Object'`, "docker"},
		{`C:\Windows\System32\cmd.exe /c go test ./...`, "go"},
		{"echo $(git log | wc -l)", "echo"},
		{"git status || echo failed", "git"},
		{"true && ls -la | sort", "ls"},
		{"cd X && go test ./...", "go"},
		{"for f in a b; do git status; done", "git"},
		{"for f in a b; do git status | tail -5; done", "git"},
		{"while true; do gh pr list; done", "gh"},
		{"if [ -f go.mod ]; then go test ./... | tail; fi", "go"},
		{"until [ -f ready ]; do git diff; done", "git"},
		{"k(){ gh pr list; }", "gh"},
		{"k() { go test ./... | tail; }", "go"},
	}
	for _, c := range cases {
		if got := ProducerProgram(c.in); got != c.want {
			t.Errorf("ProducerProgram(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProducerProgramBounded(t *testing.T) {
	allowed := map[string]bool{HeadScript: true, HeadUnknown: true, ProducerBashScript: true}
	for _, v := range headVocabulary {
		allowed[v] = true
	}
	for _, in := range []string{"secret-tool --token abc", "/opt/x/y z", "$(foo) | bar", "cmd /c secret.exe"} {
		if got := ProducerProgram(in); !allowed[got] {
			t.Errorf("ProducerProgram(%q) = %q, outside the bounded set", in, got)
		}
	}
}
