package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// packTagExpect returns the expected packed uint64 for a 4-char tag.
func packTagExpect(tag string) uint64 {
	b := [4]byte{}
	for i := 0; i < 4 && i < len(tag); i++ {
		b[i] = tag[i]
	}
	return uint64(b[0])<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3])
}

// TestBuilder_FontVariation_Emission checks that FontVariation emits
// TextStyleAxis (tk=144) with correct tag, axisValue, and parentId.
func TestBuilder_FontVariation_Emission(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		txt := ab.Text("varText").Position(0, 0)
		style := txt.Style(font, 24).
			FontVariation("wght", 700).
			FontVariation("wdth", 100)
		txt.Run("Hello", style)
	})

	axes := findByTypeKey(objects, 144) // TextStyleAxis
	if len(axes) != 2 {
		t.Fatalf("want 2 TextStyleAxis, got %d", len(axes))
	}

	wantTags := []string{"wght", "wdth"}
	wantVals := []float64{700, 100}

	for i, o := range axes {
		tsa := o.(*rive.TextStyleAxis)
		wantTag := packTagExpect(wantTags[i])
		if tsa.Tag != wantTag {
			t.Errorf("axes[%d].Tag = %d (%.4q), want %d (%.4q)", i, tsa.Tag, rune(tsa.Tag), wantTag, rune(wantTag))
		}
		if tsa.AxisValue != wantVals[i] {
			t.Errorf("axes[%d].AxisValue = %v, want %v", i, tsa.AxisValue, wantVals[i])
		}
	}
}

// TestBuilder_FontVariation_ParentId checks that TextStyleAxis.parentId
// points to the containing TextStylePaint (tk=137).
func TestBuilder_FontVariation_ParentId(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		txt := ab.Text("v").Position(0, 0)
		style := txt.Style(font, 16).FontVariation("wght", 400)
		txt.Run("v", style)
	})

	// Find TextStylePaint index (artboard-relative).
	artboardOff := -1
	for i, o := range objects {
		if o.TypeKey() == 1 { // Artboard
			artboardOff = i
			break
		}
	}
	if artboardOff < 0 {
		t.Fatal("no artboard found")
	}

	styleGlobalIdx := -1
	for i, o := range objects {
		if o.TypeKey() == 137 { // TextStylePaint
			styleGlobalIdx = i
			break
		}
	}
	if styleGlobalIdx < 0 {
		t.Fatal("no TextStylePaint found")
	}
	styleArtIdx := uint64(styleGlobalIdx - artboardOff)

	axes := findByTypeKey(objects, 144)
	if len(axes) != 1 {
		t.Fatalf("want 1 TextStyleAxis, got %d", len(axes))
	}
	tsa := axes[0].(*rive.TextStyleAxis)
	if tsa.ParentId != styleArtIdx {
		t.Errorf("TextStyleAxis.ParentId = %d, want %d (TextStylePaint artboard-relative index)", tsa.ParentId, styleArtIdx)
	}
}

// axisProps extracts (tag, axisValue) from a GenericObject's property list.
func axisProps(o rive.Object) (tag uint64, axisValue float64) {
	for _, p := range o.Properties() {
		switch p.Key {
		case 288:
			axisValue, _ = p.Value.(float64)
		case 289:
			tag, _ = p.Value.(uint64)
		}
	}
	return
}

// TestBuilder_FontVariation_RoundTrip writes + reads back and checks properties survive.
// ReadBytes returns GenericObject for all types, so we inspect via TypeKey + Properties.
func TestBuilder_FontVariation_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 200, 100)
	font := ab.EmbedFont("Var", testFont)
	txt := ab.Text("t").Position(0, 0)
	style := txt.Style(font, 18).
		FontVariation("wght", 600).
		FontVariation("ital", 1)
	txt.Run("hi", style)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	file, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	var axes []rive.Object
	for _, o := range file.Objects {
		if o.TypeKey() == 144 { // TextStyleAxis
			axes = append(axes, o)
		}
	}
	if len(axes) != 2 {
		t.Fatalf("round-trip: want 2 TextStyleAxis, got %d", len(axes))
	}

	tag0, val0 := axisProps(axes[0])
	if tag0 != packTagExpect("wght") || val0 != 600 {
		t.Errorf("round-trip axes[0]: tag=%d val=%v (want wght/600)", tag0, val0)
	}
	tag1, val1 := axisProps(axes[1])
	if tag1 != packTagExpect("ital") || val1 != 1 {
		t.Errorf("round-trip axes[1]: tag=%d val=%v (want ital/1)", tag1, val1)
	}
}
