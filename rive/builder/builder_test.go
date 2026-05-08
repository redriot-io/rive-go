package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// --- helpers ---

func mustBuild(t *testing.T, b *builder.Builder) []byte {
	t.Helper()
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes(): %v", err)
	}
	return data
}

func mustReadBytes(t *testing.T, data []byte) *rive.File {
	t.Helper()
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	return f
}

func propsByKey(props []rive.Property) map[uint32]rive.Property {
	m := make(map[uint32]rive.Property, len(props))
	for _, p := range props {
		m[p.Key] = p
	}
	return m
}

func findType(objects []rive.Object, typeKey uint32) rive.Object {
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			return o
		}
	}
	return nil
}

func countType(objects []rive.Object, typeKey uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			n++
		}
	}
	return n
}

// ── Core builder tests ────────────────────────────────────────────────────────

func TestBuilder_NoArtboard(t *testing.T) {
	_, err := builder.New().Build()
	if err == nil {
		t.Fatal("expected error for no artboards, got nil")
	}
}

func TestBuilder_MinimalArtboard(t *testing.T) {
	b := builder.New()
	b.Artboard("Main", 500, 400)
	data := mustBuild(t, b)

	f := mustReadBytes(t, data)
	// Should have: Backboard(0) + Artboard(1)
	if len(f.Objects) < 2 {
		t.Fatalf("want ≥ 2 objects, got %d", len(f.Objects))
	}

	// Backboard typeKey=23
	if f.Objects[0].TypeKey() != 23 {
		t.Errorf("objects[0] typeKey=%d, want 23 (Backboard)", f.Objects[0].TypeKey())
	}
	// Artboard typeKey=1
	if f.Objects[1].TypeKey() != 1 {
		t.Errorf("objects[1] typeKey=%d, want 1 (Artboard)", f.Objects[1].TypeKey())
	}

	// Verify artboard width/height
	ab := propsByKey(f.Objects[1].Properties())
	if v, ok := ab[7]; ok {
		if v.Value.(float64) != 500 {
			t.Errorf("artboard width = %v, want 500", v.Value)
		}
	} else {
		t.Error("artboard width (key 7) missing")
	}
	if v, ok := ab[8]; ok {
		if v.Value.(float64) != 400 {
			t.Errorf("artboard height = %v, want 400", v.Value)
		}
	} else {
		t.Error("artboard height (key 8) missing")
	}
}

func TestBuilder_EmptyArtboard(t *testing.T) {
	b := builder.New()
	b.Artboard("Empty", 100, 100)
	data := mustBuild(t, b)
	f := mustReadBytes(t, data)
	if len(f.Objects) != 2 {
		t.Fatalf("want 2 objects (Backboard+Artboard), got %d", len(f.Objects))
	}
}

func TestBuilder_RectangleWithFill(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	ab.Rectangle(10, 20, 200, 150).Fill(0xFFFF0000).Name("myRect")

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Expected: Backboard, Artboard, Shape, Rectangle, Fill, SolidColor
	if len(f.Objects) != 6 {
		t.Fatalf("want 6 objects, got %d", len(f.Objects))
	}
	// Shape typeKey=3
	if f.Objects[2].TypeKey() != 3 {
		t.Errorf("objects[2] typeKey=%d, want 3 (Shape)", f.Objects[2].TypeKey())
	}
	// Rectangle typeKey=7
	if f.Objects[3].TypeKey() != 7 {
		t.Errorf("objects[3] typeKey=%d, want 7 (Rectangle)", f.Objects[3].TypeKey())
	}
	// Fill typeKey=20
	if f.Objects[4].TypeKey() != 20 {
		t.Errorf("objects[4] typeKey=%d, want 20 (Fill)", f.Objects[4].TypeKey())
	}
	// SolidColor typeKey=18
	if f.Objects[5].TypeKey() != 18 {
		t.Errorf("objects[5] typeKey=%d, want 18 (SolidColor)", f.Objects[5].TypeKey())
	}

	// Verify shape x/y
	shapeProps := propsByKey(f.Objects[2].Properties())
	if v, ok := shapeProps[13]; ok {
		if v.Value.(float64) != 10 {
			t.Errorf("shape.X = %v, want 10", v.Value)
		}
	} else {
		t.Error("shape.X (key 13) missing")
	}
	if v, ok := shapeProps[14]; ok {
		if v.Value.(float64) != 20 {
			t.Errorf("shape.Y = %v, want 20", v.Value)
		}
	} else {
		t.Error("shape.Y (key 14) missing")
	}

	// Verify parentId chain (artboard-relative indexing).
	// parentId=0 means "artboard root" and is suppressed by the writer (gen_root.go).
	shapeParent := propsByKey(f.Objects[2].Properties())
	if v, ok := shapeParent[5]; ok && v.Value.(uint64) != 0 {
		t.Errorf("shape parentId = %v, want 0 (artboard root, suppressed)", v.Value)
	}
	// Rectangle's parent is Shape at artboard-relative index 1.
	rectParent := propsByKey(f.Objects[3].Properties())
	if v, ok := rectParent[5]; !ok || v.Value.(uint64) != 1 {
		t.Errorf("rectangle parentId = %v, want 1 (shape, artboard-relative)", v.Value)
	}
}

