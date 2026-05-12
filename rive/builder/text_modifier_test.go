package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// buildModifierScene creates a Text with one ModifierGroup containing one Range
// and one VariationModifier ("wght" = 900). Returns the built object slice.
func buildModifierScene(t *testing.T) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("Inter", []byte("FAKE-TTF"))

	txt := ab.Text("hello")
	style := txt.Style(font, 20).Fill(0xFF000000)
	txt.Run("Hello World", style)

	mg := txt.ModifierGroup("weight_mod")
	mg.Range(0.0, 1.0, 1.0)
	mg.VariationModifier("wght", 900)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objects
}

// TestTextModifier_TypeKeyOrder verifies emission order:
// Backboard → FontAsset → FAC → Artboard → Text → TextStyle → SC → Fill → TVR
// → TextModifierGroup → TextModifierRange → TextVariationModifier
func TestTextModifier_TypeKeyOrder(t *testing.T) {
	objects := buildModifierScene(t)

	wantKeys := []uint32{
		23,  // Backboard
		141, // FontAsset
		106, // FileAssetContents
		1,   // Artboard
		134, // Text
		137, // TextStylePaint
		18,  // SolidColor
		20,  // Fill
		135, // TextValueRun
		159, // TextModifierGroup
		158, // TextModifierRange
		162, // TextVariationModifier
	}

	if len(objects) != len(wantKeys) {
		t.Fatalf("object count: got %d want %d\n  got:  %v\n  want: %v",
			len(objects), len(wantKeys), typeKeySlice(objects), wantKeys)
	}
	for i, want := range wantKeys {
		if objects[i].TypeKey() != want {
			t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
		}
	}
	t.Logf("modifier typeKeys ok: %v", typeKeySlice(objects))
}

// TestTextModifier_ParentIds verifies parentId hierarchy:
// TextModifierGroup.parentId → Text; Range.parentId → Group; VM.parentId → Group.
func TestTextModifier_ParentIds(t *testing.T) {
	objects := buildModifierScene(t)

	getProp := func(o rive.Object, key uint32) (interface{}, bool) {
		for _, p := range o.Properties() {
			if p.Key == key {
				return p.Value, true
			}
		}
		return nil, false
	}

	// Locate artboard offset.
	abOffset := -1
	for i, o := range objects {
		if o.TypeKey() == 1 {
			abOffset = i
			break
		}
	}
	if abOffset < 0 {
		t.Fatal("no Artboard found")
	}

	// Locate each relevant object by typeKey.
	find := func(tk uint32) (idx int) {
		for i, o := range objects {
			if o.TypeKey() == tk {
				return i
			}
		}
		t.Fatalf("typeKey=%d not found", tk)
		return -1
	}

	textIdx := find(134)
	groupIdx := find(159)
	rangeIdx := find(158)
	vmIdx := find(162)

	textAbIdx := uint64(textIdx - abOffset)
	groupAbIdx := uint64(groupIdx - abOffset)

	// TextModifierGroup.parentId → Text (artboard-relative)
	if v, ok := getProp(objects[groupIdx], 5); !ok || v.(uint64) != textAbIdx {
		t.Errorf("TextModifierGroup.parentId=%v, want %d (Text)", v, textAbIdx)
	}

	// TextModifierRange.parentId → Group
	if v, ok := getProp(objects[rangeIdx], 5); !ok || v.(uint64) != groupAbIdx {
		t.Errorf("TextModifierRange.parentId=%v, want %d (Group)", v, groupAbIdx)
	}

	// TextVariationModifier.parentId → Group
	if v, ok := getProp(objects[vmIdx], 5); !ok || v.(uint64) != groupAbIdx {
		t.Errorf("TextVariationModifier.parentId=%v, want %d (Group)", v, groupAbIdx)
	}
}

// TestTextModifier_VariationModifierProps verifies AxisTag and AxisValue.
func TestTextModifier_VariationModifierProps(t *testing.T) {
	objects := buildModifierScene(t)

	var vmObj rive.Object
	for _, o := range objects {
		if o.TypeKey() == 162 {
			vmObj = o
			break
		}
	}
	if vmObj == nil {
		t.Fatal("no TextVariationModifier (typeKey=162) found")
	}

	var axisTag uint64
	var axisValue float64
	for _, p := range vmObj.Properties() {
		switch p.Key {
		case 320:
			axisTag, _ = p.Value.(uint64)
		case 321:
			axisValue, _ = p.Value.(float64)
		}
	}

	// packTag("wght") = 0x77676874
	const wghtTag = uint64(0x77)<<24 | uint64(0x67)<<16 | uint64(0x68)<<8 | uint64(0x74)
	if axisTag != wghtTag {
		t.Errorf("AxisTag: got 0x%x, want 0x%x (wght)", axisTag, wghtTag)
	}
	if axisValue != 900 {
		t.Errorf("AxisValue: got %v, want 900", axisValue)
	}
}

// TestTextModifier_RoundTrip writes to bytes, reads back, verifies typeKeys preserved.
func TestTextModifier_RoundTrip(t *testing.T) {
	objects := buildModifierScene(t)

	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	f2, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	want := []uint32{23, 141, 106, 1, 134, 137, 18, 20, 135, 159, 158, 162}
	if len(f2.Objects) != len(want) {
		t.Fatalf("roundtrip object count: got %d want %d\n  got:  %v\n  want: %v",
			len(f2.Objects), len(want), typeKeySlice(f2.Objects), want)
	}
	for i, wk := range want {
		if f2.Objects[i].TypeKey() != wk {
			t.Errorf("roundtrip objects[%d] typeKey=%d, want %d", i, f2.Objects[i].TypeKey(), wk)
		}
	}
	t.Logf("roundtrip ok: %d bytes", len(data))
}

// TestTextModifier_FromJSON verifies that the "modifiers" array in a JSON scene
// produces TextModifierGroup(159) + TextModifierRange(158) + TextVariationModifier(162).
func TestTextModifier_FromJSON(t *testing.T) {
	scene := []byte(`{
		"version": 1,
		"artboard": {
			"name": "Main", "width": 400, "height": 200,
			"fonts": [{"name": "Inter", "file": "inter.ttf"}],
			"children": [{
				"type": "text",
				"name": "hello",
				"x": 200, "y": 100,
				"style": {
					"font": "Inter",
					"fontSize": 20,
					"fill": "#000000"
				},
				"text": "Hello World",
				"modifiers": [
					{
						"name": "weight_mod",
						"ranges": [{"modifyFrom": 0.0, "modifyTo": 1.0}],
						"variations": [{"tag": "wght", "value": 900}]
					}
				]
			}]
		}
	}`)

	fakeFont := map[string][]byte{"inter.ttf": []byte("FAKE-TTF")}
	bld, err := fromjson.FromJSONWithFonts(scene, fakeFont)
	if err != nil {
		t.Fatalf("FromJSONWithFonts: %v", err)
	}

	data, err := bld.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	counts := map[uint32]int{}
	for _, o := range f.Objects {
		counts[o.TypeKey()]++
	}

	if counts[159] != 1 {
		t.Errorf("want 1 TextModifierGroup (159), got %d", counts[159])
	}
	if counts[158] != 1 {
		t.Errorf("want 1 TextModifierRange (158), got %d", counts[158])
	}
	if counts[162] != 1 {
		t.Errorf("want 1 TextVariationModifier (162), got %d", counts[162])
	}
	t.Logf("fromjson modifiers ok: counts=%v", counts)
}
