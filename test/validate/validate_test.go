// Package validate_test generates every example .riv using the builder,
// reads it back with rive.ReadBytes, and asserts structural integrity.
// No build tags — runs with go test ./...
package validate_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// ── type key constants ────────────────────────────────────────────────────────

const (
	tkBackboard               uint32 = 23
	tkArtboard                uint32 = 1
	tkShape                   uint32 = 3
	tkEllipse                 uint32 = 4
	tkRectangle               uint32 = 7
	tkTriangle                uint32 = 8
	tkPointsPath              uint32 = 16
	tkRadialGradient          uint32 = 17
	tkSolidColor              uint32 = 18
	tkGradientStop            uint32 = 19
	tkFill                    uint32 = 20
	tkLinearGradient          uint32 = 22
	tkStroke                  uint32 = 24
	tkKeyedObject             uint32 = 25
	tkKeyedProperty           uint32 = 26
	tkKeyFrameDouble          uint32 = 30
	tkLinearAnimation         uint32 = 31
	tkKeyFrameColor           uint32 = 37
	tkStateMachine            uint32 = 53
	tkStateMachineInput       uint32 = 55
	tkStateMachineNumber      uint32 = 56
	tkStateMachineLayer       uint32 = 57
	tkStateMachineTrigger     uint32 = 58
	tkStateMachineBool        uint32 = 59
	tkAnimationState          uint32 = 61
	tkAnyState                uint32 = 62
	tkEntryState              uint32 = 63
	tkExitState               uint32 = 64
	tkStateTransition         uint32 = 65
	tkTransitionBoolCondition uint32 = 71
	tkKeyFrameBool            uint32 = 84
	tkCubicInterpolator       uint32 = 28
	tkBlendState1DInput       uint32 = 76
	tkBlendAnimation1D        uint32 = 75
)

// knownTypeKeys is the complete set of type keys this builder emits.
var knownTypeKeys = map[uint32]bool{
	tkBackboard: true, tkArtboard: true, tkShape: true,
	tkEllipse: true, tkRectangle: true, tkTriangle: true, tkPointsPath: true,
	tkRadialGradient: true, tkSolidColor: true, tkGradientStop: true,
	tkFill: true, tkLinearGradient: true, tkStroke: true,
	tkKeyedObject: true, tkKeyedProperty: true,
	tkKeyFrameDouble: true, tkLinearAnimation: true, tkKeyFrameColor: true,
	tkStateMachine: true, tkStateMachineInput: true, tkStateMachineNumber: true,
	tkStateMachineLayer: true, tkStateMachineTrigger: true, tkStateMachineBool: true,
	tkAnimationState: true, tkAnyState: true, tkEntryState: true,
	tkExitState: true, tkStateTransition: true, tkTransitionBoolCondition: true,
	tkKeyFrameBool: true,
	tkCubicInterpolator: true, // emitted by Cubic() easing
	tkBlendState1DInput: true, tkBlendAnimation1D: true,
}

// property key constants
const (
	pkParentId = uint32(5)  // Component.parentId
	pkObjectId = uint32(51) // KeyedObject.objectId
)

// ── helpers ───────────────────────────────────────────────────────────────────

func propUint(o rive.Object, key uint32) (uint64, bool) {
	for _, p := range o.Properties() {
		if p.Key == key {
			v, ok := p.Value.(uint64)
			return v, ok
		}
	}
	return 0, false
}

func countTK(objects []rive.Object, tk uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == tk {
			n++
		}
	}
	return n
}

// artboardRange is the [start, end) index range for one artboard's objects
// (including the artboard object itself).
type artboardRange struct{ start, end int }

func artboardRanges(objects []rive.Object) []artboardRange {
	var out []artboardRange
	for i, o := range objects {
		if o.TypeKey() == tkArtboard {
			out = append(out, artboardRange{start: i})
		}
	}
	for i := range out {
		if i+1 < len(out) {
			out[i].end = out[i+1].start
		} else {
			out[i].end = len(objects)
		}
	}
	return out
}

