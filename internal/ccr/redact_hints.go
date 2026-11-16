// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import "bytes"

const (
	hintEq uint32 = 1 << iota
	hintColon
	hintPipe
	hintQuote
	hintPEMBegin
	hintPEMEnd
	hintParam
	hintValue
	hintSecure
	hintSTSKey
	hintCreds
	hintAWS
	hintASIA
	hintAKIA
	hintArnSSM
	hintSecretToken
	hintPass
	hintKeyAuthCookie
	hintBearerPriv
	hintScheme
	hintEyJ
	hintGH
	hintSK

	hintAWSMask    = hintParam | hintValue | hintSecure | hintSTSKey | hintCreds | hintAWS | hintASIA | hintArnSSM
	hintSecretMask = hintSecretToken | hintPass | hintKeyAuthCookie | hintBearerPriv | hintScheme | hintEyJ | hintGH | hintSK | hintAKIA | hintASIA
)

// scanLineHints scans l in a single pass to collect bitmask hints for all AWS
// and secret patterns, avoiding repeated regex scans per line.
func scanLineHints(l []byte) uint32 {
	var h uint32
	n := len(l)
	for i := 0; i < n; i++ {
		b := l[i]
		switch b {
		case '=':
			h |= hintEq
		case ':':
			h |= hintColon
			if i+2 < n && l[i+1] == '/' && l[i+2] == '/' {
				h |= hintScheme
			}
		case '|':
			h |= hintPipe
		case '"':
			h |= hintQuote
		case '-':
			if i+10 < n && l[i+1] == '-' && l[i+2] == '-' && l[i+3] == '-' && l[i+4] == '-' {
				if bytes.HasPrefix(l[i:], []byte("-----BEGIN ")) {
					h |= hintPEMBegin
				} else if bytes.HasPrefix(l[i:], []byte("-----END ")) {
					h |= hintPEMEnd
				}
			}
		case 'A':
			if i+3 < n && l[i+1] == 'S' && l[i+2] == 'I' && l[i+3] == 'A' {
				h |= hintASIA
			}
		}
		if i+2 >= n {
			continue
		}
		c := b | 0x20
		c1 := l[i+1] | 0x20
		c2 := l[i+2] | 0x20
		switch c {
		case 'a':
			if c1 == 'w' && c2 == 's' && i+3 < n && l[i+3] == '_' {
				h |= hintAWS
			} else if c1 == 'k' && c2 == 'i' && i+3 < n && (l[i+3]|0x20) == 'a' {
				h |= hintAKIA
			} else if c1 == 's' && c2 == 'i' && i+3 < n && (l[i+3]|0x20) == 'a' {
				h |= hintASIA
			} else if c1 == 'u' && c2 == 't' {
				h |= hintKeyAuthCookie
			} else if c1 == 'c' && c2 == 'c' {
				h |= hintSTSKey
			} else if c1 == 'r' && c2 == 'n' && i+11 < n && bytes.HasPrefix(l[i:], []byte("arn:aws:ssm:")) {
				h |= hintArnSSM
			}
		case 'b':
			if c1 == 'e' && c2 == 'a' {
				h |= hintBearerPriv
			}
		case 'c':
			if c1 == 'o' && c2 == 'o' {
				h |= hintKeyAuthCookie
			} else if c1 == 'r' && c2 == 'e' {
				h |= hintCreds
			}
		case 'e':
			if c1 == 'y' && c2 == 'j' {
				h |= hintEyJ
			}
		case 'g':
			if c1 == 'h' && i+3 < n && l[i+3] == '_' {
				h |= hintGH
			}
		case 'k':
			if c1 == 'e' && c2 == 'y' {
				h |= hintKeyAuthCookie
			}
		case 'p':
			if c1 == 'a' && c2 == 's' {
				h |= hintPass
			} else if c1 == 'a' && c2 == 'r' {
				h |= hintParam
			} else if c1 == 'r' && c2 == 'i' {
				h |= hintBearerPriv
			}
		case 's':
			if c1 == 'e' && c2 == 'c' {
				h |= hintSecretToken | hintSecure | hintSTSKey
			} else if c1 == 'e' && c2 == 's' {
				h |= hintSTSKey
			} else if c1 == 'k' && l[i+2] == '-' {
				h |= hintSK
			}
		case 't':
			if c1 == 'o' && c2 == 'k' {
				h |= hintSecretToken
			}
		case 'v':
			if c1 == 'a' && c2 == 'l' {
				h |= hintValue
			}
		}
	}
	return h
}

func mayBeSecretWithFlags(l []byte, flags uint32) bool {
	if flags&(hintEyJ|hintSK|hintAKIA|hintASIA|hintBearerPriv|hintScheme|hintGH) != 0 {
		return true
	}
	if flags&(hintEq|hintColon) == 0 {
		return false
	}
	if flags&(hintSecretToken|hintKeyAuthCookie) != 0 {
		return true
	}
	if flags&hintPass != 0 {
		return flags&hintEq != 0 ||
			containsFoldASCII(l, "password") || containsFoldASCII(l, "passwd") ||
			containsFoldASCII(l, "_pass") || containsFoldASCII(l, "-pass") || containsFoldASCII(l, ".pass")
	}
	return false
}

// containsFoldASCII reports whether haystack contains lowerNeedle (which must be
// all lowercase ASCII) using ASCII case-insensitive matching with zero allocations.
func containsFoldASCII(haystack []byte, lowerNeedle string) bool {
	n := len(lowerNeedle)
	if n == 0 {
		return true
	}
	if len(haystack) < n {
		return false
	}
	first := lowerNeedle[0]
	limit := len(haystack) - n
	for i := 0; i <= limit; i++ {
		c := haystack[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != first {
			continue
		}
		match := true
		for j := 1; j < n; j++ {
			hc := haystack[i+j]
			if hc >= 'A' && hc <= 'Z' {
				hc += 'a' - 'A'
			}
			if hc != lowerNeedle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
