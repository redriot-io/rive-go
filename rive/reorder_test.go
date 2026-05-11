package rive_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

func typeKeySeq(objects []rive.Object) []uint32 {
	out := make([]uint32, len(objects))
	for i, o := range objects {
		out[i] = o.TypeKey()
	}
	return out
}

// TestReorder_HelloWorld builds a minimal text scene equivalent to the structure
// of hello_world.riv and verifies the reorder pass places SolidColor before Fill.
//
// Expected emission order (matching official Rive encoder):
//   Backboard(23) → FontAsset(141) → FAC(106) → Artboard(1) →
//   Text(134) → TextStylePaint(137) → SolidColor(18) → Fill(20) → TextValueRun(135)
func TestReorder_HelloWorld(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 500, 100)
	font := ab.EmbedFont("TestFont", []byte("FAKE-TTF"))
	txt := ab.Text("hello")
	style := txt.Style(font, 24).Fill(0xFF000000) // black fill
	txt.Run("Hello World", style)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	wantOrder := []uint32{
		23,  // Backboard
		141, // FontAsset
		106, // FileAssetContents
		1,   // Artboard
		134, // Text
		137, // TextStylePaint
		18,  // SolidColor (before Fill — forward reference)
		20,  // Fill
		135, // TextValueRun
	}

	if len(objects) != len(wantOrder) {
		t.Fatalf("want %d objects, got %d: %v", len(wantOrder), len(objects), typeKeySeq(objects))
	}
	for i, want := range wantOrder {
		if objects[i].TypeKey() != want {
			t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
		}
	}

	// Verify SolidColor.parentId is a forward reference to Fill.
	// Artboard at global index 3. Fill at global index 7 → artboard-rel = 7-3 = 4.
	sc, ok := objects[6].(*rive.SolidColor)
	if !ok {
		t.Fatalf("objects[6] is not *rive.SolidColor: %T", objects[6])
	}
	const artboardGlobalIdx = 3
	fillGlobalIdx := 7
	wantParentId := uint64(fillGlobalIdx - artboardGlobalIdx) // = 4
	if sc.ParentId != wantParentId {
		t.Errorf("SolidColor.ParentId = %d, want %d (Fill's artboard-relative index)", sc.ParentId, wantParentId)
	}

	// Verify Fill.parentId still points to TextStylePaint.
	fill, ok := objects[7].(*rive.Fill)
	if !ok {
		t.Fatalf("objects[7] is not *rive.Fill: %T", objects[7])
	}
	// TextStylePaint at global 5 → artboard-rel = 5-3 = 2.
	if fill.ParentId != 2 {
		t.Errorf("Fill.ParentId = %d, want 2 (TextStylePaint's artboard-relative index)", fill.ParentId)
	}

	// Verify the output is readable and produces the same structure.
	data, err := rive.WriteBytes(objects)
	if err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if len(f.Objects) != len(wantOrder) {
		t.Errorf("round-trip object count: got %d, want %d", len(f.Objects), len(wantOrder))
	}
	for i, want := range wantOrder {
		if f.Objects[i].TypeKey() != want {
			t.Errorf("round-trip objects[%d] typeKey=%d, want %d", i, f.Objects[i].TypeKey(), want)
		}
	}
}

// TestReorder_ShapeWithFill verifies SolidColor before Fill for a rectangle scene.
func TestReorder_ShapeWithFill(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Rectangle(100, 100, 200, 200).Fill(0xFFFF0000)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// 0:Backboard 1:Artboard 2:Shape 3:Rectangle 4:SolidColor 5:Fill
	wantOrder := []uint32{23, 1, 3, 7, 18, 20}
	if len(objects) != len(wantOrder) {
		t.Fatalf("want %d objects, got %d: %v", len(wantOrder), len(objects), typeKeySeq(objects))
	}
	for i, want := range wantOrder {
		if objects[i].TypeKey() != want {
			t.Errorf("objects[%d] typeKey=%d, want %d", i, objects[i].TypeKey(), want)
		}
	}

	// SolidColor.parentId must be a forward ref to Fill (artboard-rel 4).
	sc, ok := objects[4].(*rive.SolidColor)
	if !ok {
		t.Fatalf("objects[4] is not *rive.SolidColor")
	}
	if sc.ParentId != 4 {
		t.Errorf("SolidColor.ParentId = %d, want 4 (Fill artboard-rel)", sc.ParentId)
	}

	// Fill.parentId must point to Shape (artboard-rel 1).
	fill, ok := objects[5].(*rive.Fill)
	if !ok {
		t.Fatalf("objects[5] is not *rive.Fill")
	}
	if fill.ParentId != 1 {
		t.Errorf("Fill.ParentId = %d, want 1 (Shape artboard-rel)", fill.ParentId)
	}
}

// TestReorder_GradientUnchanged verifies Fill+LinearGradient order is not swapped
// (the swap only applies to Fill+SolidColor pairs).
func TestReorder_GradientUnchanged(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Ellipse(200, 200, 100, 80).FillGradient(0, 0, 100, 0,
		builder.GradientStop{Position: 0.0, Color: 0xFFFF0000},
		builder.GradientStop{Position: 1.0, Color: 0xFF0000FF},
	)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Fill must precede LinearGradient (gradient case is NOT swapped).
	fillIdx := -1
	gradIdx := -1
	for i, o := range objects {
		switch o.TypeKey() {
		case 20:
			fillIdx = i
		case 22:
			gradIdx = i
		}
	}
	if fillIdx < 0 || gradIdx < 0 {
		t.Fatal("Fill or LinearGradient not found")
	}
	if fillIdx > gradIdx {
		t.Errorf("Fill (idx=%d) must come before LinearGradient (idx=%d) for gradient fills",
			fillIdx, gradIdx)
	}
}

// TestFixParentIds_Idempotent verifies FixParentIds is safe to call on
// already-correct data (no change in parentIds).
func TestFixParentIds_Idempotent(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	ab.Rectangle(0, 0, 100, 100).Fill(0xFFABCDEF)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Capture parentIds before second FixParentIds call.
	before := make([]uint64, len(objects))
	for i, o := range objects {
		for _, p := range o.Properties() {
			if p.Key == 5 {
				before[i] = p.Value.(uint64)
			}
		}
	}

	rive.FixParentIds(objects)

	// Verify parentIds unchanged.
	for i, o := range objects {
		after := uint64(0)
		for _, p := range o.Properties() {
			if p.Key == 5 {
				after = p.Value.(uint64)
			}
		}
		if before[i] != after {
			t.Errorf("objects[%d] (typeKey=%d) parentId changed: %d → %d",
				i, o.TypeKey(), before[i], after)
		}
	}
}
