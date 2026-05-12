package rive_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
	"github.com/redriot-io/rive-go/rive/fromjson"
)

// TestConformance_NewText validates multi-run text emission against the
// structural patterns observed in the official new_text.riv (T-490).
//
// Ground truth from new_text.riv (see testdata/official/new_text_structure.txt):
//   Text[15]: Style1 → Run1 → Run2(fwd-styleId) → Run3 → Style2
//   Text[22]: Run1(fwd-styleId) → Style1 → Style2 → Run2 → Run3
//
// Our builder uses "styles-first" ordering: all TextStyles, then all TextValueRuns.
// This is structurally different from the official ordering but functionally equivalent
// because styleId is a pure artboard-relative index — forward references are valid.
//
// This test asserts:
//  1. Builder multi-run: correct typeKey sequence, parentId hierarchy, styleId refs
//  2. FromJSON `runs` array: same structure via JSON-driven build
func TestConformance_NewText(t *testing.T) {
	// ── Step 1: Verify official new_text.riv parses and has expected multi-run objects ──
	t.Run("official_parse", func(t *testing.T) {
		data, err := os.ReadFile("testdata/official/new_text.riv")
		if err != nil {
			t.Skip("new_text.riv not available")
		}
		f, err := rive.ReadBytes(data)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		// Count TextValueRun and TextStyle objects.
		var tvrs, tss int
		for _, o := range f.Objects {
			switch o.TypeKey() {
			case 135:
				tvrs++
			case 137:
				tss++
			}
		}
		if tvrs < 3 {
			t.Errorf("expected ≥3 TextValueRuns in new_text.riv, got %d", tvrs)
		}
		if tss < 2 {
			t.Errorf("expected ≥2 TextStyles in new_text.riv, got %d", tss)
		}
		t.Logf("new_text.riv: %d objects, %d TextValueRuns, %d TextStyles",
			len(f.Objects), tvrs, tss)
	})

	// ── Step 2: Builder multi-run structural verification ──
	t.Run("builder_multi_run", func(t *testing.T) {
		b := builder.New()
		ab := b.Artboard("MultiRun", 400, 200)
		fontA := ab.EmbedFont("FontA", []byte("FAKE-TTF-A"))
		fontB := ab.EmbedFont("FontB", []byte("FAKE-TTF-B"))

		txt := ab.Text("mixed")
		sA := txt.Style(fontA, 24).Fill(0xFF000000)
		sB := txt.Style(fontB, 16).Fill(0xFF666666)
		txt.Run("here is ", sA)
		txt.Run("some", sB)
		txt.Run(" new text", sA)

		objects, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}

		// Expected typeKey sequence (styles-first ordering):
		// [0]BB [1]FA_A [2]FAC_A [3]FA_B [4]FAC_B [5]AB
		// [6]Text [7]TSP_A [8]SC_A [9]Fill_A [10]TSP_B [11]SC_B [12]Fill_B
		// [13]TVR1 [14]TVR2 [15]TVR3
		wantKeys := []uint32{
			23, 141, 106, 141, 106, 1,         // preamble
			134,                               // Text
			137, 18, 20,                       // StyleA + Fill chain
			137, 18, 20,                       // StyleB + Fill chain
			135, 135, 135,                     // 3 TextValueRuns
		}
		if len(objects) != len(wantKeys) {
			t.Fatalf("object count: got %d want %d\n  got:  %v\n  want: %v",
				len(objects), len(wantKeys), typeKeys(objects), wantKeys)
		}
		for i, want := range wantKeys {
			if objects[i].TypeKey() != want {
				t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
			}
		}

		// Artboard is at global[5]; artboard-relative base = 5.
		const abIdx = 5

		// Verify parentId hierarchy for text subtree.
		// parentId=0 is the default (artboard itself) and is NOT emitted; return abIdx.
		parentGlobal := func(i int) int {
			for _, p := range objects[i].Properties() {
				if p.Key == 5 {
					if v, ok := p.Value.(uint64); ok {
						return abIdx + int(v)
					}
				}
			}
			return abIdx // no parentId property → parentId=0 → Artboard
		}
		getUint := func(i int, key uint32) (uint64, bool) {
			for _, p := range objects[i].Properties() {
				if p.Key == key {
					if v, ok := p.Value.(uint64); ok {
						return v, true
					}
				}
			}
			return 0, false
		}

		// Text[6] → Artboard[5] (parentId=0)
		if pg := parentGlobal(6); pg != abIdx {
			t.Errorf("Text[6].parent=global[%d], want global[%d] (Artboard)", pg, abIdx)
		}
		// StyleA[7] → Text[6]
		if pg := parentGlobal(7); pg != 6 {
			t.Errorf("StyleA[7].parent=global[%d], want global[6] (Text)", pg)
		}
		// SolidColor_A[8] → Fill_A[9] (forward ref)
		if pg := parentGlobal(8); pg != 9 {
			t.Errorf("SC_A[8].parent=global[%d], want global[9] (Fill_A forward ref)", pg)
		}
		// Fill_A[9] → StyleA[7]
		if pg := parentGlobal(9); pg != 7 {
			t.Errorf("Fill_A[9].parent=global[%d], want global[7] (StyleA)", pg)
		}
		// StyleB[10] → Text[6]
		if pg := parentGlobal(10); pg != 6 {
			t.Errorf("StyleB[10].parent=global[%d], want global[6] (Text)", pg)
		}
		// SolidColor_B[11] → Fill_B[12] (forward ref)
		if pg := parentGlobal(11); pg != 12 {
			t.Errorf("SC_B[11].parent=global[%d], want global[12] (Fill_B forward ref)", pg)
		}
		// Fill_B[12] → StyleB[10]
		if pg := parentGlobal(12); pg != 10 {
			t.Errorf("Fill_B[12].parent=global[%d], want global[10] (StyleB)", pg)
		}

		// TVR1[13] → Text[6], styleId → StyleA[7] (artboard-rel = 7-5 = 2)
		if pg := parentGlobal(13); pg != 6 {
			t.Errorf("TVR1[13].parent=global[%d], want global[6] (Text)", pg)
		}
		if sid, ok := getUint(13, 272); !ok || int(abIdx)+int(sid) != 7 {
			t.Errorf("TVR1[13].styleId=%d resolves to global[%d], want global[7] (StyleA)", sid, int(abIdx)+int(sid))
		}

		// TVR2[14] → Text[6], styleId → StyleB[10] (artboard-rel = 10-5 = 5)
		if sid, ok := getUint(14, 272); !ok || int(abIdx)+int(sid) != 10 {
			t.Errorf("TVR2[14].styleId=%d resolves to global[%d], want global[10] (StyleB)", sid, int(abIdx)+int(sid))
		}

		// TVR3[15] → Text[6], styleId → StyleA[7] (same as TVR1)
		if sid, ok := getUint(15, 272); !ok || int(abIdx)+int(sid) != 7 {
			t.Errorf("TVR3[15].styleId=%d resolves to global[%d], want global[7] (StyleA)", sid, int(abIdx)+int(sid))
		}

		t.Logf("builder multi-run ok: %v", typeKeys(objects))
	})

	// ── Step 3: FromJSON `runs` array multi-run ──
	t.Run("fromjson_runs_array", func(t *testing.T) {
		scene := `{
			"version": 1,
			"artboard": {
				"name": "MultiRunJSON",
				"width": 400,
				"height": 200,
				"fonts": [
					{"name": "FontA", "file": "FAKE_A"},
					{"name": "FontB", "file": "FAKE_B"}
				],
				"children": [{
					"type": "text",
					"name": "mixed",
					"x": 200, "y": 100,
					"styles": [
						{"name": "styleA", "font": "FontA", "fontSize": 24, "fill": "#000000"},
						{"name": "styleB", "font": "FontB", "fontSize": 16, "fill": "#666666"}
					],
					"runs": [
						{"text": "here is ", "style": "styleA"},
						{"text": "some",     "style": "styleB"},
						{"text": " new text","style": "styleA"}
					]
				}]
			}
		}`
		// Inject fake font bytes so fromjson doesn't try to open files.
		fakeBytes := map[string][]byte{
			"FAKE_A": []byte("FAKE-TTF-A"),
			"FAKE_B": []byte("FAKE-TTF-B"),
		}
		b, err := fromjson.FromJSONWithFonts([]byte(scene), fakeBytes)
		if err != nil {
			t.Fatalf("FromJSONWithFonts: %v", err)
		}
		objects, err := b.Bytes()
		if err != nil {
			t.Fatalf("Bytes: %v", err)
		}
		f, err := rive.ReadBytes(objects)
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}

		var tvrs, tss int
		for _, o := range f.Objects {
			switch o.TypeKey() {
			case 135:
				tvrs++
			case 137:
				tss++
			}
		}
		if tvrs != 3 {
			t.Errorf("want 3 TextValueRuns, got %d", tvrs)
		}
		if tss != 2 {
			t.Errorf("want 2 TextStyles, got %d", tss)
		}
		t.Logf("fromjson runs: %d TVRs, %d TextStyles", tvrs, tss)
	})
}

