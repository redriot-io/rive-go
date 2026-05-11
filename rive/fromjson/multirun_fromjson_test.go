package fromjson_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// fakeFonts is a minimal bytes map that satisfies FromJSONWithFonts for tests
// that don't care about actual glyph rendering.
var fakeFonts = map[string][]byte{
	"fontA.ttf": []byte("FAKE-TTF-A"),
	"fontB.ttf": []byte("FAKE-TTF-B"),
	"fontC.ttf": []byte("FAKE-TTF-C"),
}

func fromJSONWithFake(t *testing.T, scene string) []rive.Object {
	t.Helper()
	b, err := fromjson.FromJSONWithFonts([]byte(scene), fakeFonts)
	if err != nil {
		t.Fatalf("FromJSONWithFonts: %v", err)
	}
	raw, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(raw)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	return f.Objects
}

func countTypeKey(objects []rive.Object, tk uint32) int {
	n := 0
	for _, o := range objects {
		if o.TypeKey() == tk {
			n++
		}
	}
	return n
}

// ── TestFromJSONMultiRun_TwoStyles ────────────────────────────────────────────

func TestFromJSONMultiRun_TwoStyles(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [
				{"name": "FontA", "file": "fontA.ttf"},
				{"name": "FontB", "file": "fontB.ttf"}
			],
			"children": [{
				"type": "text", "name": "mixed", "x": 200, "y": 100,
				"styles": [
					{"name": "heading", "font": "FontA", "fontSize": 32, "fill": "#000000"},
					{"name": "body",    "font": "FontB", "fontSize": 16, "fill": "#666666"}
				],
				"runs": [
					{"text": "Title\n", "style": "heading"},
					{"text": "Body text here", "style": "body"}
				]
			}]
		}
	}`

	objs := fromJSONWithFake(t, scene)

	if n := countTypeKey(objs, 135); n != 2 {
		t.Errorf("want 2 TextValueRuns, got %d", n)
	}
	if n := countTypeKey(objs, 137); n != 2 {
		t.Errorf("want 2 TextStyles, got %d", n)
	}
}

// ── TestFromJSONMultiRun_ThreeRuns ────────────────────────────────────────────

func TestFromJSONMultiRun_ThreeRuns(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [
				{"name": "FA", "file": "fontA.ttf"},
				{"name": "FB", "file": "fontB.ttf"}
			],
			"children": [{
				"type": "text", "name": "t", "x": 200, "y": 100,
				"styles": [
					{"name": "sA", "font": "FA", "fontSize": 24, "fill": "#111111"},
					{"name": "sB", "font": "FB", "fontSize": 18, "fill": "#888888"}
				],
				"runs": [
					{"text": "here is ", "style": "sA"},
					{"text": "some",     "style": "sB"},
					{"text": " new text","style": "sA"}
				]
			}]
		}
	}`

	objs := fromJSONWithFake(t, scene)

	if n := countTypeKey(objs, 135); n != 3 {
		t.Errorf("want 3 TextValueRuns, got %d", n)
	}
	if n := countTypeKey(objs, 137); n != 2 {
		t.Errorf("want 2 TextStyles, got %d", n)
	}
	// Verify the .riv round-trips (output starts with RIVE)
	b, err := fromjson.FromJSONWithFonts([]byte(scene), fakeFonts)
	if err != nil {
		t.Fatalf("FromJSONWithFonts: %v", err)
	}
	raw, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(raw[:4]) != "RIVE" {
		t.Errorf("output not a valid .riv file")
	}
}

// ── TestFromJSONMultiRun_StyleReuse ───────────────────────────────────────────

// One style referenced by multiple runs (style-reuse pattern).
func TestFromJSONMultiRun_StyleReuse(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [{"name": "FA", "file": "fontA.ttf"}],
			"children": [{
				"type": "text", "name": "t", "x": 200, "y": 100,
				"styles": [
					{"name": "s1", "font": "FA", "fontSize": 20, "fill": "#000000"}
				],
				"runs": [
					{"text": "First ", "style": "s1"},
					{"text": "second ", "style": "s1"},
					{"text": "third",   "style": "s1"}
				]
			}]
		}
	}`

	objs := fromJSONWithFake(t, scene)

	if n := countTypeKey(objs, 135); n != 3 {
		t.Errorf("want 3 TextValueRuns, got %d", n)
	}
	if n := countTypeKey(objs, 137); n != 1 {
		t.Errorf("want 1 TextStyle (reused), got %d", n)
	}
}

// ── TestFromJSONMultiRun_BackwardCompatSingleRun ──────────────────────────────

// The original single-run format (style + text) must still work alongside
// the new multi-run format in the same artboard.
func TestFromJSONMultiRun_BackwardCompatSingleRun(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [{"name": "FA", "file": "fontA.ttf"}],
			"children": [
				{
					"type": "text", "name": "single", "x": 100, "y": 100,
					"style": {"font": "FA", "fontSize": 24, "fill": "#000000"},
					"text": "Single run"
				},
				{
					"type": "text", "name": "multi", "x": 100, "y": 150,
					"styles": [{"name": "s", "font": "FA", "fontSize": 16}],
					"runs":   [{"text": "Run A", "style": "s"}, {"text": " Run B", "style": "s"}]
				}
			]
		}
	}`

	objs := fromJSONWithFake(t, scene)

	// 2 Text objects, 2 TextStyles total (1 per text), 3 TVRs (1+2).
	if n := countTypeKey(objs, 134); n != 2 {
		t.Errorf("want 2 Text objects, got %d", n)
	}
	if n := countTypeKey(objs, 135); n != 3 {
		t.Errorf("want 3 TVRs (1 single + 2 multi), got %d", n)
	}
	if n := countTypeKey(objs, 137); n != 2 {
		t.Errorf("want 2 TextStyles, got %d", n)
	}
}

// ── TestFromJSONMultiRun_ErrorMissingStyle ────────────────────────────────────

func TestFromJSONMultiRun_ErrorMissingStyle(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [{"name": "FA", "file": "fontA.ttf"}],
			"children": [{
				"type": "text", "name": "t", "x": 0, "y": 0,
				"styles": [{"name": "sA", "font": "FA", "fontSize": 12}],
				"runs":   [{"text": "hello", "style": "NONEXISTENT"}]
			}]
		}
	}`
	_, err := fromjson.FromJSONWithFonts([]byte(scene), fakeFonts)
	if err == nil {
		t.Error("expected error for undefined style name, got nil")
	}
}

// ── TestFromJSONMultiRun_ErrorEmptyRuns ───────────────────────────────────────

func TestFromJSONMultiRun_ErrorRunsWithoutStyles(t *testing.T) {
	scene := `{
		"version": 1,
		"artboard": {
			"name": "A", "width": 400, "height": 200,
			"fonts": [{"name": "FA", "file": "fontA.ttf"}],
			"children": [{
				"type": "text", "name": "t", "x": 0, "y": 0,
				"runs": [{"text": "hello", "style": "s"}]
			}]
		}
	}`
	_, err := fromjson.FromJSONWithFonts([]byte(scene), fakeFonts)
	if err == nil {
		t.Error("expected error when runs set but styles absent, got nil")
	}
}
