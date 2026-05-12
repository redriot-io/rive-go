package fromjson_test

import (
	"strings"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// buildTextAnimFromJSON parses a JSON scene using fake fonts and returns parsed objects.
func buildTextAnimFromJSON(t *testing.T, scene string) []rive.Object {
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

// keyedObjectIds returns all objectId values (property key 51) from KeyedObjects (typeKey=25).
func keyedObjectIds(objects []rive.Object) []uint64 {
	var ids []uint64
	for _, o := range objects {
		if o.TypeKey() != 25 {
			continue
		}
		for _, p := range o.Properties() {
			if p.Key == 51 {
				if v, ok := p.Value.(uint64); ok {
					ids = append(ids, v)
				}
			}
		}
	}
	return ids
}

// keyedPropertyKeys returns all property key values (key 53) from KeyedProperties (typeKey=26).
func keyedPropertyKeys(objects []rive.Object) []uint64 {
	var keys []uint64
	for _, o := range objects {
		if o.TypeKey() != 26 {
			continue
		}
		for _, p := range o.Properties() {
			if p.Key == 53 {
				if v, ok := p.Value.(uint64); ok {
					keys = append(keys, v)
				}
			}
		}
	}
	return keys
}

// artboardRelativeIdxByTypeKey returns the artboard-relative index of the first
// object with the given typeKey in the parsed object list.
func artboardRelativeIdxByTypeKey(t *testing.T, objects []rive.Object, typeKey uint32) uint64 {
	t.Helper()
	abIdx := -1
	targetIdx := -1
	for i, o := range objects {
		if o.TypeKey() == 1 && abIdx < 0 {
			abIdx = i
		}
		if o.TypeKey() == typeKey && targetIdx < 0 {
			targetIdx = i
		}
	}
	if abIdx < 0 {
		t.Fatal("no Artboard (typeKey=1) in objects")
	}
	if targetIdx < 0 {
		t.Fatalf("no object with typeKey=%d in objects", typeKey)
	}
	return uint64(targetIdx - abIdx)
}

const textAnimBase = `{
  "version": 1,
  "artboard": {"name": "T", "width": 400, "height": 300,
    "fonts": [{"name": "fontA", "file": "fontA.ttf"}],
    "children": [{"type": "text", "name": "mytext", "x": 10, "y": 20,
      "style": {"font": "fontA", "fontSize": 16, "fill": "#000000"},
      "text": "Hello World"}],
    "animations": [%s]
  }
}`

// TestFromJSON_TextAnim_FontSize verifies that "mytext.style.fontSize" targets the
// TextStylePaint (typeKey=137) with property key 274.
func TestFromJSON_TextAnim_FontSize(t *testing.T) {
	animJSON := `{"name": "grow", "duration": 2.0,
    "tracks": [
      {"target": "mytext.style.fontSize",
       "keyframes": [{"time": 0, "value": 16}, {"time": 2, "value": 32}]}
    ]}`
	scene := strings.Replace(textAnimBase, "%s", animJSON, 1)
	objs := buildTextAnimFromJSON(t, scene)

	// Expected: KeyedObject targets TextStylePaint (typeKey=137)
	wantObjId := artboardRelativeIdxByTypeKey(t, objs, 137)

	ids := keyedObjectIds(objs)
	if len(ids) != 1 {
		t.Fatalf("want 1 KeyedObject, got %d", len(ids))
	}
	if ids[0] != wantObjId {
		t.Errorf("KeyedObject.objectId = %d, want %d (TextStylePaint artboard-relative index)", ids[0], wantObjId)
	}

	// KeyedProperty must target fontSize (key=274)
	propKeys := keyedPropertyKeys(objs)
	if len(propKeys) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(propKeys))
	}
	if propKeys[0] != 274 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 274 (fontSize)", propKeys[0])
	}

	// 2 KeyFrameDouble (typeKey=30) for the 2 keyframes
	if n := countTypeKey(objs, 30); n != 2 {
		t.Errorf("want 2 KeyFrameDouble, got %d", n)
	}
}