// typeKeys returns a slice of TypeKey values for display in test output.
func typeKeys(objects []rive.Object) []uint32 {
	out := make([]uint32, len(objects))
	for i, o := range objects {
		out[i] = o.TypeKey()
	}
	return out
}

// TestConformance_OfficialToC inspects official Rive editor exports to determine
// how bytes-typed properties (key 212, 223) appear in the ToC.
// This is the ground-truth test for our writer's ToC behavior.
//
// Confirmed ground truth (run 2026-05-11):
//   hello_world.riv:  key 212 IN ToC, field-index=1 (PropertyTypeString proxy)
//   new_text.riv:     key 212 IN ToC, field-index=1
//   ellipsis.riv:     key 212 IN ToC, field-index=1
//   ball_test.riv:    key 212 NOT in ToC (no embedded font)
//   blend_test.riv:   key 212 NOT in ToC (no embedded font)
func TestConformance_OfficialToC(t *testing.T) {
	files := []struct {
		path string
		desc string
	}{
		{"testdata/official/hello_world.riv", "1 Text, embedded font (official)"},
		{"testdata/official/new_text.riv", "multi-style text, embedded font (official)"},
		{"testdata/official/ellipsis.riv", "text overflow, embedded font (official)"},
		{"testdata/official/ball_test.riv", "shapes + state machine, no font (official)"},
		{"testdata/official/blend_test.riv", "blend state machine, no font (official)"},
	}

	bytesKeys := []struct {
		key  uint32
		name string
	}{
		{212, "FileAssetContents.bytes"},
		{223, "Mesh.triangleIndexBytes"},
		{588, "DataBind.sourcePathIds"},
		{920, "DataBind.path"},
		{963, "ViewModelInstanceListItem.viewModelPathIds"},
	}

	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Logf("SKIP %s — not available", f.path)
			continue
		}

		parsed, err := rive.ReadBytes(data)
		if err != nil {
			t.Errorf("FAIL parse %s: %v", f.path, err)
			continue
		}

		t.Logf("=== %s (%s, %d bytes, major=%d) ===",
			f.path, f.desc, len(data), parsed.MajorVersion)

		anyFound := false
		for _, bk := range bytesKeys {
			pt, inToC := parsed.PropertyTypeOf(bk.key)
			if inToC {
				t.Logf("  key %3d (%s): IN ToC  field-index=%d", bk.key, bk.name, pt)
				anyFound = true
			} else {
				t.Logf("  key %3d (%s): NOT in ToC", bk.key, bk.name)
			}
		}
		if !anyFound {
			t.Logf("  (no bytes-type keys found in ToC — omit approach confirmed)")
		}
	}
}

