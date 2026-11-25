// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestCommandHead(t *testing.T) {
	cases := []struct{ in, want string }{
		{"FOO=1 node a.js", "node"},
		{"./deploy-x.sh", "script"},
		{"scripts/run.sh --x", "script"},
		{"/opt/x/bin/foo", "unknown"},
		{"python3 -c 'print(1)'", "python"},
		{"cat <<EOF\nhi\nEOF", "cat"},
		{"", "unknown"},
		{"   ", "unknown"},
		{"A=1 B=2", "unknown"},
		{"sudo docker ps", "docker"},
		{"env FOO=1 jq .", "jq"},
		{"time npm test", "npm"},
		{"timeout 30 curl http://x", "curl"},
		{"/usr/bin/python3 x.py", "python"},
		{"mysecrettool --flag", "unknown"},
		{"cd x && node a.js", "node"},
		{"cd /a; export X=1; go test ./...", "go"},
		{"source .venv/bin/activate && python3 x.py", "python"},
		{"cd x", "cd"},
		{"cd x && ./run.sh", "script"},
		{"cd x && export A=1", "cd"},
		{"cd 'a;b' && jq .", "jq"},
		{"cd x || true; mysecrettool", "unknown"},
		{"cd X && go test ./...", "go"},
		{"for f in a b; do git status; done", "git"},
		{"while read -r f; do git status; done", "git"},
		{"if [ -f go.mod ]; then go test ./...; fi", "go"},
		{"until [ -f ready ]; do gh pr list; done", "gh"},
		{"k(){ gh pr list; }", "gh"},
		{"k() { gh pr list; }", "gh"},
		{"function k { go test ./...; }", "go"},
		{"k(){}", "unknown"},
		{"for f in a b; do done", "unknown"},
	}
	for _, c := range cases {
		if got := CommandHead(c.in); got != c.want {
			t.Errorf("CommandHead(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCommandHeadNeverLeaksTokens(t *testing.T) {
	vocab := map[string]bool{HeadScript: true, HeadUnknown: true}
	for _, v := range headVocabulary {
		vocab[v] = true
	}
	for _, in := range []string{
		"/opt/secret/bin/foo --token abc", `C:\tools\deploy.exe x`, "./my-private.sh",
		"FOO=bar customtool arg", "/usr/local/bin/node x", "sudo /srv/app/run", "weirdprog",
		"cd /home/secret && /opt/priv/tool x", "source /secret/env.sh; ./private.sh",
		"for f in a b; do /opt/secret; done", "k(){ /opt/secret; }",
	} {
		got := CommandHead(in)
		if strings.ContainsAny(got, `/\`) || !vocab[got] {
			t.Errorf("CommandHead(%q) = %q leaks or is outside the vocabulary", in, got)
		}
		for _, tok := range strings.Fields(in) {
			if got == tok && !vocab[tok] {
				t.Errorf("CommandHead(%q) returned token %q", in, tok)
			}
		}
	}
}

func TestLineBucket(t *testing.T) {
	cases := map[int]string{-1: "0", 0: "0", 1: "1-20", 20: "1-20", 21: "21-140", 140: "21-140", 141: "141-1000", 1000: "141-1000", 1001: ">1000"}
	for n, want := range cases {
		if got := LineBucket(n); got != want {
			t.Errorf("LineBucket(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCountLines(t *testing.T) {
	for in, want := range map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "a\nb\n": 2} {
		if got := CountLines(in); got != want {
			t.Errorf("CountLines(%q) = %d, want %d", in, got, want)
		}
	}
}
