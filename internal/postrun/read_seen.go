// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const (
	readSeenSubdir   = "read-seen"
	readSeenMinBytes = 2 << 10
)

// fileReadHeads are the shell programs whose output is a file's text. Only
// these are deduplicated by content: a repeated test run or git status can
// print the same bytes for a reason the model needs to see again.
var fileReadHeads = map[string]bool{"cat": true, "sed": true, "head": true, "tail": true, "bat": true}

// seenRead remembers, per session, the CCR handle of file text the model has
// already received, keyed by a hash of the bytes. It stores neither the text
// nor any path. A second identical text in the same session returns a short
// pointer to the first handle; anything else, and any error, returns raw
// unchanged (changed false). o.Keep stores the first copy.
func seenRead(raw string, o Options, session string) (out, id string, changed bool) {
	return seenReadWith(raw, o, session, filePointer)
}

func seenReadWith(raw string, o Options, session string, pointer pointerFunc) (out, id string, changed bool) {
	defer func() {
		if recover() != nil {
			out, id, changed = raw, "", false
		}
	}()
	if len(raw) < readSeenMinBytes || o.Keep == nil || session == "" {
		return raw, "", false
	}
	dir, err := paths.Join(readSeenSubdir)
	if err != nil {
		return raw, "", false
	}
	return seenReadAt(raw, o, session, dir, time.Now(), pointer)
}

func seenReadAt(raw string, o Options, session, dir string, now time.Time, pointer pointerFunc) (string, string, bool) {
	sess := sha256.Sum256([]byte("downshift-read-seen-session-v1:" + session))
	sum := sha256.Sum256([]byte(raw))
	key := filepath.Join(dir, hex.EncodeToString(sess[:8])+"-"+hex.EncodeToString(sum[:16])+".json")

	if prev, hit := readDedupSnapshot(key, readDedupTTL, now); hit && prev.ID != "" && prev.Size == int64(len(raw)) {
		ptr := pointer(raw, readRetrieveArg(session, prev.ID))
		if len(ptr) < len(raw) {
			return ptr, prev.ID, true
		}
	}
	id, ok := o.Keep([]byte(raw))
	if !ok || id == "" {
		return raw, "", false
	}
	writeDedupSnapshot(key, readSnapshot{Size: int64(len(raw)), At: now.UnixNano(), ID: id}, now)
	pruneDedupDir(dir, key, readDedupTTL, now)
	return raw, "", false
}

// seenSession is the session the dedup keys on: the hashed one the runner
// passes, else the payload's own (hashed again inside the key).
func seenSession(o Options, payload string) string {
	if o.Session != "" {
		return o.Session
	}
	return payload
}

// dedupShellRead applies seenRead to the stdout of a file-reading shell
// command; any other unclassified output goes through dedupShellAny.
func dedupShellRead(command, stdout, stderr string, o Options, payloadSession string) (string, string, bool) {
	if !fileReadHeads[execwrap.CommandHead(command)] {
		return dedupShellAny(command, stdout, stderr, o, payloadSession)
	}
	return seenRead(stdout, o, seenSession(o, payloadSession))
}
