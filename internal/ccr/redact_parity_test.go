// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"bytes"
	"testing"
)

// redactParitySeeds are the shapes the redact tests cover, plus the inputs
// that once slipped past the ASCII prefilter.
var redactParitySeeds = []string{
	"{\n  \"Credentials\": {\n    \"AccessKeyId\": \"ASIAIOSFODNN7EXAMPLE\",\n    \"SecretAccessKey\": \"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\",\n    \"SessionToken\": \"AQoDYXdzEJr1111111111111111EXAMPLETOKEN\",\n    \"Expiration\": \"2026-11-07T12:00:00Z\"\n  }\n}",
	"{\"Credentials\":{\"AccessKeyId\":\"ASIAIOSFODNN7EXAMPLE\",\"SecretAccessKey\":\"k\",\"SessionToken\":\"t\"}}",
	"CREDENTIALS\tASIAIOSFODNN7EXAMPLE\t2026-11-07T12:00:00Z\tsecret\tsession",
	"| AccessKeyId     | ASIAIOSFODNN7EXAMPLE |\n| Expiration      | 2026-11-07T12:00:00Z     |\n| SecretAccessKey | s |\n| SessionToken    | t |",
	"AccessKeyId = ASIAIOSFODNN7EXAMPLE\nExpiration: 2026-11-07T12:00:00Z\nSecretAccessKey: s\nSessionToken: t",
	"Loaded temporary credentials for ASIAIOSFODNN7EXAMPLE",
	"{\n  \"Parameter\": {\n    \"Name\": \"/production/db/password\",\n    \"Type\": \"SecureString\",\n    \"Value\": \"v\",\n    \"ARN\": \"arn:aws:ssm:us-east-1:123456789012:parameter/p\"\n  }\n}",
	"{\n  \"Parameters\": [\n    {\n      \"Name\": \"/app/secret1\",\n      \"Value\": \"one\"\n    }\n  ],\n  \"InvalidParameters\": []\n}",
	"{\"Parameter\":{\"Name\":\"/my/secret\",\"Type\":\"SecureString\",\"Value\":\"v\",\"Version\":1}}",
	"PARAMETERS\tarn:aws:ssm:us-east-1:123456789012:parameter/my/param\t2026\t1\t/my/param\tSecureString\ttext\tv",
	"arn:aws:ssm:us-east-1:123456789012:parameter/my/param\t2026\t1\t/my/param\tSecureString\ttext\tv",
	"| Name     | /my/secret/param |\n| Type     | SecureString     |\n| Value    | v |\n| Version  | 1                |",
	"|| arn:aws:ssm:... | /my/param | SecureString | v | 1 ||",
	"AWS_SESSION_TOKEN=t\nexport AWS_SESSION_TOKEN=\"t\"\nexport AWS_SECRET_ACCESS_KEY='s'\nAWS_ACCESS_KEY_ID=ASIAIOSFODNN7EXAMPLE",
	"{\n  \"Metric\": {\n    \"Value\": \"42.5\"\n  }\n}",
	"{\"Config\":{\"Setting\":\"theme\",\"Value\":\"dark\"}}",
	"Authorization: Basic dXNlcjpwYXNz\nSet-Cookie: session=abc123\ndsn=postgres://user:hunter2@db:5432/app",
	"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig\nusing ghp_abcdefABCDEF0123456789\nOPENAI sk-proj0123456789\nkey AKIAABCDEFGHIJKLMNOP",
	"DB_PASS=hunter2\nNPM_AUTH = xyz\nexport GITHUB_TOKEN=x\ntoken: abc123\nclient_secret = xyz\nCookie: sid=1",
	"--- PASS: TestTokenBudget (0.00s)\ninternal/ccr/secret.go:12:5: declared and not used\ntokens: 12400 saved",
	"before\n-----BEGIN OPENSSH PRIVATE KEY-----\nb3Blbn\n-----END OPENSSH PRIVATE KEY-----\nafter",
	"ARN:AWS:SSM:us-east-1:1:parameter/p\t2026\t1\t/p\tSecureString\ttext\tv",
	"to\u212Aen=abc",
	"client_\u017Fecret=abc",
	"CREDENTIALS SessionToken: v x y",
	"CREDENTIALS | AccessKeyId | v | x y",
}

// FuzzRedactParity holds the prefilter to its contract: it may skip a regex
// only when the regex cannot match, so the output equals the regex-only
// reference for any input.
func FuzzRedactParity(f *testing.F) {
	for _, s := range redactParitySeeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		want := redactReference(append([]byte(nil), in...))
		got := redact(append([]byte(nil), in...))
		if !bytes.Equal(got, want) {
			t.Fatalf("redact diverges from reference\nin:   %q\ngot:  %q\nwant: %q", in, got, want)
		}
	})
}

// TestRedact_PrefilterCaseFolding pins the gaps the ASCII prefilter had: a
// mixed-case SSM ARN and Unicode letters that (?i) folds onto ASCII ones.
func TestRedact_PrefilterCaseFolding(t *testing.T) {
	cases := []struct{ name, in string }{
		{"upper ssm arn", "ARN:AWS:SSM:us-east-1:1:parameter/p\t2026\t1\t/p\tSecureString\ttext\tv"},
		{"mixed ssm arn", "Arn:Aws:Ssm:us-east-1:1:parameter/p\t2026\t1\t/p\tString\ttext\tv"},
		{"kelvin sign token", "to\u212Aen=abc"},
		{"long s secret", "client_\u017Fecret=abc"},
		{"long s password", "pa\u017F\u017Fword: hunter2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := string(redactReference([]byte(c.in)))
			if want == c.in {
				t.Fatalf("reference keeps %q; the case no longer exercises a regex", c.in)
			}
			if got := string(redact([]byte(c.in))); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
