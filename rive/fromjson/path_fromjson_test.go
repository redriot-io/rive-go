package fromjson_test

import (
	"encoding/json"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func buildPathScene(t *testing.T, childrenJSON string) *rive.File {
	t.Helper()
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[` + childrenJSON + `]}}`
	b, err := fromjson.FromJSON([]byte(doc))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	return f
}

func countTypeP(objects []rive.Object, typeKey uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			n++
		}
	}
	return n
}

func propsByKeyP(props []rive.Property) map[uint32]rive.Property {
	m := make(map[uint32]rive.Property, len(props))
	for _, p := range props {
		m[p.Key] = p
	}
	return m
}

func findTypeP(objects []rive.Object, typeKey uint32) rive.Object {
	for _, o := range objects {
		if o.TypeKey() == typeKey {
			return o
		}
	}
	return nil
}

// ── Test 1: Triangle from JSON ─────────────────────────────────────────────

func TestPath_FromJSON_Triangle(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "tri",
		"x": 200, "y": 200,
		"closed": true,
		"fill": "#FF6600",
		"vertices": [
			{"x": 0, "y": -80},
			{"x": 70, "y": 40},
			{"x": -70, "y": 40}
		]
	}`)

	if countTypeP(f.Objects, 3) != 1 {
		t.Errorf("want 1 Shape, got %d", countTypeP(f.Objects, 3))
	}
	if countTypeP(f.Objects, 16) != 1 {
		t.Errorf("want 1 PointsPath, got %d", countTypeP(f.Objects, 16))
	}
	if countTypeP(f.Objects, 5) != 3 {
		t.Errorf("want 3 StraightVertex, got %d", countTypeP(f.Objects, 5))
	}
	// IsClosed (key 32) must be set
	pp := findTypeP(f.Objects, 16)
	if pp == nil {
		t.Fatal("no PointsPath found")
	}
	if _, ok := propsByKeyP(pp.Properties())[32]; !ok {
		t.Error("PointsPath: IsClosed (key 32) not emitted")
	}
	if countTypeP(f.Objects, 20) != 1 {
		t.Errorf("want 1 Fill, got %d", countTypeP(f.Objects, 20))
	}
}

// ── Test 2: Open polyline ─────────────────────────────────────────────────

func TestPath_FromJSON_OpenPolyline(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "line",
		"x": 0, "y": 0,
		"closed": false,
		"stroke": {"color": "#000000", "width": 2},
		"vertices": [
			{"x": 0, "y": 0},
			{"x": 100, "y": 0},
			{"x": 100, "y": 100}
		]
	}`)

	pp := findTypeP(f.Objects, 16)
	if pp == nil {
		t.Fatal("no PointsPath found")
	}
	// IsClosed should NOT be emitted for open path
	if _, ok := propsByKeyP(pp.Properties())[32]; ok {
		t.Error("IsClosed (key 32) should not be emitted for open path")
	}
	if countTypeP(f.Objects, 24) != 1 {
		t.Errorf("want 1 Stroke, got %d", countTypeP(f.Objects, 24))
	}
}

// ── Test 3: Cubic bezier vertices ─────────────────────────────────────────

func TestPath_FromJSON_CubicVertices(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "curve",
		"x": 200, "y": 200,
		"closed": true,
		"fill": "#9900FF",
		"vertices": [
			{"x": -60, "y": 0, "in": [-60, -40], "out": [-60, 40]},
			{"x": 0, "y": -60, "in": [-40, -60], "out": [40, -60]},
			{"x": 60, "y": 0, "in": [60, 40], "out": [60, -40]}
		]
	}`)

	// CubicDetachedVertex typeKey = 6
	if countTypeP(f.Objects, 6) != 3 {
		t.Errorf("want 3 CubicDetachedVertex, got %d", countTypeP(f.Objects, 6))
	}
	if countTypeP(f.Objects, 5) != 0 {
		t.Errorf("want 0 StraightVertex, got %d", countTypeP(f.Objects, 5))
	}
}

// ── Test 4: Corner radius on straight vertices ────────────────────────────

