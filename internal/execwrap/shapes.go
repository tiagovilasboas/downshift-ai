// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift-ai/internal/compressor"

// Shapes says which lossless shape rewrites may run on unclassified output.
// The zero value applies none.
type Shapes struct {
	On        bool // the lossless shape rewrites may run (the zero value runs none)
	Limited   bool // the agent piped through a head or tail these lines already fit: no byte cut
	SearchLed bool // the command is led by a search, so rows are grep rows
}

// ShapesFor reads the shapes a command's output allows. The shapes keep every
// row, so they run inside a head or tail the agent asked for too; Limited then
// keeps the byte cut off, so every row the agent asked for still appears.
func ShapesFor(command string, data []byte) Shapes {
	if len(data) == 0 {
		return Shapes{}
	}
	return Shapes{On: true, Limited: withinRequestedLimit(command, data), SearchLed: LedBySearch(command)}
}

// rewrite tries, in order, grep grouping (search-led only), the diff rules and
// path prefix factoring, and returns the first that applies. Each keeps every
// changed line, path and match; the caller stores the original in CCR. It runs
// only with the balanced cut on, before the byte cut.
func (s Shapes) rewrite(data []byte, balanced bool) ([]byte, bool) {
	if !s.On || !balanced {
		return data, false
	}
	if s.SearchLed {
		if out, ok := compressor.GroupGrepLines(data); ok {
			return out, true
		}
	}
	if out, ok := compressor.DiffShaped(data); ok {
		return out, true
	}
	return compressor.FactorPathPrefixes(data)
}
