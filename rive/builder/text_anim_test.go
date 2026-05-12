package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// buildTextAnimScene creates an artboard with one text object (one style, one run)
// and returns the built objects + the style ref for animation targeting.
func buildTextAnimScene(t *testing.T, animate func(anim *builder.AnimationBuilder, style *builder.TextStyleRef)) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 500, 400)
	font := ab.EmbedFont("TestFont", testFont)

	txt := ab.Text("title").Position(50, 100)
	style := txt.Style(font, 16)
	style.Fill(0xFF000000)
	txt.Run("Hello World", style)

	anim := ab.Animation("grow", builder.WithDuration(120), builder.WithFPS(60), builder.WithLoop(builder.Loop))
	animate(anim, style)

	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objs
}

// artboardRelativeIdx returns the artboard-relative index of the first object with
// the given typeKey. Artboard-relative = global index − artboard global index.
func artboardRelativeIdx(t *testing.T, objects []rive.Object, typeKey uint32) uint64 {
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
		t.Fatal("no Artboard found")
	}
	if targetIdx < 0 {
		t.Fatalf("no object with typeKey=%d found", typeKey)
	}
	return uint64(targetIdx - abIdx)
}

// keyedObjectId reads the objectId (property key 51) from a KeyedObject.
func keyedObjectId(t *testing.T, obj rive.Object) uint64 {
	t.Helper()
	for _, p := range obj.Properties() {
		if p.Key == 51 {
			if v, ok := p.Value.(uint64); ok {
				return v
			}
		}
	}
	t.Fatal("KeyedObject missing ObjectId (key=51)")
	return 0
}

// keyedPropertyKey reads the animatedPropertyKey (property key 53) from a KeyedProperty.
func keyedPropertyKey(t *testing.T, obj rive.Object) uint64 {
	t.Helper()
	for _, p := range obj.Properties() {
		if p.Key == 53 {
			if v, ok := p.Value.(uint64); ok {
				return v
			}
		}
	}
	t.Fatal("KeyedProperty missing propertyKey (key=52)")
	return 0
}

// TestTextAnim_FontSizeKeyframes verifies that animating fontSize on a TextStyle
// produces a KeyedObject whose ObjectId points to the TextStyle (typeKey=137),
// not the Text or SolidColor.
func TestTextAnim_FontSizeKeyframes(t *testing.T) {
	objs := buildTextAnimScene(t, func(anim *builder.AnimationBuilder, style *builder.TextStyleRef) {
		anim.KeyframeFloat(style, builder.PropFontSize, 0, 16.0)
		anim.KeyframeFloat(style, builder.PropFontSize, 120, 32.0)
	})

	// Determine expected artboard-relative index of the TextStylePaint (typeKey=137)
	wantObjId := artboardRelativeIdx(t, objs, 137) // TextStylePaint

	// Verify exactly 1 KeyedObject exists
	kos := collectType(objs, 25)
	if len(kos) != 1 {
		t.Fatalf("want 1 KeyedObject, got %d", len(kos))
	}

	// KeyedObject.ObjectId must point to the TextStyle
	gotObjId := keyedObjectId(t, kos[0])
	if gotObjId != wantObjId {
		t.Errorf("KeyedObject.ObjectId = %d, want %d (TextStyle artboard-relative index)",
			gotObjId, wantObjId)
	}

	// Verify 1 KeyedProperty targeting fontSize (key=274)
	kps := collectType(objs, 26)
	if len(kps) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(kps))
	}
	if got := keyedPropertyKey(t, kps[0]); got != 274 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 274 (fontSize)", got)
	}

	// Verify 2 KeyFrameDouble (typeKey=30)
	if n := countType(objs, 30); n != 2 {
		t.Errorf("want 2 KeyFrameDouble, got %d", n)
	}
}

