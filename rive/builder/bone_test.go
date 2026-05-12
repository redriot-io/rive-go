package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// TestBone_HierarchyEmissionOrder verifies bones are emitted before shapes,
// in declaration order: RootBone → Bone1 → Bone2 → Artboard → Shape.
func TestBone_HierarchyEmissionOrder(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)

	root := ab.RootBone("hip", builder.WithTranslation(250, 400), builder.WithLength(50))
	ab.Bone(root, "spine", builder.WithLength(80), builder.WithRotation(0.1))

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Expected: Backboard(23) → Artboard(1) → RootBone(41) → Bone(40)
	want := []uint32{23, 1, 41, 40}
	if len(objects) < len(want) {
		t.Fatalf("too few objects: %d", len(objects))
	}
	for i, wk := range want {
		if objects[i].TypeKey() != wk {
			t.Errorf("objects[%d]: want typeKey %d, got %d", i, wk, objects[i].TypeKey())
		}
	}
}

// TestBone_ParentIds verifies bone parentId chain is correct.
func TestBone_ParentIds(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)

	root := ab.RootBone("hip", builder.WithLength(50))
	ab.Bone(root, "spine", builder.WithLength(80))

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	getPropUint := func(obj rive.Object, key uint32) (uint64, bool) {
		for _, p := range obj.Properties() {
			if p.Key == key {
				if v, ok := p.Value.(uint64); ok {
					return v, true
				}
			}
		}
		return 0, false
	}

	// objects[1] = Artboard, objects[2] = RootBone(41), objects[3] = Bone(40)
	if objects[2].TypeKey() != 41 {
		t.Fatalf("objects[2] typeKey=%d, want 41 (RootBone)", objects[2].TypeKey())
	}
	if objects[3].TypeKey() != 40 {
		t.Fatalf("objects[3] typeKey=%d, want 40 (Bone)", objects[3].TypeKey())
	}

	// RootBone: parentId should not be emitted (default=0 = artboard)
	if _, has := getPropUint(objects[2], 5); has {
		t.Error("RootBone.parentId should not be emitted (default artboard)")
	}

	// Bone: parentId should be 1 (artboard-relative: points to RootBone at artboard+1)
	parentId, ok := getPropUint(objects[3], 5)
	if !ok {
		t.Error("Bone.parentId (key 5) missing")
	} else if parentId != 1 {
		t.Errorf("Bone.parentId: want 1, got %d", parentId)
	}
}

// TestBone_Properties verifies bone property values are emitted correctly.
func TestBone_Properties(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)

	ab.RootBone("hip",
		builder.WithTranslation(100, 200),
		builder.WithLength(60),
		builder.WithRotation(0.5),
	)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// objects[2] = RootBone
	rb := objects[2]
	if rb.TypeKey() != 41 {
		t.Fatalf("want RootBone(41), got %d", rb.TypeKey())
	}

	propMap := map[uint32]interface{}{}
	for _, p := range rb.Properties() {
		propMap[p.Key] = p.Value
	}

	// key 4 = name
	if v, ok := propMap[uint32(4)]; !ok || v.(string) != "hip" {
		t.Errorf("RootBone.name: got %v", v)
	}
	// key 90 = X, key 91 = Y
	if v, ok := propMap[uint32(90)]; !ok || v.(float64) != 100 {
		t.Errorf("RootBone.X: got %v, want 100", v)
	}
	if v, ok := propMap[uint32(91)]; !ok || v.(float64) != 200 {
		t.Errorf("RootBone.Y: got %v, want 200", v)
	}
	// key 89 = length
	if v, ok := propMap[uint32(89)]; !ok || v.(float64) != 60 {
		t.Errorf("RootBone.Length: got %v, want 60", v)
	}
}

// TestBone_SkinBinding verifies Skin(43) + Tendon(44) emission after shape.
func TestBone_SkinBinding(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)

	root := ab.RootBone("hip", builder.WithLength(50))
	child := ab.Bone(root, "spine", builder.WithLength(80))

	rect := ab.Rectangle(100, 100, 200, 200)
	rect.BindSkin(root, child)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Find Skin(43) and Tendon(44) objects
	skinCount, tendonCount := 0, 0
	skinIdx := -1
	for i, obj := range objects {
		switch obj.TypeKey() {
		case 43:
			skinCount++
			skinIdx = i
		case 44:
			tendonCount++
		}
	}

	if skinCount != 1 {
		t.Errorf("expected 1 Skin(43), got %d", skinCount)
	}
	if tendonCount != 2 {
		t.Errorf("expected 2 Tendon(44) for 2 bones, got %d", tendonCount)
	}

	// Tendons must immediately follow the Skin
	if skinIdx >= 0 && skinIdx+2 < len(objects) {
		if objects[skinIdx+1].TypeKey() != 44 {
			t.Error("objects after Skin[0] should be Tendon(44)")
		}
		if objects[skinIdx+2].TypeKey() != 44 {
			t.Error("objects after Skin[1] should be Tendon(44)")
		}
	}

	// Round-trip
	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	rtSkins := 0
	for _, obj := range f.Objects {
		if obj.TypeKey() == 43 {
			rtSkins++
		}
	}
	if rtSkins != 1 {
		t.Errorf("roundtrip: expected 1 Skin, got %d", rtSkins)
	}
}