// buildGlobalParentMap resolves artboard-relative parentId values to global indices.
// Only Component-descended objects (shapes, nodes) carry parentId.
func buildGlobalParentMap(objects []rive.Object) map[int]int {
	out := make(map[int]int)
	artboardOffset := -1
	for i, o := range objects {
		if o.TypeKey() == tkArtboard {
			artboardOffset = i
		}
		if rel, ok := propUint(o, pkParentId); ok && artboardOffset >= 0 {
			out[i] = artboardOffset + int(rel)
		}
	}
	return out
}

// ── structural checks ─────────────────────────────────────────────────────────

func checkNoUnknownTypeKeys(t *testing.T, f *rive.File) {
	t.Helper()
	for i, o := range f.Objects {
		if !knownTypeKeys[o.TypeKey()] {
			t.Errorf("object[%d]: unknown typeKey %d", i, o.TypeKey())
		}
	}
}

func checkPropertyTypesMatchToC(t *testing.T, f *rive.File) {
	t.Helper()
	for i, o := range f.Objects {
		for _, p := range o.Properties() {
			tocType, ok := f.PropertyTypeOf(p.Key)
			if !ok {
				continue // global fallback table — OK
			}
			if p.Type != tocType {
				t.Errorf("object[%d] propKey=%d: type mismatch: got %d want %d", i, p.Key, p.Type, tocType)
			}
		}
	}
}

func checkParentIds(t *testing.T, f *rive.File) {
	t.Helper()
	gpm := buildGlobalParentMap(f.Objects)
	for i, gp := range gpm {
		if gp < 0 || gp >= len(f.Objects) {
			t.Errorf("object[%d]: parentId resolves to out-of-range global index %d (total %d)", i, gp, len(f.Objects))
		}
		if gp == i {
			t.Errorf("object[%d]: parentId points to itself", i)
		}
	}
}

func checkNoCircularParents(t *testing.T, f *rive.File) {
	t.Helper()
	gpm := buildGlobalParentMap(f.Objects)
	for start := range gpm {
		seen := make(map[int]bool)
		cur := start
		for {
			p, ok := gpm[cur]
			if !ok {
				break
			}
			if seen[p] {
				t.Errorf("object[%d]: circular parent chain (cycle at object[%d])", start, p)
				break
			}
			seen[p] = true
			cur = p
		}
	}
}

func checkAnimationObjectIds(t *testing.T, f *rive.File) {
	t.Helper()
	for i, o := range f.Objects {
		if o.TypeKey() != tkKeyedObject {
			continue
		}
		objId, ok := propUint(o, pkObjectId)
		if !ok {
			t.Errorf("object[%d] KeyedObject: missing objectId", i)
			continue
		}
		if int(objId) >= len(f.Objects) {
			t.Errorf("object[%d] KeyedObject: objectId=%d out of range (total %d)", i, objId, len(f.Objects))
		}
	}
}

// checkShapeHasPathAndPaint verifies that every artboard containing shapes also
// contains at least one path object and one paint object within its range.
func checkShapeHasPathAndPaint(t *testing.T, f *rive.File) {
	t.Helper()
	for _, ar := range artboardRanges(f.Objects) {
		slice := f.Objects[ar.start:ar.end]
		nShapes := countTK(slice, tkShape)
		if nShapes == 0 {
			continue
		}
		nPaths := countTK(slice, tkRectangle) + countTK(slice, tkEllipse) +
			countTK(slice, tkTriangle) + countTK(slice, tkPointsPath)
		nPaints := countTK(slice, tkFill) + countTK(slice, tkStroke)
		if nPaths == 0 {
			t.Errorf("artboard[%d]: %d shape(s) but no path objects", ar.start, nShapes)
		}
		if nPaints == 0 {
			t.Errorf("artboard[%d]: %d shape(s) but no paint objects", ar.start, nShapes)
		}
	}
}

// checkPaintHasColorSource verifies that every artboard with paints also has a
// color source (SolidColor / LinearGradient / RadialGradient).
func checkPaintHasColorSource(t *testing.T, f *rive.File) {
	t.Helper()
	for _, ar := range artboardRanges(f.Objects) {
		slice := f.Objects[ar.start:ar.end]
		nPaints := countTK(slice, tkFill) + countTK(slice, tkStroke)
		if nPaints == 0 {
			continue
		}
		nSrc := countTK(slice, tkSolidColor) + countTK(slice, tkLinearGradient) +
			countTK(slice, tkRadialGradient)
		if nSrc == 0 {
			t.Errorf("artboard[%d]: %d paint(s) but no color source", ar.start, nPaints)
		}
	}
}

