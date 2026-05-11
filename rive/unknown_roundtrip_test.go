package rive_test

import (
	"encoding/binary"
	"testing"

	"github.com/redriot-io/rive-go/rive"
)

// encodeVarint returns the unsigned LEB128 encoding of v.
func encodeVarint(v uint64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

// encodeUint32LE returns v as 4 little-endian bytes.
func encodeUint32LE(v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return b[:]
}

// buildUnknownTypeRiv constructs a minimal valid .riv binary containing:
//   - Backboard (typeKey=23, no properties)
//   - Artboard (typeKey=1, name="T")
//   - UnknownFutureType (typeKey=9999, k65000=uint(42), k65001=uint(7))
//
// Property keys 65000 and 65001 are synthetic — not in any known Rive schema —
// so they appear in the file's ToC with type=uint (field-index=0).
func buildUnknownTypeRiv() []byte {
	var buf []byte

	// Fingerprint "RIVE"
	buf = append(buf, 'R', 'I', 'V', 'E')
	// major=7, minor=0, fileID=0
	buf = append(buf, encodeVarint(7)...)
	buf = append(buf, encodeVarint(0)...)
	buf = append(buf, encodeVarint(0)...)

	// ToC: synthetic keys 65000 and 65001, both uint (field-index=0).
	// Terminator follows.
	buf = append(buf, encodeVarint(65000)...)
	buf = append(buf, encodeVarint(65001)...)
	buf = append(buf, encodeVarint(0)...) // terminator

	// ToC type bits: 1 uint32 for 2 keys.
	// bits 0-1 = type of key 65000 = 0 (uint)
	// bits 2-3 = type of key 65001 = 0 (uint)
	buf = append(buf, encodeUint32LE(0)...)

	// Object 1: Backboard (typeKey=23), no properties
	buf = append(buf, encodeVarint(23)...)
	buf = append(buf, encodeVarint(0)...) // prop terminator

	// Object 2: Artboard (typeKey=1), name="T" (key=4 is in globalPropTypes as string)
	buf = append(buf, encodeVarint(1)...)
	buf = append(buf, encodeVarint(4)...)  // key 4 = Component.name (string)
	buf = append(buf, encodeVarint(1)...)  // string len=1
	buf = append(buf, 'T')                 // "T"
	buf = append(buf, encodeVarint(0)...)  // prop terminator

	// Object 3: UnknownFutureType (typeKey=9999)
	buf = append(buf, encodeVarint(9999)...)
	buf = append(buf, encodeVarint(65000)...) // synthetic key
	buf = append(buf, encodeVarint(42)...)    // uint value 42
	buf = append(buf, encodeVarint(65001)...) // synthetic key
	buf = append(buf, encodeVarint(7)...)     // uint value 7
	buf = append(buf, encodeVarint(0)...)     // prop terminator

	return buf
}

// TestRoundTrip_UnknownTypeKey verifies that:
//  1. The reader does not panic or error on an unknown typeKey (9999).
//  2. The unknown object's typeKey and properties are preserved as-is.
//  3. WriteBytes re-emits synthetic property keys into the ToC so a subsequent
//     ReadBytes can successfully decode the round-tripped file.
//
// This proves forward-compat: future Rive types added to the format don't
// break existing files or corrupt the object stream on read→write→read.
func TestRoundTrip_UnknownTypeKey(t *testing.T) {
	original := buildUnknownTypeRiv()

	// ── Read 1 ────────────────────────────────────────────────────────────────
	f1, err := rive.ReadBytes(original)
	if err != nil {
		t.Fatalf("ReadBytes(original): %v — reader must not error on unknown typeKey 9999", err)
	}
	if len(f1.Objects) != 3 {
		t.Fatalf("object count after first read: got %d, want 3 (Backboard, Artboard, UnknownType)", len(f1.Objects))
	}

	// Verify Backboard and Artboard parsed correctly.
	if f1.Objects[0].TypeKey() != 23 {
		t.Errorf("objects[0] typeKey=%d, want 23 (Backboard)", f1.Objects[0].TypeKey())
	}
	if f1.Objects[1].TypeKey() != 1 {
		t.Errorf("objects[1] typeKey=%d, want 1 (Artboard)", f1.Objects[1].TypeKey())
	}

	// Verify the unknown object is preserved with the right typeKey.
	unknown := f1.Objects[2]
	if unknown.TypeKey() != 9999 {
		t.Fatalf("objects[2] typeKey=%d, want 9999 (unknown future type)", unknown.TypeKey())
	}

	// Verify both synthetic properties are present with correct values.
	props := unknown.Properties()
	if len(props) != 2 {
		t.Fatalf("unknown object prop count=%d, want 2 (k65000, k65001)", len(props))
	}
	if props[0].Key != 65000 {
		t.Errorf("props[0].Key=%d, want 65000", props[0].Key)
	}
	if v, ok := props[0].Value.(uint64); !ok || v != 42 {
		t.Errorf("props[0] (key=65000) Value=%v (%T), want uint64(42)", props[0].Value, props[0].Value)
	}
	if props[1].Key != 65001 {
		t.Errorf("props[1].Key=%d, want 65001", props[1].Key)
	}
	if v, ok := props[1].Value.(uint64); !ok || v != 7 {
		t.Errorf("props[1] (key=65001) Value=%v (%T), want uint64(7)", props[1].Value, props[1].Value)
	}

	// ── Write ─────────────────────────────────────────────────────────────────
	// WriteBytes must include synthetic keys 65000 and 65001 in the ToC so
	// the subsequent ReadBytes can look up their wire types.
	written, err := rive.WriteBytes(f1.Objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	// ── Read 2 ────────────────────────────────────────────────────────────────
	f2, err := rive.ReadBytes(written)
	if err != nil {
		t.Fatalf("ReadBytes(written): %v — writer must emit synthetic keys into ToC for round-trip decode", err)
	}
	if len(f2.Objects) != 3 {
		t.Fatalf("round-trip object count: got %d, want 3", len(f2.Objects))
	}

	// Verify the unknown type survived the full round-trip.
	u2 := f2.Objects[2]
	if u2.TypeKey() != 9999 {
		t.Errorf("round-trip objects[2] typeKey=%d, want 9999", u2.TypeKey())
	}
	p2 := u2.Properties()
	if len(p2) != 2 {
		t.Fatalf("round-trip unknown object prop count=%d, want 2", len(p2))
	}
	if p2[0].Key != 65000 || p2[0].Value.(uint64) != 42 {
		t.Errorf("round-trip props[0]: key=%d val=%v, want key=65000 val=42", p2[0].Key, p2[0].Value)
	}
	if p2[1].Key != 65001 || p2[1].Value.(uint64) != 7 {
		t.Errorf("round-trip props[1]: key=%d val=%v, want key=65001 val=7", p2[1].Key, p2[1].Value)
	}

	// Verify the round-trip file's ToC includes the synthetic keys.
	_, has65000 := f2.PropertyTypeOf(65000)
	_, has65001 := f2.PropertyTypeOf(65001)
	if !has65000 {
		t.Error("round-trip ToC missing key 65000 — writer did not preserve unknown property key")
	}
	if !has65001 {
		t.Error("round-trip ToC missing key 65001 — writer did not preserve unknown property key")
	}

	t.Logf("unknown type round-trip ok: typeKey=9999 preserved across read→write→read (%d→%d bytes)",
		len(original), len(written))
}
