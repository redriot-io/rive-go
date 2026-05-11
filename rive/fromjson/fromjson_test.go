package fromjson_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func mustFromJSON(t *testing.T, data string) []byte {
	t.Helper()
	b, err := fromjson.FromJSON([]byte(data))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	out, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes(): %v", err)
	}
	return out
}

func mustRead(t *testing.T, data []byte) *rive.File {
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

func countType(objects []rive.Object, tk uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == tk {
			n++
		}
	}
	return n
}

func collectType(objects []rive.Object, tk uint32) []rive.Object {
	var out []rive.Object
	for _, o := range objects {
		if o.TypeKey() == tk {
			out = append(out, o)
		}
	}
	return out
}

// ── basic parse ───────────────────────────────────────────────────────────────

func TestFromJSON_MinimalRect(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Main",
			"width": 400,
			"height": 400,
			"children": [
				{
					"type": "rectangle",
					"name": "box",
					"x": 200, "y": 200,
					"width": 100, "height": 100,
					"fill": "#FF0000"
				}
			]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// Backboard, Artboard, Shape, Rectangle, Fill, SolidColor = 6
	if len(f.Objects) != 6 {
		t.Fatalf("want 6 objects, got %d", len(f.Objects))
	}
	// SolidColor should have colorValue = 0xFFFF0000
	// (reordered before its parent Fill: objects[4]=SolidColor, objects[5]=Fill)
	sc := f.Objects[4]
	if sc.TypeKey() != 18 {
		t.Fatalf("objects[4] typeKey=%d, want 18 (SolidColor)", sc.TypeKey())
	}
	props := propsByKey(sc.Properties())
	if v, ok := props[37]; !ok {
		t.Error("SolidColor.colorValue (key 37) missing")
	} else if got := uint32(v.Value.(uint64)); got != 0xFFFF0000 {
		t.Errorf("colorValue = 0x%08X, want 0xFFFF0000", got)
	}
}