// TestConformance_OurWriterMatchesOfficial ensures our writer's ToC output
// matches the official encoder's behavior for bytes properties (key 212).
//
// Ground truth (from TestConformance_OfficialToC output):
//   - official hello_world.riv: key 212 IN ToC, field-index=1 (PropertyTypeString proxy)
//   - official new_text.riv:    key 212 IN ToC, field-index=1
//   - official ellipsis.riv:    key 212 IN ToC, field-index=1
func TestConformance_OurWriterMatchesOfficial(t *testing.T) {
	officialData, err := os.ReadFile("testdata/official/hello_world.riv")
	if err != nil {
		t.Skip("testdata/official/hello_world.riv not available")
	}
	officialFile, err := rive.ReadBytes(officialData)
	if err != nil {
		t.Fatalf("parse official hello_world.riv: %v", err)
	}

	oursData, err := os.ReadFile("../docs/preview/fromjson_hello_world.riv")
	if err != nil {
		t.Skip("docs/preview/fromjson_hello_world.riv not available")
	}
	oursFile, err := rive.ReadBytes(oursData)
	if err != nil {
		t.Fatalf("parse our fromjson_hello_world.riv: %v", err)
	}

	officialType, officialHas212 := officialFile.PropertyTypeOf(212)
	oursType, oursHas212 := oursFile.PropertyTypeOf(212)

	t.Logf("official: key 212 in ToC=%v field-index=%d", officialHas212, officialType)
	t.Logf("ours:     key 212 in ToC=%v field-index=%d", oursHas212, oursType)

	if !oursHas212 {
		t.Error("our writer omitted key 212 from ToC — must include with field-index=1 matching official encoder")
	}
	if officialHas212 != oursHas212 {
		t.Errorf("ToC presence mismatch for key 212: official=%v, ours=%v", officialHas212, oursHas212)
	}
	if officialHas212 && oursHas212 && officialType != oursType {
		t.Errorf("ToC field-index mismatch for key 212: official=%d, ours=%d", officialType, oursType)
	}
}

