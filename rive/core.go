package rive

// Object is implemented by all concrete (non-abstract) Rive types.
type Object interface {
	TypeKey() uint32
	Properties() []Property
}

// Property is a single serializable property value.
type Property struct {
	Key   uint32
	Type  PropertyType
	Value interface{}
}

// PropertyType is the wire encoding type for a property.
type PropertyType uint8

const (
	PropertyTypeUint   PropertyType = 0 // uint64; also encodes bool (0/1)
	PropertyTypeString PropertyType = 1
	PropertyTypeFloat  PropertyType = 2 // stored as float64, wire as float32
	PropertyTypeColor  PropertyType = 3 // uint32 RGBA LE
	PropertyTypeBytes  PropertyType = 4 // []byte with LEB128 length prefix
)

func boolToUint64(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}
