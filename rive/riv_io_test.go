package rive_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/redriot-io/rive-go/rive"
)

// ── Fixture helpers ───────────────────────────────────────────────────────────

const fixtureDir = "../test/fixtures"

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

// ── ReadFile: golden fixtures ─────────────────────────────────────────────────

func TestReadFile_BallTest(t *testing.T) {
	data := loadFixture(t, "ball_test.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes(ball_test.riv) = %v", err)
	}
	if len(f.Objects) == 0 {
		t.Fatal("expected at least one object")
	}
	t.Logf("ball_test.riv: %d objects, major=%d minor=%d fileID=%d",
		len(f.Objects), f.MajorVersion, f.MinorVersion, f.FileID)
}

func TestReadFile_ArtboardWidth(t *testing.T) {
	data := loadFixture(t, "artboard_width_test.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes(artboard_width_test.riv) = %v", err)
	}
	if len(f.Objects) == 0 {
		t.Fatal("expected at least one object")
	}
	t.Logf("artboard_width_test.riv: %d objects", len(f.Objects))
}

func TestReadFile_BlendTest(t *testing.T) {
	data := loadFixture(t, "blend_test.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes(blend_test.riv) = %v", err)
	}
	if len(f.Objects) == 0 {
		t.Fatal("expected at least one object")
	}
	t.Logf("blend_test.riv: %d objects", len(f.Objects))
}

func TestReadFile_ClickEvent(t *testing.T) {
	data := loadFixture(t, "click_event.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes(click_event.riv) = %v", err)
	}
	t.Logf("click_event.riv: %d objects", len(f.Objects))
}

func TestReadFile_CubicValueTest(t *testing.T) {
	data := loadFixture(t, "cubic_value_test.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes(cubic_value_test.riv) = %v", err)
	}
	t.Logf("cubic_value_test.riv: %d objects", len(f.Objects))
}

func TestReadFile_ReadsCorrectVersions(t *testing.T) {
	data := loadFixture(t, "blend_test.riv")
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if f.MajorVersion != 7 {
		t.Errorf("MajorVersion = %d, want 7", f.MajorVersion)
	}
	if f.MinorVersion != 0 {
		t.Errorf("MinorVersion = %d, want 0", f.MinorVersion)
	}
}

// ── ReadFile: error cases ─────────────────────────────────────────────────────

func TestReadFile_InvalidFingerprint(t *testing.T) {
	bad := []byte("XXXX\x07\x00\x00")
	_, err := rive.ReadBytes(bad)
	if err == nil {
		t.Fatal("expected error for invalid fingerprint")
	}
	t.Logf("got expected error: %v", err)
}

func TestReadFile_WrongMajor(t *testing.T) {
	// RIVE + major=99 + rest zeroed
	bad := append([]byte("RIVE"), 99, 0, 0)
	_, err := rive.ReadBytes(bad)
	if err == nil {
		t.Fatal("expected error for wrong major version")
	}
	t.Logf("got expected error: %v", err)
}