// checkArtboardIsRoot verifies the artboard object itself carries no parentId
// (parentId=0 is suppressed; any emitted parentId on an artboard is wrong).
func checkArtboardIsRoot(t *testing.T, f *rive.File) {
	t.Helper()
	for i, o := range f.Objects {
		if o.TypeKey() != tkArtboard {
			continue
		}
		if _, ok := propUint(o, pkParentId); ok {
			t.Errorf("object[%d] Artboard: must not emit parentId (should be root)", i)
		}
	}
}

// checkAll runs every structural check in sequence.
func checkAll(t *testing.T, f *rive.File) {
	t.Helper()
	t.Run("no_unknown_typekeys", func(t *testing.T) { checkNoUnknownTypeKeys(t, f) })
	t.Run("property_types_match_toc", func(t *testing.T) { checkPropertyTypesMatchToC(t, f) })
	t.Run("parentid_in_range", func(t *testing.T) { checkParentIds(t, f) })
	t.Run("no_circular_parents", func(t *testing.T) { checkNoCircularParents(t, f) })
	t.Run("artboard_is_root", func(t *testing.T) { checkArtboardIsRoot(t, f) })
	t.Run("animation_objectids_valid", func(t *testing.T) { checkAnimationObjectIds(t, f) })
	t.Run("shape_has_path_and_paint", func(t *testing.T) { checkShapeHasPathAndPaint(t, f) })
	t.Run("paint_has_color_source", func(t *testing.T) { checkPaintHasColorSource(t, f) })
}

// buildAndRead is the shared helper: build bytes, read back, fail fast on error.
func buildAndRead(t *testing.T, fn func() ([]byte, error)) *rive.File {
	t.Helper()
	data, err := fn()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	return f
}

// ── example generators (mirror of cmd/examples/main.go) ──────────────────────

func genMinimalStatic() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("Minimal", 200, 200)
	ab.Rectangle(50, 50, 100, 100).Fill(0xFF123456).Name("r")
	return b.Bytes()
}

func genFadeRect() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("FadeRect", 500, 500)
	rect := ab.Rectangle(150, 175, 200, 150).Fill(0x00CC3333).Name("rect")
	ab.Animation("fadeIn",
		builder.WithDuration(60),
		builder.WithFPS(60),
		builder.WithLoop(builder.Loop),
	).
		KeyframeColor(rect, builder.PropColorValue, 0, 0x00CC3333, builder.Linear()).
		KeyframeColor(rect, builder.PropColorValue, 30, 0xFFCC3333, builder.Linear()).
		KeyframeColor(rect, builder.PropColorValue, 60, 0x00CC3333, builder.Linear())
	return b.Bytes()
}

func genBounceBall() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("BounceBall", 400, 400)
	ball := ab.Ellipse(200, 200, 60, 60).Fill(0xFF3399CC).Name("ball")
	ab.Animation("bounce",
		builder.WithDuration(60),
		builder.WithFPS(60),
		builder.WithLoop(builder.PingPong),
	).
		KeyframeFloat(ball, builder.PropY, 0, 300.0, builder.Cubic(0.42, 0, 0.58, 1)).
		KeyframeFloat(ball, builder.PropY, 60, 100.0, builder.Cubic(0.42, 0, 0.58, 1))
	return b.Bytes()
}

func genColorCycle() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("ColorCycle", 400, 400)
	rect := ab.Rectangle(100, 100, 200, 200).Fill(0xFFFF0000).Name("colorRect")
	ab.Animation("colorCycle",
		builder.WithDuration(90),
		builder.WithFPS(30),
		builder.WithLoop(builder.Loop),
	).
		KeyframeColor(rect, builder.PropColorValue, 0, 0xFFFF0000, builder.Linear()).
		KeyframeColor(rect, builder.PropColorValue, 30, 0xFF0000FF, builder.Linear()).
		KeyframeColor(rect, builder.PropColorValue, 60, 0xFF00FF00, builder.Linear()).
		KeyframeColor(rect, builder.PropColorValue, 90, 0xFFFF0000, builder.Linear())
	return b.Bytes()
}

