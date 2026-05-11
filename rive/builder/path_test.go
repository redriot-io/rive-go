package builder_test

import (
	"math"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// ── Test 1: Triangle — 3 StraightVertex, closed ──────────────────────────────

func TestPath_Triangle(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).Name("tri").
		LineTo(0, -80).
		LineTo(70, 40).
		LineTo(-70, 40).
		Close().
		Fill(0xFFFF4400)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Expect: Shape(3), PointsPath(16), 3×StraightVertex(5), Fill(20), SolidColor(18)
	if countType(f.Objects, 3) != 1 {
		t.Errorf("want 1 Shape, got %d", countType(f.Objects, 3))
	}
	if countType(f.Objects, 16) != 1 {
		t.Errorf("want 1 PointsPath, got %d", countType(f.Objects, 16))
	}
	if countType(f.Objects, 5) != 3 {
		t.Errorf("want 3 StraightVertex, got %d", countType(f.Objects, 5))
	}

	// PointsPath should have IsClosed=true (key 32)
	pp := findType(f.Objects, 16)
	if pp == nil {
		t.Fatal("no PointsPath found")
	}
	props := propsByKey(pp.Properties())
	if _, ok := props[32]; !ok {
		t.Error("PointsPath: expected IsClosed (key 32) to be emitted")
	}
}

// ── Test 2: Open path (polyline) ─────────────────────────────────────────────

func TestPath_OpenPolyline(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	// Open path: no .Close()
	ab.Path(0, 0).
		LineTo(0, 0).
		LineTo(100, 0).
		LineTo(100, 100).
		Stroke(2.0, 0xFF000000)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 5) != 3 {
		t.Errorf("want 3 StraightVertex, got %d", countType(f.Objects, 5))
	}

	// IsClosed key (32) should NOT be emitted for an open path
	pp := findType(f.Objects, 16)
	if pp == nil {
		t.Fatal("no PointsPath found")
	}
	props := propsByKey(pp.Properties())
	if _, ok := props[32]; ok {
		t.Error("PointsPath: IsClosed (key 32) should not be emitted for open path")
	}

	// Should have Stroke(24) and SolidColor(18)
	if countType(f.Objects, 24) != 1 {
		t.Errorf("want 1 Stroke, got %d", countType(f.Objects, 24))
	}
	if countType(f.Objects, 18) != 1 {
		t.Errorf("want 1 SolidColor (stroke color), got %d", countType(f.Objects, 18))
	}
}

// ── Test 3: Corner radius on straight vertices ────────────────────────────────

func TestPath_CornerRadius(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).
		LineToR(-60, -60, 10.0).
		LineToR(60, -60, 10.0).
		LineToR(60, 60, 10.0).
		LineToR(-60, 60, 10.0).
		Close().
		Fill(0xFF00AAFF)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 5) != 4 {
		t.Errorf("want 4 StraightVertex, got %d", countType(f.Objects, 5))
	}

	// Every StraightVertex should emit Radius (key 26)
	radCount := 0
	for _, o := range f.Objects {
		if o.TypeKey() == 5 {
			props := propsByKey(o.Properties())
			if _, ok := props[26]; ok {
				radCount++
			}
		}
	}
	if radCount != 4 {
		t.Errorf("want 4 vertices with Radius, got %d", radCount)
	}
}

// ── Test 4: Cubic bezier path ─────────────────────────────────────────────────