func TestPath_FromJSON_CornerRadius(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "rounded",
		"x": 200, "y": 200,
		"closed": true,
		"fill": "#00AAFF",
		"vertices": [
			{"x": -60, "y": -60, "radius": 15},
			{"x": 60, "y": -60, "radius": 15},
			{"x": 60, "y": 60, "radius": 15},
			{"x": -60, "y": 60, "radius": 15}
		]
	}`)

	if countTypeP(f.Objects, 5) != 4 {
		t.Errorf("want 4 StraightVertex, got %d", countTypeP(f.Objects, 5))
	}
	n := 0
	for _, o := range f.Objects {
		if o.TypeKey() == 5 {
			if _, ok := propsByKeyP(o.Properties())[26]; ok {
				n++
			}
		}
	}
	if n != 4 {
		t.Errorf("want 4 vertices with Radius (key 26), got %d", n)
	}
}

// ── Test 5: Mixed straight + cubic vertices ───────────────────────────────

func TestPath_FromJSON_MixedVertices(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "mixed",
		"x": 200, "y": 200,
		"closed": true,
		"fill": "#44AAFF",
		"vertices": [
			{"x": -60, "y": 0},
			{"x": 0, "y": -60, "in": [-60, -60], "out": [0, -60]},
			{"x": 60, "y": 0},
			{"x": 0, "y": 60, "in": [60, 60], "out": [0, 60]}
		]
	}`)

	if countTypeP(f.Objects, 5) != 2 {
		t.Errorf("want 2 StraightVertex, got %d", countTypeP(f.Objects, 5))
	}
	if countTypeP(f.Objects, 6) != 2 {
		t.Errorf("want 2 CubicDetachedVertex, got %d", countTypeP(f.Objects, 6))
	}
}

// ── Test 6: ClippingShape from JSON ──────────────────────────────────────

func TestPath_FromJSON_ClippingShape(t *testing.T) {
	f := buildPathScene(t, `
		{
			"type": "path",
			"name": "mask",
			"x": 200, "y": 200,
			"closed": true,
			"vertices": [
				{"x": 0, "y": -80},
				{"x": 80, "y": 0},
				{"x": 0, "y": 80},
				{"x": -80, "y": 0}
			]
		},
		{
			"type": "rectangle",
			"name": "target",
			"x": 200, "y": 200,
			"width": 160, "height": 160,
			"fill": "#FF0000",
			"clip": "mask"
		}
	`)

	// ClippingShape typeKey = 42
	if countTypeP(f.Objects, 42) != 1 {
		t.Errorf("want 1 ClippingShape, got %d", countTypeP(f.Objects, 42))
	}
	cs := findTypeP(f.Objects, 42)
	if cs == nil {
		t.Fatal("no ClippingShape")
	}
	// SourceId (key 92) must be set
	if _, ok := propsByKeyP(cs.Properties())[92]; !ok {
		t.Error("ClippingShape: missing SourceId (key 92)")
	}
}

// ── Test 7: Path with fill + stroke ──────────────────────────────────────

func TestPath_FromJSON_FillAndStroke(t *testing.T) {
	f := buildPathScene(t, `{
		"type": "path",
		"name": "outlined",
		"x": 200, "y": 200,
		"closed": true,
		"fill": "#FFFF00",
		"stroke": {"color": "#222222", "width": 3},
		"vertices": [
			{"x": 0, "y": -60},
			{"x": 52, "y": 30},
			{"x": -52, "y": 30}
		]
	}`)

	if countTypeP(f.Objects, 20) != 1 {
		t.Errorf("want 1 Fill, got %d", countTypeP(f.Objects, 20))
	}
	if countTypeP(f.Objects, 24) != 1 {
		t.Errorf("want 1 Stroke, got %d", countTypeP(f.Objects, 24))
	}
	if countTypeP(f.Objects, 18) != 2 {
		t.Errorf("want 2 SolidColor (fill+stroke), got %d", countTypeP(f.Objects, 18))
	}
}

// ── Test 8: Animated path fill color track ───────────────────────────────

func TestPath_FromJSON_AnimatedColor(t *testing.T) {
	doc := `{
		"version": 1,
		"artboard": {
			"name": "Main",
			"width": 400,
			"height": 400,
			"children": [{
				"type": "path",
				"name": "tri",
				"x": 200, "y": 200,
				"closed": true,
				"fill": "#FF0000",
				"vertices": [
					{"x": 0, "y": -60},
					{"x": 52, "y": 30},
					{"x": -52, "y": 30}
				]
			}],
			"animations": [{
				"name": "blink",
				"duration": 1.0,
				"fps": 60,
				"tracks": [{
					"target": "tri.fill.color",
					"keyframes": [
						{"time": 0.0, "value": "#FF0000"},
						{"time": 0.5, "value": "#0000FF", "easing": "linear"},
						{"time": 1.0, "value": "#FF0000", "easing": "linear"}
					]
				}]
			}]
		}
	}`
	b, err := fromjson.FromJSON([]byte(doc))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// LinearAnimation typeKey=31
	if countTypeP(f.Objects, 31) != 1 {
		t.Errorf("want 1 LinearAnimation, got %d", countTypeP(f.Objects, 31))
	}
	// KeyFrameColor typeKey=37
	if countTypeP(f.Objects, 37) != 3 {
		t.Errorf("want 3 KeyFrameColor, got %d", countTypeP(f.Objects, 37))
	}
}

// ── Test 9: Validation — too few vertices for closed path ─────────────────

