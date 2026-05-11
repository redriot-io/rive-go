// Package fontcheck parses TTF/OTF cmap tables to verify glyph coverage.
package fontcheck

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ParseCmap parses the 'cmap' table from a raw TTF/OTF byte slice and returns
// a function that reports whether a rune maps to a non-zero glyph ID.
//
// It targets platform 3 (Windows), encoding 1 (Unicode BMP), format 4 — the
// subtable present in virtually every font that covers standard Unicode text.
// Returns an error for malformed fonts; never panics.
func ParseCmap(data []byte) (func(r rune) bool, error) {
	u16 := func(off int) (uint16, bool) {
		if off < 0 || off+2 > len(data) {
			return 0, false
		}
		return binary.BigEndian.Uint16(data[off:]), true
	}
	u32 := func(off int) (uint32, bool) {
		if off < 0 || off+4 > len(data) {
			return 0, false
		}
		return binary.BigEndian.Uint32(data[off:]), true
	}

	if len(data) < 12 {
		return nil, errors.New("fontcheck: data too short")
	}
	numTables, _ := u16(4) // TrueType offset table: sfVersion(4) numTables(2)

	// Locate 'cmap' in the table directory (12 bytes header, 16 bytes/entry).
	cmapOff := -1
	for i := 0; i < int(numTables); i++ {
		base := 12 + i*16
		tag, ok := u32(base)
		if !ok {
			break
		}
		if tag == 0x636D6170 { // 'cmap'
			v, ok := u32(base + 8)
			if !ok {
				return nil, errors.New("fontcheck: bad cmap table offset")
			}
			cmapOff = int(v)
			break
		}
	}
	if cmapOff < 0 {
		return nil, errors.New("fontcheck: no cmap table")
	}

	// cmap header: version(2) numSubtables(2)
	numSub, ok := u16(cmapOff + 2)
	if !ok {
		return nil, errors.New("fontcheck: bad cmap header")
	}

	// Find platform=3 encoding=1 (Windows Unicode BMP) subtable.
	subOff := -1
	for i := 0; i < int(numSub); i++ {
		base := cmapOff + 4 + i*8
		pid, ok1 := u16(base)
		eid, ok2 := u16(base + 2)
		rel, ok3 := u32(base + 4)
		if !ok1 || !ok2 || !ok3 {
			break
		}
		if pid == 3 && eid == 1 {
			subOff = cmapOff + int(rel)
			break
		}
	}
	if subOff < 0 {
		return nil, errors.New("fontcheck: no Windows Unicode BMP cmap (platform 3, encoding 1)")
	}

	format, ok := u16(subOff)
	if !ok {
		return nil, errors.New("fontcheck: cannot read cmap subtable format")
	}
	if format != 4 {
		return nil, fmt.Errorf("fontcheck: cmap subtable format %d, want 4", format)
	}

	segCountX2, ok := u16(subOff + 6)
	if !ok {
		return nil, errors.New("fontcheck: bad format 4 header")
	}
	segCount := int(segCountX2) / 2
	if segCount == 0 {
		return func(rune) bool { return false }, nil
	}

	// Format 4 layout within the subtable (all offsets from subOff):
	//   [14]                 endCode[segCount]
	//   [14+segCount*2]      reservedPad (2 bytes)
	//   [14+segCount*2+2]    startCode[segCount]
	//   [14+segCount*4+2]    idDelta[segCount]
	//   [14+segCount*6+2]    idRangeOffset[segCount]
	//   [14+segCount*8+2 ..] glyphIdArray
	endCodeBase      := subOff + 14
	startCodeBase    := endCodeBase + segCount*2 + 2
	idDeltaBase      := startCodeBase + segCount*2
	idRangeOffBase   := idDeltaBase + segCount*2

	endCodes      := make([]uint16, segCount)
	startCodes    := make([]uint16, segCount)
	idDeltas      := make([]int16, segCount)
	idRangeOffs   := make([]uint16, segCount)
	for i := 0; i < segCount; i++ {
		ec, ok1 := u16(endCodeBase + i*2)
		sc, ok2 := u16(startCodeBase + i*2)
		d,  ok3 := u16(idDeltaBase + i*2)
		ro, ok4 := u16(idRangeOffBase + i*2)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, fmt.Errorf("fontcheck: segment %d read out of range (segCount=%d)", i, segCount)
		}
		endCodes[i]    = ec
		startCodes[i]  = sc
		idDeltas[i]    = int16(d)
		idRangeOffs[i] = ro
	}

	hasGlyph := func(r rune) bool {
		if r < 0 || r > 0xFFFF {
			return false // BMP cmap covers U+0000–U+FFFF only
		}
		c := uint16(r)
		for i := 0; i < segCount; i++ {
			if c > endCodes[i] {
				continue
			}
			if c < startCodes[i] {
				return false // gap between segments; not covered
			}
			// c is in [startCodes[i], endCodes[i]]
			if idRangeOffs[i] == 0 {
				return (int(c)+int(idDeltas[i]))&0xFFFF != 0
			}
			// idRangeOffset[i] is a byte offset measured from the address of
			// idRangeOffset[i] itself (OpenType spec §cmap format 4).
			ptrBase := idRangeOffBase + i*2
			glyphByte := ptrBase + int(idRangeOffs[i]) + int(c-startCodes[i])*2
			gid, ok := u16(glyphByte)
			if !ok || gid == 0 {
				return false
			}
			return (int(gid)+int(idDeltas[i]))&0xFFFF != 0
		}
		return false
	}

	return hasGlyph, nil
}
