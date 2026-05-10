package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

const tkKeyFrameId = 50 // KeyFrameId typeKey

func TestKeyframeDrawOrder_EmitsDrawTargetAndRules(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ball := ab.Ellipse(200, 200, 40, 40).Fill(0xFFFF6B6B).Name("ball")
	box := ab.Rectangle(200, 200, 100, 100).Fill(0xFF1A1A2E).Name("box")

	anim := ab.Animation("pass", builder.WithDuration(120), builder.WithFPS(60), builder.WithLoop(builder.Loop))
	// At frame 0: no rule (default order)
	// At frame 30: ball draws above box
	// At frame 90: ball draws below box
	anim.KeyframeDrawOrder(ball, 0, nil, builder.PlacementAbove)
	anim.KeyframeDrawOrder(ball, 30, box, builder.PlacementAbove)
	anim.KeyframeDrawOrder(ball, 90, box, builder.PlacementBelow)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Expect exactly 2 DrawTarget objects (above + below, for the 2 unique placements of box)
	// and exactly 1 DrawRules object (the animated one)
	dts := collectType(objects, tkDrawTarget)
	drs := collectType(objects, tkDrawRules)
	if len(dts) != 2 {
		t.Errorf("expected 2 DrawTarget objects, got %d", len(dts))
	}
	if len(drs) != 1 {
		t.Errorf("expected 1 DrawRules object, got %d", len(drs))
	}

	// DrawRules parent must be ball.shapeIdx (not 0)
	dr := drs[0].(*rive.DrawRules)
	if dr.ParentId == 0 {
		t.Errorf("DrawRules.ParentId = 0; expected ball's artboard-relative index")
	}
	// DrawRules.DrawTargetId must be sentinel (animated; suppressed at rest)
	if dr.DrawTargetId != ^uint64(0) {
		t.Errorf("animated DrawRules.DrawTargetId should be sentinel (^uint64(0)), got %d", dr.DrawTargetId)
	}

	// Expect exactly 3 KeyFrameId objects (one per draw order keyframe)
	kfIds := collectType(objects, tkKeyFrameId)
	if len(kfIds) != 3 {
		t.Fatalf("expected 3 KeyFrameId objects, got %d", len(kfIds))
	}

	// Frame 0 keyframe: value = sentinel (no rule)
	kf0 := kfIds[0].(*rive.KeyFrameId)
	if kf0.Frame != 0 {
		t.Errorf("kf[0].Frame = %d, want 0", kf0.Frame)
	}
	if kf0.Value != ^uint64(0) {
		t.Errorf("kf[0].Value = %d, want sentinel (no rule)", kf0.Value)
	}

	// Frame 30 keyframe: value = index of DrawTarget(above)
	kf30 := kfIds[1].(*rive.KeyFrameId)
	if kf30.Frame != 30 {
		t.Errorf("kf[1].Frame = %d, want 30", kf30.Frame)
	}
	if kf30.Value == ^uint64(0) {
		t.Errorf("kf[1].Value should reference a DrawTarget, not sentinel")
	}

	// Frame 90 keyframe: different DrawTarget (below)
	kf90 := kfIds[2].(*rive.KeyFrameId)
	if kf90.Frame != 90 {
		t.Errorf("kf[2].Frame = %d, want 90", kf90.Frame)
	}
	if kf90.Value == kf30.Value {
		t.Errorf("above and below should reference different DrawTarget objects")
	}

	// All draw order objects must appear BEFORE the LinearAnimation
	animIdx := -1
	lastDTIdx := -1
	lastDRIdx := -1
	for i, o := range objects {
		if o.TypeKey() == 31 && animIdx < 0 { // LinearAnimation
			animIdx = i
		}
		if o.TypeKey() == tkDrawTarget {
			lastDTIdx = i
		}
		if o.TypeKey() == tkDrawRules {
			lastDRIdx = i
		}
	}
	if animIdx < 0 {
		t.Fatal("no LinearAnimation found")
	}
	if lastDTIdx >= animIdx {
		t.Errorf("DrawTarget (idx=%d) must appear before LinearAnimation (idx=%d)", lastDTIdx, animIdx)
	}
	if lastDRIdx >= animIdx {
		t.Errorf("DrawRules (idx=%d) must appear before LinearAnimation (idx=%d)", lastDRIdx, animIdx)
	}
}

