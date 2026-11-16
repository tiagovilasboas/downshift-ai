// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package paths resolves Downshift's local state directory with legacy fallback.
package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvStateDir overrides the state directory (absolute path).
	EnvStateDir = "DOWNSHIFT_STATE_DIR"
	DirName     = ".downshift"
	LegacyName  = ".harness-downshift"
)

// Source describes how StateDir was chosen.
type Source string

const (
	SourceEnv     Source = "env"
	SourceDefault Source = "default"
	SourceLegacy  Source = "legacy"
)

// Resolve returns the effective state directory and how it was chosen.
func Resolve() (dir string, source Source, err error) {
	if v := os.Getenv(EnvStateDir); v != "" {
		return v, SourceEnv, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	newDir := filepath.Join(home, DirName)
	legacyDir := filepath.Join(home, LegacyName)
	if isDir(newDir) {
		return newDir, SourceDefault, nil
	}
	if isDir(legacyDir) {
		return legacyDir, SourceLegacy, nil
	}
	return newDir, SourceDefault, nil
}

// StateDir is the effective local state directory.
func StateDir() (string, error) {
	dir, _, err := Resolve()
	return dir, err
}

// Join builds a path under the effective state directory.
func Join(elem ...string) (string, error) {
	root, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.Join(elem...)), nil
}

// DisplayStateDir is a short human label for docs and stderr (tilde when under $HOME).
func DisplayStateDir() string {
	dir, err := StateDir()
	if err != nil {
		return "~/" + DirName
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			// filepath.Rel uses the OS separator (backslash on Windows); the
			// display label is always forward-slash so docs and stderr read the
			// same on every OS.
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return dir
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