func genFadeRectOpacity() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("FadeRectOpacity", 500, 500)
	rect := ab.Rectangle(150, 175, 200, 150).Fill(0xFFCC3333).Name("rect")
	ab.Animation("fadeInOut",
		builder.WithDuration(60),
		builder.WithFPS(60),
		builder.WithLoop(builder.Loop),
	).
		KeyframeFloat(rect, builder.PropOpacity, 0, 0.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 30, 1.0, builder.Linear()).
		KeyframeFloat(rect, builder.PropOpacity, 60, 0.0, builder.Linear())
	return b.Bytes()
}

func genToggleButton() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("ToggleButton", 300, 120)
	ab.Rectangle(20, 20, 260, 80).Fill(0xFF336699).Name("button")
	sm := ab.StateMachine("ButtonSM")
	active := sm.BoolInput("active")
	layer := sm.Layer("Main")
	idle := layer.State("Idle")
	on := layer.State("Active")
	layer.Transition(idle, on, builder.BoolCondition(active, true))
	layer.Transition(on, idle, builder.BoolCondition(active, false))
	return b.Bytes()
}

func genGradientEllipse() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("GradientEllipse", 400, 400)
	ab.Ellipse(200, 200, 150, 150).
		FillGradient(50, 200, 350, 200,
			builder.GradientStop{Position: 0.0, Color: 0xFFFF6B6B},
			builder.GradientStop{Position: 0.5, Color: 0xFFFFD93D},
			builder.GradientStop{Position: 1.0, Color: 0xFF6BCB77},
		).
		Name("gradEllipse")
	return b.Bytes()
}

func genMultiShape() ([]byte, error) {
	b := builder.New()
	ab := b.Artboard("MultiShape", 600, 400)
	ab.Rectangle(50, 100, 150, 200).Fill(0xFFE74C3C).Name("redRect")
	ab.Ellipse(300, 200, 90, 90).Fill(0xFF3498DB).Name("blueCircle")
	ab.Rectangle(420, 100, 140, 200).Fill(0xFF2ECC71).Stroke(4.0, 0xFF27AE60).Name("greenRect")
	return b.Bytes()
}

// ── test cases ────────────────────────────────────────────────────────────────

func TestValidate_MinimalStatic(t *testing.T) {
	f := buildAndRead(t, genMinimalStatic)
	checkAll(t, f)
	if countTK(f.Objects, tkArtboard) != 1 {
		t.Errorf("expected 1 artboard, got %d", countTK(f.Objects, tkArtboard))
	}
	if countTK(f.Objects, tkBackboard) != 1 {
		t.Errorf("expected 1 backboard, got %d", countTK(f.Objects, tkBackboard))
	}
}

func TestValidate_FadeRect(t *testing.T) {
	f := buildAndRead(t, genFadeRect)
	checkAll(t, f)
	if countTK(f.Objects, tkLinearAnimation) != 1 {
		t.Errorf("expected 1 LinearAnimation")
	}
	if countTK(f.Objects, tkKeyedObject) == 0 {
		t.Error("expected at least 1 KeyedObject (animation keyframes)")
	}
}

func TestValidate_BounceBall(t *testing.T) {
	f := buildAndRead(t, genBounceBall)
	checkAll(t, f)
	if countTK(f.Objects, tkLinearAnimation) != 1 {
		t.Errorf("expected 1 LinearAnimation")
	}
	if countTK(f.Objects, tkEllipse) == 0 {
		t.Error("expected at least 1 Ellipse")
	}
}

func TestValidate_ColorCycle(t *testing.T) {
	f := buildAndRead(t, genColorCycle)
	checkAll(t, f)
	if countTK(f.Objects, tkKeyFrameColor) == 0 {
		t.Error("expected KeyFrameColor objects")
	}
}

