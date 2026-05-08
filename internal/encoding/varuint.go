// Package encoding implements Rive binary encoding primitives: LEB128 varints,
// BinaryWriter, and BinaryReader, matching the wire format of rive-runtime reader.h.
package encoding

import "errors"

var (
	errEmptyInput      = errors.New("encoding: empty input")
	errUnterminatedVar = errors.New("encoding: unterminated varuint (too many bytes)")
	errOverflow        = errors.New("encoding: read past end of data")
)

// EncodeVarUint appends the unsigned LEB128 encoding of v to buf and returns
// the extended slice. Matches rive-runtime binary_writer.cpp writeVarUint.
func EncodeVarUint(buf []byte, v uint64) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80 // set continuation bit
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

// DecodeVarUint reads an unsigned LEB128 varuint from data starting at offset.
// Returns (value, bytesRead, error). Error on empty input, unterminated sequence,
// or overflow beyond 10 bytes (max for uint64).
func DecodeVarUint(data []byte, offset int) (uint64, int, error) {
	if offset >= len(data) {
		return 0, 0, errEmptyInput
	}
	var val uint64
	var shift uint
	start := offset
	for {
		if offset >= len(data) {
			return 0, 0, errUnterminatedVar
		}
		b := data[offset]
		offset++
		val |= uint64(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
		if shift >= 70 { // 10 * 7 bits = max for uint64
			return 0, 0, errUnterminatedVar
		}
	}
	return val, offset - start, nil
}
