// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// splitRetrievals splits retrieval entries into reads without a session (per
// id) and reads of one session (per session reference and id, see scopedKey).
func splitRetrievals(entries []Retrieval) (byID, scoped map[string]int64) {
	byID, scoped = map[string]int64{}, map[string]int64{}
	for _, r := range entries {
		if r.Bytes <= 0 || !validKeptID(r.Kept) {
			continue
		}
		if ref := SessionRef(r.Session); ref != "" {
			scoped[scopedKey(ref, r.Kept)] += r.Bytes
			continue
		}
		byID[r.Kept] += r.Bytes
	}
	return byID, scoped
}

// scopedKey joins a session reference and a CCR id.
func scopedKey(ref, id string) string { return ref + "/" + id }

// applyLogged sets RetrievedBytes on evs from the retrievals log beside
// eventsPath: first the reads of one session against the runs of that session,
// then the reads without a session against any run that kept the id.
func applyLogged(evs []Event, eventsPath string) []Event {
	return ApplyRetrievalEntries(evs, ReadRetrievalEntries(eventsPath))
}

// ApplyRetrievalEntries sets RetrievedBytes on evs from the given Retrieval
// entries: first the reads of one session against the runs of that session,
// then the reads without a session against any run that kept the id.
// evs is modified in place and returned.
func ApplyRetrievalEntries(evs []Event, entries []Retrieval) []Event {
	byID, scoped := splitRetrievals(entries)
	evs = applyKeyed(evs, scoped, func(ev Event) []string {
		ref := SessionRef(ev.Session)
		if ref == "" {
			return nil
		}
		ids := eventKeptIDs(ev)
		if len(ids) == 0 {
			return nil
		}
		keys := make([]string, len(ids))
		for i, id := range ids {
			keys[i] = scopedKey(ref, id)
		}
		return keys
	})
	return ApplyRetrievals(evs, byID)
}

// ApplyRetrievals matches the bytes read back per id (reads without a session)
// to the runs that kept that id (in Kept or KeptIDs) and sets their
// RetrievedBytes. Content addressing gives two runs with the same output the
// same id; the bytes go to the runs in log order, each up to what it saved,
// and any remainder to the last of them, so the total shown is what was read
// back. A run whose id was never read back, and an id no run in evs kept,
// change nothing. evs is modified in place and returned.
func ApplyRetrievals(evs []Event, byID map[string]int64) []Event {
	return applyKeyed(evs, byID, eventKeptIDs)
}

// applyKeyed is the one arithmetic of ApplyRetrievals over any grouping of
// runs: keysOf names the groups of a run (nil for none) and totals holds the
// bytes read back per group. Bytes already on a run (RetrievedBytes) count
// against what it can still take.
func applyKeyed(evs []Event, totals map[string]int64, keysOf func(Event) []string) []Event {
	if len(totals) == 0 {
		return evs
	}
	last := map[string]int{}
	for i, ev := range evs {
		for _, k := range keysOf(ev) {
			if k != "" && totals[k] > 0 {
				last[k] = i
			}
		}
	}
	left := map[string]int64{}
	for k, n := range totals {
		left[k] = n
	}
	for i := range evs {
		for _, k := range keysOf(evs[i]) {
			if k == "" || left[k] <= 0 {
				continue
			}
			take := evs[i].RawBytes - evs[i].EmittedBytes - evs[i].RetrievedBytes
			if take < 0 {
				take = 0
			}
			if take > left[k] || i == last[k] {
				take = left[k]
			}
			evs[i].RetrievedBytes += take
			left[k] -= take
		}
	}
	return evs
}
