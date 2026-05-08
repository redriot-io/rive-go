package encoding

import (
	"math"
	"testing"
)

func TestReader_RoundTrip_AllTypes(t *testing.T) {
	w := NewWriter()
	w.WriteVarUint(12345678)
	w.WriteFloat32(3.14)
	w.WriteFloat64(math.Pi)
	w.WriteString("montessori")
	w.WriteBytes([]byte{0xDE, 0xAD, 0xBE, 0xEF})
	w.WriteColor(0xFFAA5500)
	w.WriteUint16(1024)
	w.WriteUint32(0xDEADBEEF)
	w.WriteByte(0xAB)

	r := NewReader(w.Bytes())

	if v, err := r.ReadVarUint(); err != nil || v != 12345678 {
		t.Errorf("VarUint: got %d, err %v", v, err)
	}
	if v, err := r.ReadFloat32(); err != nil || math.Abs(float64(v-3.14)) > 1e-5 {
		t.Errorf("Float32: got %v, err %v", v, err)
	}
	if v, err := r.ReadFloat64(); err != nil || v != math.Pi {
		t.Errorf("Float64: got %v, err %v", v, err)
	}
	if v, err := r.ReadString(); err != nil || v != "montessori" {
		t.Errorf("String: got %q, err %v", v, err)
	}
	if v, err := r.ReadBytes(); err != nil || string(v) != string([]byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("Bytes: got %v, err %v", v, err)
	}
	if v, err := r.ReadColor(); err != nil || v != 0xFFAA5500 {
		t.Errorf("Color: got 0x%08X, err %v", v, err)
	}
	if v, err := r.ReadUint16(); err != nil || v != 1024 {
		t.Errorf("Uint16: got %d, err %v", v, err)
	}
	if v, err := r.ReadUint32(); err != nil || v != 0xDEADBEEF {
		t.Errorf("Uint32: got 0x%08X, err %v", v, err)
	}
	if v, err := r.ReadByte(); err != nil || v != 0xAB {
		t.Errorf("Byte: got 0x%02X, err %v", v, err)
	}
	if !r.IsEOF() {
		t.Errorf("expected EOF after reading all fields; remaining = %d", r.Remaining())
	}
}

func TestReader_Overflow(t *testing.T) {
	// Truncated float32 (3 bytes instead of 4)
	r := NewReader([]byte{0x01, 0x02, 0x03})
	_, err := r.ReadFloat32()
	if err == nil {
		t.Error("expected error reading float32 from 3-byte buffer")
	}
	if !r.HasOverflowed() {
		t.Error("expected HasOverflowed() = true after truncated read")
	}
	// Subsequent reads must also fail once overflowed
	_, err2 := r.ReadByte()
	if err2 == nil {
		t.Error("expected error on subsequent read after overflow")
	}
}

func TestReader_Position(t *testing.T) {
	w := NewWriter()
	w.WriteUint16(10)
	w.WriteUint32(20)
	data := w.Bytes() // 6 bytes total

	r := NewReader(data)
	if r.Position() != 0 {
		t.Errorf("initial position = %d, want 0", r.Position())
	}
	_, _ = r.ReadUint16()
	if r.Position() != 2 {
		t.Errorf("after ReadUint16 position = %d, want 2", r.Position())
	}
	_, _ = r.ReadUint32()
	if r.Position() != 6 {
		t.Errorf("after ReadUint32 position = %d, want 6", r.Position())
	}
	if r.Remaining() != 0 {
		t.Errorf("remaining = %d, want 0", r.Remaining())
	}
}

func TestReader_MultipleReads(t *testing.T) {
	w := NewWriter()
	for i := uint64(0); i < 100; i++ {
		w.WriteVarUint(i * 1000)
	}
	r := NewReader(w.Bytes())
	for i := uint64(0); i < 100; i++ {
		v, err := r.ReadVarUint()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if v != i*1000 {
			t.Errorf("read %d: got %d, want %d", i, v, i*1000)
		}
	}
	if !r.IsEOF() {
		t.Errorf("expected EOF after 100 reads; remaining = %d", r.Remaining())
	}
}

func TestReader_EmptyInput(t *testing.T) {
	r := NewReader([]byte{})
	if !r.IsEOF() {
		t.Error("empty reader should be EOF immediately")
	}
	if _, err := r.ReadByte(); err == nil {
		t.Error("ReadByte on empty reader should error")
	}
	if _, err := r.ReadVarUint(); err == nil {
		t.Error("ReadVarUint on empty reader should error")
	}
	if _, err := r.ReadFloat32(); err == nil {
		t.Error("ReadFloat32 on empty reader should error")
	}
	if _, err := r.ReadString(); err == nil {
		t.Error("ReadString on empty reader should error")
	}
}

func TestReader_ReadBytes_IsCopy(t *testing.T) {
	original := []byte{1, 2, 3}
	w := NewWriter()
	w.WriteBytes(original)
	r := NewReader(w.Bytes())
	got, err := r.ReadBytes()
	if err != nil {
		t.Fatal(err)
	}
	// Mutate got — should not affect original
	got[0] = 99
	if original[0] == 99 {
		t.Error("ReadBytes returned a slice aliasing the original buffer")
	}
}

func TestReader_String_MultiByteLen(t *testing.T) {
	// >127 byte string requires 2-byte LEB128 length prefix
	s := string(make([]byte, 200))
	w := NewWriter()
	w.WriteString(s)
	r := NewReader(w.Bytes())
	got, err := r.ReadString()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 200 {
		t.Errorf("got string len %d, want 200", len(got))
	}
}

func TestReader_Overflow_String(t *testing.T) {
	// Claim string length 100 but only provide 5 bytes of content
	w := NewWriter()
	w.WriteVarUint(100) // length prefix = 100
	w.WriteByte('a')
	w.WriteByte('b')
	w.WriteByte('c')
	// Only 3 bytes of content
	r := NewReader(w.Bytes())
	_, err := r.ReadString()
	if err == nil {
		t.Error("expected error reading string with insufficient data")
	}
}