// TestFromJSON_TextAnim_FillColor verifies that "mytext.style.fill.color" targets the
// SolidColor (typeKey=18) with property key 37.
func TestFromJSON_TextAnim_FillColor(t *testing.T) {
	animJSON := `{"name": "fade", "duration": 1.0,
    "tracks": [
      {"target": "mytext.style.fill.color",
       "keyframes": [{"time": 0, "value": "#000000"}, {"time": 1, "value": "#FF0000"}]}
    ]}`
	scene := strings.Replace(textAnimBase, "%s", animJSON, 1)
	objs := buildTextAnimFromJSON(t, scene)

	// Expected: KeyedObject targets SolidColor (typeKey=18)
	wantObjId := artboardRelativeIdxByTypeKey(t, objs, 18)

	ids := keyedObjectIds(objs)
	if len(ids) != 1 {
		t.Fatalf("want 1 KeyedObject, got %d", len(ids))
	}
	if ids[0] != wantObjId {
		t.Errorf("KeyedObject.objectId = %d, want %d (SolidColor artboard-relative index)", ids[0], wantObjId)
	}

	// KeyedProperty must target colorValue (key=37)
	propKeys := keyedPropertyKeys(objs)
	if len(propKeys) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(propKeys))
	}
	if propKeys[0] != 37 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 37 (colorValue)", propKeys[0])
	}

	// 2 KeyFrameColor (typeKey=37) for the 2 keyframes
	if n := countTypeKey(objs, 37); n != 2 {
		t.Errorf("want 2 KeyFrameColor, got %d", n)
	}
}

// TestFromJSON_TextAnim_LetterSpacing verifies "mytext.style.letterSpacing" → propKey=390.
func TestFromJSON_TextAnim_LetterSpacing(t *testing.T) {
	animJSON := `{"name": "space", "duration": 1.0,
    "tracks": [
      {"target": "mytext.style.letterSpacing",
       "keyframes": [{"time": 0, "value": 0}, {"time": 1, "value": 10}]}
    ]}`
	scene := strings.Replace(textAnimBase, "%s", animJSON, 1)
	objs := buildTextAnimFromJSON(t, scene)

	propKeys := keyedPropertyKeys(objs)
	if len(propKeys) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(propKeys))
	}
	if propKeys[0] != 390 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 390 (letterSpacing)", propKeys[0])
	}
}

// TestFromJSON_TextAnim_UnknownStyleProp verifies a clear error for unsupported sub-paths.
func TestFromJSON_TextAnim_UnknownStyleProp(t *testing.T) {
	animJSON := `{"name": "bad", "duration": 1.0,
    "tracks": [
      {"target": "mytext.style.nonexistent",
       "keyframes": [{"time": 0, "value": 0}]}
    ]}`
	scene := strings.Replace(textAnimBase, "%s", animJSON, 1)
	_, err := fromjson.FromJSONWithFonts([]byte(scene), fakeFonts)
	if err == nil {
		t.Fatal("expected error for unknown style property, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q should mention the unknown property name", err.Error())
	}
}

// TestFromJSON_TextAnim_Combined verifies fontSize + color animated together
// produce 2 KeyedObjects (TextStyle + SolidColor) and correct property keys.
func TestFromJSON_TextAnim_Combined(t *testing.T) {
	animJSON := `{"name": "combo", "duration": 3.0, "loop": "loop",
    "tracks": [
      {"target": "mytext.style.fontSize",
       "keyframes": [{"time": 0, "value": 16}, {"time": 3, "value": 48}]},
      {"target": "mytext.style.fill.color",
       "keyframes": [{"time": 0, "value": "#000000"}, {"time": 3, "value": "#FF0000"}]}
    ]}`
	scene := strings.Replace(textAnimBase, "%s", animJSON, 1)
	objs := buildTextAnimFromJSON(t, scene)

	// 2 KeyedObjects (TextStyle for fontSize, SolidColor for color)
	if n := countTypeKey(objs, 25); n != 2 {
		t.Errorf("want 2 KeyedObjects, got %d", n)
	}
	// 2 KeyedProperties
	if n := countTypeKey(objs, 26); n != 2 {
		t.Errorf("want 2 KeyedProperties, got %d", n)
	}
	// 2 KeyFrameDouble (fontSize)
	if n := countTypeKey(objs, 30); n != 2 {
		t.Errorf("want 2 KeyFrameDouble, got %d", n)
	}
	// 2 KeyFrameColor (color)
	if n := countTypeKey(objs, 37); n != 2 {
		t.Errorf("want 2 KeyFrameColor, got %d", n)
	}

	propKeys := keyedPropertyKeys(objs)
	found274, found37 := false, false
	for _, k := range propKeys {
		if k == 274 {
			found274 = true
		}
		if k == 37 {
			found37 = true
		}
	}
	if !found274 {
		t.Error("expected KeyedProperty with key=274 (fontSize)")
	}
	if !found37 {
		t.Error("expected KeyedProperty with key=37 (colorValue)")
	}
}
