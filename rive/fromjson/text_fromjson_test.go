package fromjson_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// testdataFontDir is the directory holding a small real TTF for fromjson file tests.
const testdataFontDir = "testdata/fonts"

// testFontPath returns the path to the bundled test font.
func testFontPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(testdataFontDir, "test.ttf")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("test font not found at %s: %v", p, err)
	}
	return p
}

// writeSceneFile writes JSON to a temp file and returns its path.
func writeSceneFile(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	// Copy test font into temp dir so relative "file" refs work
	fontSrc, _ := os.ReadFile(testFontPath(t))
	_ = os.WriteFile(filepath.Join(tmp, "test.ttf"), fontSrc, 0644)

	scenePath := filepath.Join(tmp, "scene.json")
	if err := os.WriteFile(scenePath, []byte(content), 0644); err != nil {
		t.Fatalf("write scene: %v", err)
	}
	return scenePath
}

// findObjects returns all objects of a given typeKey from built scene.
func buildAndFind(t *testing.T, scenePath string, tk uint32) []rive.Object {
	t.Helper()
	b, err := fromjson.FromJSONFile(scenePath)
	if err != nil {
		t.Fatalf("FromJSONFile: %v", err)
	}
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var out []rive.Object
	for _, o := range objects {
		if o.TypeKey() == tk {
			out = append(out, o)
		}
	}
	return out
}

const helloWorldScene = `{
  "version": 1,
  "artboard": {
    "name": "Main",
    "width": 400,
    "height": 200,
    "fonts": [{"name": "Inter", "file": "test.ttf"}],
    "children": [{
      "type": "text",
      "name": "greeting",
      "x": 200, "y": 100,
      "align": "center",
      "style": {"font": "Inter", "fontSize": 32, "fill": "#000000"},
      "text": "Hello World"
    }]
  }
}`

// Test 1: basic hello world parses without error.
func TestFromJSON_Text_HelloWorld(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	texts := buildAndFind(t, path, 134)
	if len(texts) != 1 {
		t.Fatalf("want 1 Text, got %d", len(texts))
	}
	txt := texts[0].(*rive.Text)
	if txt.Name != "greeting" {
		t.Errorf("Name = %q, want greeting", txt.Name)
	}
	if txt.AlignValue != 2 { // center
		t.Errorf("AlignValue = %d, want 2 (center)", txt.AlignValue)
	}
}

// Test 2: FontAsset is emitted with the right name.
func TestFromJSON_Text_FontAssetName(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	fonts := buildAndFind(t, path, 141)
	if len(fonts) != 1 {
		t.Fatalf("want 1 FontAsset, got %d", len(fonts))
	}
	fa := fonts[0].(*rive.FontAsset)
	if fa.Name != "Inter" {
		t.Errorf("FontAsset.Name = %q, want Inter", fa.Name)
	}
}

// Test 3: FileAssetContents carries font bytes.
func TestFromJSON_Text_FontBytesPresent(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	contents := buildAndFind(t, path, 106)
	if len(contents) != 1 {
		t.Fatalf("want 1 FileAssetContents, got %d", len(contents))
	}
	fac := contents[0].(*rive.FileAssetContents)
	if len(fac.Bytes) == 0 {
		t.Error("FileAssetContents.Bytes is empty")
	}
}

// Test 4: TextStyle fontSize is parsed correctly.
func TestFromJSON_Text_FontSize(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	styles := buildAndFind(t, path, 573)
	if len(styles) != 1 {
		t.Fatalf("want 1 TextStyle, got %d", len(styles))
	}
	ts := styles[0].(*rive.TextStyle)
	if ts.FontSize != 32 {
		t.Errorf("FontSize = %g, want 32", ts.FontSize)
	}
}

// Test 5: TextValueRun carries the text content.
func TestFromJSON_Text_RunContent(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	runs := buildAndFind(t, path, 135)
	if len(runs) != 1 {
		t.Fatalf("want 1 TextValueRun, got %d", len(runs))
	}
	tvr := runs[0].(*rive.TextValueRun)
	if tvr.Text != "Hello World" {
		t.Errorf("Text = %q, want 'Hello World'", tvr.Text)
	}
}

// Test 6: SolidColor fill is black (0xFF000000 for "#000000").
func TestFromJSON_Text_FillColor(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	colors := buildAndFind(t, path, 18) // SolidColor
	if len(colors) != 1 {
		t.Fatalf("want 1 SolidColor, got %d", len(colors))
	}
	sc := colors[0].(*rive.SolidColor)
	if sc.ColorValue != 0xFF000000 {
		t.Errorf("ColorValue = %#x, want 0xFF000000", sc.ColorValue)
	}
}