func TestBuilder_EllipseWithGradient(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Ellipse(200, 200, 100, 80).FillGradient(0, 0, 100, 0,
		builder.GradientStop{Position: 0.0, Color: 0xFFFF0000},
		builder.GradientStop{Position: 1.0, Color: 0xFF0000FF},
	)
	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Backboard, Artboard, Shape, Ellipse, Fill, LinearGradient, Stop, Stop
	if len(f.Objects) != 8 {
		t.Fatalf("want 8 objects, got %d: %v", len(f.Objects), typeKeyList(f.Objects))
	}
	// LinearGradient typeKey=22
	if f.Objects[5].TypeKey() != 22 {
		t.Errorf("objects[5] typeKey=%d, want 22 (LinearGradient)", f.Objects[5].TypeKey())
	}
	// Two GradientStops typeKey=19
	if f.Objects[6].TypeKey() != 19 || f.Objects[7].TypeKey() != 19 {
		t.Errorf("expected GradientStop (19) at [6],[7], got %d, %d",
			f.Objects[6].TypeKey(), f.Objects[7].TypeKey())
	}
}

func TestBuilder_MultipleShapes(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Multi", 600, 600)
	ab.Rectangle(0, 0, 100, 100).Fill(0xFFFF0000)
	ab.Rectangle(100, 0, 100, 100).Fill(0xFF00FF00)
	ab.Ellipse(200, 0, 50, 50).Fill(0xFF0000FF)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// 3 shapes: each has Shape(3) + path(7 or 4) + Fill(20) + SolidColor(18)
	// = 3*4 = 12 child objects + 2 (Backboard+Artboard) = 14
	if len(f.Objects) != 14 {
		t.Fatalf("want 14 objects, got %d: %v", len(f.Objects), typeKeyList(f.Objects))
	}

	// Verify second shape's parentId: artboard-relative 0 = artboard root, suppressed.
	if v, ok := propsByKey(f.Objects[6].Properties())[5]; ok && v.Value.(uint64) != 0 {
		t.Errorf("second shape parentId = %v, want 0 (artboard root, suppressed)", v.Value)
	}
}

func TestBuilder_Stroke(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Rectangle(50, 50, 200, 100).
		Fill(0xFFFFFFFF).
		Stroke(3.0, 0xFF000000)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Backboard, Artboard, Shape, Rectangle, Fill, SolidColor, Stroke, SolidColor
	if len(f.Objects) != 8 {
		t.Fatalf("want 8 objects, got %d: %v", len(f.Objects), typeKeyList(f.Objects))
	}
	// Stroke typeKey=24 at index 6
	if f.Objects[6].TypeKey() != 24 {
		t.Errorf("objects[6] typeKey=%d, want 24 (Stroke)", f.Objects[6].TypeKey())
	}
}

// ── Animation tests ───────────────────────────────────────────────────────────

