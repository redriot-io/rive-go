package rive

import (
	"errors"
	"fmt"
	"io"

	"github.com/redriot-io/rive-go/internal/encoding"
)

// ── Errors ────────────────────────────────────────────────────────────────────

var (
	ErrInvalidFingerprint   = errors.New("rive: invalid file fingerprint (not a .riv file)")
	ErrUnsupportedMajor     = errors.New("rive: unsupported major version")
	ErrUnknownPropertyType  = errors.New("rive: unknown property type — not in ToC or global table")
	ErrTruncated            = errors.New("rive: unexpected end of data (truncated file)")
)

// ── GenericObject ─────────────────────────────────────────────────────────────

// GenericObject holds a type's data when the type key is unknown to this
// runtime, enabling round-trip preservation without understanding the content.
type GenericObject struct {
	typeKey    uint32
	properties []Property
}

func (g *GenericObject) TypeKey() uint32          { return g.typeKey }
func (g *GenericObject) Properties() []Property   { return g.properties }

// ── File ──────────────────────────────────────────────────────────────────────

// File is a parsed .riv file.
type File struct {
	MajorVersion uint32
	MinorVersion uint32
	FileID       uint64
	Objects      []Object

	// propertyTypes is the ToC map: property key → wire type.
	propertyTypes map[uint32]PropertyType
}

// PropertyTypeOf returns the wire type of a property key from the ToC.
// Returns false if the key was not found in the file's ToC.
func (f *File) PropertyTypeOf(key uint32) (PropertyType, bool) {
	t, ok := f.propertyTypes[key]
	return t, ok
}

// ── ReadBytes / ReadFile ──────────────────────────────────────────────────────

// ReadBytes parses .riv bytes and returns the object graph.
func ReadBytes(data []byte) (*File, error) {
	r := encoding.NewReader(data)
	return parseRiv(r)
}

// ReadFile parses a .riv file from r and returns the object graph.
func ReadFile(r io.Reader) (*File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("rive: read: %w", err)
	}
	return ReadBytes(data)
}

// ── Core parser ───────────────────────────────────────────────────────────────

func parseRiv(r *encoding.BinaryReader) (*File, error) {
	// ── Fingerprint ───────────────────────────────────────────────────────────
	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, ErrTruncated
		}
		if b != riveFingerprint[i] {
			return nil, ErrInvalidFingerprint
		}
	}

	// ── Versions ──────────────────────────────────────────────────────────────
	major, err := r.ReadVarUint()
	if err != nil {
		return nil, fmt.Errorf("rive: read major version: %w", err)
	}
	if uint32(major) != defaultMajor {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedMajor, major, defaultMajor)
	}

	minor, err := r.ReadVarUint()
	if err != nil {
		return nil, fmt.Errorf("rive: read minor version: %w", err)
	}

	fileID, err := r.ReadVarUint()
	if err != nil {
		return nil, fmt.Errorf("rive: read file ID: %w", err)
	}

	// ── ToC ───────────────────────────────────────────────────────────────────
	var tocKeys []uint32
	for {
		k, err := r.ReadVarUint()
		if err != nil {
			return nil, fmt.Errorf("rive: read ToC key: %w", err)
		}
		if k == 0 {
			break
		}
		tocKeys = append(tocKeys, uint32(k))
	}

	// Read packed 2-bit types: C++ algorithm reads one uint32 per 4 keys,
	// using bits 0-1, 2-3, 4-5, 6-7 within each uint32.
	propTypes := make(map[uint32]PropertyType, len(tocKeys))
	{
		var currentInt uint32
		currentBit := 8 // triggers immediate uint32 read on first key
		for _, k := range tocKeys {
			if currentBit == 8 {
				v, err := r.ReadUint32()
				if err != nil {
					return nil, fmt.Errorf("rive: read ToC type bits: %w", err)
				}
				currentInt = v
				currentBit = 0
			}
			propTypes[k] = PropertyType((currentInt >> uint(currentBit)) & 3)
			currentBit += 2
		}
	}

	// ── Object stream ─────────────────────────────────────────────────────────
	f := &File{
		MajorVersion:  uint32(major),
		MinorVersion:  uint32(minor),
		FileID:        fileID,
		propertyTypes: propTypes,
	}

	objIdx := 0
	for !r.IsEOF() {
		typeKey, err := r.ReadVarUint()
		if err != nil {
			return nil, fmt.Errorf("rive: object %d type key: %w", objIdx, err)
		}
		if typeKey == 0 {
			break // defensive: stray zero
		}

		obj, err := readObject(r, uint32(typeKey), propTypes, objIdx)
		if err != nil {
			return nil, err
		}
		f.Objects = append(f.Objects, obj)
		objIdx++
	}

	return f, nil
}

// readObject reads one object (properties until 0 terminator) from the stream.
// All objects are deserialized as GenericObject — preserving properties as
// key-value pairs for a lossless read→write round-trip.
func readObject(r *encoding.BinaryReader, typeKey uint32, tocTypes map[uint32]PropertyType, objIdx int) (Object, error) {
	g := &GenericObject{typeKey: typeKey}

	propIdx := 0
	for {
		propKey, err := r.ReadVarUint()
		if err != nil {
			return nil, fmt.Errorf("rive: object %d prop %d key: %w", objIdx, propIdx, err)
		}
		if propKey == 0 {
			break
		}

		ptype, ok := lookupPropType(uint32(propKey), tocTypes)
		if !ok {
			return nil, fmt.Errorf("rive: object %d prop key %d: %w", objIdx, propKey, ErrUnknownPropertyType)
		}

		val, err := readPropertyValue(r, ptype)
		if err != nil {
			return nil, fmt.Errorf("rive: object %d prop %d (key=%d): %w", objIdx, propIdx, propKey, err)
		}
		g.properties = append(g.properties, Property{Key: uint32(propKey), Type: ptype, Value: val})
		propIdx++
	}

	return g, nil
}

// lookupPropType finds the wire type for a property key.
// Prefers the compiled-in globalPropTypes table over the file's ToC because the
// 2-bit ToC encoding cannot represent PropertyTypeBytes (value 4 truncates to 0).
// Falls back to the ToC for property keys not in the compiled-in table.
func lookupPropType(key uint32, tocTypes map[uint32]PropertyType) (PropertyType, bool) {
	if t, ok := globalPropTypes[key]; ok {
		return t, true
	}
	if t, ok := tocTypes[key]; ok {
		return t, true
	}
	return 0, false
}

// readPropertyValue reads one property value according to its wire type.
// Values are stored as:
//   - PropertyTypeUint   → uint64
//   - PropertyTypeString → string
//   - PropertyTypeFloat  → float64 (converted from float32 wire)
//   - PropertyTypeColor  → uint64 (cast from uint32; matches generated Properties() encoding)
//   - PropertyTypeBytes  → []byte
func readPropertyValue(r *encoding.BinaryReader, ptype PropertyType) (interface{}, error) {
	switch ptype {
	case PropertyTypeUint:
		v, err := r.ReadVarUint()
		return v, err
	case PropertyTypeString:
		s, err := r.ReadString()
		return s, err
	case PropertyTypeFloat:
		v, err := r.ReadFloat32()
		return float64(v), err
	case PropertyTypeColor:
		v, err := r.ReadColor()
		return uint64(v), err
	case PropertyTypeBytes:
		b, err := r.ReadBytes()
		return b, err
	default:
		return nil, fmt.Errorf("unhandled property type %d", ptype)
	}
}
