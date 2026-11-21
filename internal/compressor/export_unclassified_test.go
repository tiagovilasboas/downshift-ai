// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// IsKeepLine exposes reKeepLine to the external unclassified baseline test.
func IsKeepLine(l string) bool { return reKeepLine.MatchString(l) }
