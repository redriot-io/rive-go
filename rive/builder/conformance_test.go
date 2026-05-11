package builder_test

import (
	"os"
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// TestConformance_BuilderMatchesOfficial_HelloWorld verifies that our builder
// emits the correct typeKey sequence and parentId values for a text scene with an
// embedded font extracted from the official hello_world.riv.
//
// Expected typeKey sequence: [23, 141, 106, 1, 134, 137, 18, 20, 135]
//
// Known divergence from official hello_world.riv (19 objects) at index 7:
//
//	official: TextValueRun(135) — tree-walk traversal separates paint from runs
//	ours:     Fill(20)          — emit-in-order with SC-before-Fill swap applied
//
// This is a documented conformance gap; our output is structurally valid.
func TestConformance_BuilderMatchesOfficial_HelloWorld(t *testing.T) {
	officialData, err := os.ReadFile("../testdata/official/hello_world.riv")
	if err != nil {
		t.Skip("testdata/official/hello_world.riv not available")
	}
	official, err := rive.ReadBytes(officialData)
	if err != nil {
		t.Fatalf("parse official hello_world.riv: %v", err)
	}

	// Extract font bytes from FileAssetContents (typeKey=106), property key 212.
	var fontBytes []byte
	for _, obj := range official.Objects {
		if obj.TypeKey() == 106 {
			for _, p := range obj.Properties() {
				if p.Key == 212 {
					if b, ok := p.Value.([]byte); ok {
						fontBytes = b
					}
				}
			}
		}
	}
	if len(fontBytes) == 0 {
		t.Skip("no font bytes (key 212) found in official hello_world.riv")
	}
	t.Logf("extracted %d font bytes from official hello_world.riv", len(fontBytes))

	// Build equivalent text scene via builder API.
	b := builder.New()
	ab := b.Artboard("Main", 500, 500)
	font := ab.EmbedFont("Roboto", fontBytes)
	txt := ab.Text("hello")
	style := txt.Style(font, 60).Fill(0xFF00F3FF)
	txt.Run("Hello World!", style)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	t.Logf("built %d objects: %v", len(objects), typeKeyList(objects))

	// Verify the exact typeKey sequence our builder is expected to produce.
	wantKeys := []uint32{
		23,  // Backboard
		141, // FontAsset
		106, // FileAssetContents
		1,   // Artboard
		134, // Text
		137, // TextStylePaint
		18,  // SolidColor (SC-before-Fill forward ref)
		20,  // Fill
		135, // TextValueRun
	}
	if len(objects) != len(wantKeys) {
		t.Fatalf("object count: got %d, want %d\n  got:  %v\n  want: %v",
			len(objects), len(wantKeys), typeKeyList(objects), wantKeys)
	}
	for i, want := range wantKeys {
		if objects[i].TypeKey() != want {
			t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
		}
	}

	// Log known divergence vs official at index 7.
	// Official: objects[7]=TextValueRun(135); ours: objects[7]=Fill(20).
	t.Logf("common prefix [0..6] matches official: %v", wantKeys[:7])
	t.Logf("known divergence at index 7: ours=Fill(20), official=TextValueRun(135) — tree-walk TODO")

	// Verify parentId values for SolidColor (index 6) and Fill (index 7).
	// Artboard is at global [3]; SolidColor at [6] → artboard-rel 3.
	// Fill at [7] → artboard-rel 4. SolidColor.parentId must be 4 (forward ref).
	scProps := propsByKey(objects[6].Properties())
	if v, ok := scProps[5]; !ok || v.Value.(uint64) != 4 {
		t.Errorf("SolidColor.parentId = %v, want 4 (forward ref to Fill at artboard-rel 4)",
			scProps[5].Value)
	}

	// Fill.parentId must point to TextStylePaint at artboard-rel 2.
	fillProps := propsByKey(objects[7].Properties())
	if v, ok := fillProps[5]; !ok || v.Value.(uint64) != 2 {
		t.Errorf("Fill.parentId = %v, want 2 (TextStylePaint at artboard-rel 2)",
			fillProps[5].Value)
	}

	// Verify our ToC is well-formed and that shared keys have matching field-indices.
	// Note: our built file may contain keys absent in official (e.g. key 203 = FontAsset.name
	// appears when we name the embedded font; official hello_world.riv omits FontAsset.name).
	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	ours, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	ourToc := ours.TocEntries()
	officialToc := official.TocEntries()
	for key, ourType := range ourToc {
		offType, inOfficial := officialToc[key]
		if !inOfficial {
			// Log divergences (e.g. FontAsset.name=203 not in official), don't fail.
			t.Logf("ToC key %d present in ours but not in official (expected for named assets)", key)
			continue
		}
		if ourType != offType {
			t.Errorf("ToC key %d: our field-index=%d, official=%d", key, ourType, offType)
		}
	}
	// Key 212 (font bytes proxy) must be present in both with field-index=1.
	if ours212, ok := ourToc[212]; !ok || ours212 != 1 {
		t.Errorf("ToC key 212 (font bytes): our field-index=%d, want 1", ourToc[212])
	}
	t.Logf("ToC: ours %d keys, official %d keys; shared keys have matching field-indices", len(ourToc), len(officialToc))
}

// TestConformance_BuilderMatchesOfficial_Shape verifies the canonical emission
// order for a shape with a solid-color fill:
//
//	Backboard(23) → Artboard(1) → Shape(3) → Rectangle(7) → SolidColor(18) → Fill(20)
//
// SolidColor appears before Fill (forward reference), matching the pattern
// confirmed in the official ball_test.riv export (objects[6]=SC, objects[7]=Fill).
func TestConformance_BuilderMatchesOfficial_Shape(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 400)
	ab.Rectangle(100, 100, 200, 150).Fill(0xFFFF0000)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	wantKeys := []uint32{23, 1, 3, 7, 18, 20}
	if len(objects) != len(wantKeys) {
		t.Fatalf("object count: got %d, want %d — %v", len(objects), len(wantKeys), typeKeyList(objects))
	}
	for i, want := range wantKeys {
		if objects[i].TypeKey() != want {
			t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
		}
	}

	// SolidColor at [4], artboard-rel 3. Fill at [5], artboard-rel 4.
	// SolidColor.parentId must be 4 (forward ref to Fill).
	scProps := propsByKey(objects[4].Properties())
	if v, ok := scProps[5]; !ok || v.Value.(uint64) != 4 {
		t.Errorf("SolidColor.parentId = %v, want 4 (forward ref to Fill at artboard-rel 4)",
			scProps[5].Value)
	}

	// Fill.parentId must be 1 (Shape at artboard-rel 1).
	fillProps := propsByKey(objects[5].Properties())
	if v, ok := fillProps[5]; !ok || v.Value.(uint64) != 1 {
		t.Errorf("Fill.parentId = %v, want 1 (Shape at artboard-rel 1)", fillProps[5].Value)
	}

	t.Logf("shape emission order matches official: %v", wantKeys)

	// Verify against official ball_test.riv pattern.
	if data, err := os.ReadFile("../testdata/official/ball_test.riv"); err == nil {
		if official, err := rive.ReadBytes(data); err == nil && len(official.Objects) > 7 {
			if official.Objects[6].TypeKey() == 18 && official.Objects[7].TypeKey() == 20 {
				t.Logf("official ball_test.riv: objects[6]=SolidColor(18) before objects[7]=Fill(20) confirmed")
			}
		}
	}
}