// TestTextAnim_ColorKeyframes verifies that animating fill color on a TextStyle
// produces a KeyedObject whose ObjectId points to the SolidColor child (typeKey=18),
// not the TextStyle itself — because color targets animColorIdx().
func TestTextAnim_ColorKeyframes(t *testing.T) {
	objs := buildTextAnimScene(t, func(anim *builder.AnimationBuilder, style *builder.TextStyleRef) {
		anim.KeyframeColor(style, builder.PropColorValue, 0, 0xFF000000)
		anim.KeyframeColor(style, builder.PropColorValue, 120, 0xFFFF0000)
	})

	// Determine expected artboard-relative index of the SolidColor (typeKey=18)
	wantObjId := artboardRelativeIdx(t, objs, 18) // first SolidColor = TextStyle's fill color

	// Verify exactly 1 KeyedObject
	kos := collectType(objs, 25)
	if len(kos) != 1 {
		t.Fatalf("want 1 KeyedObject, got %d", len(kos))
	}

	// KeyedObject.ObjectId must point to the SolidColor (via animColorIdx)
	gotObjId := keyedObjectId(t, kos[0])
	if gotObjId != wantObjId {
		t.Errorf("KeyedObject.ObjectId = %d, want %d (SolidColor artboard-relative index)",
			gotObjId, wantObjId)
	}

	// Verify 1 KeyedProperty targeting colorValue (key=37)
	kps := collectType(objs, 26)
	if len(kps) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(kps))
	}
	if got := keyedPropertyKey(t, kps[0]); got != 37 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 37 (colorValue)", got)
	}

	// Verify 2 KeyFrameColor (typeKey=37 — same as propKey, different context)
	if n := countType(objs, 37); n != 2 {
		t.Errorf("want 2 KeyFrameColor (typeKey=37), got %d", n)
	}
}

// TestTextAnim_MultiplePropsCombined verifies fontSize + color can be animated
// simultaneously on the same TextStyle without conflict.
func TestTextAnim_MultiplePropsCombined(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 400)
	font := ab.EmbedFont("F", testFont)

	txt := ab.Text("headline").Position(50, 100)
	style := txt.Style(font, 16)
	style.Fill(0xFF000000)
	txt.Run("Grow and fade", style)

	anim := ab.Animation("anim", builder.WithDuration(180), builder.WithFPS(60), builder.WithLoop(builder.Loop))
	// Animate fontSize on TextStyle
	anim.KeyframeFloat(style, builder.PropFontSize, 0, 16.0)
	anim.KeyframeFloat(style, builder.PropFontSize, 180, 48.0)
	// Animate color on SolidColor child
	anim.KeyframeColor(style, builder.PropColorValue, 0, 0xFF000000)
	anim.KeyframeColor(style, builder.PropColorValue, 180, 0xFFFF0000)

	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Expect 2 KeyedObjects (one for TextStyle, one for SolidColor — different indices)
	kos := collectType(objs, 25)
	if len(kos) != 2 {
		t.Errorf("want 2 KeyedObjects (TextStyle + SolidColor), got %d", len(kos))
	}

	// 2 KeyedProperties (fontSize + colorValue)
	kps := collectType(objs, 26)
	if len(kps) != 2 {
		t.Errorf("want 2 KeyedProperties, got %d", len(kps))
	}

	// 2 KeyFrameDouble (fontSize)
	if n := countType(objs, 30); n != 2 {
		t.Errorf("want 2 KeyFrameDouble (fontSize), got %d", n)
	}

	// 2 KeyFrameColor (color)
	if n := countType(objs, 37); n != 2 {
		t.Errorf("want 2 KeyFrameColor (colorValue), got %d", n)
	}
}

// TestTextAnim_LetterSpacing verifies letterSpacing (key=390) can be animated.
func TestTextAnim_LetterSpacing(t *testing.T) {
	objs := buildTextAnimScene(t, func(anim *builder.AnimationBuilder, style *builder.TextStyleRef) {
		anim.KeyframeFloat(style, builder.PropLetterSpacing, 0, 0.0)
		anim.KeyframeFloat(style, builder.PropLetterSpacing, 60, 10.0)
	})

	wantObjId := artboardRelativeIdx(t, objs, 137)
	kos := collectType(objs, 25)
	if len(kos) != 1 {
		t.Fatalf("want 1 KeyedObject, got %d", len(kos))
	}
	if gotObjId := keyedObjectId(t, kos[0]); gotObjId != wantObjId {
		t.Errorf("KeyedObject.ObjectId = %d, want %d (TextStyle)", gotObjId, wantObjId)
	}
	kps := collectType(objs, 26)
	if len(kps) != 1 {
		t.Fatalf("want 1 KeyedProperty, got %d", len(kps))
	}
	if got := keyedPropertyKey(t, kps[0]); got != 390 {
		t.Errorf("KeyedProperty.propertyKey = %d, want 390 (letterSpacing)", got)
	}
}