func TestPath_CubicBezier(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	// A simple S-curve with two cubic vertices
	ab.Path(200, 200).Name("curve").
		CubicTo(-50, -50, -80, -80, -20, -80).
		CubicTo(50, 50, 20, 80, 80, 80).
		Close().
		Fill(0xFF9900FF)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// CubicDetachedVertex typeKey = 6
	if countType(f.Objects, 6) != 2 {
		t.Errorf("want 2 CubicDetachedVertex, got %d", countType(f.Objects, 6))
	}

	// Verify that InRotation/InDistance/OutRotation/OutDistance are set on a vertex.
	// First cubic vertex: anchor (-50,-50), in=(-80,-80), out=(-20,-80)
	// inRotation = atan2(-80-(-50), -80-(-50)) = atan2(-30,-30) = -3π/4 ≈ -2.356
	for _, o := range f.Objects {
		if o.TypeKey() != 6 {
			continue
		}
		props := propsByKey(o.Properties())
		// InRotation (key 84) and OutRotation (key 86) must be present
		if _, ok := props[84]; !ok {
			t.Error("CubicDetachedVertex: missing InRotation (key 84)")
		}
		if _, ok := props[86]; !ok {
			t.Error("CubicDetachedVertex: missing OutRotation (key 86)")
		}
		break
	}
}

// ── Test 5: Fill + Stroke on the same path ────────────────────────────────────

func TestPath_FillAndStroke(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 300, 300)
	ab.Path(150, 150).
		LineTo(0, -50).
		LineTo(43, 25).
		LineTo(-43, 25).
		Close().
		Fill(0xFFFFFF00).
		Stroke(3.0, 0xFF222222)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 20) != 1 {
		t.Errorf("want 1 Fill, got %d", countType(f.Objects, 20))
	}
	if countType(f.Objects, 24) != 1 {
		t.Errorf("want 1 Stroke, got %d", countType(f.Objects, 24))
	}
	if countType(f.Objects, 18) != 2 {
		t.Errorf("want 2 SolidColor (fill+stroke), got %d", countType(f.Objects, 18))
	}
}

// ── Test 6: ClippingShape ─────────────────────────────────────────────────────

func TestPath_ClippingShape(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)

	// Clip mask: a diamond path
	mask := ab.Path(200, 200).Name("mask").
		LineTo(0, -80).
		LineTo(80, 0).
		LineTo(0, 80).
		LineTo(-80, 0).
		Close()

	// Target: a rectangle clipped by the diamond mask
	target := ab.Rectangle(100, 100, 200, 200).Fill(0xFFFF0000).Name("clipped")
	target.ClipWith(mask)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// ClippingShape typeKey = 42
	if countType(f.Objects, 42) != 1 {
		t.Errorf("want 1 ClippingShape, got %d", countType(f.Objects, 42))
	}

	// Verify ClippingShape properties: SourceId (key 92) should be set
	cs := findType(f.Objects, 42)
	if cs == nil {
		t.Fatal("no ClippingShape found")
	}
	props := propsByKey(cs.Properties())
	if _, ok := props[92]; !ok {
		t.Error("ClippingShape: missing SourceId (key 92)")
	}
}

// ── Test 7: Path transform — opacity, rotation, scale ────────────────────────

func TestPath_Transform(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).
		LineTo(0, -40).
		LineTo(40, 20).
		LineTo(-40, 20).
		Close().
		Fill(0xFF0088FF).
		Opacity(0.5).
		Rotation(45.0).
		Scale(2.0, 2.0)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	shape := findType(f.Objects, 3)
	if shape == nil {
		t.Fatal("no Shape found")
	}
	props := propsByKey(shape.Properties())

	// Opacity (key 18): 0.5 ≠ 1.0, should be emitted
	if _, ok := props[18]; !ok {
		t.Error("Shape: missing Opacity (key 18)")
	}
	// Rotation (key 15): non-zero, should be emitted
	if _, ok := props[15]; !ok {
		t.Error("Shape: missing Rotation (key 15)")
	}
	// ScaleX, ScaleY (keys 16, 17): 2.0 ≠ 1.0, should be emitted
	if _, ok := props[16]; !ok {
		t.Error("Shape: missing ScaleX (key 16)")
	}
	if _, ok := props[17]; !ok {
		t.Error("Shape: missing ScaleY (key 17)")
	}

	// Verify rotation is approximately 45° in radians
	rot := props[15].Value.(float64)
	expected := 45.0 * math.Pi / 180.0
	if math.Abs(rot-expected) > 1e-6 {
		t.Errorf("Rotation: got %v, want ~%v", rot, expected)
	}
}

