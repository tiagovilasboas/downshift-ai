// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// fileReaders are the producer programs whose output is file content.
var fileReaders = map[string]bool{"cat": true, "sed": true, "head": true, "tail": true, "bat": true}

// readsFile reports whether the command's output is most likely a file read.
func readsFile(command string) bool { return fileReaders[ProducerProgram(command)] }

// rewriteStored is rewrite plus the lossy passes. The lossy ones run only when
// keep can store the original and the agent set no head or tail limit: they
// store it and end the text with the marker that names the id, so what they
// dropped stays retrievable. keep is the caller's store port, which also
// records the id and adds the session flag to the id it returns.
func (s Shapes) rewriteStored(data []byte, balanced bool, keep func([]byte) (string, bool)) ([]byte, bool) {
	if out, ok := s.rewrite(data, balanced); ok {
		return out, true
	}
	if keep == nil || !s.On || !balanced || s.Limited {
		return data, false
	}
	out, ok := s.rewriteLossy(data)
	if !ok {
		return data, false
	}
	id, stored := keep(data)
	if !stored {
		return data, false
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return append(out, MarkerWithID(len(data), len(out), id)...), true
}

// rewriteLossy runs the passes that drop text the agent can retrieve: shell
// JSON pruning, then near-repeat folding and long-line capping. The line cap
// skips a file read, where long lines are content.
func (s Shapes) rewriteLossy(data []byte) ([]byte, bool) {
	if out, ok := compressor.CondenseShellJSON(data); ok {
		return out, true
	}
	out, changed := compressor.FoldNearRepeats(data)
	if !s.FileRead {
		var capped bool
		if out, capped = compressor.CapLongLines(out); capped {
			changed = true
		}
	}
	return out, changed
}
