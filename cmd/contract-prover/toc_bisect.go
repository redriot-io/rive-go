package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

// runToCBisection runs ToC key bisection for all proven types.
// For each type, it tests which ToC keys in the baseline .riv are REQUIRED —
// i.e., removing that key from the ToC causes WASM to fail.
// Returns a map from typeName → sorted list of required key ints.
func runToCBisection(harness string) map[string][]int {
	results := make(map[string][]int)
	for _, typeName := range typeOrder {
		buildFn, ok := buildFuncs[typeName]
		if !ok {
			continue
		}
		required, err := tocBisectType(typeName, harness, buildFn)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  ERROR: %v\n", typeName, err)
			results[typeName] = []int{}
			continue
		}
		results[typeName] = required
	}
	return results
}

// tocBisectType discovers which ToC keys in the baseline .riv are required.
// Returns a sorted list of required key ints (may be empty/nil).
func tocBisectType(typeName, harness string, buildFn func() ([]byte, error)) ([]int, error) {
	baseline, err := buildFn()
	if err != nil {
		return nil, fmt.Errorf("build baseline: %w", err)
	}

	tocKeys, err := parseToCKeys(baseline)
	if err != nil {
		return nil, fmt.Errorf("parse ToC: %w", err)
	}

	if len(tocKeys) == 0 {
		fmt.Printf("TOC_BISECT %-20s  no ToC keys\n", typeName)
		return []int{}, nil
	}

	fmt.Printf("TOC_BISECT %-20s  testing %d key(s): %v\n", typeName, len(tocKeys), tocKeys)

	// Verify baseline passes before bisecting.
	baselinePath, err := writeTempRiv(baseline)
	if err != nil {
		return nil, fmt.Errorf("write baseline temp: %w", err)
	}
	defer os.Remove(baselinePath)

	code, _, _ := runHarness(harness, baselinePath)
	if code != 0 {
		fmt.Printf("TOC_BISECT %-20s  WARNING: baseline fails (exit=%d) — skipping\n", typeName, code)
		return []int{}, nil
	}

	// Test each key: remove it from the ToC and see if WASM fails.
	var required []int
	for _, key := range tocKeys {
		patched, err := rebuildWithoutToCKey(baseline, key)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  key=%d: patch error: %v\n", typeName, key, err)
			continue
		}
		path, err := writeTempRiv(patched)
		if err != nil {
			fmt.Printf("TOC_BISECT %-20s  key=%d: write temp error: %v\n", typeName, key, err)
			continue
		}
		exitCode, _, _ := runHarness(harness, path)
		os.Remove(path)

		if exitCode != 0 {
			fmt.Printf("TOC_REQUIRED %-20s  key=%d (removing causes exit=%d)\n", typeName, key, exitCode)
			required = append(required, int(key))
		} else {
			fmt.Printf("TOC_OPTIONAL %-20s  key=%d\n", typeName, key)
		}
	}

	sort.Ints(required)
	return required, nil
}

// parseToCKeys reads the property keys listed in the ToC section of a .riv file.
// Format after "RIVE" + 3 varints (major, minor, fileID):
//
//	varuint(key1), varuint(key2), ..., varuint(0)  ← 0 is the terminator
func parseToCKeys(data []byte) ([]uint32, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short (%d bytes)", len(data))
	}
	pos := 4 // skip "RIVE" fingerprint
	// Skip major, minor, fileID varints.
	for i := 0; i < 3; i++ {
		_, pos = tocDecodeVarUint(data, pos)
	}
	var keys []uint32
	for pos < len(data) {
		k, newPos := tocDecodeVarUint(data, pos)
		pos = newPos
		if k == 0 {
			break
		}
		keys = append(keys, uint32(k))
	}
	return keys, nil
}

// rebuildWithoutToCKey returns a copy of the .riv bytes with excludeKey removed from the ToC.
// The object stream is left untouched; only the ToC header section is patched.
func rebuildWithoutToCKey(data []byte, excludeKey uint32) ([]byte, error) {
	return rebuildWithoutToCKeys(data, map[uint32]bool{excludeKey: true})
}

