package rive

import (
	"io"

	"github.com/redriot-io/rive-go/internal/encoding"
)

const (
	riveFingerprint = "RIVE"
	defaultMajor    = uint32(7)
	defaultMinor    = uint32(0)
)

// WriteOption configures WriteFile / WriteBytes.
type WriteOption func(*writeConfig)

type writeConfig struct {
	major  uint32
	minor  uint32
	fileID uint64
}

func defaultWriteConfig() *writeConfig {
	return &writeConfig{major: defaultMajor, minor: defaultMinor}
}

// WithFileID sets the file ID embedded in the header (default 0).
func WithFileID(id uint64) WriteOption { return func(c *writeConfig) { c.fileID = id } }

// WithMajorVersion sets the major version (default 7).
func WithMajorVersion(v uint32) WriteOption { return func(c *writeConfig) { c.major = v } }

// WithMinorVersion sets the minor version (default 0).
func WithMinorVersion(v uint32) WriteOption { return func(c *writeConfig) { c.minor = v } }

// WriteBytes serializes objects to .riv format and returns the bytes.
func WriteBytes(objects []Object, opts ...WriteOption) ([]byte, error) {
	cfg := defaultWriteConfig()
	for _, o := range opts {
		o(cfg)
	}
	bw := encoding.NewWriter()
	if err := writeRiv(bw, objects, cfg); err != nil {
		return nil, err
	}
	// Return a copy so callers can't mutate the internal buffer.
	out := make([]byte, bw.Len())
	copy(out, bw.Bytes())
	return out, nil
}

// WriteFile serializes objects to .riv format and writes to w.
func WriteFile(w io.Writer, objects []Object, opts ...WriteOption) error {
	b, err := WriteBytes(objects, opts...)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func writeRiv(bw *encoding.BinaryWriter, objects []Object, cfg *writeConfig) error {
	// ── Header ───────────────────────────────────────────────────────────────
	for i := 0; i < len(riveFingerprint); i++ {
		bw.WriteByte(riveFingerprint[i])
	}
	bw.WriteVarUint(uint64(cfg.major))
	bw.WriteVarUint(uint64(cfg.minor))
	bw.WriteVarUint(cfg.fileID)

	// ── Collect ToC ───────────────────────────────────────────────────────────
	// Scan all objects in order, collecting unique property keys on first
	// occurrence. The order here determines the ToC listing order, which must
	// match the bit-pack order written below.
	type tocEntry struct {
		key     uint32
		propType PropertyType
	}
	var orderedToc []tocEntry
	seenKey := map[uint32]bool{}

	for _, obj := range objects {
		for _, p := range obj.Properties() {
			if seenKey[p.Key] {
				continue
			}
			seenKey[p.Key] = true
			// Only include keys from the format contract's ToC allowlist.
			// CoreRegistry-known keys (the vast majority) are excluded from the ToC;
			// the runtime resolves their types from its compiled-in table.
			// ToCIncludeKeys encodes the field index directly, including the
			// bytes-proxy rule (key 212 → field_index=1 instead of 4).
			fieldIdx, include := ToCIncludeKeys[p.Key]
			if !include {
				continue
			}
			orderedToc = append(orderedToc, tocEntry{p.Key, PropertyType(fieldIdx)})
		}
	}

	// ── Write ToC: keys terminated by 0 ──────────────────────────────────────
	for _, e := range orderedToc {
		bw.WriteVarUint(uint64(e.key))
	}
	bw.WriteVarUint(0) // terminator

	// ── Write type bits: 1 uint32 per 4 keys ─────────────────────────────────
	// C++ reading algorithm (runtime_header.hpp):
	//   currentBit starts at 8 (forces immediate uint32 read on first key).
	//   For key at index i: belongs to uint32 at index i/4, bit offset (i%4)*2.
	//   Only bits 0-7 of each uint32 carry data (4 keys × 2 bits).
	numWords := (len(orderedToc) + 3) / 4
	words := make([]uint32, numWords)
	for i, e := range orderedToc {
		wordIdx := i / 4
		bitPos := uint32((i % 4) * 2)
		words[wordIdx] |= uint32(e.propType&3) << bitPos
	}
	for _, w := range words {
		bw.WriteUint32(w)
	}

	// ── Object stream ─────────────────────────────────────────────────────────
	for _, obj := range objects {
		bw.WriteVarUint(uint64(obj.TypeKey()))
		for _, p := range obj.Properties() {
			bw.WriteVarUint(uint64(p.Key))
			writePropertyValue(bw, p)
		}
		bw.WriteVarUint(0) // property terminator
	}

	return nil
}

func writePropertyValue(bw *encoding.BinaryWriter, p Property) {
	switch p.Type {
	case PropertyTypeUint:
		bw.WriteVarUint(p.Value.(uint64))
	case PropertyTypeString:
		bw.WriteString(p.Value.(string))
	case PropertyTypeFloat:
		bw.WriteFloat32(float32(p.Value.(float64)))
	case PropertyTypeColor:
		// Color values stored as uint64 in Property.Value (cast from uint32 field).
		bw.WriteColor(uint32(p.Value.(uint64)))
	case PropertyTypeBytes:
		bw.WriteBytes(p.Value.([]byte))
	}
}
