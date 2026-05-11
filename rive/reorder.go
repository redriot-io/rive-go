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

// findAllArtboards returns the global indices of all Artboard (typeKey=1) objects
// in ascending order (preserving stream order).
func findAllArtboards(objects []Object) []int {
	var idxs []int
	for i, obj := range objects {
		if obj.TypeKey() == 1 {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

// buildParentMapRange constructs a mapping of child → parent for the artboard at
// artboardIdx, covering children in [artboardIdx+1, endIdx).
// Must be called before any reordering so that parentId values are still valid.
func buildParentMapRange(objects []Object, artboardIdx, endIdx int) map[Object]Object {
	artboard := objects[artboardIdx]
	parentOf := make(map[Object]Object, endIdx-artboardIdx)
	for i := artboardIdx + 1; i < endIdx; i++ {
		obj := objects[i]
		pid := getParentId(obj)
		if pid == 0 {
			parentOf[obj] = artboard
		} else {
			ref := artboardIdx + int(pid)
			if ref < len(objects) {
				parentOf[obj] = objects[ref]
			}
		}
	}
	return parentOf
}

// fixParentIdsRange recalculates artboard-relative parentId values for children of
// the artboard at artboardIdx, in the range [artboardIdx+1, endIdx), using the
// supplied parent-child map (built before reordering, keyed by object pointer).
func fixParentIdsRange(objects []Object, artboardIdx, endIdx int, parentOf map[Object]Object) {
	artboard := objects[artboardIdx]

	// Resolve current global index of every object in the (possibly reordered) slice.
	globalIdx := make(map[Object]int, len(objects))
	for i, obj := range objects {
		globalIdx[obj] = i
	}

	for i := artboardIdx + 1; i < endIdx; i++ {
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
			ps.setParentId(uint64(globalIdx[parent] - artboardIdx))
		}
	}
}

// ReorderByContract reorders objects to match the official Rive encoder's
// emission order derived from format_contract.json, then fixes all
// artboard-relative parentId values for every artboard in the file.
//
// Currently enforces one conformance rule: SolidColor (typeKey=18) must appear
// immediately before its parent Fill (typeKey=20) in the binary stream — a
// forward reference that matches official Rive editor output.
//
// Supports files with multiple artboards: each artboard's children are processed
// independently so parentId recalculation is scoped correctly.
func ReorderByContract(objects []Object) []Object {
	if len(objects) == 0 {
		return objects
	}
	artboardIdxs := findAllArtboards(objects)
	if len(artboardIdxs) == 0 {
		return objects
	}

	// Build parent maps for ALL artboards BEFORE any reordering.
	parentMaps := make([]map[Object]Object, len(artboardIdxs))
	for ai, abIdx := range artboardIdxs {
		endIdx := len(objects)
		if ai+1 < len(artboardIdxs) {
			endIdx = artboardIdxs[ai+1]
		}
		parentMaps[ai] = buildParentMapRange(objects, abIdx, endIdx)
	}

	// Copy and apply the SolidColor-before-Fill swap over the entire object list.
	result := make([]Object, len(objects))
	copy(result, objects)
	for i := 0; i < len(result)-1; i++ {
		if result[i].TypeKey() == 20 && result[i+1].TypeKey() == 18 {
			// Fill at i, SolidColor at i+1 → swap to canonical order.
			result[i], result[i+1] = result[i+1], result[i]
		}
	}

	// Recalculate parentIds for each artboard's children using the pre-reorder maps.
	for ai, abIdx := range artboardIdxs {
		endIdx := len(result)
		if ai+1 < len(artboardIdxs) {
			endIdx = artboardIdxs[ai+1]
		}
		fixParentIdsRange(result, abIdx, endIdx, parentMaps[ai])
	}

	return result
}

// FixParentIds recalculates artboard-relative parentId values for all objects
// using their current parentId to reconstruct parent-child relationships.
// Handles multiple artboards. Idempotent: safe to call on an already-correct list.
func FixParentIds(objects []Object) {
	artboardIdxs := findAllArtboards(objects)
	for ai, abIdx := range artboardIdxs {
		endIdx := len(objects)
		if ai+1 < len(artboardIdxs) {
			endIdx = artboardIdxs[ai+1]
		}
		parentOf := buildParentMapRange(objects, abIdx, endIdx)
		fixParentIdsRange(objects, abIdx, endIdx, parentOf)
	}
}
