// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import "bytes"

// redactReference is redact as it was before the byte-scan prefilter
// (redact_hints.go, commit bdd446fa): every regex runs on every line. The
// prefilter must only skip regexes that cannot match, so redact and this
// reference agree on every input (FuzzRedactParity).
func redactReference(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	lines := bytes.Split(raw, []byte("\n"))
	inPEM := false
	inSSMBlock := false
	ssmBraceDepth := 0

	for i, l := range lines {
		if !inPEM && rePEMBegin.Match(l) {
			inPEM = true
		}
		if inPEM {
			if rePEMEnd.Match(l) {
				inPEM = false
			}
			lines[i] = []byte(redactedLine)
			continue
		}

		if reSSMBlockBegin.Match(l) {
			inSSMBlock = true
		}

		lineModified := false
		current := l

		if inSSMBlock {
			ssmBraceDepth += bytes.Count(current, []byte("{")) + bytes.Count(current, []byte("[")) -
				bytes.Count(current, []byte("}")) - bytes.Count(current, []byte("]"))
			if reSSMJSONValue.Match(current) {
				current = reSSMJSONValue.ReplaceAll(current, []byte(`${1}"`+RedactedMark+`"`))
			}
			if ssmBraceDepth <= 0 && (bytes.Contains(current, []byte("}")) || bytes.Contains(current, []byte("]"))) {
				inSSMBlock = false
				ssmBraceDepth = 0
			}
			lines[i] = current
			continue
		}

		if reSSMTableValue.Match(current) {
			current = reSSMTableValue.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		} else if reSSMTableSecure.Match(current) {
			current = reSSMTableSecure.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		} else if reSSMTextParameters.Match(current) {
			current = reSSMTextParameters.ReplaceAll(current, []byte(`${1}${2}${3}`+RedactedMark))
			lineModified = true
		} else if reSSMTextSingle.Match(current) {
			current = reSSMTextSingle.ReplaceAll(current, []byte(`${1}${2}${3}`+RedactedMark))
			lineModified = true
		}

		if reSTSJSON.Match(current) {
			current = reSTSJSON.ReplaceAll(current, []byte(`${1}"`+RedactedMark+`"`))
			lineModified = true
		}
		if reSTSCredentialsText.Match(current) {
			current = reSTSCredentialsText.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${3}`+RedactedMark+`${5}`+RedactedMark+`${7}`))
			lineModified = true
		}
		if reSTSTable.Match(current) {
			current = reSTSTable.ReplaceAll(current, []byte(`${1}`+RedactedMark+`${2}`))
			lineModified = true
		}
		if reSTSCLIKeyValue.Match(current) {
			current = reSTSCLIKeyValue.ReplaceAll(current, []byte(`${1}`+RedactedMark))
			lineModified = true
		}

		if reAWSEnvQuotedDouble.Match(current) {
			current = reAWSEnvQuotedDouble.ReplaceAll(current, []byte(`${1}${2}="`+RedactedMark+`"`))
			lineModified = true
		} else if reAWSEnvQuotedSingle.Match(current) {
			current = reAWSEnvQuotedSingle.ReplaceAll(current, []byte(`${1}${2}='`+RedactedMark+`'`))
			lineModified = true
		} else if reAWSEnvUnquoted.Match(current) {
			current = reAWSEnvUnquoted.ReplaceAll(current, []byte(`${1}${2}=`+RedactedMark))
			lineModified = true
		}

		if reASIAKey.Match(current) {
			current = reASIAKey.ReplaceAll(current, []byte(RedactedMark))
			lineModified = true
		}

		if lineModified {
			lines[i] = current
			continue
		}

		if reSecret.Match(current) {
			lines[i] = []byte(redactedLine)
		}
	}

	return bytes.Join(lines, []byte("\n"))
}
