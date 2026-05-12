package main

import (
	"encoding/binary"
	"fmt"

	"github.com/redriot-io/rive-go/rive"
)

// parseFvarAxes returns the axis tags present in the 'fvar' table of the given
// TTF/OTF binary. Returns nil axes (no error) if the font is not variable.
func parseFvarAxes(data []byte) ([]uint32, error) {
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
		return nil, nil
	}
	numTables, _ := u16(4)

	fvarOff := -1
	for i := 0; i < int(numTables); i++ {
		base := 12 + i*16
		tag, ok := u32(base)
		if !ok {
			break
		}
		if tag == 0x66766172 { // 'fvar'
			v, ok := u32(base + 8)
			if !ok {
				return nil, nil
			}
			fvarOff = int(v)
			break
		}
	}
	if fvarOff < 0 {
		return nil, nil // not a variable font
	}

	// fvar header: majorVersion(2) minorVersion(2) axesArrayOffset(2) reserved(2) axisCount(2) axisSize(2) ...
	axesArrayOffset, ok1 := u16(fvarOff + 4)
	axisCount, ok2 := u16(fvarOff + 8)
	axisSize, ok3 := u16(fvarOff + 10)
	if !ok1 || !ok2 || !ok3 {
		return nil, nil
	}
	if axisSize < 20 {
		axisSize = 20
	}

	axes := make([]uint32, 0, int(axisCount))
	for i := 0; i < int(axisCount); i++ {
		off := fvarOff + int(axesArrayOffset) + i*int(axisSize)
		tag, ok := u32(off)
		if !ok {
			break
		}
		axes = append(axes, tag)
	}
	return axes, nil
}

// unpackTag converts a packed uint64 OpenType tag to its 4-char string form.
func unpackTag(t uint64) string {
	return string([]byte{byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t)})
}

// verifyFvarAxes checks that every TextStyleAxis in f has its axis tag present
// in the embedded font's fvar table. Missing axes are reported as warnings.
func verifyFvarAxes(f *rive.File) (passes, errs []string) {
	objs := f.Objects

	// Collect FontAsset entries paired with their embedded bytes.
	type fontEntry struct {
		name  string
		bytes []byte
	}
	var fonts []fontEntry
	for i := 0; i < len(objs); i++ {
		if objs[i].TypeKey() != 141 { // FontAsset
			continue
		}
		name := ""
		for _, p := range objs[i].Properties() {
			if p.Key == 203 {
				name, _ = p.Value.(string)
			}
		}
		var fb []byte
		if i+1 < len(objs) && objs[i+1].TypeKey() == 106 {
			for _, p := range objs[i+1].Properties() {
				if p.Key == 212 {
					fb, _ = p.Value.([]byte)
				}
			}
			i++
		}
		fonts = append(fonts, fontEntry{name: name, bytes: fb})
	}

	if len(fonts) == 0 {
		return
	}

	// Find artboard (typeKey=1) — needed to resolve artboard-relative parentIds.
	artboardIdx := -1
	for i, o := range objs {
		if o.TypeKey() == 1 {
			artboardIdx = i
			break
		}
	}
	if artboardIdx < 0 {
		return
	}

	// Build styleFont: global object index → fontAssetId for TextStylePaint (tk=137).
	styleFont := make(map[int]int)
	for i, o := range objs {
		if o.TypeKey() != 137 {
			continue
		}
		for _, p := range o.Properties() {
			if p.Key == 279 { // fontAssetId
				if v, ok := p.Value.(uint64); ok {
					styleFont[i] = int(v)
				}
			}
		}
	}

	// Parse fvar for each font (cached).
	type fvarEntry struct {
		axes []uint32
	}
	fvars := make([]fvarEntry, len(fonts))
	for i, fe := range fonts {
		axes, _ := parseFvarAxes(fe.bytes)
		fvars[i] = fvarEntry{axes: axes}
	}

	// Check each TextStyleAxis (typeKey=144).
	for i, o := range objs {
		if o.TypeKey() != 144 {
			continue
		}

		var tag uint64
		var parentId uint64
		hasParent := false
		for _, p := range o.Properties() {
			switch p.Key {
			case 289: // tag
				if v, ok := p.Value.(uint64); ok {
					tag = v
				}
			case 5: // parentId (Component)
				if v, ok := p.Value.(uint64); ok {
					parentId = v
					hasParent = true
				}
			}
		}

		tagStr := unpackTag(tag)
		if !hasParent {
			continue
		}

		styleGlobal := artboardIdx + int(parentId)
		fontIdx, ok := styleFont[styleGlobal]
		if !ok {
			continue
		}
		if fontIdx < 0 || fontIdx >= len(fonts) {
			continue
		}

		fe := fvars[fontIdx]
		if fe.axes == nil {
			errs = append(errs, fmt.Sprintf("⚠ TextStyleAxis[%d] tag=%q: font %q has no fvar table (not a variable font)", i, tagStr, fonts[fontIdx].name))
			continue
		}

		found := false
		for _, a := range fe.axes {
			if a == uint32(tag) {
				found = true
				break
			}
		}
		if found {
			passes = append(passes, fmt.Sprintf("TextStyleAxis tag=%q: found in font %q fvar", tagStr, fonts[fontIdx].name))
		} else {
			errs = append(errs, fmt.Sprintf("⚠ TextStyleAxis[%d] tag=%q: axis not found in font %q fvar (%d axes present)", i, tagStr, fonts[fontIdx].name, len(fe.axes)))
		}
	}

	return
}
