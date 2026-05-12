package fromjson_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
)

// textProps extracts key overflow/alignment/sizing properties from a TypeKey=134 object.
type textProps struct {
	align, sizing, overflow uint64
	width, height           float64
}

// getTextProps finds the first Text (typeKey=134) in objects and returns its properties.
// Returns nil if none found.
func getTextProps(objects []rive.Object) *textProps {
	for _, o := range objects {
		if o.TypeKey() != 134 {
			continue
		}
		p := &textProps{}
		for _, prop := range o.Properties() {
			switch prop.Key {
			case 281:
				if v, ok := prop.Value.(uint64); ok {
					p.align = v
				}
			case 284:
				if v, ok := prop.Value.(uint64); ok {
					p.sizing = v
				}
			case 285:
				if v, ok := prop.Value.(float64); ok {
					p.width = v
				}
			case 286:
				if v, ok := prop.Value.(float64); ok {
					p.height = v
				}
			case 287:
				if v, ok := prop.Value.(uint64); ok {
					p.overflow = v
				}
			}
		}
		return p
	}
	return nil
}

func TestFromJSON_Overflow_Default(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"T","width":400,"height":300,"fonts":[{"name":"fontA","file":"fontA.ttf"}],"children":[{"type":"text","name":"lbl","x":0,"y":0,"style":{"font":"fontA","fontSize":16},"text":"hi"}]}}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object found")
	}
	if p.overflow != 0 {
		t.Errorf("overflow: got %d, want 0 (visible default)", p.overflow)
	}
	if p.sizing != 0 {
		t.Errorf("sizing: got %d, want 0 (auto_width default)", p.sizing)
	}
}

func TestFromJSON_Overflow_Ellipsis_Fixed(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {
    "name": "T", "width": 500, "height": 500,
    "fonts": [{"name": "fontA", "file": "fontA.ttf"}],
    "children": [{
      "type": "text", "name": "lbl", "x": 129, "y": 175,
      "overflow": "ellipsis",
      "sizing": "fixed",
      "width": 120, "height": 24,
      "style": {"font": "fontA", "fontSize": 20, "fill": "#FFFFFF"},
      "text": "one two three"
    }]
  }
}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object found")
	}
	if p.overflow != 3 {
		t.Errorf("overflow: got %d, want 3 (ellipsis)", p.overflow)
	}
	if p.sizing != 2 {
		t.Errorf("sizing: got %d, want 2 (fixed)", p.sizing)
	}
	if p.width == 0 {
		t.Error("width should be non-zero")
	}
	if p.height == 0 {
		t.Error("height should be non-zero")
	}
}

func TestFromJSON_Overflow_Hidden(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"T","width":400,"height":300,"fonts":[{"name":"fontA","file":"fontA.ttf"}],"children":[{"type":"text","name":"lbl","x":0,"y":0,"overflow":"hidden","sizing":"fixed","width":200,"height":50,"style":{"font":"fontA","fontSize":16},"text":"hi"}]}}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object")
	}
	if p.overflow != 1 {
		t.Errorf("overflow: got %d, want 1 (hidden)", p.overflow)
	}
}

func TestFromJSON_Overflow_Clipped(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"T","width":400,"height":300,"fonts":[{"name":"fontA","file":"fontA.ttf"}],"children":[{"type":"text","name":"lbl","x":0,"y":0,"overflow":"clipped","sizing":"auto_height","style":{"font":"fontA","fontSize":16},"text":"hi"}]}}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object")
	}
	if p.overflow != 2 {
		t.Errorf("overflow: got %d, want 2 (clipped)", p.overflow)
	}
	if p.sizing != 1 {
		t.Errorf("sizing: got %d, want 1 (auto_height)", p.sizing)
	}
}

func TestFromJSON_Overflow_Fit(t *testing.T) {
	scene := `{"version":1,"artboard":{"name":"T","width":400,"height":300,"fonts":[{"name":"fontA","file":"fontA.ttf"}],"children":[{"type":"text","name":"lbl","x":0,"y":0,"overflow":"fit","sizing":"fixed","width":200,"height":50,"style":{"font":"fontA","fontSize":16},"text":"hi"}]}}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object")
	}
	if p.overflow != 4 {
		t.Errorf("overflow: got %d, want 4 (fit)", p.overflow)
	}
}

func TestFromJSON_Align_Center_Overflow_Ellipsis(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {
    "name": "T", "width": 400, "height": 300,
    "fonts": [{"name": "fontA", "file": "fontA.ttf"}],
    "children": [{
      "type": "text", "name": "lbl", "x": 200, "y": 150,
      "align": "center",
      "overflow": "ellipsis",
      "sizing": "fixed",
      "width": 150, "height": 30,
      "style": {"font": "fontA", "fontSize": 14, "fill": "#333333"},
      "text": "centered text"
    }]
  }
}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object")
	}
	if p.align != 2 {
		t.Errorf("align: got %d, want 2 (center)", p.align)
	}
	if p.overflow != 3 {
		t.Errorf("overflow: got %d, want 3 (ellipsis)", p.overflow)
	}
	if p.sizing != 2 {
		t.Errorf("sizing: got %d, want 2 (fixed)", p.sizing)
	}
}

func TestFromJSON_Overflow_MultiRun_Ellipsis(t *testing.T) {
	scene := `{
  "version": 1,
  "artboard": {
    "name": "T", "width": 400, "height": 300,
    "fonts": [{"name": "fontA", "file": "fontA.ttf"}],
    "children": [{
      "type": "text", "name": "mr", "x": 10, "y": 20,
      "overflow": "ellipsis",
      "sizing": "fixed",
      "width": 200, "height": 40,
      "styles": [
        {"name": "s1", "font": "fontA", "fontSize": 16, "fill": "#000000"},
        {"name": "s2", "font": "fontA", "fontSize": 12, "fill": "#666666"}
      ],
      "runs": [
        {"text": "hello ", "style": "s1"},
        {"text": "world", "style": "s2"}
      ]
    }]
  }
}`
	objs := fromJSONWithFake(t, scene)
	p := getTextProps(objs)
	if p == nil {
		t.Fatal("no Text object")
	}
	if p.overflow != 3 {
		t.Errorf("overflow: got %d, want 3 (ellipsis)", p.overflow)
	}
	if p.sizing != 2 {
		t.Errorf("sizing: got %d, want 2 (fixed)", p.sizing)
	}
	var tvrs int
	for _, o := range objs {
		if o.TypeKey() == 135 {
			tvrs++
		}
	}
	if tvrs != 2 {
		t.Errorf("TVR count: got %d, want 2", tvrs)
	}
}
