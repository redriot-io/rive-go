package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// testFont is a minimal font byte slice — used purely for binary-structure tests
// (the builder stores bytes verbatim; no TTF parsing is performed here).
var testFont = []byte("FAKE-TTF-BYTES-FOR-TESTING")

func buildTextScene(t *testing.T, configure func(ab *builder.ArtboardBuilder, font *builder.FontRef)) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("TestFont", testFont)
	configure(ab, font)
	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	return objects
}

// findByTypeKey returns all objects with the given typeKey.
func findByTypeKey(objects []rive.Object, tk uint32) []rive.Object {
	var out []rive.Object
	for _, o := range objects {
		if o.TypeKey() == tk {
			out = append(out, o)
		}
	}
	return out
}

// Test 1: EmbedFont emits FontAsset + FileAssetContents.
func TestText_EmbedFont(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		// no text — just font
	})

	fonts := findByTypeKey(objects, 141) // FontAsset
	if len(fonts) != 1 {
		t.Fatalf("want 1 FontAsset, got %d", len(fonts))
	}
	fa := fonts[0].(*rive.FontAsset)
	if fa.Name != "TestFont" {
		t.Errorf("FontAsset.Name = %q, want TestFont", fa.Name)
	}

	contents := findByTypeKey(objects, 106) // FileAssetContents
	if len(contents) != 1 {
		t.Fatalf("want 1 FileAssetContents, got %d", len(contents))
	}
	fac := contents[0].(*rive.FileAssetContents)
	if string(fac.Bytes) != string(testFont) {
		t.Errorf("FileAssetContents.Bytes mismatch")
	}
}

// Test 2: FontAsset emitted before Text (emission order).
func TestText_FontBeforeText(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("hello").Position(100, 100)
		style := text.Style(font, 24).Fill(0xFF000000)
		text.Run("Hello", style)
	})

	fontIdx := -1
	textIdx := -1
	for i, o := range objects {
		switch o.TypeKey() {
		case 141: // FontAsset
			fontIdx = i
		case 134: // Text
			textIdx = i
		}
	}
	if fontIdx < 0 || textIdx < 0 {
		t.Fatalf("FontAsset or Text not found")
	}
	if fontIdx >= textIdx {
		t.Errorf("FontAsset (idx=%d) must come before Text (idx=%d)", fontIdx, textIdx)
	}
}

// Test 3: Text object has correct properties.
func TestText_Properties(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("greeting").Position(200, 100).Align(builder.AlignCenter)
		style := text.Style(font, 32)
		text.Run("Hello World", style)
	})

	texts := findByTypeKey(objects, 134) // Text
	if len(texts) != 1 {
		t.Fatalf("want 1 Text, got %d", len(texts))
	}
	txt := texts[0].(*rive.Text)
	if txt.Name != "greeting" {
		t.Errorf("Text.Name = %q, want greeting", txt.Name)
	}
	if txt.X != 200 || txt.Y != 100 {
		t.Errorf("Text.X/Y = %g/%g, want 200/100", txt.X, txt.Y)
	}
	if txt.AlignValue != 2 { // center
		t.Errorf("Text.AlignValue = %d, want 2 (center)", txt.AlignValue)
	}
}

// Test 4: TextStyle has correct fontSize and fontAssetId.
func TestText_StyleProperties(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		style := text.Style(font, 48).Fill(0xFFFF0000)
		text.Run("Hi", style)
	})

	styles := findByTypeKey(objects, 573) // TextStyle
	if len(styles) != 1 {
		t.Fatalf("want 1 TextStyle, got %d", len(styles))
	}
	ts := styles[0].(*rive.TextStyle)
	if ts.FontSize != 48 {
		t.Errorf("TextStyle.FontSize = %g, want 48", ts.FontSize)
	}
	if ts.LineHeight != -1 {
		t.Errorf("TextStyle.LineHeight = %g, want -1 (auto)", ts.LineHeight)
	}
}

// Test 5: TextStyle.fontAssetId references FontAsset by artboard-relative index.
func TestText_FontAssetIdWiring(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		style := text.Style(font, 24)
		text.Run("X", style)
	})

	// Compute artboard offset: index of Artboard object
	artboardOffset := uint64(0)
	for i, o := range objects {
		if o.TypeKey() == 1 { // Artboard
			artboardOffset = uint64(i)
			break
		}
	}

	// Find FontAsset's artboard-relative index
	fontArtIdx := uint64(0)
	for i, o := range objects {
		if o.TypeKey() == 141 {
			fontArtIdx = uint64(i) - artboardOffset
			break
		}
	}

	// Verify TextStyle references it
	for _, o := range objects {
		if o.TypeKey() == 573 {
			ts := o.(*rive.TextStyle)
			if ts.FontAssetId != fontArtIdx {
				t.Errorf("TextStyle.FontAssetId = %d, want %d", ts.FontAssetId, fontArtIdx)
			}
			return
		}
	}
	t.Fatal("TextStyle not found")
}