func TestReadFile_Empty(t *testing.T) {
	_, err := rive.ReadBytes([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestReadFile_Truncated(t *testing.T) {
	data := loadFixture(t, "blend_test.riv")
	// Truncate at various points — all should produce errors.
	cutpoints := []int{1, 2, 3, 5, 7, 10, 15, 20}
	for _, cut := range cutpoints {
		if cut >= len(data) {
			break
		}
		truncated := data[:cut]
		_, err := rive.ReadBytes(truncated)
		if err == nil {
			t.Errorf("ReadBytes(truncated at %d) expected error, got nil", cut)
		}
	}
}

// ── WriteFile: header verification ───────────────────────────────────────────

func TestWrite_Header(t *testing.T) {
	out, err := rive.WriteBytes(nil) // no objects
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 7 {
		t.Fatalf("output too short: %d bytes", len(out))
	}
	// fingerprint
	if string(out[0:4]) != "RIVE" {
		t.Errorf("fingerprint = %q, want RIVE", out[0:4])
	}
	// major = 7 (single-byte LEB128 = 7)
	if out[4] != 7 {
		t.Errorf("major = %d, want 7", out[4])
	}
	// minor = 0
	if out[5] != 0 {
		t.Errorf("minor = %d, want 0", out[5])
	}
	// file_id = 0
	if out[6] != 0 {
		t.Errorf("file_id[0] = %d, want 0", out[6])
	}
	t.Logf("header OK: %x", out[:len(out)])
}

func TestWrite_HeaderWithOptions(t *testing.T) {
	out, err := rive.WriteBytes(nil, rive.WithFileID(42), rive.WithMinorVersion(3))
	if err != nil {
		t.Fatal(err)
	}
	if string(out[0:4]) != "RIVE" {
		t.Errorf("fingerprint wrong")
	}
	// major=7, minor=3, fileID=42
	if out[4] != 7 {
		t.Errorf("major = %d", out[4])
	}
	if out[5] != 3 {
		t.Errorf("minor = %d", out[5])
	}
	if out[6] != 42 {
		t.Errorf("fileID byte = %d", out[6])
	}
}

// ── WriteFile: ToC structure ──────────────────────────────────────────────────

func TestWrite_ToC_Empty(t *testing.T) {
	out, err := rive.WriteBytes(nil)
	if err != nil {
		t.Fatal(err)
	}
	// With no objects: header (7 bytes) + ToC terminator (1 byte) = 8 bytes.
	// No type words needed (0 keys → 0 uint32s).
	if len(out) != 8 {
		t.Errorf("empty file = %d bytes, want 8", len(out))
	}
	// Byte 7 should be ToC terminator 0.
	if out[7] != 0 {
		t.Errorf("ToC terminator = %d, want 0", out[7])
	}
}

func TestWrite_ToC_SingleUintProperty(t *testing.T) {
	// Write a single object with one uint property.
	// ToC should: key (varuint) + 0 terminator + 1 uint32 word (4 bytes).
	n := &rive.Node{}
	n.X = 1.0 // float, key 13
	out, err := rive.WriteBytes([]rive.Object{n})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Node{X:1} output: %x", out)
	// Verify can be read back.
	f, err := rive.ReadBytes(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(f.Objects) != 1 {
		t.Fatalf("objects = %d, want 1", len(f.Objects))
	}
}

// ── WriteFile: object stream ──────────────────────────────────────────────────

func TestWrite_ObjectStream(t *testing.T) {
	b := &rive.Backboard{}
	out, err := rive.WriteBytes([]rive.Object{b})
	if err != nil {
		t.Fatal(err)
	}
	f, err := rive.ReadBytes(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(f.Objects) != 1 {
		t.Fatalf("objects = %d, want 1", len(f.Objects))
	}
	if f.Objects[0].TypeKey() != 23 {
		t.Errorf("typeKey = %d, want 23 (Backboard)", f.Objects[0].TypeKey())
	}
}

// ── Round-trip: minimal ───────────────────────────────────────────────────────

func TestWriteRead_RoundTrip_Minimal(t *testing.T) {
	// Build a minimal valid scene: Backboard + Artboard with name+width+height.
	bb := &rive.Backboard{}

	art := &rive.Artboard{}
	art.Name = "TestArtboard"
	art.Width = 320
	art.Height = 240

	objects := []rive.Object{bb, art}

	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	t.Logf("wrote %d bytes", len(data))

	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if len(f.Objects) != 2 {
		t.Fatalf("objects = %d, want 2", len(f.Objects))
	}
	if f.Objects[0].TypeKey() != 23 { // Backboard
		t.Errorf("obj[0] typeKey = %d, want 23", f.Objects[0].TypeKey())
	}
	if f.Objects[1].TypeKey() != 1 { // Artboard
		t.Errorf("obj[1] typeKey = %d, want 1", f.Objects[1].TypeKey())
	}

	// Check Artboard name and dimensions via Properties().
	artProps := f.Objects[1].Properties()
	propMap := make(map[uint32]rive.Property)
	for _, p := range artProps {
		propMap[p.Key] = p
	}

	nameProp, ok := propMap[4] // Component.Name key=4, string
	if !ok {
		t.Error("name property (key=4) missing from round-tripped Artboard")
	} else if nameProp.Value.(string) != "TestArtboard" {
		t.Errorf("name = %q, want %q", nameProp.Value.(string), "TestArtboard")
	}
}

// ── Round-trip: all property types ───────────────────────────────────────────

func TestWriteRead_RoundTrip_AllTypes(t *testing.T) {
	// Verify each PropertyType encodes and decodes correctly.

	// uint: Node.ParentId (key=5), string: Component.Name (key=4), float: Node.X (key=13)
	n := &rive.Node{}
	n.ParentId = 7
	n.X = 42.5
	n.Name = "anim1" // Component.Name is key=4

	// bool (stored as uint): LinearAnimation.Quantize (key=376)
	la := &rive.LinearAnimation{}
	la.Quantize = true

	// color: SolidColor.ColorValue (key=37)
	sc := &rive.SolidColor{}
	sc.ColorValue = 0xFF3344AA

	objects := []rive.Object{n, la, sc}
	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if len(f.Objects) != 3 {
		t.Fatalf("objects = %d, want 3", len(f.Objects))
	}

	// Node: check ParentId, X, and Name (Component.Name = key 4)
	nProps := propsByKey(f.Objects[0].Properties())
	if v, ok := nProps[5]; ok {
		if v.Value.(uint64) != 7 {
			t.Errorf("Node.ParentId = %v, want 7", v.Value)
		}
	} else {
		t.Error("Node.ParentId (key=5) missing")
	}
	if v, ok := nProps[13]; ok {
		got := v.Value.(float64)
		if math.Abs(got-42.5) > 1e-3 {
			t.Errorf("Node.X = %v, want 42.5", got)
		}
	} else {
		t.Error("Node.X (key=13) missing")
	}
	if v, ok := nProps[4]; ok {
		if v.Value.(string) != "anim1" {
			t.Errorf("Name = %q, want anim1", v.Value.(string))
		}
	} else {
		t.Error("Name (key=4) missing")
	}

	// LinearAnimation: check Quantize
	laProps := propsByKey(f.Objects[1].Properties())
	if v, ok := laProps[376]; ok {
		if v.Value.(uint64) != 1 {
			t.Errorf("LinearAnimation.Quantize = %v, want 1", v.Value)
		}
	} else {
		t.Error("Quantize (key=376) missing")
	}

	// SolidColor: check color
	scProps := propsByKey(f.Objects[2].Properties())
	if v, ok := scProps[37]; ok {
		if v.Value.(uint64) != uint64(0xFF3344AA) {
			t.Errorf("ColorValue = 0x%08X, want 0xFF3344AA", v.Value.(uint64))
		}
	} else {
		t.Error("SolidColor.ColorValue (key=37) missing")
	}
}

// ── Round-trip: golden file ───────────────────────────────────────────────────

func TestRoundTrip_GoldenFile_Blend(t *testing.T) {
	roundTripGolden(t, "blend_test.riv")
}

func TestRoundTrip_GoldenFile_Ball(t *testing.T) {
	roundTripGolden(t, "ball_test.riv")
}

func TestRoundTrip_GoldenFile_ArtboardWidth(t *testing.T) {
	roundTripGolden(t, "artboard_width_test.riv")
}

func roundTripGolden(t *testing.T, name string) {
	t.Helper()
	data := loadFixture(t, name)

	// First read.
	file1, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("first ReadBytes(%s): %v", name, err)
	}

	// Write back.
	data2, err := rive.WriteBytes(file1.Objects, rive.WithFileID(file1.FileID))
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	// Second read.
	file2, err := rive.ReadBytes(data2)
	if err != nil {
		t.Fatalf("second ReadBytes: %v", err)
	}

	// Structural comparison.
	if len(file1.Objects) != len(file2.Objects) {
		t.Fatalf("objects: %d → %d (mismatch after round-trip)", len(file1.Objects), len(file2.Objects))
	}
	for i, o1 := range file1.Objects {
		o2 := file2.Objects[i]
		if o1.TypeKey() != o2.TypeKey() {
			t.Errorf("obj[%d] typeKey: %d → %d", i, o1.TypeKey(), o2.TypeKey())
		}
		compareProperties(t, i, o1.Properties(), o2.Properties())
	}
	t.Logf("round-trip %s: %d objects ✓", name, len(file1.Objects))
}

// compareProperties checks that two property slices contain the same key-value
// pairs (order-independent via map).
func compareProperties(t *testing.T, objIdx int, p1, p2 []rive.Property) {
	t.Helper()
	m1 := propsByKey(p1)
	m2 := propsByKey(p2)

	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok {
			t.Errorf("obj[%d] prop key %d present in original, missing after round-trip", objIdx, k)
			continue
		}
		if v1.Type != v2.Type {
			t.Errorf("obj[%d] prop %d type: %d → %d", objIdx, k, v1.Type, v2.Type)
			continue
		}
		switch v1.Type {
		case rive.PropertyTypeFloat:
			// Float round-trip through float32 — compare within float32 precision.
			f1 := float32(v1.Value.(float64))
			f2 := float32(v2.Value.(float64))
			if math.Float32bits(f1) != math.Float32bits(f2) {
				t.Errorf("obj[%d] prop %d float: %v → %v", objIdx, k, f1, f2)
			}
		default:
			if v1.Value != v2.Value {
				// For []byte, compare byte-by-byte.
				b1, isBytes := v1.Value.([]byte)
				b2, _ := v2.Value.([]byte)
				if isBytes {
					if !bytes.Equal(b1, b2) {
						t.Errorf("obj[%d] prop %d bytes differ", objIdx, k)
					}
				} else {
					t.Errorf("obj[%d] prop %d value: %v → %v", objIdx, k, v1.Value, v2.Value)
				}
			}
		}
	}
	for k := range m2 {
		if _, ok := m1[k]; !ok {
			t.Errorf("obj[%d] prop key %d added after round-trip (wasn't in original)", objIdx, k)
		}
	}
}

// ── GenericObject: forward compatibility ──────────────────────────────────────

func TestGenericObject_UnknownTypeKey(t *testing.T) {
	// Craft a minimal .riv with a known property type (uint, key=4) but
	// an unknown type key (9999). Reader should return a GenericObject.
	var buf bytes.Buffer

	w := newManualWriter(&buf)
	w.writeRaw("RIVE")
	w.writeLEB128(7)   // major
	w.writeLEB128(0)   // minor
	w.writeLEB128(0)   // fileID
	// ToC: key 4 (name, string)
	w.writeLEB128(4)
	w.writeLEB128(0)   // terminator
	// Type bits: 1 uint32, key 4 = string type (1) at bits 0-1
	w.writeUint32LE(1) // bits 0-1 = 01 = string
	// Object: unknown type key 9999, prop 4 = "hello", then 0
	w.writeLEB128(9999)
	w.writeLEB128(4)
	w.writeString("hello")
	w.writeLEB128(0) // end of object

	f, err := rive.ReadBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("ReadBytes with unknown type: %v", err)
	}
	if len(f.Objects) != 1 {
		t.Fatalf("objects = %d, want 1", len(f.Objects))
	}
	if f.Objects[0].TypeKey() != 9999 {
		t.Errorf("typeKey = %d, want 9999", f.Objects[0].TypeKey())
	}
	props := f.Objects[0].Properties()
	if len(props) != 1 || props[0].Key != 4 || props[0].Value.(string) != "hello" {
		t.Errorf("properties = %v, want [{4 string hello}]", props)
	}
}

// ── ReadFile: io.Reader interface ─────────────────────────────────────────────

func TestReadFile_IoReader(t *testing.T) {
	data := loadFixture(t, "blend_test.riv")
	r := bytes.NewReader(data)
	f, err := rive.ReadFile(r)
	if err != nil {
		t.Fatalf("ReadFile(io.Reader): %v", err)
	}
	if len(f.Objects) == 0 {
		t.Error("no objects parsed")
	}
}

// ── WriteFile: io.Writer interface ────────────────────────────────────────────

func TestWriteFile_IoWriter(t *testing.T) {
	var buf bytes.Buffer
	n := &rive.Node{}
	n.X = 1.5
	err := rive.WriteFile(&buf, []rive.Object{n})
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if buf.Len() < 8 {
		t.Fatalf("output too short: %d bytes", buf.Len())
	}
	if string(buf.Bytes()[:4]) != "RIVE" {
		t.Error("missing RIVE fingerprint")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func propsByKey(props []rive.Property) map[uint32]rive.Property {
	m := make(map[uint32]rive.Property, len(props))
	for _, p := range props {
		m[p.Key] = p
	}
	return m
}

// manualWriter builds a raw .riv byte stream for crafted test cases.
type manualWriter struct {
	buf *bytes.Buffer
}

func newManualWriter(buf *bytes.Buffer) *manualWriter { return &manualWriter{buf: buf} }

func (w *manualWriter) writeRaw(s string) { w.buf.WriteString(s) }

func (w *manualWriter) writeLEB128(v uint64) {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		w.buf.WriteByte(b)
		if v == 0 {
			break
		}
	}
}

func (w *manualWriter) writeUint32LE(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.buf.Write(b[:])
}

func (w *manualWriter) writeString(s string) {
	w.writeLEB128(uint64(len(s)))
	w.buf.WriteString(s)
}