// TestConformance_OfficialFilesReadable ensures all downloaded official files
// parse without error. A parse failure here means our reader is broken.
func TestConformance_OfficialFilesReadable(t *testing.T) {
	files := []string{
		"testdata/official/hello_world.riv",
		"testdata/official/new_text.riv",
		"testdata/official/ellipsis.riv",
		"testdata/official/ball_test.riv",
		"testdata/official/blend_test.riv",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("not available: %v", err)
			}

			f, err := rive.ReadBytes(data)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			if len(f.Objects) == 0 {
				t.Error("no objects parsed")
			}
			t.Logf("ok: %d objects, major=%d", len(f.Objects), f.MajorVersion)
		})
	}
}

// TestConformance_RoundTrip verifies that ReadBytes → WriteBytes → ReadBytes
// produces a structurally identical file: same object count, same TypeKeys in
// order, and same property key/type/value sets per object.
//
// Covers all 5 official Rive files to exercise font-free, single-font, and
// multi-font paths. ellipsis.riv and new_text.riv exercise large embedded fonts.
func TestConformance_RoundTrip(t *testing.T) {
	files := []struct {
		path string
		desc string
	}{
		{"testdata/official/ball_test.riv", "shapes + state machine, no font"},
		{"testdata/official/blend_test.riv", "blend state machine, no font"},
		{"testdata/official/hello_world.riv", "text + embedded font (bytes round-trip)"},
		{"testdata/official/ellipsis.riv", "text overflow + ellipsis, embedded font"},
		{"testdata/official/new_text.riv", "multi-style text, 5 fonts, animations"},
	}

	for _, f := range files {
		f := f
		t.Run(f.path, func(t *testing.T) {
			data, err := os.ReadFile(f.path)
			if err != nil {
				t.Skipf("not available: %v", err)
			}

			f1, err := rive.ReadBytes(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			data2, err := rive.WriteBytes(f1.Objects,
				rive.WithMajorVersion(f1.MajorVersion),
				rive.WithMinorVersion(f1.MinorVersion),
				rive.WithFileID(f1.FileID),
			)
			if err != nil {
				t.Fatalf("write: %v", err)
			}

			f2, err := rive.ReadBytes(data2)
			if err != nil {
				t.Fatalf("re-read: %v", err)
			}

			if len(f2.Objects) != len(f1.Objects) {
				t.Fatalf("object count: got %d, want %d", len(f2.Objects), len(f1.Objects))
			}

			for i := range f1.Objects {
				o1, o2 := f1.Objects[i], f2.Objects[i]

				if o2.TypeKey() != o1.TypeKey() {
					t.Errorf("object[%d]: TypeKey got %d, want %d", i, o2.TypeKey(), o1.TypeKey())
					continue
				}

				p1, p2 := o1.Properties(), o2.Properties()
				if len(p2) != len(p1) {
					t.Errorf("object[%d] (typeKey=%d): prop count got %d, want %d",
						i, o1.TypeKey(), len(p2), len(p1))
					continue
				}

				for j := range p1 {
					if p2[j].Key != p1[j].Key {
						t.Errorf("object[%d] prop[%d]: key got %d, want %d",
							i, j, p2[j].Key, p1[j].Key)
					}
					if p2[j].Type != p1[j].Type {
						t.Errorf("object[%d] prop[%d] (key=%d): type got %d, want %d",
							i, j, p1[j].Key, p2[j].Type, p1[j].Type)
					}
					if !reflect.DeepEqual(p2[j].Value, p1[j].Value) {
						t.Errorf("object[%d] prop[%d] (key=%d, type=%d): value mismatch",
							i, j, p1[j].Key, p1[j].Type)
					}
				}
			}

			t.Logf("ok: %d objects, %d bytes → %d bytes (%s)",
				len(f1.Objects), len(data), len(data2), f.desc)
		})
	}
}

// TestConformance_Ellipsis validates text overflow and sizing against
// the official ellipsis.riv (T-491 / T-495).
//
// Ground truth from ellipsis.riv (see testdata/official/ellipsis_structure.txt):
//   Text[4]: sizingValue=2 (fixed), overflowValue=3 (ellipsis), width≈120.53, height≈23.50
//   alignValue NOT in ToC (default=0, left)
//
// The builder cannot replicate TextStyleAxis (variable font) objects,
// so byte-exact output is not the goal. We verify:
//  1. official_parse: official file has expected property values on the Text object
//  2. builder_ellipsis: builder produces matching property values
//  3. fromjson_ellipsis: FromJSONWithFonts produces matching property values
func TestConformance_Ellipsis(t *testing.T) {
	// helper: extract Text (typeKey=134) properties from a parsed object list
	type textP struct {
		align, sizing, overflow uint64
		width, height           float64
		found                   bool
	}
	getTextP := func(objects []rive.Object) textP {
		for _, o := range objects {
			if o.TypeKey() != 134 {
				continue
			}
			var p textP
			p.found = true
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
		return textP{}
	}

	assertText := func(t *testing.T, p textP, wantAlign, wantSizing, wantOverflow uint64, label string) {
		t.Helper()
		if !p.found {
			t.Fatalf("%s: no Text object (typeKey=134) found", label)
		}
		if p.align != wantAlign {
			t.Errorf("%s: alignValue got %d, want %d", label, p.align, wantAlign)
		}
		if p.sizing != wantSizing {
			t.Errorf("%s: sizingValue got %d, want %d", label, p.sizing, wantSizing)
		}
		if p.overflow != wantOverflow {
			t.Errorf("%s: overflowValue got %d, want %d", label, p.overflow, wantOverflow)
		}
		if wantSizing == 2 {
			if p.width == 0 {
				t.Errorf("%s: width should be non-zero for fixed sizing", label)
			}
			if p.height == 0 {
				t.Errorf("%s: height should be non-zero for fixed sizing", label)
			}
		}
	}

	t.Run("official_parse", func(t *testing.T) {
		data, err := os.ReadFile("testdata/official/ellipsis.riv")
		if err != nil {
			t.Skipf("ellipsis.riv not found: %v", err)
		}
		f, err := rive.ReadBytes(data)
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		p := getTextP(f.Objects)
		// ellipsis.riv: sizingValue=2 (fixed), overflowValue=3 (ellipsis), align=0 (left default, not emitted)
		assertText(t, p, 0, 2, 3, "official")
	})

	t.Run("builder_ellipsis", func(t *testing.T) {
		fakeFont := []byte("FAKE-TTF-BYTES")
		b := builder.New()
		ab := b.Artboard("New Artboard", 500, 500)
		font := ab.EmbedFont("Inter", fakeFont)

		txt := ab.Text("text1").
			Position(129.47, 175.14).
			Sizing(builder.SizingFixed).
			Size(120.53, 23.50).
			Overflow(builder.OverflowEllipsis)
		style := txt.Style(font, 20)
		style.Fill(0xFFFFFFFF)
		txt.Run("one two three", style)

		rivBytes, err := b.Bytes()
		if err != nil {
			t.Fatalf("Bytes: %v", err)
		}
		f, err := rive.ReadBytes(rivBytes)
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		p := getTextP(f.Objects)
		// align=0 (left, default), sizing=2 (fixed), overflow=3 (ellipsis)
		assertText(t, p, 0, 2, 3, "builder")
	})

	t.Run("fromjson_ellipsis", func(t *testing.T) {
		scene := []byte(`{
  "version": 1,
  "artboard": {
    "name": "New Artboard", "width": 500, "height": 500,
    "fonts": [{"name": "Inter", "file": "inter.ttf"}],
    "children": [{
      "type": "text", "name": "text1", "x": 129.47, "y": 175.14,
      "overflow": "ellipsis",
      "sizing": "fixed",
      "width": 120.53, "height": 23.50,
      "style": {"font": "Inter", "fontSize": 20, "fill": "#FFFFFF"},
      "text": "one two three"
    }]
  }
}`)
		fonts := map[string][]byte{"inter.ttf": []byte("FAKE-TTF-BYTES")}
		bld, err := fromjson.FromJSONWithFonts(scene, fonts)
		if err != nil {
			t.Fatalf("FromJSONWithFonts: %v", err)
		}
		rivBytes, err := bld.Bytes()
		if err != nil {
			t.Fatalf("Bytes: %v", err)
		}
		f, err := rive.ReadBytes(rivBytes)
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		p := getTextP(f.Objects)
		assertText(t, p, 0, 2, 3, "fromjson")
	})
}

// TestConformance_Image validates the image asset emission pattern against
// the official image_asset_test.riv (T-503).
//
// Ground truth from image_structure.txt:
//   [0] Backboard(23), [1] ImageAsset(105), [2] FileAssetContents(106),
//   [3] Artboard(1), [4] Image(100)
//   ImageAsset: name="customLibraryImageAsset", assetId(cloud)=123
//   FileAssetContents: 245 embedded bytes
//   Image: assetId=0 (first ImageAsset, 0-indexed), parentId absent (=artboard)
func TestConformance_Image(t *testing.T) {
	getProp := func(obj rive.Object, key uint32) (interface{}, bool) {
		for _, p := range obj.Properties() {
			if p.Key == key {
				return p.Value, true
			}
		}
		return nil, false
	}

	wantKeys := []uint32{23, 105, 106, 1, 100}

	// ── Step 1: Parse official image_asset_test.riv ──
	t.Run("official_parse", func(t *testing.T) {
		data, err := os.ReadFile("testdata/official/image_asset_test.riv")
		if err != nil {
			t.Skip("image_asset_test.riv not available")
		}
		f, err := rive.ReadBytes(data)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if len(f.Objects) != 5 {
			t.Fatalf("object count: got %d, want 5", len(f.Objects))
		}
		for i, want := range wantKeys {
			if f.Objects[i].TypeKey() != want {
				t.Errorf("objects[%d] typeKey=%d, want %d", i, f.Objects[i].TypeKey(), want)
			}
		}

		// ImageAsset[1]: name and cloud assetId
		if v, ok := getProp(f.Objects[1], 203); !ok || v.(string) != "customLibraryImageAsset" {
			t.Errorf("ImageAsset.name: got %v, want customLibraryImageAsset", v)
		}
		if v, ok := getProp(f.Objects[1], 204); !ok || v.(uint64) != 123 {
			t.Errorf("ImageAsset.assetId: got %v, want 123", v)
		}

		// FileAssetContents[2]: bytes present and 245 bytes
		v, ok := getProp(f.Objects[2], 212)
		if !ok {
			t.Error("FileAssetContents.bytes (key 212) missing")
		} else if b, ok := v.([]byte); !ok || len(b) != 245 {
			t.Errorf("FileAssetContents.bytes: got %d bytes, want 245", len(v.([]byte)))
		}

		// Image[4]: assetId=0, parentId=0 (artboard).
		// Note: the official encoder explicitly emits parentId=0 even though it is the default.
		if v, ok := getProp(f.Objects[4], 206); !ok || v.(uint64) != 0 {
			t.Errorf("Image.assetId (key 206): got %v, want 0", v)
		}
		if v, has := getProp(f.Objects[4], 5); has {
			if v.(uint64) != 0 {
				t.Errorf("Image.parentId (key 5): got %v, want 0 (artboard)", v)
			}
		}

		t.Logf("official_parse ok: %d objects, typeKeys=%v", len(f.Objects), typeKeys(f.Objects))
	})

	// ── Step 2: Build equivalent via builder API, write → read back, compare ──
	t.Run("builder_roundtrip", func(t *testing.T) {
		fakePNG := []byte("FAKE-PNG-BYTES")
		b := builder.New()
		ab := b.Artboard("My Artboard", 500, 500)
		asset := ab.EmbedImage("customLibraryImageAsset", fakePNG)
		ab.Image(asset)

		objects, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}

		if len(objects) != len(wantKeys) {
			t.Fatalf("object count: got %d, want %d\n  got:  %v\n  want: %v",
				len(objects), len(wantKeys), typeKeys(objects), wantKeys)
		}
		for i, want := range wantKeys {
			if objects[i].TypeKey() != want {
				t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
			}
		}

		// Image node: assetId=0 (0-indexed first ImageAsset), parentId absent
		var assetId uint64
		assetIdFound := false
		for _, p := range objects[4].Properties() {
			if p.Key == 206 {
				assetId = p.Value.(uint64)
				assetIdFound = true
			}
			if p.Key == 5 {
				t.Errorf("Image.parentId should be absent (artboard default), got %v", p.Value)
			}
		}
		if !assetIdFound {
			t.Error("Image.assetId (key 206) missing from builder output")
		} else if assetId != 0 {
			t.Errorf("Image.assetId: got %d, want 0", assetId)
		}

		// Write → ReadBytes → verify roundtrip structural dimensions
		data, err := rive.WriteBytes(objects)
		if err != nil {
			t.Fatalf("WriteBytes: %v", err)
		}
		f2, err := rive.ReadBytes(data)
		if err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		if len(f2.Objects) != len(wantKeys) {
			t.Fatalf("roundtrip object count: got %d, want %d", len(f2.Objects), len(wantKeys))
		}
		for i, want := range wantKeys {
			if f2.Objects[i].TypeKey() != want {
				t.Errorf("roundtrip objects[%d] typeKey=%d, want %d", i, f2.Objects[i].TypeKey(), want)
			}
		}

		// Image[4] after roundtrip: assetId=0, parentId absent
		if v, ok := getProp(f2.Objects[4], 206); !ok || v.(uint64) != 0 {
			t.Errorf("roundtrip Image.assetId: got %v, want 0", v)
		}
		if _, has := getProp(f2.Objects[4], 5); has {
			t.Error("roundtrip Image.parentId should be absent")
		}

		t.Logf("builder_roundtrip ok: typeKeys=%v → %d bytes", typeKeys(objects), len(data))
	})
}