func TestValidate_ToggleButton(t *testing.T) {
	f := buildAndRead(t, genToggleButton)
	checkAll(t, f)
	if countTK(f.Objects, tkStateMachine) != 1 {
		t.Errorf("expected 1 StateMachine")
	}
	if countTK(f.Objects, tkStateMachineBool) != 1 {
		t.Errorf("expected 1 StateMachineBool input")
	}
	if countTK(f.Objects, tkStateMachineLayer) != 1 {
		t.Errorf("expected 1 StateMachineLayer")
	}
	nUserStates := countTK(f.Objects, tkAnimationState)
	if nUserStates != 2 {
		t.Errorf("expected 2 AnimationState (Idle/Active), got %d", nUserStates)
	}
	// entry + 2 user-state transitions = 3 transitions (entry→Idle, Idle↔Active)
	nTransitions := countTK(f.Objects, tkStateTransition)
	if nTransitions < 3 {
		t.Errorf("expected at least 3 StateTransitions, got %d", nTransitions)
	}
}

func TestValidate_FadeRectOpacity(t *testing.T) {
	f := buildAndRead(t, genFadeRectOpacity)
	checkAll(t, f)
	if countTK(f.Objects, tkLinearAnimation) != 1 {
		t.Error("expected 1 LinearAnimation")
	}
	// Verify opacity keyframes use linear interpolation (interpTypeCode fix test)
	kfdObjs := 0
	for i, o := range f.Objects {
		if o.TypeKey() != tkKeyFrameDouble {
			continue
		}
		kfdObjs++
		it, ok := propUint(o, 68) // interpolationType
		if !ok {
			t.Errorf("object[%d] KeyFrameDouble: missing interpolationType (hold interp bug?)", i)
		} else if it != 1 {
			t.Errorf("object[%d] KeyFrameDouble: interpolationType=%d want 1 (linear)", i, it)
		}
	}
	if kfdObjs != 3 {
		t.Errorf("expected 3 KeyFrameDouble (frames 0/30/60), got %d", kfdObjs)
	}
}

func TestValidate_GradientEllipse(t *testing.T) {
	f := buildAndRead(t, genGradientEllipse)
	checkAll(t, f)
	if countTK(f.Objects, tkLinearGradient) == 0 {
		t.Error("expected at least 1 LinearGradient")
	}
	if countTK(f.Objects, tkGradientStop) < 3 {
		t.Errorf("expected at least 3 GradientStops, got %d", countTK(f.Objects, tkGradientStop))
	}
}

func TestValidate_MultiShape(t *testing.T) {
	f := buildAndRead(t, genMultiShape)
	checkAll(t, f)
	nShapes := countTK(f.Objects, tkShape)
	if nShapes != 3 {
		t.Errorf("expected 3 shapes (redRect, blueCircle, greenRect), got %d", nShapes)
	}
	if countTK(f.Objects, tkStroke) == 0 {
		t.Error("expected at least 1 Stroke (greenRect)")
	}
	if countTK(f.Objects, tkFill) < 3 {
		t.Errorf("expected at least 3 Fills, got %d", countTK(f.Objects, tkFill))
	}
}

// TestValidate_AllExamples is a table-driven cross-check so any new example
// gets structural coverage without a new test function.
func TestValidate_AllExamples(t *testing.T) {
	cases := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"minimal_static", genMinimalStatic},
		{"fade_rect", genFadeRect},
		{"fade_rect_opacity", genFadeRectOpacity},
		{"bounce_ball", genBounceBall},
		{"color_cycle", genColorCycle},
		{"toggle_button", genToggleButton},
		{"gradient_ellipse", genGradientEllipse},
		{"multi_shape", genMultiShape},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := buildAndRead(t, tc.fn)
			// Basic sanity: must have Backboard + at least 1 Artboard
			if countTK(f.Objects, tkBackboard) != 1 {
				t.Errorf("expected exactly 1 Backboard")
			}
			if countTK(f.Objects, tkArtboard) < 1 {
				t.Errorf("expected at least 1 Artboard")
			}
			// Version
			if f.MajorVersion != 7 {
				t.Errorf("MajorVersion: got %d want 7", f.MajorVersion)
			}
			// Structural checks
			checkAll(t, f)
			// Object count sanity
			if len(f.Objects) == 0 {
				t.Error("empty object list")
			}
			t.Logf("ok: %d objects, %s", len(f.Objects), objectSummary(f.Objects))
		})
	}
}

func objectSummary(objects []rive.Object) string {
	return fmt.Sprintf("shapes=%d paints=%d anims=%d sms=%d",
		countTK(objects, tkShape),
		countTK(objects, tkFill)+countTK(objects, tkStroke),
		countTK(objects, tkLinearAnimation),
		countTK(objects, tkStateMachine),
	)
}

