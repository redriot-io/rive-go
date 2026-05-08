package encoding

import (
	"encoding/binary"
	"math"
)

// BinaryWriter accumulates bytes in Rive wire format (little-endian IEEE 754,
// LEB128 varints, length-prefixed strings and byte slices).
type BinaryWriter struct {
	buf []byte
}

// NewWriter returns a ready-to-use BinaryWriter.
func NewWriter() *BinaryWriter { return &BinaryWriter{} }

// WriteVarUint appends an unsigned LEB128-encoded integer.
func (w *BinaryWriter) WriteVarUint(v uint64) {
	w.buf = EncodeVarUint(w.buf, v)
}

// WriteFloat32 appends a 32-bit IEEE 754 float in little-endian byte order.
func (w *BinaryWriter) WriteFloat32(v float32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	w.buf = append(w.buf, b[:]...)
}

// WriteFloat64 appends a 64-bit IEEE 754 float in little-endian byte order.
func (w *BinaryWriter) WriteFloat64(v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	w.buf = append(w.buf, b[:]...)
}

// WriteString appends a LEB128 byte-length prefix followed by the raw UTF-8
// bytes of s. No null terminator is written.
func (w *BinaryWriter) WriteString(s string) {
	w.WriteVarUint(uint64(len(s)))
	w.buf = append(w.buf, s...)
}

// WriteBytes appends a LEB128 length prefix followed by the raw bytes of b.
// A nil or empty slice writes a single zero-byte length prefix.
func (w *BinaryWriter) WriteBytes(b []byte) {
	w.WriteVarUint(uint64(len(b)))
	w.buf = append(w.buf, b...)
}

// WriteColor appends a uint32 ARGB color value in little-endian byte order
// (4 raw bytes: BB GG RR AA on wire).
func (w *BinaryWriter) WriteColor(c uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], c)
	w.buf = append(w.buf, b[:]...)
}

// WriteUint16 appends a uint16 in little-endian byte order.
func (w *BinaryWriter) WriteUint16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

// WriteUint32 appends a uint32 in little-endian byte order.
func (w *BinaryWriter) WriteUint32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

// WriteByte appends a single raw byte.
func (w *BinaryWriter) WriteByte(v byte) {
	w.buf = append(w.buf, v)
}

// Bytes returns the accumulated buffer. The slice is valid until the next Reset.
func (w *BinaryWriter) Bytes() []byte { return w.buf }

// Len returns the number of bytes written so far.
func (w *BinaryWriter) Len() int { return len(w.buf) }

// Reset discards all buffered data.
func (w *BinaryWriter) Reset() { w.buf = w.buf[:0] }
