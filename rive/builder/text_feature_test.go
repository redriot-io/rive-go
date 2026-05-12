package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// featureProps extracts tag (key 356) and featureValue (key 357) from a TextStyleFeature object.
// FeatureValue defaults to 1 in the Rive protocol; it is only emitted when non-default (i.e. ≠1).
// Returns (tag, featureValue, featureValuePresent).
func featureProps(o rive.Object) (tag uint64, featureValue uint64, fvPresent bool) {
	featureValue = 1 // default per Rive protocol
	for _, p := range o.Properties() {
		switch p.Key {
		case 356:
			if v, ok := p.Value.(uint64); ok {
				tag = v
			}
		case 357:
			if v, ok := p.Value.(uint64); ok {
				featureValue = v
				fvPresent = true
			}
		}
	}
	return
}

// TestTextFeature_Emission verifies that Feature() emits TextStyleFeature (typeKey=164)
// objects as children of the TextStyle, after TextStyleAxis objects.
func TestTextFeature_Emission(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("Inter", []byte("FAKE-TTF"))

	txt := ab.Text("hello")
	style := txt.Style(font, 20).
		Fill(0xFF000000).
		Feature("liga", 1). // enable ligatures
		Feature("kern", 0)  // disable kerning

	txt.Run("Hello World", style)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Count TextStyleFeature objects (typeKey=164)
	var features []rive.Object
	for _, o := range objects {
		if o.TypeKey() == 164 {
			features = append(features, o)
		}
	}
	if len(features) != 2 {
		t.Fatalf("want 2 TextStyleFeature objects, got %d (typeKeys: %v)", len(features), typeKeySlice(objects))
	}

	// Feature 0: "liga" = 1 (default — FeatureValue NOT emitted per Rive protocol)
	tag0, val0, fvPresent0 := featureProps(features[0])
	// packTag("liga") = 0x6c696761
	const ligaTag = uint64(0x6c) << 24 | uint64(0x69) << 16 | uint64(0x67) << 8 | uint64(0x61)
	if tag0 != ligaTag {
		t.Errorf("feature[0].tag: got 0x%x, want 0x%x (liga)", tag0, ligaTag)
	}
	if val0 != 1 {
		t.Errorf("feature[0].featureValue: got %d, want 1 (default)", val0)
	}
	if fvPresent0 {
		t.Error("feature[0].featureValue should not be emitted when value==1 (default)")
	}

	// Feature 1: "kern" = 0 (explicit non-default — FeatureValue IS emitted)
	tag1, val1, fvPresent1 := featureProps(features[1])
	// packTag("kern") = 0x6b65726e
	const kernTag = uint64(0x6b) << 24 | uint64(0x65) << 16 | uint64(0x72) << 8 | uint64(0x6e)
	if tag1 != kernTag {
		t.Errorf("feature[1].tag: got 0x%x, want 0x%x (kern)", tag1, kernTag)
	}
	if val1 != 0 {
		t.Errorf("feature[1].featureValue: got %d, want 0", val1)
	}
	if !fvPresent1 {
		t.Error("feature[1].featureValue should be emitted when value==0 (non-default)")
	}

	t.Logf("TextStyleFeature emission ok: typeKeys=%v", typeKeySlice(objects))
}

// TestTextFeature_RoundTrip writes the scene to bytes, reads back, and checks TypeKeys.
func TestTextFeature_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("Inter", []byte("FAKE-TTF"))

	txt := ab.Text("hello")
	style := txt.Style(font, 20).Fill(0xFF000000).Feature("liga", 1)
	txt.Run("Hello", style)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	var count int
	for _, o := range f.Objects {
		if o.TypeKey() == 164 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("roundtrip: want 1 TextStyleFeature (typeKey=164), got %d", count)
	}
}

// TestTextFeature_ParentId verifies the TextStyleFeature's ParentId points to its TextStyle.
func TestTextFeature_ParentId(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("Inter", []byte("FAKE-TTF"))

	txt := ab.Text("hello")
	style := txt.Style(font, 20).Fill(0xFF000000).Feature("liga", 1)
	txt.Run("Hello", style)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Find TextStyle (typeKey=137) index and TextStyleFeature (typeKey=164).
	styleIdx := -1
	featureIdx := -1
	for i, o := range objects {
		if o.TypeKey() == 137 {
			styleIdx = i
		}
		if o.TypeKey() == 164 {
			featureIdx = i
		}
	}
	if styleIdx < 0 {
		t.Fatal("no TextStyle (typeKey=137) found")
	}
	if featureIdx < 0 {
		t.Fatal("no TextStyleFeature (typeKey=164) found")
	}

	// ParentId is artboard-relative: feature.ParentId should equal style.idx.
	// Artboard is at global index 2 (BB=0, FontAsset=1, FAC=2 or similar) — verify via Properties.
	// The artboard is always global index = (total preamble objects before artboard).
	// Simpler: just check that feature's parentId property matches style's artboard-relative idx.
	abOffset := -1
	for i, o := range objects {
		if o.TypeKey() == 1 { // Artboard
			abOffset = i
			break
		}
	}
	if abOffset < 0 {
		t.Fatal("no Artboard (typeKey=1) found")
	}

	styleAbIdx := uint64(styleIdx - abOffset)

	for _, p := range objects[featureIdx].Properties() {
		if p.Key == 5 { // parentId
			got, ok := p.Value.(uint64)
			if !ok {
				t.Fatalf("parentId not uint64: %T", p.Value)
			}
			if got != styleAbIdx {
				t.Errorf("TextStyleFeature.parentId=%d, want %d (style artboard-relative idx)", got, styleAbIdx)
			}
			return
		}
	}
	t.Error("TextStyleFeature has no parentId property")
}

// TestTextFeature_FromJSON verifies that the "features" array in a JSON scene
// produces TextStyleFeature (typeKey=164) objects in the built .riv.
func TestTextFeature_FromJSON(t *testing.T) {
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
					"fill": "#000000",
					"features": [
						{"tag": "liga", "value": 1},
						{"tag": "kern", "value": 0}
					]
				},
				"text": "Hello World"
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

	var count int
	for _, o := range f.Objects {
		if o.TypeKey() == 164 {
			count++
		}
	}
	if count != 2 {
		t.Errorf("fromjson: want 2 TextStyleFeature (typeKey=164), got %d", count)
	}
	t.Logf("fromjson features ok: %d TextStyleFeature objects", count)
}

// TestTextFeature_FromJSON_MultiRun verifies features work in the multi-run "styles" format.
func TestTextFeature_FromJSON_MultiRun(t *testing.T) {
	scene := []byte(`{
		"version": 1,
		"artboard": {
			"name": "Main", "width": 400, "height": 200,
			"fonts": [{"name": "Inter", "file": "inter.ttf"}],
			"children": [{
				"type": "text",
				"name": "hello",
				"x": 200, "y": 100,
				"styles": [
					{
						"name": "s1",
						"font": "Inter",
						"fontSize": 20,
						"fill": "#000000",
						"features": [{"tag": "liga", "value": 1}]
					}
				],
				"runs": [{"text": "Hello", "style": "s1"}]
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

	var count int
	for _, o := range f.Objects {
		if o.TypeKey() == 164 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("fromjson multi-run: want 1 TextStyleFeature (typeKey=164), got %d", count)
	}
	t.Logf("fromjson multi-run features ok")
}