func TestPath_FromJSON_Validation_TooFewVertices(t *testing.T) {
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[{
		"type":"path","name":"bad","x":0,"y":0,"closed":true,
		"vertices":[{"x":0,"y":0},{"x":100,"y":0}]
	}]}}`
	errs := fromjson.ValidateJSON([]byte(doc))
	found := false
	for _, e := range errs {
		if !fromjson.IsWarning(e) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for closed path with <3 vertices")
	}
}

// ── Test 10: Validation — clip source not found ───────────────────────────

func TestPath_FromJSON_Validation_ClipNotFound(t *testing.T) {
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[{
		"type":"rectangle","name":"rect","x":200,"y":200,"width":100,"height":100,
		"fill":"#FF0000","clip":"nonexistent"
	}]}}`
	errs := fromjson.ValidateJSON([]byte(doc))
	found := false
	for _, e := range errs {
		if !fromjson.IsWarning(e) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for missing clip source")
	}
}

// ── Test 11: Validation — cubic vertex missing out ────────────────────────

func TestPath_FromJSON_Validation_CubicMissingOut(t *testing.T) {
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[{
		"type":"path","name":"bad","x":0,"y":0,"closed":false,
		"vertices":[{"x":0,"y":0,"in":[10,10]}]
	}]}}`
	errs := fromjson.ValidateJSON([]byte(doc))
	found := false
	for _, e := range errs {
		if !fromjson.IsWarning(e) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for cubic vertex with in but no out")
	}
}

// ── Test 12: ValidateJSON accepts valid path ──────────────────────────────

func TestPath_FromJSON_Validation_ValidPath(t *testing.T) {
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[{
		"type":"path","name":"tri","x":200,"y":200,"closed":true,"fill":"#FF6600",
		"vertices":[{"x":0,"y":-80},{"x":70,"y":40},{"x":-70,"y":40}]
	}]}}`
	errs := fromjson.ValidateJSON([]byte(doc))
	for _, e := range errs {
		if !fromjson.IsWarning(e) {
			t.Errorf("unexpected error: %v", e)
		}
	}
}

// ── Test 13: Radial gradient fill on path ────────────────────────────────

func TestPath_FromJSON_RadialGradient(t *testing.T) {
	fillJSON, _ := json.Marshal(map[string]interface{}{
		"type":   "radial_gradient",
		"center": [2]float64{0, 0},
		"radius": 80.0,
		"stops": []map[string]interface{}{
			{"position": 0.0, "color": "#FFFFFF"},
			{"position": 1.0, "color": "#000088"},
		},
	})
	child := `{"type":"path","name":"diamond","x":200,"y":200,"closed":true,"fill":` +
		string(fillJSON) + `,"vertices":[{"x":0,"y":-80},{"x":80,"y":0},{"x":0,"y":80},{"x":-80,"y":0}]}`
	f := buildPathScene(t, child)

	if countTypeP(f.Objects, 17) != 1 { // RadialGradient
		t.Errorf("want 1 RadialGradient, got %d", countTypeP(f.Objects, 17))
	}
	if countTypeP(f.Objects, 19) != 2 { // GradientStop
		t.Errorf("want 2 GradientStop, got %d", countTypeP(f.Objects, 19))
	}
}

// ── Test 14: Path + Rectangle coexist ────────────────────────────────────

func TestPath_FromJSON_MixedShapeTypes(t *testing.T) {
	f := buildPathScene(t, `
		{"type":"path","name":"tri","x":100,"y":200,"closed":true,"fill":"#FF0000",
		 "vertices":[{"x":0,"y":-50},{"x":43,"y":25},{"x":-43,"y":25}]},
		{"type":"rectangle","name":"box","x":300,"y":200,"width":80,"height":80,"fill":"#0000FF"}
	`)

	if countTypeP(f.Objects, 3) != 2 {
		t.Errorf("want 2 Shape, got %d", countTypeP(f.Objects, 3))
	}
	if countTypeP(f.Objects, 16) != 1 {
		t.Errorf("want 1 PointsPath, got %d", countTypeP(f.Objects, 16))
	}
	if countTypeP(f.Objects, 7) != 1 { // Rectangle
		t.Errorf("want 1 Rectangle, got %d", countTypeP(f.Objects, 7))
	}
}

// ── Test 15: BuildScene error — clip a path child (not allowed) ───────────

func TestPath_FromJSON_ClipError_PathNotClippable(t *testing.T) {
	doc := `{"version":1,"artboard":{"name":"Main","width":400,"height":400,"children":[
		{"type":"path","name":"mask","x":200,"y":200,"closed":true,
		 "vertices":[{"x":0,"y":-50},{"x":43,"y":25},{"x":-43,"y":25}]},
		{"type":"path","name":"target","x":200,"y":200,"closed":true,"fill":"#FF0000","clip":"mask",
		 "vertices":[{"x":0,"y":-30},{"x":26,"y":15},{"x":-26,"y":15}]}
	]}}`
	_, err := fromjson.FromJSON([]byte(doc))
	if err == nil {
		t.Error("expected error when clipping a path child (only rect/ellipse can be clipped)")
	}
}