// ── Test 8: Round-trip encode/decode ─────────────────────────────────────────

func TestPath_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	ab.Path(250, 250).Name("star").
		LineTo(0, -100).
		LineTo(29, -40).
		LineTo(95, -31).
		LineTo(47, 15).
		LineTo(59, 81).
		LineTo(0, 50).
		LineTo(-59, 81).
		LineTo(-47, 15).
		LineTo(-95, -31).
		LineTo(-29, -40).
		Close().
		Fill(0xFFFFD700)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty output")
	}

	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	if countType(f.Objects, 5) != 10 {
		t.Errorf("want 10 StraightVertex (star), got %d", countType(f.Objects, 5))
	}
	if countType(f.Objects, 16) != 1 {
		t.Errorf("want 1 PointsPath, got %d", countType(f.Objects, 16))
	}
}

// ── Test 9: Animated vertex positions ────────────────────────────────────────

func TestPath_AnimatedVertex(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)

	p := ab.Path(200, 200).Name("morphRect").
		LineTo(-50, -50).
		LineTo(50, -50).
		LineTo(50, 50).
		LineTo(-50, 50).
		Close().
		Fill(0xFF00FF88)

	// Animate vertex 0 (top-left) Y position: squish to flat at frame 30
	v0 := p.VertexAt(0)
	if v0 == nil {
		t.Fatal("VertexAt(0) returned nil")
	}

	ab.Animation("morph", builder.WithDuration(60)).
		KeyframeFloat(v0, builder.PropVertexY, 0, -50.0, builder.Linear()).
		KeyframeFloat(v0, builder.PropVertexY, 30, 0.0, builder.Linear()).
		KeyframeFloat(v0, builder.PropVertexY, 60, -50.0, builder.Linear())

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Should have LinearAnimation (57) with keyed objects
	if countType(f.Objects, 31) != 1 {
		t.Errorf("want 1 LinearAnimation, got %d", countType(f.Objects, 31))
	}
	// KeyedObject(25) pointing at the vertex
	if countType(f.Objects, 25) < 1 {
		t.Errorf("want ≥1 KeyedObject, got %d", countType(f.Objects, 25))
	}
}

// ── Test 10: Animated path color ─────────────────────────────────────────────

func TestPath_AnimatedColor(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)

	p := ab.Path(200, 200).
		LineTo(0, -60).
		LineTo(52, 30).
		LineTo(-52, 30).
		Close().
		Fill(0xFFFF0000)

	ab.Animation("blink", builder.WithDuration(60)).
		KeyframeColor(p, builder.PropColorValue, 0, 0xFFFF0000, builder.Linear()).
		KeyframeColor(p, builder.PropColorValue, 30, 0xFF0000FF, builder.Linear()).
		KeyframeColor(p, builder.PropColorValue, 60, 0xFFFF0000, builder.Linear())

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 31) != 1 {
		t.Errorf("want 1 LinearAnimation, got %d", countType(f.Objects, 31))
	}
	// Color keyframes: 3 KeyFrameColor (37) objects
	if countType(f.Objects, 37) != 3 {
		t.Errorf("want 3 KeyFrameColor, got %d", countType(f.Objects, 37))
	}
}

// ── Test 11: Mixed StraightVertex + CubicVertex in one path ──────────────────

func TestPath_MixedVertices(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).Name("mixed").
		LineTo(-60, 0).
		CubicTo(0, -60, -60, -60, 0, -60).
		LineTo(60, 0).
		CubicTo(0, 60, 60, 60, 0, 60).
		Close().
		Fill(0xFF44AAFF)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 5) != 2 {
		t.Errorf("want 2 StraightVertex, got %d", countType(f.Objects, 5))
	}
	if countType(f.Objects, 6) != 2 {
		t.Errorf("want 2 CubicDetachedVertex, got %d", countType(f.Objects, 6))
	}
}

// ── Test 12: Multiple paths in one artboard ───────────────────────────────────

