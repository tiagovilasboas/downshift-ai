// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// Sampler keeps a raw copy of 1 in N outputs that no compressor handled, so
// the next compaction gain can be measured. It is local only (a separate
// directory from the store, same TTL), and each sample's metadata is code and
// counts: family, label, size and time, never a command, a path or text.
type Sampler struct {
	Dir      string
	TTL      time.Duration
	MaxTotal int64
	MinBytes int
	N        uint64
	Now      func() time.Time
}

// Defaults of the sampler.
const (
	DefaultSampleN        uint64 = 10
	DefaultSampleMinBytes        = 4 << 10
	DefaultSampleMaxTotal int64  = 20 << 20
)

// SampleMeta is the sidecar of one sample.
type SampleMeta struct {
	ID     string    `json:"id"`
	Family string    `json:"family"`
	Label  string    `json:"label"`
	Bytes  int       `json:"bytes"`
	Time   time.Time `json:"time"`
}

// DefaultSampler returns a Sampler under the Downshift state directory.
func DefaultSampler() (Sampler, error) {
	dir, err := paths.Join("ccr-samples")
	if err != nil {
		return Sampler{}, err
	}
	return Sampler{Dir: dir, TTL: 24 * time.Hour, MaxTotal: DefaultSampleMaxTotal,
		MinBytes: DefaultSampleMinBytes, N: DefaultSampleN, Now: time.Now}, nil
}

func (s Sampler) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// Chosen reports whether the sample with content sum is one of the 1 in N.
func (s Sampler) chosen(sum [32]byte) bool {
	n := s.N
	if n <= 1 {
		return true
	}
	return binary.BigEndian.Uint64(sum[:8])%n == 0
}

// cleanLabel keeps a label to code-like characters, so no text slips in.
func cleanLabel(v string) string {
	if len(v) > 40 {
		v = v[:40]
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.', r == ':':
			return r
		}
		return -1
	}, v)
}

// Maybe stores raw (with secret lines withheld) when it is at least MinBytes
// and the hash of its content picks it. It reports whether it stored one.
// Best effort: any error means not stored.
func (s Sampler) Maybe(raw []byte, family, label string) bool {
	if len(raw) < s.MinBytes {
		return false
	}
	clean := redact(raw)
	sum := sha256.Sum256(clean)
	if !s.chosen(sum) {
		return false
	}
	id := hex.EncodeToString(sum[:])[:12]
	if paths.PrivateDir(s.Dir) != nil {
		return false
	}
	now := s.now()
	meta, err := json.Marshal(SampleMeta{ID: id, Family: cleanLabel(family), Label: cleanLabel(label), Bytes: len(clean), Time: now.UTC()})
	if err != nil {
		return false
	}
	st := Store{Dir: s.Dir}
	if st.write(filepath.Join(s.Dir, id+".txt"), clean, now) != nil {
		return false
	}
	if st.write(filepath.Join(s.Dir, id+".json"), meta, now) != nil {
		return false
	}
	s.prune(id, now)
	return true
}

// Samples lists the live samples, oldest first.
func (s Sampler) Samples() []SampleMeta {
	files, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil
	}
	var out []SampleMeta
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Dir, f.Name()))
		var m SampleMeta
		if err != nil || json.Unmarshal(b, &m) != nil || !ValidID(m.ID) {
			continue
		}
		if s.TTL > 0 && s.now().Sub(m.Time) > s.TTL {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// Raw returns the stored bytes of a sample.
func (s Sampler) Raw(id string) ([]byte, error) {
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	return os.ReadFile(filepath.Join(s.Dir, id+".txt"))
}

type sampleFile struct {
	id    string
	size  int64
	mtime time.Time
}

// prune drops expired samples, then the oldest while over budget; never keep.
func (s Sampler) prune(keep string, now time.Time) {
	files, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}
	byID := map[string]*sampleFile{}
	var total int64
	for _, f := range files {
		info, err := f.Info()
		name := f.Name()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if strings.HasPrefix(name, ".put-") && now.Sub(info.ModTime()) > staleTemp {
			_ = os.Remove(filepath.Join(s.Dir, name))
			continue
		}
		ext := filepath.Ext(name)
		if ext != ".txt" && ext != ".json" {
			continue
		}
		id := strings.TrimSuffix(name, ext)
		e := byID[id]
		if e == nil {
			e = &sampleFile{id: id, mtime: info.ModTime()}
			byID[id] = e
		}
		e.size += info.Size()
		total += info.Size()
	}
	var live []*sampleFile
	for _, e := range byID {
		if e.id != keep && s.TTL > 0 && now.Sub(e.mtime) > s.TTL {
			s.remove(e.id)
			total -= e.size
			continue
		}
		live = append(live, e)
	}
	if s.MaxTotal <= 0 {
		return
	}
	sort.Slice(live, func(i, j int) bool { return live[i].mtime.Before(live[j].mtime) })
	for _, e := range live {
		if total <= s.MaxTotal {
			return
		}
		if e.id != keep {
			s.remove(e.id)
			total -= e.size
		}
	}
}

func (s Sampler) remove(id string) {
	_ = os.Remove(filepath.Join(s.Dir, id+".txt"))
	_ = os.Remove(filepath.Join(s.Dir, id+".json"))
}