func TestBuilder_FadeAnimation(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	rect := ab.Rectangle(100, 100, 200, 200).Fill(0xFFFF0000)
	ab.Animation("fadeIn", builder.WithDuration(30), builder.WithFPS(60)).
		KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear())

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Verify LinearAnimation exists
	la := findType(f.Objects, 31) // LinearAnimation typeKey=31
	if la == nil {
		t.Fatal("LinearAnimation (typeKey 31) not found")
	}
	laProps := propsByKey(la.Properties())

	// fps default is 60, should not be emitted (matches default)
	// duration=30 should be emitted
	if v, ok := laProps[57]; !ok || v.Value.(uint64) != 30 {
		t.Errorf("animation duration = %v, want 30", laProps[57].Value)
	}

	// Verify KeyedObject exists
	if findType(f.Objects, 25) == nil {
		t.Error("KeyedObject (typeKey 25) not found")
	}
	// Verify KeyedProperty exists
	if findType(f.Objects, 26) == nil {
		t.Error("KeyedProperty (typeKey 26) not found")
	}
	// Verify two KeyFrameDouble exist
	if n := countType(f.Objects, 30); n != 2 {
		t.Errorf("want 2 KeyFrameDouble (typeKey 30), got %d", n)
	}

	// Round-trip passes (no error reading back)
	_ = f
}

func TestBuilder_MoveAnimation(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFFFF0000)
	ab.Animation("move").
		KeyframeFloat(rect, builder.PropX, 0, 0.0).
		KeyframeFloat(rect, builder.PropX, 60, 400.0).
		KeyframeFloat(rect, builder.PropY, 0, 0.0).
		KeyframeFloat(rect, builder.PropY, 60, 300.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// x and y are different properties on the same object → 1 KeyedObject, 2 KeyedProperties
	if n := countType(f.Objects, 25); n != 1 {
		t.Errorf("want 1 KeyedObject, got %d", n)
	}
	if n := countType(f.Objects, 26); n != 2 {
		t.Errorf("want 2 KeyedProperty (x and y), got %d", n)
	}
	if n := countType(f.Objects, 30); n != 4 {
		t.Errorf("want 4 KeyFrameDouble (2 per prop), got %d", n)
	}
}

func TestBuilder_CubicInterp(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	rect := ab.Rectangle(0, 0, 100, 100).Fill(0xFFFF0000)
	ab.Animation("ease").
		KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Cubic(0.42, 0, 0.58, 1.0)).
		KeyframeFloat(rect, builder.PropOpacity, 60, 1.0, builder.Linear())

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// First keyframe should have interpolationType=2 (cubic), emitted
	kfs := collectType(f.Objects, 30)
	if len(kfs) < 1 {
		t.Fatal("no KeyFrameDouble found")
	}
	// First keyframe at frame=0, interp=cubic(2)
	kf0Props := propsByKey(kfs[0].Properties())
	if v, ok := kf0Props[68]; !ok || v.Value.(uint64) != 2 {
		t.Errorf("first keyframe interpolationType = %v, want 2 (cubic)", kf0Props[68].Value)
	}
}

