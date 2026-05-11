package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// typeKey constants for draw order objects
const (
	tkDrawTarget = 48
	tkDrawRules  = 49
)

func TestDrawAbove_EmitsCorrectObjects(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	top := ab.Rectangle(100, 100, 50, 50).Fill(0xFFFF0000).Name("top")
	bottom := ab.Rectangle(100, 100, 50, 50).Fill(0xFF0000FF).Name("bottom")
	// top should render above bottom
	top.DrawAbove(bottom)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Verify exactly one DrawTarget and one DrawRules
	dts := collectType(objects, tkDrawTarget)
	drs := collectType(objects, tkDrawRules)
	if len(dts) != 1 {
		t.Fatalf("expected 1 DrawTarget, got %d", len(dts))
	}
	if len(drs) != 1 {
		t.Fatalf("expected 1 DrawRules, got %d", len(drs))
	}

	// DrawTarget must have PlacementValue=0 (above) and DrawableId = bottom.shapeIdx
	dt := dts[0].(*rive.DrawTarget)
	if dt.PlacementValue != builder.PlacementAbove {
		t.Errorf("DrawTarget.PlacementValue = %d, want %d (above)", dt.PlacementValue, builder.PlacementAbove)
	}
	// top is declared first; in forward-emit order it's emitted first, shapeIdx=1.
	// bottom is declared second, shapeIdx=5 (each rectangle emits 4 objects: Shape + path + SolidColor + Fill).
	if dt.DrawableId != 5 {
		t.Errorf("DrawTarget.DrawableId = %d, want 5 (bottom shapeIdx)", dt.DrawableId)
	}

	// DrawRules.ParentId must equal top.shapeIdx
	// top is declared first; emitted second (reverse order), so top.shapeIdx = 1+N where N=objects for bottom shape
	dr := drs[0].(*rive.DrawRules)
	if dr.ParentId == 0 {
		t.Errorf("DrawRules.ParentId is 0 — should be source shape's artboard-relative index")
	}
	// DrawRules.DrawTargetId must be the artboard-relative index of the DrawTarget
	artboardOffset := uint64(1) // Backboard is index 0; artboard is index 1; artboardOffset=1
	dtIdx := uint64(0)
	for i, o := range objects {
		if o == dts[0] {
			dtIdx = uint64(i) - artboardOffset
			break
		}
	}
	if dr.DrawTargetId != dtIdx {
		t.Errorf("DrawRules.DrawTargetId = %d, want %d", dr.DrawTargetId, dtIdx)
	}

	// DrawRules must be emitted after all shape objects (post-pass ordering)
	lastShapeIdx := -1
	firstDRIdx := -1
	for i, o := range objects {
		if o.TypeKey() == 3 { // Shape typeKey
			lastShapeIdx = i
		}
		if o.TypeKey() == tkDrawRules && firstDRIdx < 0 {
			firstDRIdx = i
		}
	}
	if firstDRIdx <= lastShapeIdx {
		t.Errorf("DrawRules (idx=%d) must come after last Shape (idx=%d)", firstDRIdx, lastShapeIdx)
	}
}

func TestDrawBelow_PlacementValue(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	back := ab.Rectangle(100, 100, 60, 60).Fill(0xFF00FF00).Name("back")
	front := ab.Rectangle(100, 100, 60, 60).Fill(0xFFFF00FF).Name("front")
	back.DrawBelow(front)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	dts := collectType(objects, tkDrawTarget)
	if len(dts) != 1 {
		t.Fatalf("expected 1 DrawTarget, got %d", len(dts))
	}
	dt := dts[0].(*rive.DrawTarget)
	if dt.PlacementValue != builder.PlacementBelow {
		t.Errorf("PlacementValue = %d, want %d (below)", dt.PlacementValue, builder.PlacementBelow)
	}
}

func TestDrawRule_MultipleRules(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 400, 400)
	a := ab.Rectangle(50, 50, 40, 40).Fill(0xFFFF0000).Name("a")
	bShape := ab.Rectangle(80, 80, 40, 40).Fill(0xFF00FF00).Name("b")
	c := ab.Rectangle(110, 110, 40, 40).Fill(0xFF0000FF).Name("c")

	// a draws above b AND above c
	a.DrawAbove(bShape).DrawAbove(c)

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	dts := collectType(objects, tkDrawTarget)
	drs := collectType(objects, tkDrawRules)
	if len(dts) != 2 {
		t.Fatalf("expected 2 DrawTargets, got %d", len(dts))
	}
	if len(drs) != 2 {
		t.Fatalf("expected 2 DrawRules, got %d", len(drs))
	}
}

func TestDrawRule_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 300, 300)
	red := ab.Rectangle(150, 150, 80, 80).Fill(0xFFFF0000).Name("red")
	blue := ab.Rectangle(150, 150, 80, 80).Fill(0xFF0000FF).Name("blue")
	red.DrawAbove(blue)

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(data)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	if countType(f.Objects, tkDrawTarget) != 1 {
		t.Errorf("round-trip: expected 1 DrawTarget")
	}
	if countType(f.Objects, tkDrawRules) != 1 {
		t.Errorf("round-trip: expected 1 DrawRules")
	}
}

func TestDrawRule_NoRules_NoExtraObjects(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Main", 200, 200)
	ab.Rectangle(50, 50, 40, 40).Fill(0xFFFF0000).Name("solo")

	objects, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if countType(objects, tkDrawTarget) != 0 {
		t.Errorf("expected 0 DrawTargets for scene with no draw rules")
	}
	if countType(objects, tkDrawRules) != 0 {
		t.Errorf("expected 0 DrawRules for scene with no draw rules")
	}
}
