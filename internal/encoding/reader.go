package encoding

import (
	"encoding/binary"
	"math"
)

// BinaryReader reads Rive wire-format binary data sequentially.
// Once any read exceeds the available data, the reader enters an overflow state
// and all subsequent reads return a zero value and an error.
type BinaryReader struct {
	data     []byte
	pos      int
	overflow bool
}

// NewReader returns a BinaryReader positioned at the start of data.
func NewReader(data []byte) *BinaryReader {
	return &BinaryReader{data: data}
}

// ReadVarUint reads an unsigned LEB128-encoded integer.
func (r *BinaryReader) ReadVarUint() (uint64, error) {
	if r.overflow {
		return 0, errOverflow
	}
	v, n, err := DecodeVarUint(r.data, r.pos)
	if err != nil {
		r.overflow = true
		return 0, err
	}
	r.pos += n
	return v, nil
}

// ReadFloat32 reads a 32-bit IEEE 754 float (little-endian).
func (r *BinaryReader) ReadFloat32() (float32, error) {
	b, err := r.readExact(4)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b)), nil
}

// ReadFloat64 reads a 64-bit IEEE 754 float (little-endian).
func (r *BinaryReader) ReadFloat64() (float64, error) {
	b, err := r.readExact(8)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
}

// ReadString reads a LEB128 length-prefixed UTF-8 string (no null terminator).
func (r *BinaryReader) ReadString() (string, error) {
	n, err := r.ReadVarUint()
	if err != nil {
		return "", err
	}
	b, err := r.readExact(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ReadBytes reads a LEB128 length-prefixed byte slice.
// The returned slice is a copy and does not alias the reader's buffer.
func (r *BinaryReader) ReadBytes() ([]byte, error) {
	n, err := r.ReadVarUint()
	if err != nil {
		return nil, err
	}
	b, err := r.readExact(int(n))
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

// ReadColor reads a 4-byte little-endian uint32 ARGB color.
func (r *BinaryReader) ReadColor() (uint32, error) {
	b, err := r.readExact(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// ReadUint16 reads a 2-byte little-endian uint16.
func (r *BinaryReader) ReadUint16() (uint16, error) {
	b, err := r.readExact(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

// ReadUint32 reads a 4-byte little-endian uint32.
func (r *BinaryReader) ReadUint32() (uint32, error) {
	b, err := r.readExact(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// ReadByte reads a single byte.
func (r *BinaryReader) ReadByte() (byte, error) {
	b, err := r.readExact(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// IsEOF reports whether the reader has consumed all data.
func (r *BinaryReader) IsEOF() bool { return r.pos >= len(r.data) }

// HasOverflowed reports whether any read has attempted to exceed the data bounds.
func (r *BinaryReader) HasOverflowed() bool { return r.overflow }

// Position returns the current byte offset.
func (r *BinaryReader) Position() int { return r.pos }

// Remaining returns the number of bytes not yet consumed.
func (r *BinaryReader) Remaining() int {
	if r.pos >= len(r.data) {
		return 0
	}
	return len(r.data) - r.pos
}

// readExact returns a slice of exactly n bytes from the current position and
// advances the position. Sets overflow and returns an error if n bytes are
// unavailable, or if the reader has already overflowed.
func (r *BinaryReader) readExact(n int) ([]byte, error) {
	if r.overflow {
		return nil, errOverflow
	}
	if r.pos+n > len(r.data) {
		r.overflow = true
		return nil, errOverflow
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}
