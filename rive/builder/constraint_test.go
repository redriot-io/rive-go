package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// buildConstraintScene creates a minimal two-bone artboard and runs configure.
func buildConstraintScene(t *testing.T, configure func(ab *builder.ArtboardBuilder, root, child *builder.BoneRef)) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	root := ab.RootBone("root", builder.WithTranslation(0, 200), builder.WithLength(80))
	child := ab.Bone(root, "child", builder.WithLength(60))
	configure(ab, root, child)
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objects
}

// findFirst returns the first object with the given TypeKey, or nil.
func findFirst(objects []rive.Object, tk uint32) rive.Object {
	for _, o := range objects {
		if o.TypeKey() == tk {
			return o
		}
	}
	return nil
}

// typeKeySlice extracts TypeKeys from an object slice.
func typeKeySlice(objects []rive.Object) []uint32 {
	out := make([]uint32, len(objects))
	for i, o := range objects {
		out[i] = o.TypeKey()
	}
	return out
}

// TestConstraint_IKEmission checks IKConstraint (tk=81) emits after bones.
func TestConstraint_IKEmission(t *testing.T) {
	objects := buildConstraintScene(t, func(ab *builder.ArtboardBuilder, root, child *builder.BoneRef) {
		ab.IKConstraint("arm_ik", root, child, builder.WithChainLength(2))
	})

	// Expected: Backboard(23), Artboard(1), RootBone(41), Bone(40), IKConstraint(81)
	tks := typeKeySlice(objects)
	want := []uint32{23, 1, 41, 40, 81}
	for i, wantTK := range want {
		if i >= len(tks) {
			t.Fatalf("objects[%d] missing (want TypeKey=%d)", i, wantTK)
		}
		if tks[i] != wantTK {
			t.Errorf("objects[%d] TypeKey=%d, want %d; full: %v", i, tks[i], wantTK, tks)
		}
	}
}

// TestConstraint_IKProperties verifies IKConstraint concrete fields.
func TestConstraint_IKProperties(t *testing.T) {
	objects := buildConstraintScene(t, func(ab *builder.ArtboardBuilder, root, child *builder.BoneRef) {
		ab.IKConstraint("arm_ik", root, child,
			builder.WithChainLength(2),
			builder.WithInvertDirection(true),
		)
	})

	o := findFirst(objects, 81)
	if o == nil {
		t.Fatal("no IKConstraint (tk=81) found")
	}
	ik, ok := o.(*rive.IKConstraint)
	if !ok {
		t.Fatalf("TypeKey=81 is %T, want *rive.IKConstraint", o)
	}
	if ik.ParentBoneCount != 2 {
		t.Errorf("ParentBoneCount = %d, want 2", ik.ParentBoneCount)
	}
	if !ik.InvertDirection {
		t.Errorf("InvertDirection = false, want true")
	}
	// TargetId should be child bone's artboard-relative index (2: Artboard=0,Root=1,Child=2).
	if ik.TargetId != 2 {
		t.Errorf("TargetId = %d, want 2 (child bone artboard-relative idx)", ik.TargetId)
	}
	// ParentId should be root bone's artboard-relative index (1).
	if ik.ParentId != 1 {
		t.Errorf("ParentId = %d, want 1 (root bone artboard-relative idx)", ik.ParentId)
	}
}

// TestConstraint_DistanceProperties verifies DistanceConstraint concrete fields.
func TestConstraint_DistanceProperties(t *testing.T) {
	objects := buildConstraintScene(t, func(ab *builder.ArtboardBuilder, root, child *builder.BoneRef) {
		ab.DistanceConstraint("keep_dist", root, child,
			builder.WithConstraintDistance(50.0),
			builder.WithConstraintStrength(0.8),
		)
	})

	o := findFirst(objects, 82)
	if o == nil {
		t.Fatal("no DistanceConstraint (tk=82) found")
	}
	dc, ok := o.(*rive.DistanceConstraint)
	if !ok {
		t.Fatalf("TypeKey=82 is %T, want *rive.DistanceConstraint", o)
	}
	if dc.Distance != 50.0 {
		t.Errorf("Distance = %v, want 50.0", dc.Distance)
	}
	if dc.Strength != 0.8 {
		t.Errorf("Strength = %v, want 0.8", dc.Strength)
	}
	if dc.TargetId != 2 {
		t.Errorf("TargetId = %d, want 2 (child bone idx)", dc.TargetId)
	}
}

// TestConstraint_TransformProperties verifies TransformConstraint concrete fields.
func TestConstraint_TransformProperties(t *testing.T) {
	objects := buildConstraintScene(t, func(ab *builder.ArtboardBuilder, root, child *builder.BoneRef) {
		ab.TransformConstraint("copy_xform", root, child,
			builder.WithConstraintOrigin(0.5, 0.5),
		)
	})

	o := findFirst(objects, 83)
	if o == nil {
		t.Fatal("no TransformConstraint (tk=83) found")
	}
	tc, ok := o.(*rive.TransformConstraint)
	if !ok {
		t.Fatalf("TypeKey=83 is %T, want *rive.TransformConstraint", o)
	}
	if tc.OriginX != 0.5 {
		t.Errorf("OriginX = %v, want 0.5", tc.OriginX)
	}
	if tc.OriginY != 0.5 {
		t.Errorf("OriginY = %v, want 0.5", tc.OriginY)
	}
	if tc.TargetId != 2 {
		t.Errorf("TargetId = %d, want 2 (child bone idx)", tc.TargetId)
	}
}
