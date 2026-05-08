package encoding

import (
	"math"
	"strings"
	"testing"
	"testing/quick"
	"unicode/utf8"
)

func TestWriter_VarUint(t *testing.T) {
	for _, tc := range varuintTable {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter()
			w.WriteVarUint(tc.value)
			if string(w.Bytes()) != string(tc.want) {
				t.Errorf("WriteVarUint(%d) = %v, want %v", tc.value, w.Bytes(), tc.want)
			}
		})
	}
}

func TestWriter_Float32(t *testing.T) {
	cases := []float32{0.0, 1.0, -1.0, 100.0, math.MaxFloat32, float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))}
	for _, v := range cases {
		w := NewWriter()
		w.WriteFloat32(v)
		b := w.Bytes()
		if len(b) != 4 {
			t.Errorf("WriteFloat32(%v) produced %d bytes, want 4", v, len(b))
		}
		// Round-trip via reader
		r := NewReader(b)
		got, err := r.ReadFloat32()
		if err != nil {
			t.Errorf("ReadFloat32 error: %v", err)
			continue
		}
		if math.IsNaN(float64(v)) {
			if !math.IsNaN(float64(got)) {
				t.Errorf("NaN round-trip: got %v", got)
			}
		} else if got != v {
			t.Errorf("Float32 round-trip %v → %v", v, got)
		}
	}
}

func TestWriter_Float64(t *testing.T) {
	cases := []float64{0.0, 1.0, -1.0, math.Pi, math.MaxFloat64, math.NaN(), math.Inf(1)}
	for _, v := range cases {
		w := NewWriter()
		w.WriteFloat64(v)
		b := w.Bytes()
		if len(b) != 8 {
			t.Errorf("WriteFloat64(%v) produced %d bytes, want 8", v, len(b))
		}
		r := NewReader(b)
		got, err := r.ReadFloat64()
		if err != nil {
			t.Errorf("ReadFloat64 error: %v", err)
			continue
		}
		if math.IsNaN(v) {
			if !math.IsNaN(got) {
				t.Errorf("NaN round-trip: got %v", got)
			}
		} else if got != v {
			t.Errorf("Float64 round-trip %v → %v", v, got)
		}
	}
}

func TestWriter_String(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"café",
		"日本語",
		"🎨🎭🎪",
		strings.Repeat("x", 128),   // needs 2-byte length prefix
		strings.Repeat("x", 16384), // needs 3-byte length prefix
	}
	for _, s := range cases {
		w := NewWriter()
		w.WriteString(s)
		r := NewReader(w.Bytes())
		got, err := r.ReadString()
		if err != nil {
			t.Errorf("ReadString(%q) error: %v", s, err)
			continue
		}
		if got != s {
			t.Errorf("String round-trip: got %q, want %q", got, s)
		}
	}
}

func TestWriter_String_NoNullTerminator(t *testing.T) {
	w := NewWriter()
	w.WriteString("hi")
	b := w.Bytes()
	// Byte 0: LEB128 length = 2, Bytes 1-2: 'h','i' — total 3 bytes, no null
	if len(b) != 3 {
		t.Errorf("WriteString(\"hi\") produced %d bytes, want 3 (1 len + 2 data)", len(b))
	}
	for _, by := range b {
		if by == 0x00 {
			t.Error("null terminator found in WriteString output")
		}
	}
}

func TestWriter_Bytes(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{0x01, 0x02, 0x03},
		make([]byte, 300),
	}
	for _, b := range cases {
		w := NewWriter()
		w.WriteBytes(b)
		r := NewReader(w.Bytes())
		got, err := r.ReadBytes()
		if err != nil {
			t.Errorf("ReadBytes error: %v", err)
			continue
		}
		if len(got) != len(b) {
			t.Errorf("Bytes round-trip length: got %d, want %d", len(got), len(b))
		}
	}
}

func TestWriter_Color(t *testing.T) {
	cases := []uint32{0x00000000, 0xFFFFFFFF, 0xFFFF0000, 0x80FF00FF}
	for _, c := range cases {
		w := NewWriter()
		w.WriteColor(c)
		b := w.Bytes()
		if len(b) != 4 {
			t.Errorf("WriteColor(0x%08X) produced %d bytes, want 4", c, len(b))
		}
		r := NewReader(b)
		got, err := r.ReadColor()
		if err != nil {
			t.Errorf("ReadColor error: %v", err)
			continue
		}
		if got != c {
			t.Errorf("Color round-trip 0x%08X → 0x%08X", c, got)
		}
	}
}

func TestWriter_Uint16_Uint32(t *testing.T) {
	u16cases := []uint16{0, 1, 255, 256, 65535}
	for _, v := range u16cases {
		w := NewWriter()
		w.WriteUint16(v)
		b := w.Bytes()
		if len(b) != 2 {
			t.Errorf("WriteUint16(%d) produced %d bytes", v, len(b))
		}
		r := NewReader(b)
		got, err := r.ReadUint16()
		if err != nil || got != v {
			t.Errorf("Uint16 round-trip %d → %d, err %v", v, got, err)
		}
	}

	u32cases := []uint32{0, 1, 255, 256, 65535, math.MaxUint32}
	for _, v := range u32cases {
		w := NewWriter()
		w.WriteUint32(v)
		b := w.Bytes()
		if len(b) != 4 {
			t.Errorf("WriteUint32(%d) produced %d bytes", v, len(b))
		}
		r := NewReader(b)
		got, err := r.ReadUint32()
		if err != nil || got != v {
			t.Errorf("Uint32 round-trip %d → %d, err %v", v, got, err)
		}
	}
}

func TestWriter_Byte(t *testing.T) {
	for _, v := range []byte{0x00, 0x01, 0x7F, 0x80, 0xFF} {
		w := NewWriter()
		w.WriteByte(v)
		b := w.Bytes()
		if len(b) != 1 || b[0] != v {
			t.Errorf("WriteByte(0x%02X) = %v", v, b)
		}
	}
}

func TestWriter_LenAndReset(t *testing.T) {
	w := NewWriter()
	if w.Len() != 0 {
		t.Errorf("initial Len = %d, want 0", w.Len())
	}
	w.WriteUint32(42)
	if w.Len() != 4 {
		t.Errorf("after WriteUint32 Len = %d, want 4", w.Len())
	}
	w.Reset()
	if w.Len() != 0 {
		t.Errorf("after Reset Len = %d, want 0", w.Len())
	}
}

func TestWriter_Accumulates(t *testing.T) {
	w := NewWriter()
	w.WriteUint16(1)
	w.WriteUint32(2)
	w.WriteByte(3)
	if w.Len() != 7 {
		t.Errorf("accumulated Len = %d, want 7", w.Len())
	}
}

func TestFloat32_PropertyBased(t *testing.T) {
	f := func(bits uint32) bool {
		v := math.Float32frombits(bits)
		w := NewWriter()
		w.WriteFloat32(v)
		r := NewReader(w.Bytes())
		got, err := r.ReadFloat32()
		if err != nil {
			return false
		}
		if math.IsNaN(float64(v)) {
			return math.IsNaN(float64(got))
		}
		return got == v
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestString_PropertyBased(t *testing.T) {
	f := func(s string) bool {
		if !utf8.ValidString(s) {
			return true // skip invalid UTF-8
		}
		w := NewWriter()
		w.WriteString(s)
		r := NewReader(w.Bytes())
		got, err := r.ReadString()
		return err == nil && got == s
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