// Test 7: Validation — missing text content.
func TestFromJSON_Text_ValidationMissingText(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":100,"height":100,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "style":{"font":"F","fontSize":12},"text":""}]
  }}`
	path := writeSceneFile(t, scene)
	_, err := fromjson.FromJSONFile(path)
	if err == nil {
		t.Error("expected error for empty text content, got nil")
	}
}

// Test 8: Validation — unknown font reference.
func TestFromJSON_Text_ValidationUnknownFont(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":100,"height":100,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "style":{"font":"MISSING","fontSize":12},"text":"hi"}]
  }}`
	path := writeSceneFile(t, scene)
	_, err := fromjson.FromJSONFile(path)
	if err == nil {
		t.Error("expected error for unknown font, got nil")
	}
}

// Test 9: Validation — fontSize zero.
func TestFromJSON_Text_ValidationFontSizeZero(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":100,"height":100,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "style":{"font":"F","fontSize":0},"text":"hi"}]
  }}`
	path := writeSceneFile(t, scene)
	_, err := fromjson.FromJSONFile(path)
	if err == nil {
		t.Error("expected error for fontSize=0, got nil")
	}
}

// Test 10: text and rectangle coexist in same artboard.
func TestFromJSON_Text_MixedChildren(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":400,"height":200,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[
      {"type":"rectangle","name":"bg","x":200,"y":100,"width":400,"height":200,
       "fill":"#FFFFFF"},
      {"type":"text","name":"label","x":200,"y":100,
       "style":{"font":"F","fontSize":24,"fill":"#000000"},"text":"Hello"}
    ]
  }}`
	path := writeSceneFile(t, scene)
	b, err := fromjson.FromJSONFile(path)
	if err != nil {
		t.Fatalf("FromJSONFile: %v", err)
	}
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	textCount := 0
	shapeCount := 0
	for _, o := range objects {
		switch o.TypeKey() {
		case 134:
			textCount++
		case 7: // Shape
			shapeCount++
		}
	}
	if textCount != 1 {
		t.Errorf("want 1 Text, got %d", textCount)
	}
	if shapeCount != 1 {
		t.Errorf("want 1 Shape, got %d", shapeCount)
	}
}

// Test 11: text alignment left (default).
func TestFromJSON_Text_AlignLeft(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":400,"height":200,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "style":{"font":"F","fontSize":12},"text":"left"}]
  }}`
	path := writeSceneFile(t, scene)
	texts := buildAndFind(t, path, 134)
	if len(texts) == 0 {
		t.Fatal("no Text found")
	}
	txt := texts[0].(*rive.Text)
	if txt.AlignValue != 0 { // left
		t.Errorf("AlignValue = %d, want 0 (left)", txt.AlignValue)
	}
}

// Test 12: text alignment right.
func TestFromJSON_Text_AlignRight(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {"name":"M","width":400,"height":200,
    "fonts":[{"name":"F","file":"test.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "align":"right",
      "style":{"font":"F","fontSize":12},"text":"right"}]
  }}`
	path := writeSceneFile(t, scene)
	texts := buildAndFind(t, path, 134)
	if len(texts) == 0 {
		t.Fatal("no Text found")
	}
	txt := texts[0].(*rive.Text)
	if txt.AlignValue != 1 { // right
		t.Errorf("AlignValue = %d, want 1 (right)", txt.AlignValue)
	}
}

// Test 13: FromJSON rejects font file references without a base dir.
func TestFromJSON_Text_FileRefRequiresBaseDir(t *testing.T) {
	scene := []byte(`{
  "version": 1,
  "artboard": {"name":"M","width":100,"height":100,
    "fonts":[{"name":"F","file":"some.ttf"}],
    "children":[{"type":"text","name":"t","x":0,"y":0,
      "style":{"font":"F","fontSize":12},"text":"hi"}]
  }}`)
	_, err := fromjson.FromJSON(scene)
	if err == nil {
		t.Error("expected error when font file referenced without base dir, got nil")
	}
}

// Test 14: roundtrip produces valid .riv bytes.
func TestFromJSON_Text_RoundTrip(t *testing.T) {
	path := writeSceneFile(t, helloWorldScene)
	b, err := fromjson.FromJSONFile(path)
	if err != nil {
		t.Fatalf("FromJSONFile: %v", err)
	}
	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "RIVE" {
		t.Errorf("output doesn't start with RIVE magic: %q", data[:minI(4, len(data))])
	}
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
