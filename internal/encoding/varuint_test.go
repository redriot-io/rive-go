package encoding

import (
	"math"
	"testing"
	"testing/quick"
)

var varuintTable = []struct {
	name  string
	value uint64
	want  []byte
}{
	{"zero", 0, []byte{0x00}},
	{"one", 1, []byte{0x01}},
	{"127", 127, []byte{0x7F}},
	{"128", 128, []byte{0x80, 0x01}},
	{"255", 255, []byte{0xFF, 0x01}},
	{"256", 256, []byte{0x80, 0x02}},
	{"16383", 16383, []byte{0xFF, 0x7F}},
	{"16384", 16384, []byte{0x80, 0x80, 0x01}},
	{"MaxUint32", math.MaxUint32, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x0F}},
	{"MaxUint64", math.MaxUint64, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x01}},
}

func TestEncodeDecodeVarUint(t *testing.T) {
	for _, tc := range varuintTable {
		t.Run(tc.name, func(t *testing.T) {
			got := EncodeVarUint(nil, tc.value)
			if string(got) != string(tc.want) {
				t.Errorf("EncodeVarUint(%d) = %v, want %v", tc.value, got, tc.want)
			}
			val, n, err := DecodeVarUint(tc.want, 0)
			if err != nil {
				t.Fatalf("DecodeVarUint: unexpected error: %v", err)
			}
			if val != tc.value {
				t.Errorf("DecodeVarUint value = %d, want %d", val, tc.value)
			}
			if n != len(tc.want) {
				t.Errorf("DecodeVarUint bytesRead = %d, want %d", n, len(tc.want))
			}
		})
	}
}

func TestVarUint_SingleByte(t *testing.T) {
	for v := uint64(0); v <= 127; v++ {
		enc := EncodeVarUint(nil, v)
		if len(enc) != 1 {
			t.Errorf("value %d encoded as %d bytes, want 1", v, len(enc))
		}
		if enc[0] != byte(v) {
			t.Errorf("value %d encoded as 0x%02X, want 0x%02X", v, enc[0], byte(v))
		}
	}
}

func TestVarUint_Boundaries(t *testing.T) {
	cases := []struct {
		v    uint64
		size int
	}{
		{127, 1},
		{128, 2},
		{16383, 2},
		{16384, 3},
		{2097151, 3},
		{2097152, 4},
	}
	for _, c := range cases {
		enc := EncodeVarUint(nil, c.v)
		if len(enc) != c.size {
			t.Errorf("value %d: got %d bytes, want %d", c.v, len(enc), c.size)
		}
	}
}

func TestVarUint_Overflow(t *testing.T) {
	// 10 bytes all with continuation bit set — unterminated
	data := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80}
	_, _, err := DecodeVarUint(data, 0)
	if err == nil {
		t.Error("expected error for unterminated varuint, got nil")
	}
}

func TestVarUint_Empty(t *testing.T) {
	_, _, err := DecodeVarUint([]byte{}, 0)
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestVarUint_OffsetDecoding(t *testing.T) {
	// Encode two values back-to-back and decode at each offset
	buf := EncodeVarUint(nil, 300)
	buf = EncodeVarUint(buf, 42)

	v1, n1, err := DecodeVarUint(buf, 0)
	if err != nil || v1 != 300 {
		t.Fatalf("first decode: got %d err %v", v1, err)
	}
	v2, n2, err := DecodeVarUint(buf, n1)
	if err != nil || v2 != 42 {
		t.Fatalf("second decode: got %d err %v", v2, err)
	}
	if n1+n2 != len(buf) {
		t.Errorf("bytes consumed %d+%d != %d", n1, n2, len(buf))
	}
}

func TestVarUint_PropertyBased(t *testing.T) {
	f := func(v uint64) bool {
		enc := EncodeVarUint(nil, v)
		got, n, err := DecodeVarUint(enc, 0)
		return err == nil && got == v && n == len(enc)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
