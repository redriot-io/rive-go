package rive

// parentSetter is implemented by all rive Objects that embed Component.
// The method is added to *Component in this file (same package as gen_root.go),
// which promotes it to every concrete type via Go embedding.
type parentSetter interface {
	setParentId(uint64)
}

func (o *Component) setParentId(id uint64) { o.ParentId = id }

// getParentId reads artboard-relative parentId from an object's Properties.
// Returns 0 if key 5 is absent (artboard-root child with suppressed parentId).
func getParentId(obj Object) uint64 {
	for _, p := range obj.Properties() {
		if p.Key == 5 {
			return p.Value.(uint64)
		}
	}
	return 0
}

// findFirstArtboard returns the global index of the first Artboard (typeKey=1),
// or -1 if none is found.
func findFirstArtboard(objects []Object) int {
	for i, obj := range objects {
		if obj.TypeKey() == 1 {
			return i
		}
	}
	return -1
}

// buildParentMap constructs a mapping of object → parent object using the
// CURRENT artboard-relative parentId values. Must be called before reordering.
// Objects before the artboard (Backboard, FontAsset, etc.) are excluded.
func buildParentMap(objects []Object, artboardGlobalIdx int) map[Object]Object {
	artboard := objects[artboardGlobalIdx]
	parentOf := make(map[Object]Object, len(objects))
	for i := artboardGlobalIdx + 1; i < len(objects); i++ {
		obj := objects[i]
		pid := getParentId(obj)
		if pid == 0 {
			parentOf[obj] = artboard
		} else {
			parentOf[obj] = objects[artboardGlobalIdx+int(pid)]
		}
	}
	return parentOf
}

// fixParentIds recalculates artboard-relative parentId for every artboard child
// using the supplied parent-child map (keyed by object pointer).
func fixParentIds(objects []Object, artboardGlobalIdx int, parentOf map[Object]Object) {
	artboard := objects[artboardGlobalIdx]

	// Current global index of every object (pointer → position)
	globalIdx := make(map[Object]int, len(objects))
	for i, obj := range objects {
		globalIdx[obj] = i
	}

	for i := artboardGlobalIdx + 1; i < len(objects); i++ {
		obj := objects[i]
		parent, ok := parentOf[obj]
		if !ok {
			continue
		}
		ps, ok := obj.(parentSetter)
		if !ok {
			continue
		}
		if parent == artboard {
			ps.setParentId(0)
		} else {
			ps.setParentId(uint64(globalIdx[parent] - artboardGlobalIdx))
		}
	}
}

// ReorderByContract reorders objects to match the official Rive encoder's
// emission order derived from format_contract.json, then fixes all
// artboard-relative parentId values.
//
// Currently enforces one conformance rule: SolidColor (typeKey=18) must appear
// immediately before its parent Fill (typeKey=20) in the binary stream — a
// forward reference that matches official Rive editor output.
func ReorderByContract(objects []Object) []Object {
	if len(objects) == 0 {
		return objects
	}
	artboardGlobalIdx := findFirstArtboard(objects)
	if artboardGlobalIdx < 0 {
		return objects
	}

	// Capture parent-child relationships from the correct pre-reorder parentIds.
	parentOf := buildParentMap(objects, artboardGlobalIdx)

	// Copy and apply the SolidColor-before-Fill swap.
	result := make([]Object, len(objects))
	copy(result, objects)

	for i := 0; i < len(result)-1; i++ {
		if result[i].TypeKey() == 20 && result[i+1].TypeKey() == 18 {
			// Fill at i, SolidColor at i+1 → swap: SolidColor at i, Fill at i+1.
			result[i], result[i+1] = result[i+1], result[i]
		}
	}

	// Recalculate parentIds from the pre-reorder parent map.
	fixParentIds(result, artboardGlobalIdx, parentOf)

	return result
}

// FixParentIds recalculates artboard-relative parentId values for all objects
// using their current parentId to reconstruct parent-child relationships.
// Idempotent: safe to call on an already-correct list.
func FixParentIds(objects []Object) {
	artboardGlobalIdx := findFirstArtboard(objects)
	if artboardGlobalIdx < 0 {
		return
	}
	parentOf := buildParentMap(objects, artboardGlobalIdx)
	fixParentIds(objects, artboardGlobalIdx, parentOf)
}