func TestBuilder_MultiPropAnimation(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	r1 := ab.Rectangle(0, 0, 100, 100).Fill(0xFF0000FF)
	r2 := ab.Rectangle(200, 0, 100, 100).Fill(0xFF00FF00)

	ab.Animation("multi").
		KeyframeFloat(r1, builder.PropOpacity, 0, 0.0).
		KeyframeFloat(r1, builder.PropOpacity, 30, 1.0).
		KeyframeFloat(r2, builder.PropX, 0, 200.0).
		KeyframeFloat(r2, builder.PropX, 30, 400.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// 2 shapes → 2 KeyedObjects
	if n := countType(f.Objects, 25); n != 2 {
		t.Errorf("want 2 KeyedObjects (one per shape), got %d", n)
	}
}

// ── State machine tests ───────────────────────────────────────────────────────

func TestBuilder_BasicStateMachine(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Rectangle(50, 50, 100, 100).Fill(0xFFFF0000)
	sm := ab.StateMachine("Main State Machine")
	layer := sm.Layer("Layer 1")
	idle := layer.State("Idle")
	active := layer.State("Active")
	layer.Transition(idle, active)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Verify StateMachine (typeKey=53) exists
	if findType(f.Objects, 53) == nil {
		t.Error("StateMachine (typeKey 53) not found")
	}
	// Verify StateMachineLayer (typeKey=57) exists
	if findType(f.Objects, 57) == nil {
		t.Error("StateMachineLayer (typeKey 57) not found")
	}
}

func TestBuilder_SentinelInjection(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	layer := sm.Layer("Layer 1")
	layer.State("Idle")

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// AnyState (62), EntryState (63), ExitState (64) must all be present
	if findType(f.Objects, 62) == nil {
		t.Error("AnyState (typeKey 62) not injected")
	}
	if findType(f.Objects, 63) == nil {
		t.Error("EntryState (typeKey 63) not injected")
	}
	if findType(f.Objects, 64) == nil {
		t.Error("ExitState (typeKey 64) not injected")
	}
}

func TestBuilder_BoolTransition(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	active := sm.BoolInput("active")
	layer := sm.Layer("Layer 1")
	idle := layer.State("Idle")
	on := layer.State("On")
	layer.Transition(idle, on, builder.BoolCondition(active, true))

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// StateMachineBool (typeKey=59) exists
	if findType(f.Objects, 59) == nil {
		t.Error("StateMachineBool (typeKey 59) not found")
	}
	// StateTransition (typeKey=65) exists
	if findType(f.Objects, 65) == nil {
		t.Error("StateTransition (typeKey 65) not found")
	}
	// TransitionBoolCondition (typeKey=71) exists
	if findType(f.Objects, 71) == nil {
		t.Error("TransitionBoolCondition (typeKey 71) not found")
	}
}

func TestBuilder_TriggerTransition(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	sm := ab.StateMachine("SM")
	fire := sm.TriggerInput("fire")
	layer := sm.Layer("Layer 1")
	idle := layer.State("Idle")
	burst := layer.State("Burst")
	layer.Transition(idle, burst, builder.TriggerCondition(fire))

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// StateMachineTrigger (typeKey=58)
	if findType(f.Objects, 58) == nil {
		t.Error("StateMachineTrigger (typeKey 58) not found")
	}
	// TransitionTriggerCondition (typeKey=68)
	if findType(f.Objects, 68) == nil {
		t.Error("TransitionTriggerCondition (typeKey 68) not found")
	}
}

// ── Integration / round-trip ──────────────────────────────────────────────────

func TestBuilder_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Scene", 800, 600)
	rect := ab.Rectangle(100, 100, 200, 150).
		Fill(0xFF336699).
		Name("box")
	ab.Animation("slide", builder.WithDuration(60)).
		KeyframeFloat(rect, builder.PropX, 0, 100.0).
		KeyframeFloat(rect, builder.PropX, 60, 700.0)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	// First read
	f1, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("first ReadBytes: %v", err)
	}

	// Write back
	data2, err := rive.WriteBytes(f1.Objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	// Second read
	f2, err := rive.ReadBytes(data2)
	if err != nil {
		t.Fatalf("second ReadBytes: %v", err)
	}

	if len(f1.Objects) != len(f2.Objects) {
		t.Fatalf("object count mismatch: %d → %d", len(f1.Objects), len(f2.Objects))
	}
}

func TestBuilder_OutputLoadsInReader(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	rect := ab.Rectangle(0, 0, 200, 200).Fill(0xFFAA33CC).Name("r")
	sm := ab.StateMachine("SM")
	tog := sm.BoolInput("toggle")
	layer := sm.Layer("L1")
	s1 := layer.State("Off")
	s2 := layer.State("On")
	layer.Transition(s1, s2, builder.BoolCondition(tog, true))

	_ = rect
	data := mustBuild(t, b)
	if _, err := rive.ReadBytes(data); err != nil {
		t.Fatalf("ReadBytes after complex build: %v", err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func typeKeyList(objects []rive.Object) []uint32 {
	out := make([]uint32, len(objects))
	for i, o := range objects {
		out[i] = o.TypeKey()
	}
	return out
}

func collectType(objects []rive.Object, typeKey uint32) []rive.Object {
	var out []rive.Object
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			out = append(out, o)
		}
	}
	return out
}