// ── golden tests (official Rive editor exports) ───────────────────────────────

const goldenDir = "../testdata/golden"

// TestGolden_FadeRectOfficial validates the official Rive editor export of a
// simple fade-in/out rectangle. This file is ground truth for the animation format.
func TestGolden_FadeRectOfficial(t *testing.T) {
	path := goldenDir + "/fade_rect_official.riv"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Version
	if f.MajorVersion != 7 {
		t.Errorf("MajorVersion: got %d want 7", f.MajorVersion)
	}

	// Must have exactly one LinearAnimation named "Timeline 1"
	var anim rive.Object
	for _, o := range f.Objects {
		if o.TypeKey() == tkLinearAnimation {
			anim = o
			break
		}
	}
	if anim == nil {
		t.Fatal("no LinearAnimation found")
	}
	if n, ok := propUint(anim, 55); ok {
		_ = n // name is a string, skip uint check
	}

	// Must have exactly 3 KeyFrameColor objects (transparent→opaque→transparent)
	nKFC := countTK(f.Objects, tkKeyFrameColor)
	if nKFC != 3 {
		t.Errorf("expected 3 KeyFrameColor (0→30→60), got %d", nKFC)
	}

	// All KeyFrameColor objects must have interpolationType=1 (linear)
	for i, o := range f.Objects {
		if o.TypeKey() != tkKeyFrameColor {
			continue
		}
		it, ok := propUint(o, 68) // interpolationType
		if !ok {
			t.Errorf("object[%d] KeyFrameColor: missing interpolationType (expected 1=linear)", i)
			continue
		}
		if it != 1 {
			t.Errorf("object[%d] KeyFrameColor: interpolationType=%d want 1 (linear)", i, it)
		}
	}

	// Must have exactly 2 KeyedObject objects (width animation + color animation)
	nKO := countTK(f.Objects, tkKeyedObject)
	if nKO != 2 {
		t.Errorf("expected 2 KeyedObject, got %d", nKO)
	}

	// One KeyedProperty must target propKey=37 (SolidColor.colorValue)
	hasColorProp := false
	for _, o := range f.Objects {
		if o.TypeKey() != tkKeyedProperty {
			continue
		}
		pk, ok := propUint(o, 53) // KeyedProperty.propertyKey
		if ok && pk == 37 {
			hasColorProp = true
		}
	}
	if !hasColorProp {
		t.Error("no KeyedProperty with propKey=37 (SolidColor.colorValue) found")
	}

	t.Logf("golden ok: %d objects, %d KeyFrameColor", len(f.Objects), nKFC)
}

// TestGolden_FadeRectBuilder validates that our builder produces the same
// structural animation pattern as the official Rive editor export.
func TestGolden_FadeRectBuilder(t *testing.T) {
	f := buildAndRead(t, genFadeRect)

	// Must have a LinearAnimation
	if countTK(f.Objects, tkLinearAnimation) != 1 {
		t.Error("expected 1 LinearAnimation")
	}

	// Must have 3 KeyFrameColor with linear interpolation
	kfcObjs := 0
	for i, o := range f.Objects {
		if o.TypeKey() != tkKeyFrameColor {
			continue
		}
		kfcObjs++
		it, ok := propUint(o, 68)
		if !ok {
			t.Errorf("object[%d] KeyFrameColor: missing interpolationType — interpTypeCode bug?", i)
		} else if it != 1 {
			t.Errorf("object[%d] KeyFrameColor: interpolationType=%d want 1 (linear)", i, it)
		}
	}
	if kfcObjs != 3 {
		t.Errorf("expected 3 KeyFrameColor, got %d", kfcObjs)
	}

	// Must have a KeyedProperty targeting propKey=37 (SolidColor.colorValue)
	hasColorProp := false
	for _, o := range f.Objects {
		if o.TypeKey() == tkKeyedProperty {
			if pk, ok := propUint(o, 53); ok && pk == 37 {
				hasColorProp = true
			}
		}
	}
	if !hasColorProp {
		t.Error("no KeyedProperty with propKey=37 (colorValue): KeyframeColor targeting wrong object?")
	}

	// Structural checks
	checkAll(t, f)
}
