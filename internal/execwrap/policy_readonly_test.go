// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

func TestReadOnly(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		// Read-only, one command, no metacharacter.
		{"git status", true},
		{"  git status --short  ", true},
		{"git log --oneline -n 5", true},
		{"git diff --stat", true},
		{"git show HEAD", true},
		{"git branch", true},
		{"git branch -a -v", true},
		{"git branch --show-current", true},
		{"git rev-parse HEAD", true},
		{"git ls-files", true},
		{"git remote -v", true},
		{"git stash list", true},
		{"gh pr view 12", true},
		{"gh pr list", true},
		{"gh pr checks 12", true},
		{"gh issue list", true},
		{"gh run view 99", true},
		{"gh repo view", true},
		{"go version", true},
		{"go env GOPATH", true},
		{"go list ./...", true},
		{"go doc fmt.Println", true},
		{"mise list", true},
		{"asdf current", true},
		{"direnv status", true},
		{"ls -la internal", true},
		{"wc -l main.go", true},
		{"wc -l a.go b.go", true},
		{"find . -type f -maxdepth 2", true},
		{"grep -rn func internal", true},
		{"grep -rl foo internal", true},
		{"rg -n func internal", true},
		{"rg --files internal", true},
		{"cat main.go", true},
		{"head -n 40 main.go", true},
		{"tail -40 main.go", true},
		{"sed -n 10,20p main.go", true},
		{"jq . package.json", true},
		{"git fetch origin", true},
		{"git fetch --all --tags", true},
		{"agent-desktop snapshot --app Files -i", true},
		{"agent-desktop find --role button --name Save", true},

		// Chained or with any shell metacharacter.
		{"go version ; perl /tmp/x.pl", false},
		{"git status && rm -rf x", false},
		{"git status || true", false},
		{"git status | cat", false},
		{"git status &", false},
		{"git status $(id)", false},
		{"git status `id`", false},
		{"git status > out", false},
		{"git status < in", false},
		{"git status\nperl x", false},
		{"git status\tx", false},
		{"cd sub && git status", false},

		// Env assignments can redirect a program to run code.
		{"GIT_EXTERNAL_DIFF=/tmp/x git diff", false},

		// Runners, builds, tests and installs execute project code.
		{"make check", false},
		{"go test ./...", false},
		{"go run .", false},
		{"go build ./...", false},
		{"go generate ./...", false},
		{"uv run pytest", false},
		{"npm test", false},
		{"pytest", false},
		{"cargo test", false},

		// Mutating subcommands and flags.
		{"git pull", false},
		{"git commit -m x", false},
		{"git branch -D main", false},
		{"git branch new", false},
		{"git stash", false},
		{"git stash pop", false},
		{"git remote add x y", false},
		{"git diff --output=/tmp/x", false},
		{"git diff --ext-diff", false},
		{"git log --textconv", false},
		{"git checkout main", false},
		{"gh pr merge 12", false},
		{"gh pr checkout 12", false},
		{"gh issue close 3", false},
		{"gh run rerun 99", false},
		{"gh api repos/x/y", false},
		{"gh auth status", false},
		{"gh repo delete x", false},
		{"go env -w GOFLAGS=-x", false},
		{"go env -u GOFLAGS", false},
		{"go list -toolexec=/tmp/x ./...", false},
		{"go list --toolexec /tmp/x ./...", false},
		{"go list -mod=mod ./...", false},
		{"mise use node@20", false},
		{"mise install", false},
		{"asdf global node 20", false},
		{"direnv allow", false},
		{"aws s3 cp a b", false},
		{"kubectl get pods", false},

		// Security review of the read-only set.
		{"go tool compile -o x.o m.go", false},
		{"go tool mytool", false},
		{"go tool pprof -http=:8080 x", false},
		{"go doc -http", false},
		{"go doc --http=:6060", false},
		{"git stash list --output=/tmp/x", false},
		{"git stash list -p --ext-diff", false},
		{"git stash list -p", true},
		{"gh repo view --web", false},
		{"gh pr view 1 -w", false},
		{"gh search repos x --web", false},
		{"gh search repos x", true},
		{"agent-desktop click @s1:e3", false},
		{"agent-desktop snapshot --debug", false},
		{"agent-desktop snapshot --screenshot out.png", false},

		// Unsafe / mutating forms of added read-only families stay false.
		{"find . -delete", false},
		{"find . -exec rm {} ;", false},
		{"find . -execdir sh -c x ;", false},
		{"find . -ok rm ;", false},
		{"find . -fprint out.txt", false},
		{"cat -n main.go", true},
		{"git -C dir status", true},
		{"git -C dir push", false},
		{"git -C dir branch -D main", false},
		{"cat a.go b.go", false},
		{"cat /dev/zero", false},
		{"tail -f app.log", false},
		{"sed -i s/a/b/ main.go", false},
		{"jq .a package.json", false},
		{"wc -c main.go", false},
		{"grep -rn", false},
		{"rg -n --pre cat foo internal", false},
		{"git -c core.pager=evil fetch origin", false},
		{"git fetch --upload-pack=/tmp/evil origin", false},
		{"git fetch --upload-pack /tmp/evil origin", false},
		{"git fetch -f origin", false},
		{"git fetch --force origin", false},
		{"git fetch -p origin", false},
		{"git fetch --prune origin", false},
		{"git fetch -ap origin", false},
		{"git fetch -u origin", false},
		{"git fetch --update-head-ok origin", false},
		{"git fetch origin main:evil", false},
		{"git fetch ext::evil", false},

		// Edge shapes.
		{"", false},
		{"git", false},
		{"downshift compact -- git status", false},
	}
	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			if got := ReadOnly(c.command); got != c.want {
				t.Fatalf("ReadOnly(%q) = %v, want %v", c.command, got, c.want)
			}
		})
	}
}