// Test 6: TextValueRun has correct text and styleId.
func TestText_ValueRun(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		style := text.Style(font, 24)
		text.Run("Hello World", style)
	})

	runs := findByTypeKey(objects, 135) // TextValueRun
	if len(runs) != 1 {
		t.Fatalf("want 1 TextValueRun, got %d", len(runs))
	}
	tvr := runs[0].(*rive.TextValueRun)
	if tvr.Text != "Hello World" {
		t.Errorf("TextValueRun.Text = %q, want 'Hello World'", tvr.Text)
	}
}

// Test 7: Fill under TextStyle emits Fill + SolidColor.
func TestText_FillUnderStyle(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		style := text.Style(font, 24).Fill(0xFF0000FF) // blue
		text.Run("X", style)
	})

	fills := findByTypeKey(objects, 20) // Fill
	if len(fills) != 1 {
		t.Fatalf("want 1 Fill, got %d", len(fills))
	}
	colors := findByTypeKey(objects, 18) // SolidColor
	if len(colors) != 1 {
		t.Fatalf("want 1 SolidColor, got %d", len(colors))
	}
	sc := colors[0].(*rive.SolidColor)
	if sc.ColorValue != 0xFF0000FF {
		t.Errorf("SolidColor.ColorValue = %#x, want 0xFF0000FF", sc.ColorValue)
	}
}

// Test 8: TextStyle with LetterSpacing and LineHeight.
func TestText_StyleSpacing(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		style := text.Style(font, 16).
			LetterSpacing(2.5).
			LineHeight(1.4)
		text.Run("X", style)
	})
	for _, o := range objects {
		if o.TypeKey() == 573 {
			ts := o.(*rive.TextStyle)
			if ts.LetterSpacing != 2.5 {
				t.Errorf("LetterSpacing = %g, want 2.5", ts.LetterSpacing)
			}
			if ts.LineHeight != 1.4 {
				t.Errorf("LineHeight = %g, want 1.4", ts.LineHeight)
			}
			return
		}
	}
	t.Fatal("TextStyle not found")
}

// Test 9: Multiple runs reference same or different styles.
func TestText_MultipleRuns(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(0, 0)
		s1 := text.Style(font, 24).Fill(0xFF000000)
		s2 := text.Style(font, 36).Fill(0xFFFF0000)
		text.Run("Hello ", s1)
		text.Run("World", s2)
	})

	runs := findByTypeKey(objects, 135) // TextValueRun
	if len(runs) != 2 {
		t.Fatalf("want 2 TextValueRun, got %d", len(runs))
	}
	styles := findByTypeKey(objects, 573) // TextStyle
	if len(styles) != 2 {
		t.Fatalf("want 2 TextStyle, got %d", len(styles))
	}
	// styleIds should differ
	r1 := runs[0].(*rive.TextValueRun)
	r2 := runs[1].(*rive.TextValueRun)
	if r1.StyleId == r2.StyleId {
		t.Errorf("runs share same styleId %d — expected different", r1.StyleId)
	}
}

// Test 10: Text sizing mode Fixed with explicit width/height.
func TestText_SizingFixed(t *testing.T) {
	objects := buildTextScene(t, func(ab *builder.ArtboardBuilder, font *builder.FontRef) {
		text := ab.Text("t").Position(50, 50).
			Sizing(builder.SizingFixed).
			Size(300, 50)
		style := text.Style(font, 18)
		text.Run("Fixed box", style)
	})
	for _, o := range objects {
		if o.TypeKey() == 134 {
			txt := o.(*rive.Text)
			if txt.SizingValue != 2 {
				t.Errorf("SizingValue = %d, want 2 (fixed)", txt.SizingValue)
			}
			if txt.Width != 300 || txt.Height != 50 {
				t.Errorf("Width/Height = %g/%g, want 300/50", txt.Width, txt.Height)
			}
			return
		}
	}
	t.Fatal("Text not found")
}

// Test 11: Multiple fonts in one artboard.
func TestText_MultipleFonts(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font1 := ab.EmbedFont("FontA", []byte("ttf-a"))
	font2 := ab.EmbedFont("FontB", []byte("ttf-b"))

	t1 := ab.Text("t1").Position(10, 10)
	t1.Run("A", t1.Style(font1, 20))

	t2 := ab.Text("t2").Position(10, 50)
	t2.Run("B", t2.Style(font2, 20))

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	fontAssets := findByTypeKey(objects, 141)
	if len(fontAssets) != 2 {
		t.Fatalf("want 2 FontAsset, got %d", len(fontAssets))
	}
	fileContents := findByTypeKey(objects, 106)
	if len(fileContents) != 2 {
		t.Fatalf("want 2 FileAssetContents, got %d", len(fileContents))
	}
}

// Test 12: Text round-trip through WriteBytes.
func TestText_RoundTripBytes(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 200)
	font := ab.EmbedFont("TestFont", testFont)
	text := ab.Text("greeting").Position(200, 100).Align(builder.AlignCenter)
	style := text.Style(font, 32).Fill(0xFF000000)
	text.Run("Hello World", style)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(data) == 0 {
		t.Error("Bytes returned empty data")
	}
	// Basic .riv magic check: starts with "RIVE"
	if len(data) < 4 || string(data[:4]) != "RIVE" {
		t.Errorf("output does not start with RIVE magic: %q", data[:min(4, len(data))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