func TestKeyframeDrawOrder_HoldInterpolation(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 300, 300)
	src := ab.Rectangle(150, 150, 60, 60).Fill(0xFFFF0000).Name("src")
	tgt := ab.Rectangle(150, 150, 60, 60).Fill(0xFF0000FF).Name("tgt")

	ab.Animation("switch", builder.WithDuration(60)).
		KeyframeDrawOrder(src, 0, nil, builder.PlacementAbove).
		KeyframeDrawOrder(src, 30, tgt, builder.PlacementAbove)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, o := range objects {
		if o.TypeKey() != tkKeyFrameId {
			continue
		}
		kf := o.(*rive.KeyFrameId)
		// InterpolationType=0 is hold (the default; suppressed when 0)
		if kf.InterpolationType != 0 {
			t.Errorf("KeyFrameId.InterpolationType = %d, want 0 (hold)", kf.InterpolationType)
		}
	}
}

func TestKeyframeDrawOrder_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ball := ab.Ellipse(200, 200, 50, 50).Fill(0xFFFF6B6B).Name("ball")
	box := ab.Rectangle(200, 200, 120, 120).Fill(0xFF1A1A2E).Name("box")

	ab.Animation("demo", builder.WithDuration(120), builder.WithLoop(builder.Loop)).
		KeyframeDrawOrder(ball, 0, nil, builder.PlacementAbove).
		KeyframeDrawOrder(ball, 60, box, builder.PlacementAbove)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	if countType(f.Objects, tkDrawTarget) != 1 {
		t.Errorf("round-trip: expected 1 DrawTarget")
	}
	if countType(f.Objects, tkDrawRules) != 1 {
		t.Errorf("round-trip: expected 1 DrawRules")
	}
	if countType(f.Objects, tkKeyFrameId) != 2 {
		t.Errorf("round-trip: expected 2 KeyFrameId objects")
	}
}

func TestKeyframeDrawOrder_MixedWithStaticRules(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	a := ab.Rectangle(100, 200, 60, 60).Fill(0xFFFF0000).Name("a")
	bShape := ab.Rectangle(200, 200, 60, 60).Fill(0xFF00FF00).Name("b")
	c := ab.Rectangle(300, 200, 60, 60).Fill(0xFF0000FF).Name("c")

	// Static rule: a always above c
	a.DrawAbove(c)

	// Animated rule: b switches above/below a
	ab.Animation("anim", builder.WithDuration(120), builder.WithLoop(builder.Loop)).
		KeyframeDrawOrder(bShape, 0, nil, builder.PlacementAbove).
		KeyframeDrawOrder(bShape, 60, a, builder.PlacementAbove)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Static rule produces 1 DrawTarget + 1 DrawRules; animated adds 1 DrawTarget + 1 DrawRules
	if countType(objects, tkDrawTarget) != 2 {
		t.Errorf("expected 2 DrawTarget objects (1 static + 1 animated), got %d", countType(objects, tkDrawTarget))
	}
	if countType(objects, tkDrawRules) != 2 {
		t.Errorf("expected 2 DrawRules objects (1 static + 1 animated), got %d", countType(objects, tkDrawRules))
	}
}

func TestKeyframeDrawOrder_NoRules_NoExtraObjects(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 200, 200)
	shape := ab.Rectangle(100, 100, 40, 40).Fill(0xFFFF0000).Name("shape")
	ab.Animation("spin", builder.WithDuration(60), builder.WithLoop(builder.Loop)).
		KeyframeFloat(shape, builder.PropRotation, 0, 0, builder.Linear()).
		KeyframeFloat(shape, builder.PropRotation, 60, 6.283, builder.Linear())

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if countType(objects, tkKeyFrameId) != 0 {
		t.Errorf("expected 0 KeyFrameId for animation with no draw order keyframes")
	}
}
