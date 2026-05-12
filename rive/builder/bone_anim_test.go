package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// TestBuilder_BoneAnimation verifies that a BoneRef can be used as an animation
// target: KeyedObject(25) → KeyedProperty(26, propKey=15) → KeyFrameDouble(30)×3.
func TestBuilder_BoneAnimation(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	root := ab.RootBone("root", builder.WithTranslation(0, 200), builder.WithLength(80))
	ab.Bone(root, "child", builder.WithLength(60))

	// Animate root bone rotation over 60 frames.
	anim := ab.Animation("walk", builder.WithDuration(60))
	anim.KeyframeFloat(root, 15, 0, 0.0)
	anim.KeyframeFloat(root, 15, 30, 3.14)
	anim.KeyframeFloat(root, 15, 60, 6.28)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// root bone is emitted first after Artboard → artboard-relative idx=1.
	// (global: [Backboard=0, Artboard=1, RootBone=2, Bone=3, LinearAnim=4, ...])
	// artboardOffset=1, so RootBone artboard-relative = 2-1 = 1.
	const wantBoneIdx = uint64(1)
	const wantPropKey = uint64(15)

	// Locate KeyedObject, KeyedProperty, and KeyFrameDouble.
	var kobj *rive.KeyedObject
	var kprop *rive.KeyedProperty
	var kframes []*rive.KeyFrameDouble

	for _, o := range objects {
		switch v := o.(type) {
		case *rive.KeyedObject:
			kobj = v
		case *rive.KeyedProperty:
			kprop = v
		case *rive.KeyFrameDouble:
			kframes = append(kframes, v)
		}
	}

	if kobj == nil {
		t.Fatal("no KeyedObject (tk=25) found")
	}
	if kobj.ObjectId != wantBoneIdx {
		t.Errorf("KeyedObject.ObjectId = %d, want %d (root bone artboard-relative idx)", kobj.ObjectId, wantBoneIdx)
	}

	if kprop == nil {
		t.Fatal("no KeyedProperty (tk=26) found")
	}
	if kprop.PropertyKey != wantPropKey {
		t.Errorf("KeyedProperty.PropertyKey = %d, want %d (rotation)", kprop.PropertyKey, wantPropKey)
	}

	if len(kframes) != 3 {
		t.Fatalf("want 3 KeyFrameDouble, got %d", len(kframes))
	}

	wantVals := []float64{0.0, 3.14, 6.28}
	for i, kf := range kframes {
		if kf.Value != wantVals[i] {
			t.Errorf("kframes[%d].Value = %v, want %v", i, kf.Value, wantVals[i])
		}
	}
}

// TestBuilder_BoneAnimation_TranslationX verifies translation X (propKey=13) targeting.
func TestBuilder_BoneAnimation_TranslationX(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	root := ab.RootBone("root", builder.WithTranslation(0, 0), builder.WithLength(50))

	anim := ab.Animation("slide")
	anim.KeyframeFloat(root, 13, 0, 0.0)
	anim.KeyframeFloat(root, 13, 60, 100.0)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var kprop *rive.KeyedProperty
	for _, o := range objects {
		if kp, ok := o.(*rive.KeyedProperty); ok {
			kprop = kp
			break
		}
	}
	if kprop == nil {
		t.Fatal("no KeyedProperty found")
	}
	if kprop.PropertyKey != 13 {
		t.Errorf("PropertyKey = %d, want 13 (translation X)", kprop.PropertyKey)
	}
}