func TestPath_MultipleInArtboard(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 600, 400)

	// Path 1: triangle
	ab.Path(150, 200).
		LineTo(0, -60).LineTo(52, 30).LineTo(-52, 30).
		Close().Fill(0xFFFF0000)

	// Path 2: square (4 vertices)
	ab.Path(450, 200).
		LineTo(-50, -50).LineTo(50, -50).LineTo(50, 50).LineTo(-50, 50).
		Close().Fill(0xFF0000FF)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	if countType(f.Objects, 3) != 2 {
		t.Errorf("want 2 Shape, got %d", countType(f.Objects, 3))
	}
	if countType(f.Objects, 16) != 2 {
		t.Errorf("want 2 PointsPath, got %d", countType(f.Objects, 16))
	}
	if countType(f.Objects, 5) != 7 {
		t.Errorf("want 7 StraightVertex total (3+4), got %d", countType(f.Objects, 5))
	}
}

// ── Test 13: Path with radial gradient fill ───────────────────────────────────

func TestPath_RadialGradientFill(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).
		LineTo(0, -80).LineTo(80, 0).LineTo(0, 80).LineTo(-80, 0).
		Close().
		FillRadialGradient(0, 0, 80, 0,
			builder.GradientStop{Position: 0.0, Color: 0xFFFFFFFF},
			builder.GradientStop{Position: 1.0, Color: 0xFF000088},
		)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// RadialGradient typeKey = 17
	if countType(f.Objects, 17) != 1 {
		t.Errorf("want 1 RadialGradient, got %d", countType(f.Objects, 17))
	}
	// GradientStop typeKey = 19
	if countType(f.Objects, 19) != 2 {
		t.Errorf("want 2 GradientStop, got %d", countType(f.Objects, 19))
	}
}

// ── Test 14: ParentId chain correctness ───────────────────────────────────────

func TestPath_ParentIdChain(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Path(200, 200).Name("chain").
		LineTo(-30, -30).
		LineTo(30, -30).
		LineTo(30, 30).
		Close().
		Fill(0xFFAA5500)

	data := mustBuild(t, b)
	f := mustReadBytes(t, data)

	// Find indices: Backboard=0, Artboard=1, Shape=2 (emitted last = front layer),
	// PointsPath=3, Vertex0=4, Vertex1=5, Vertex2=6, Fill=7, SolidColor=8
	// Artboard offset=1, so artboard-relative: Shape=1, PP=2, V0=3, V1=4, V2=5, Fill=6, SC=7

	// Locate the Shape object
	var shapeGlobal int = -1
	for i, o := range f.Objects {
		if o.TypeKey() == 3 {
			shapeGlobal = i
			break
		}
	}
	if shapeGlobal < 0 {
		t.Fatal("no Shape found")
	}

	// PointsPath should be right after Shape
	ppGlobal := shapeGlobal + 1
	if f.Objects[ppGlobal].TypeKey() != 16 {
		t.Errorf("expected PointsPath at global[%d], got typeKey %d", ppGlobal, f.Objects[ppGlobal].TypeKey())
	}

	// First vertex should be right after PointsPath
	v0Global := ppGlobal + 1
	if f.Objects[v0Global].TypeKey() != 5 {
		t.Errorf("expected StraightVertex at global[%d], got typeKey %d", v0Global, f.Objects[v0Global].TypeKey())
	}

	// PointsPath.ParentId should reference the Shape (artboard-relative index)
	artboardOffset := 1 // Backboard is global[0]
	shapeRelIdx := uint64(shapeGlobal - artboardOffset)
	ppProps := propsByKey(f.Objects[ppGlobal].Properties())
	if p, ok := ppProps[5]; ok {
		if p.Value.(uint64) != shapeRelIdx {
			t.Errorf("PointsPath.ParentId: got %v, want %v", p.Value, shapeRelIdx)
		}
	} else {
		t.Error("PointsPath: missing ParentId (key 5)")
	}
}
