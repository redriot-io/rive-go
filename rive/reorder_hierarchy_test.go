package rive_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// TestReorder_HierarchyAware verifies that ReorderByContract scopes
// SolidColor-before-Fill swaps to same-parent paint groups.
//
// Three sub-tests:
//  1. builder_correct_order — builder already emits SolidColor before Fill;
//     reorder must leave both subtrees intact with correct parentIds.
//  2. cross_boundary_no_swap — when a Shape's Fill is adjacent to a
//     TextStylePaint's SolidColor (from a different paint group), the reorder
//     must NOT swap them.
//  3. valid_pair_swapped — when a Fill and its own child SolidColor are
//     adjacent in wrong order, reorder MUST swap them.
func TestReorder_HierarchyAware(t *testing.T) {

	t.Run("builder_correct_order", func(t *testing.T) {
		// Build a scene with both a rectangle (shape) and a text object.
		// After T-481, the builder already emits SolidColor before Fill.
		// ReorderByContract must leave structure intact (idempotent for correct input).
		b := builder.New()
		ab := b.Artboard("Main", 400, 200)
		ab.Rectangle(50, 50, 100, 100).Fill(0xFFFF0000)
		font := ab.EmbedFont("F", []byte("FAKE-TTF"))
		txt := ab.Text("t")
		style := txt.Style(font, 24).Fill(0xFF000000)
		txt.Run("Hello", style)

		objects, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}

		// Expected: Backboard FontAsset FAC Artboard Shape Rect SC_shape Fill_shape
		//           Text TextStylePaint SC_text Fill_text TextValueRun
		wantKeys := []uint32{23, 141, 106, 1, 3, 7, 18, 20, 134, 137, 18, 20, 135}
		if len(objects) != len(wantKeys) {
			t.Fatalf("object count: got %d, want %d — %v", len(objects), len(wantKeys), typeKeySeq(objects))
		}
		for i, want := range wantKeys {
			if objects[i].TypeKey() != want {
				t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
			}
		}

		// Artboard at global[3]. Shape subtree: SC_shape at [6] ar=3, Fill_shape at [7] ar=4.
		// SC_shape.parentId must be a forward ref to Fill_shape (ar=4).
		scShape, ok := objects[6].(*rive.SolidColor)
		if !ok {
			t.Fatalf("objects[6] is not *rive.SolidColor, got %T", objects[6])
		}
		if scShape.ParentId != 4 {
			t.Errorf("shape SC.parentId=%d, want 4 (Fill_shape ar-rel)", scShape.ParentId)
		}
		fillShape, ok := objects[7].(*rive.Fill)
		if !ok {
			t.Fatalf("objects[7] is not *rive.Fill, got %T", objects[7])
		}
		if fillShape.ParentId != 1 {
			t.Errorf("shape Fill.parentId=%d, want 1 (Shape ar-rel)", fillShape.ParentId)
		}

		// Text subtree: SC_text at [10] ar=7, Fill_text at [11] ar=8.
		// TextStylePaint at [9] ar=6.
		// SC_text.parentId must be forward ref to Fill_text (ar=8).
		scText, ok := objects[10].(*rive.SolidColor)
		if !ok {
			t.Fatalf("objects[10] is not *rive.SolidColor, got %T", objects[10])
		}
		if scText.ParentId != 8 {
			t.Errorf("text SC.parentId=%d, want 8 (Fill_text ar-rel)", scText.ParentId)
		}
		fillText, ok := objects[11].(*rive.Fill)
		if !ok {
			t.Fatalf("objects[11] is not *rive.Fill, got %T", objects[11])
		}
		if fillText.ParentId != 6 {
			t.Errorf("text Fill.parentId=%d, want 6 (TextStylePaint ar-rel)", fillText.ParentId)
		}

		// Verify no cross-contamination: shape's SC parentId doesn't point to text objects.
		// ar=4 (shape Fill) is NOT in the text subtree (text starts at ar=5).
		if scShape.ParentId >= 5 {
			t.Errorf("shape SC.parentId=%d points into text subtree (ar>=5)", scShape.ParentId)
		}
		// text SC parentId=8 → text Fill (ar=8), which is in text subtree ar=[5..9].
		if scText.ParentId < 5 {
			t.Errorf("text SC.parentId=%d points into shape subtree (ar<5)", scText.ParentId)
		}
	})

	t.Run("cross_boundary_no_swap", func(t *testing.T) {
		// Construct a stream where Fill_shape (typeKey=20) is immediately followed
		// by SolidColor_text (typeKey=18) from a DIFFERENT paint group.
		// ReorderByContract must NOT swap them because SolidColor_text's parent
		// is Fill_text, not Fill_shape.
		//
		// Stream layout (global indices):
		//   [0] Backboard
		//   [1] Artboard
		//   [2] Shape         ar=1  parentId=0→Artboard
		//   [3] Fill_shape    ar=2  parentId=1→Shape
		//   [4] SolidColor_t  ar=3  parentId=7→Fill_text (cross-boundary adjacency!)
		//   [5] Text          ar=4  parentId=0→Artboard
		//   [6] TextStylePaint ar=5 parentId=4→Text
		//   [7] Fill_text     ar=6  parentId=5→TextStylePaint
		//   [8] SolidColor_s  ar=7  parentId=2→Fill_shape (wrong position, for completeness)

		bb := &rive.Backboard{}
		ab := &rive.Artboard{}

		shp := &rive.Shape{}
		shp.ParentId = 0 // Artboard

		fillShape := &rive.Fill{}
		fillShape.ParentId = 1 // Shape ar=1

		// SolidColor from text subtree: parentId points to Fill_text (ar=6 → global[7])
		// ab_idx=1, so ar=6 → global = 1+6 = 7
		scText := &rive.SolidColor{}
		scText.ColorValue = 0xFF000000
		scText.ParentId = 6 // Fill_text at ar=6

		txt := &rive.Text{}
		txt.ParentId = 0 // Artboard

		tsp := &rive.TextStylePaint{}
		tsp.ParentId = 4 // Text at ar=4

		fillText := &rive.Fill{}
		fillText.ParentId = 5 // TextStylePaint at ar=5

		scShape := &rive.SolidColor{}
		scShape.ColorValue = 0xFFFF0000
		scShape.ParentId = 2 // Fill_shape at ar=2

		objects := []rive.Object{bb, ab, shp, fillShape, scText, txt, tsp, fillText, scShape}

		// The dangerous pair: objects[3]=Fill_shape, objects[4]=SolidColor_text
		// (typeKeys 20 then 18, but different parents — must NOT be swapped).
		result := rive.ReorderByContract(objects)

		if len(result) != len(objects) {
			t.Fatalf("object count changed: %d → %d", len(objects), len(result))
		}

		// objects[3] must still be Fill_shape (typeKey=20), not SolidColor_text.
		if result[3].TypeKey() != 20 {
			t.Errorf("objects[3] typeKey=%d, want 20 (Fill_shape must not be displaced by cross-boundary swap)",
				result[3].TypeKey())
		}
		// objects[4] must still be SolidColor_text (typeKey=18).
		if result[4].TypeKey() != 18 {
			t.Errorf("objects[4] typeKey=%d, want 18 (SolidColor_text must not be displaced)",
				result[4].TypeKey())
		}
		// Specifically verify it's still scText and fillShape (not swapped).
		if result[3] != fillShape {
			t.Errorf("objects[3] object pointer changed: cross-boundary swap occurred")
		}
		if result[4] != scText {
			t.Errorf("objects[4] object pointer changed: cross-boundary swap occurred")
		}
	})

	t.Run("valid_pair_swapped", func(t *testing.T) {
		// Construct a stream where Fill(20) immediately precedes its own child
		// SolidColor(18). ReorderByContract MUST swap them to SolidColor→Fill.
		//
		// Stream layout (global indices):
		//   [0] Backboard
		//   [1] Artboard
		//   [2] Shape        ar=1  parentId=0
		//   [3] Fill_shape   ar=2  parentId=1→Shape
		//   [4] SolidColor_s ar=3  parentId=2→Fill_shape  (wrong order: Fill before SC)

		bb := &rive.Backboard{}
		ab := &rive.Artboard{}

		shp := &rive.Shape{}
		shp.ParentId = 0

		fillShape := &rive.Fill{}
		fillShape.ParentId = 1 // Shape

		scShape := &rive.SolidColor{}
		scShape.ColorValue = 0xFFFF0000
		scShape.ParentId = 2 // Fill_shape at ar=2

		objects := []rive.Object{bb, ab, shp, fillShape, scShape}

		result := rive.ReorderByContract(objects)

		if len(result) != len(objects) {
			t.Fatalf("object count changed: %d → %d", len(objects), len(result))
		}

		// After swap: objects[3]=SolidColor, objects[4]=Fill
		if result[3].TypeKey() != 18 {
			t.Errorf("objects[3] typeKey=%d, want 18 (SolidColor after swap)", result[3].TypeKey())
		}
		if result[4].TypeKey() != 20 {
			t.Errorf("objects[4] typeKey=%d, want 20 (Fill after swap)", result[4].TypeKey())
		}
		// Verify object identity (not just typeKey).
		if result[3] != scShape {
			t.Errorf("objects[3] is not scShape — swap didn't happen correctly")
		}
		if result[4] != fillShape {
			t.Errorf("objects[4] is not fillShape — swap didn't happen correctly")
		}

		// After fixParentIds: SolidColor.parentId must point to Fill (now at ar=3).
		// Artboard at global[1], SolidColor at global[3], Fill at global[4].
		// Fill ar-rel = 4-1 = 3.
		scAfter, ok := result[3].(*rive.SolidColor)
		if !ok {
			t.Fatalf("result[3] not *rive.SolidColor: %T", result[3])
		}
		if scAfter.ParentId != 3 {
			t.Errorf("SolidColor.parentId=%d after reorder+fixParentIds, want 3 (Fill ar-rel=3)", scAfter.ParentId)
		}
	})
}