// rebuildWithoutToCKeys returns a copy of the .riv bytes with all excludeKeys removed from ToC.
//
// .riv binary layout (after fingerprint):
//
//	varuint(major) varuint(minor) varuint(fileID)
//	[varuint(key) ...] varuint(0)          ← ToC key list
//	[uint32 ...]                            ← type-bit words (1 per 4 keys)
//	[object stream]
func rebuildWithoutToCKeys(data []byte, excludeKeys map[uint32]bool) ([]byte, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("data too short (%d bytes)", len(data))
	}
	var out bytes.Buffer

	// Copy "RIVE" fingerprint (4 bytes).
	out.Write(data[:4])
	pos := 4

	// Copy major, minor, fileID varints verbatim.
	for i := 0; i < 3; i++ {
		start := pos
		for pos < len(data) && data[pos]&0x80 != 0 {
			pos++
		}
		pos++ // include final byte of this varint
		out.Write(data[start:pos])
	}

	// Parse the existing ToC key list.
	type tocEntry struct {
		key      uint32
		propType uint8
	}
	var allEntries []tocEntry
	for pos < len(data) {
		k, newPos := tocDecodeVarUint(data, pos)
		pos = newPos
		if k == 0 {
			break
		}
		allEntries = append(allEntries, tocEntry{key: uint32(k)})
	}

	// Decode type bits that follow the key list.
	oldCount := len(allEntries)
	oldWordCount := (oldCount + 3) / 4
	typeBitsStart := pos
	if typeBitsStart+oldWordCount*4 > len(data) {
		return nil, fmt.Errorf("type bits extend past end of data (need %d bytes at %d, have %d)",
			oldWordCount*4, typeBitsStart, len(data)-typeBitsStart)
	}
	for i := range allEntries {
		wordIdx := i / 4
		bitPos := uint(i%4) * 2
		off := typeBitsStart + wordIdx*4
		word := uint32(data[off]) |
			uint32(data[off+1])<<8 |
			uint32(data[off+2])<<16 |
			uint32(data[off+3])<<24
		allEntries[i].propType = uint8((word >> bitPos) & 3)
	}
	typeBitsEnd := typeBitsStart + oldWordCount*4

	// Build filtered entry list (excluding requested keys).
	var filtered []tocEntry
	for _, e := range allEntries {
		if !excludeKeys[e.key] {
			filtered = append(filtered, e)
		}
	}

	// Write new ToC key list.
	for _, e := range filtered {
		tocEncodeVarUint(&out, uint64(e.key))
	}
	tocEncodeVarUint(&out, 0) // terminator

	// Write new type-bit words.
	newWordCount := (len(filtered) + 3) / 4
	newWords := make([]uint32, newWordCount)
	for i, e := range filtered {
		wordIdx := i / 4
		bitPos := uint(i%4) * 2
		newWords[wordIdx] |= uint32(e.propType) << bitPos
	}
	for _, w := range newWords {
		out.WriteByte(byte(w))
		out.WriteByte(byte(w >> 8))
		out.WriteByte(byte(w >> 16))
		out.WriteByte(byte(w >> 24))
	}

	// Append the object stream unchanged.
	out.Write(data[typeBitsEnd:])

	return out.Bytes(), nil
}

// tocDecodeVarUint reads a LEB128 unsigned varint from data[pos].
func tocDecodeVarUint(data []byte, pos int) (uint64, int) {
	var result uint64
	var shift uint
	for pos < len(data) {
		b := data[pos]
		pos++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, pos
}

// tocEncodeVarUint appends a LEB128 unsigned varint to buf.
func tocEncodeVarUint(buf *bytes.Buffer, v uint64) {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if v == 0 {
			break
		}
	}
}

// tocUnion returns the sorted union of two int slices (no duplicates).
func tocUnion(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		seen[v] = true
	}
	if len(seen) == 0 {
		return []int{}
	}
	out := make([]int, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// tocCoerce converts nil to an empty slice so JSON emits [] instead of null.
func tocCoerce(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