func TestFromJSON_Ellipse(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 200, "height": 200,
			"children": [{"type": "ellipse", "name": "c", "x": 100, "y": 100, "width": 80, "height": 80, "fill": "#00FF00"}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// Ellipse typeKey=4
	if countType(f.Objects, 4) != 1 {
		t.Error("expected 1 Ellipse (typeKey 4)")
	}
}

func TestFromJSON_ColorFormats(t *testing.T) {
	cases := []struct {
		color string
		want  uint32
	}{
		{"#F00", 0xFFFF0000},
		{"#FF0000", 0xFFFF0000},
		{"#FFFF0000", 0xFFFF0000},
		{"#00CC3333", 0x00CC3333},
	}
	for _, tc := range cases {
		j := `{"version":1,"artboard":{"name":"A","width":100,"height":100,` +
			`"children":[{"type":"rectangle","name":"r","x":0,"y":0,"width":10,"height":10,"fill":"` +
			tc.color + `"}]}}`
		data := mustFromJSON(t, j)
		f := mustRead(t, data)
		// SolidColor is reordered before Fill; find it by type.
		scs := collectType(f.Objects, 18)
		if len(scs) == 0 {
			t.Fatalf("color %s: no SolidColor found", tc.color)
		}
		sc := scs[0]
		props := propsByKey(sc.Properties())
		got := uint32(props[37].Value.(uint64))
		if got != tc.want {
			t.Errorf("color %s: got 0x%08X, want 0x%08X", tc.color, got, tc.want)
		}
	}
}

func TestFromJSON_GradientFill(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 400, "height": 400,
			"children": [{
				"type": "ellipse", "name": "e", "x": 200, "y": 200, "width": 150, "height": 150,
				"fill": {
					"type": "linear_gradient",
					"start": [-75, 0], "end": [75, 0],
					"stops": [
						{"position": 0.0, "color": "#FF0000"},
						{"position": 1.0, "color": "#0000FF"}
					]
				}
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// LinearGradient typeKey=22
	if countType(f.Objects, 22) != 1 {
		t.Error("expected 1 LinearGradient")
	}
	// 2 GradientStops typeKey=19
	if countType(f.Objects, 19) != 2 {
		t.Errorf("expected 2 GradientStops, got %d", countType(f.Objects, 19))
	}
}

func TestFromJSON_Stroke(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 200, "height": 200,
			"children": [{
				"type": "rectangle", "name": "r", "x": 100, "y": 100, "width": 80, "height": 80,
				"fill": "#FFFFFF", "stroke": {"color": "#000000", "width": 2}
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// Stroke typeKey=24
	if countType(f.Objects, 24) != 1 {
		t.Error("expected 1 Stroke")
	}
}

// ── transform ─────────────────────────────────────────────────────────────────

func TestFromJSON_InitialRotation(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 200, "height": 200,
			"children": [{"type": "rectangle", "name": "r", "x": 100, "y": 100,
				"width": 50, "height": 50, "rotation": 45, "fill": "#FF0000"}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	shape := f.Objects[2] // Backboard, Artboard, Shape
	props := propsByKey(shape.Properties())
	v, ok := props[15] // rotation key
	if !ok {
		t.Fatal("shape.rotation (key 15) not emitted")
	}
	const want = 45 * math.Pi / 180
	if got := v.Value.(float64); math.Abs(got-want) > 1e-5 {
		t.Errorf("rotation = %v, want %.6f (π/4)", got, want)
	}
}

func TestFromJSON_InitialScale(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 200, "height": 200,
			"children": [{"type": "rectangle", "name": "r", "x": 100, "y": 100,
				"width": 50, "height": 50, "scaleX": 2.0, "scaleY": 0.5, "fill": "#FF0000"}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	shape := f.Objects[2]
	props := propsByKey(shape.Properties())
	if v, ok := props[16]; !ok || v.Value.(float64) != 2.0 {
		t.Errorf("scaleX = %v, want 2.0", props[16].Value)
	}
	if v, ok := props[17]; !ok || v.Value.(float64) != 0.5 {
		t.Errorf("scaleY = %v, want 0.5", props[17].Value)
	}
}

// ── animations ────────────────────────────────────────────────────────────────

func TestFromJSON_FloatAnimation(t *testing.T) {
	// Use 1.5s * 60fps = 90 frames. Default is 60, so 90 will be emitted.
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 400,
			"children": [{"type": "rectangle", "name": "box", "x": 200, "y": 200,
				"width": 100, "height": 100, "fill": "#FF0000"}],
			"animations": [{
				"name": "slide",
				"duration": 1.5,
				"fps": 60,
				"loop": "loop",
				"tracks": [{
					"target": "box.x",
					"keyframes": [
						{"time": 0.0, "value": 0, "easing": "linear"},
						{"time": 1.5, "value": 400}
					]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// LinearAnimation should exist
	if countType(f.Objects, 31) != 1 {
		t.Error("expected 1 LinearAnimation")
	}
	// 2 KeyFrameDouble
	if countType(f.Objects, 30) != 2 {
		t.Errorf("expected 2 KeyFrameDouble, got %d", countType(f.Objects, 30))
	}
	// duration = 90 frames (1.5s * 60fps), not the default 60 → emitted
	la := collectType(f.Objects, 31)[0]
	laProps := propsByKey(la.Properties())
	if v, ok := laProps[57]; !ok || v.Value.(uint64) != 90 {
		t.Errorf("animation duration = %v, want 90", laProps[57].Value)
	}
}

func TestFromJSON_RotationAnimation_ConvertsDegrees(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 400,
			"children": [{"type": "rectangle", "name": "sq", "x": 200, "y": 200,
				"width": 100, "height": 100, "fill": "#FF0000"}],
			"animations": [{
				"name": "spin",
				"duration": 1.0, "fps": 60, "loop": "loop",
				"tracks": [{
					"target": "sq.rotation",
					"keyframes": [
						{"time": 0.0, "value": 0},
						{"time": 1.0, "value": 360, "easing": "linear"}
					]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	kfs := collectType(f.Objects, 30)
	if len(kfs) != 2 {
		t.Fatalf("expected 2 KeyFrameDouble, got %d", len(kfs))
	}
	// Second keyframe should be 360° = 2π radians
	lastProps := propsByKey(kfs[1].Properties())
	v, ok := lastProps[70]
	if !ok {
		t.Fatal("KeyFrameDouble.value (key 70) missing")
	}
	if got := v.Value.(float64); math.Abs(got-2*math.Pi) > 1e-5 {
		t.Errorf("rotation keyframe = %v, want 2π (6.28318...)", got)
	}
}

func TestFromJSON_ColorAnimation(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 400,
			"children": [{"type": "rectangle", "name": "box", "x": 200, "y": 200,
				"width": 100, "height": 100, "fill": "#FF0000"}],
			"animations": [{
				"name": "fade",
				"duration": 1.0, "fps": 60, "loop": "loop",
				"tracks": [{
					"target": "box.fill.color",
					"keyframes": [
						{"time": 0.0, "value": "#00FF0000", "easing": "linear"},
						{"time": 0.5, "value": "#FFFF0000"},
						{"time": 1.0, "value": "#00FF0000"}
					]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// 3 KeyFrameColor (typeKey 37)
	if countType(f.Objects, 37) != 3 {
		t.Errorf("expected 3 KeyFrameColor, got %d", countType(f.Objects, 37))
	}
}

func TestFromJSON_EasingPresets(t *testing.T) {
	for _, easing := range []string{"linear", "hold", "ease-in", "ease-out", "ease-in-out"} {
		easing := easing
		t.Run(easing, func(t *testing.T) {
			j, _ := json.Marshal(easing)
			scene := `{"version":1,"artboard":{"name":"A","width":100,"height":100,` +
				`"children":[{"type":"rectangle","name":"r","x":50,"y":50,"width":10,"height":10,"fill":"#F00"}],` +
				`"animations":[{"name":"a","duration":1.0,"fps":60,"tracks":[` +
				`{"target":"r.x","keyframes":[{"time":0,"value":0,"easing":` + string(j) + `},{"time":1,"value":100}]}` +
				`]}]}}`
			data := mustFromJSON(t, scene)
			if _, err := rive.ReadBytes(data); err != nil {
				t.Errorf("easing %q: ReadBytes: %v", easing, err)
			}
		})
	}
}

func TestFromJSON_CubicEasing(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 400, "height": 400,
			"children": [{"type":"rectangle","name":"r","x":200,"y":200,"width":100,"height":100,"fill":"#FF0000"}],
			"animations": [{"name":"a","duration":1.0,"fps":60,
				"tracks":[{"target":"r.x","keyframes":[
					{"time":0,"value":0,"easing":{"x1":0.42,"y1":0,"x2":0.58,"y2":1}},
					{"time":1,"value":400}
				]}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)
	// CubicEaseInterpolator typeKey=28
	if countType(f.Objects, 28) != 1 {
		t.Errorf("expected 1 CubicEaseInterpolator, got %d", countType(f.Objects, 28))
	}
}

func TestFromJSON_PingPongLoop(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {"name": "A", "width": 400, "height": 400,
			"children": [{"type":"rectangle","name":"r","x":200,"y":200,"width":100,"height":100,"fill":"#FF0000"}],
			"animations": [{"name":"a","duration":1.0,"fps":60,"loop":"pingpong",
				"tracks":[{"target":"r.x","keyframes":[{"time":0,"value":0},{"time":1,"value":400}]}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)
	la := collectType(f.Objects, 31)[0]
	laProps := propsByKey(la.Properties())
	// loopValue key=59, PingPong=2
	if v, ok := laProps[59]; !ok || v.Value.(uint64) != 2 {
		t.Errorf("loopValue = %v, want 2 (pingpong)", laProps[59].Value)
	}
}

// ── state machines ────────────────────────────────────────────────────────────

func TestFromJSON_StateMachine(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Toggle", "width": 400, "height": 400,
			"children": [{"type":"rectangle","name":"btn","x":200,"y":200,"width":100,"height":100,"fill":"#3498DB"}],
			"animations": [
				{"name":"toOn","duration":0.3,"fps":60,"tracks":[]},
				{"name":"toOff","duration":0.3,"fps":60,"tracks":[]}
			],
			"state_machines": [{
				"name": "ToggleSM",
				"inputs": [{"name":"isOn","type":"bool"}],
				"layers": [{
					"name": "main",
					"states": [
						{"name":"off","animation":"toOff"},
						{"name":"on","animation":"toOn"}
					],
					"transitions": [
						{"from":"off","to":"on","conditions":[{"input":"isOn","value":true}]},
						{"from":"on","to":"off","conditions":[{"input":"isOn","value":false}]}
					]
				}]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	f := mustRead(t, data)

	// StateMachine typeKey=53
	if countType(f.Objects, 53) != 1 {
		t.Error("expected 1 StateMachine")
	}
	// StateMachineBool typeKey=59
	if countType(f.Objects, 59) != 1 {
		t.Error("expected 1 StateMachineBool input")
	}
}

// ── error cases ───────────────────────────────────────────────────────────────

func TestFromJSON_InvalidJSON(t *testing.T) {
	_, err := fromjson.FromJSON([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFromJSON_WrongVersion(t *testing.T) {
	_, err := fromjson.FromJSON([]byte(`{"version":2,"artboard":{"name":"A","width":100,"height":100}}`))
	if err == nil {
		t.Fatal("expected error for version 2")
	}
	if got := err.Error(); !contains(got, "version") {
		t.Errorf("error should mention 'version', got %q", got)
	}
}

func TestFromJSON_MissingArtboardName(t *testing.T) {
	_, err := fromjson.FromJSON([]byte(`{"version":1,"artboard":{"width":100,"height":100}}`))
	if err == nil {
		t.Fatal("expected error for missing artboard name")
	}
}

func TestFromJSON_DuplicateChildName(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":400,"height":400,
		"children":[
			{"type":"rectangle","name":"box","x":0,"y":0,"width":100,"height":100,"fill":"#F00"},
			{"type":"rectangle","name":"box","x":100,"y":0,"width":100,"height":100,"fill":"#0F0"}
		]}}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !contains(err.Error(), "duplicate") {
		t.Errorf("error should mention 'duplicate', got %q", err.Error())
	}
}

func TestFromJSON_UnknownTargetShape(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":400,"height":400,
		"children":[{"type":"rectangle","name":"box","x":0,"y":0,"width":100,"height":100,"fill":"#F00"}],
		"animations":[{"name":"a","duration":1,"fps":60,"tracks":[
			{"target":"nonexistent.x","keyframes":[{"time":0,"value":0},{"time":1,"value":100}]}
		]}]
	}}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for unknown target shape")
	}
}

func TestFromJSON_UnknownTargetProperty(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":400,"height":400,
		"children":[{"type":"rectangle","name":"box","x":0,"y":0,"width":100,"height":100,"fill":"#F00"}],
		"animations":[{"name":"a","duration":1,"fps":60,"tracks":[
			{"target":"box.unknownProp","keyframes":[{"time":0,"value":0},{"time":1,"value":100}]}
		]}]
	}}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for unknown property")
	}
}

func TestFromJSON_BadColorInFill(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":100,"height":100,
		"children":[{"type":"rectangle","name":"r","x":0,"y":0,"width":10,"height":10,"fill":"notacolor"}]
	}}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for invalid color")
	}
}

func TestFromJSON_UnknownShapeType(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":100,"height":100,
		"children":[{"type":"triangle","name":"t","x":0,"y":0,"width":50,"height":50,"fill":"#F00"}]
	}}`
	_, err := fromjson.FromJSON([]byte(scene))
	if err == nil {
		t.Fatal("expected error for unknown shape type")
	}
}

// ── ValidateJSON ──────────────────────────────────────────────────────────────

func TestValidateJSON_Valid(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"A","width":400,"height":400,
		"children":[{"type":"rectangle","name":"box","x":200,"y":200,"width":100,"height":100,"fill":"#F00"}]
	}}`
	errs := fromjson.ValidateJSON([]byte(scene))
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateJSON_MultipleErrors(t *testing.T) {
	scene := `{"version":2,"artboard":{"width":-10,"height":0}}`
	errs := fromjson.ValidateJSON([]byte(scene))
	if len(errs) < 3 {
		t.Errorf("expected ≥3 errors (version + name + width), got %d: %v", len(errs), errs)
	}
}

// ── round-trip ────────────────────────────────────────────────────────────────

func TestFromJSON_RoundTrip_Complex(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "Complex", "width": 500, "height": 500,
			"children": [
				{"type":"rectangle","name":"bg","x":250,"y":250,"width":500,"height":500,"fill":"#F5F5F5"},
				{"type":"ellipse","name":"ball","x":100,"y":250,"width":60,"height":60,"fill":"#E74C3C"},
				{"type":"rectangle","name":"bar","x":250,"y":450,"width":400,"height":20,
					"fill":{"type":"linear_gradient","start":[-200,0],"end":[200,0],
						"stops":[{"position":0,"color":"#3498DB"},{"position":1,"color":"#9B59B6"}]},
					"stroke":{"color":"#2C3E50","width":2}
				}
			],
			"animations": [{
				"name": "bounce",
				"duration": 1.5, "fps": 60, "loop": "pingpong",
				"tracks": [
					{"target":"ball.x","keyframes":[{"time":0,"value":100,"easing":"ease-in-out"},{"time":1.5,"value":400}]},
					{"target":"ball.y","keyframes":[{"time":0,"value":250},{"time":0.75,"value":100,"easing":"ease-out"},{"time":1.5,"value":250}]},
					{"target":"ball.scaleX","keyframes":[{"time":0,"value":1},{"time":0.75,"value":1.2},{"time":1.5,"value":1}]},
					{"target":"ball.scaleY","keyframes":[{"time":0,"value":1},{"time":0.75,"value":0.8},{"time":1.5,"value":1}]}
				]
			}]
		}
	}`
	data := mustFromJSON(t, scene)
	if _, err := rive.ReadBytes(data); err != nil {
		t.Fatalf("round-trip ReadBytes: %v", err)
	}
}

// ── helper ────────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// TestFromJSON_AutumnLeavesStructure verifies the autumn_leaves LLM-showcase scene
// generates a binary with the correct animation structure:
// - 1 LinearAnimation (typeKey=31) with name="falling", dur=360, loop=1
// - 1 CubicEaseInterpolator (typeKey=28) for ease-in-out keyframes
// - 5 KeyedObject (typeKey=25) referencing leaf shapes at objectIds 7,11,15,19,23
// - 16 KeyedProperty tracks total
func TestFromJSON_AutumnLeavesStructure(t *testing.T) {
	const scene = `{
  "version": 1,
  "artboard": {
    "name": "AutumnLeaves", "width": 400, "height": 400,
    "children": [
      {"type":"rectangle","name":"sky","x":200,"y":200,"width":400,"height":400,
       "fill":{"type":"linear_gradient","start":[0,-200],"end":[0,200],
         "stops":[{"position":0.0,"color":"#FF87CEEB"},{"position":1.0,"color":"#FFFFECD2"}]}},
      {"type":"ellipse","name":"leaf1","x":80,"y":20,"width":18,"height":28,"rotation":15,"fill":"#FFCC3300"},
      {"type":"ellipse","name":"leaf2","x":200,"y":140,"width":14,"height":22,"rotation":-10,"fill":"#FFDD6600"},
      {"type":"ellipse","name":"leaf3","x":300,"y":260,"width":16,"height":26,"rotation":25,"fill":"#FF882200"},
      {"type":"ellipse","name":"leaf4","x":360,"y":60,"width":12,"height":20,"rotation":-20,"fill":"#FFEE4400"},
      {"type":"ellipse","name":"leaf5","x":140,"y":190,"width":24,"height":36,"rotation":5,"opacity":0.9,"fill":"#FFBB4400"}
    ],
    "animations": [{"name":"falling","duration":6.0,"fps":60,"loop":"loop","tracks":[
      {"target":"leaf1.y","keyframes":[{"time":0.0,"value":20,"easing":"linear"},{"time":4.0,"value":430,"easing":"linear"}]},
      {"target":"leaf1.x","keyframes":[{"time":0.0,"value":80,"easing":"ease-in-out"},{"time":1.3,"value":110,"easing":"ease-in-out"},{"time":2.6,"value":65,"easing":"ease-in-out"},{"time":4.0,"value":95,"easing":"ease-in-out"}]},
      {"target":"leaf1.rotation","keyframes":[{"time":0.0,"value":15,"easing":"ease-in-out"},{"time":2.0,"value":-30,"easing":"ease-in-out"},{"time":4.0,"value":45,"easing":"ease-in-out"}]},
      {"target":"leaf2.y","keyframes":[{"time":0.0,"value":140,"easing":"linear"},{"time":5.0,"value":430,"easing":"linear"}]},
      {"target":"leaf2.x","keyframes":[{"time":0.0,"value":200,"easing":"ease-in-out"},{"time":1.6,"value":170,"easing":"ease-in-out"},{"time":3.3,"value":220,"easing":"ease-in-out"},{"time":5.0,"value":185,"easing":"ease-in-out"}]},
      {"target":"leaf2.rotation","keyframes":[{"time":0.0,"value":-10,"easing":"ease-in-out"},{"time":2.5,"value":35,"easing":"ease-in-out"},{"time":5.0,"value":-15,"easing":"ease-in-out"}]},
      {"target":"leaf3.y","keyframes":[{"time":0.0,"value":260,"easing":"linear"},{"time":3.5,"value":430,"easing":"linear"}]},
      {"target":"leaf3.x","keyframes":[{"time":0.0,"value":300,"easing":"ease-in-out"},{"time":1.1,"value":270,"easing":"ease-in-out"},{"time":2.3,"value":320,"easing":"ease-in-out"},{"time":3.5,"value":285,"easing":"ease-in-out"}]},
      {"target":"leaf3.rotation","keyframes":[{"time":0.0,"value":25,"easing":"ease-in-out"},{"time":1.7,"value":-20,"easing":"ease-in-out"},{"time":3.5,"value":40,"easing":"ease-in-out"}]},
      {"target":"leaf4.y","keyframes":[{"time":0.0,"value":60,"easing":"linear"},{"time":4.5,"value":430,"easing":"linear"}]},
      {"target":"leaf4.x","keyframes":[{"time":0.0,"value":360,"easing":"ease-in-out"},{"time":1.5,"value":330,"easing":"ease-in-out"},{"time":3.0,"value":375,"easing":"ease-in-out"},{"time":4.5,"value":340,"easing":"ease-in-out"}]},
      {"target":"leaf4.rotation","keyframes":[{"time":0.0,"value":-20,"easing":"ease-in-out"},{"time":2.2,"value":25,"easing":"ease-in-out"},{"time":4.5,"value":-30,"easing":"ease-in-out"}]},
      {"target":"leaf5.y","keyframes":[{"time":0.0,"value":190,"easing":"linear"},{"time":5.5,"value":430,"easing":"linear"}]},
      {"target":"leaf5.x","keyframes":[{"time":0.0,"value":140,"easing":"ease-in-out"},{"time":1.8,"value":175,"easing":"ease-in-out"},{"time":3.6,"value":115,"easing":"ease-in-out"},{"time":5.5,"value":160,"easing":"ease-in-out"}]},
      {"target":"leaf5.rotation","keyframes":[{"time":0.0,"value":5,"easing":"ease-in-out"},{"time":2.7,"value":-40,"easing":"ease-in-out"},{"time":5.5,"value":20,"easing":"ease-in-out"}]},
      {"target":"leaf5.scaleX","keyframes":[{"time":0.0,"value":1,"easing":"ease-in-out"},{"time":2.7,"value":1.15,"easing":"ease-in-out"},{"time":5.5,"value":0.95,"easing":"ease-in-out"}]}
    ]}]
  }
}`

	data := mustFromJSON(t, scene)
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Count object type keys.
	typeCounts := map[uint32]int{}
	for _, obj := range f.Objects {
		typeCounts[obj.TypeKey()]++
	}

	// 1 LinearAnimation (typeKey=31)
	if got := typeCounts[31]; got != 1 {
		t.Errorf("LinearAnimation count: got %d, want 1", got)
	}
	// 1 CubicEaseInterpolator (typeKey=28) — ease-in-out uses default params, still emitted as an object
	if got := typeCounts[28]; got != 1 {
		t.Errorf("CubicEaseInterpolator count: got %d, want 1", got)
	}
	// 5 KeyedObjects (typeKey=25), one per leaf
	if got := typeCounts[25]; got != 5 {
		t.Errorf("KeyedObject count: got %d, want 5 (one per leaf)", got)
	}
	// 16 KeyedProperties (typeKey=26)
	if got := typeCounts[26]; got != 16 {
		t.Errorf("KeyedProperty count: got %d, want 16 (3×5 leaves + 1 leaf5.scaleX)", got)
	}
	// 99 total objects
	if got := len(f.Objects); got != 99 {
		t.Errorf("total objects: got %d, want 99", got)
	}
}
