package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

func buildOverflowScene(t *testing.T, overflow builder.TextOverflow, align builder.TextAlign, sizing builder.TextSizing, w, h float64) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Test", 400, 300)
	fontRef := ab.EmbedFont("abel.ttf", testFont)

	txt := ab.Text("label").
		Position(10, 20).
		Align(align).
		Overflow(overflow).
		Sizing(sizing)
	if sizing == builder.SizingFixed {
		txt.Size(w, h)
	}
	style := txt.Style(fontRef, 16)
	style.Fill(0xFF000000)
	txt.Run("hello world", style)

	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objs
}

func findText(objects []rive.Object) *rive.Text {
	for _, o := range objects {
		if txt, ok := o.(*rive.Text); ok {
			return txt
		}
	}
	return nil
}

func TestOverflow_Default(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowVisible, builder.AlignLeft, builder.SizingAutoWidth, 0, 0)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.OverflowValue != 0 {
		t.Errorf("OverflowValue: got %d, want 0 (visible)", txt.OverflowValue)
	}
	if txt.AlignValue != 0 {
		t.Errorf("AlignValue: got %d, want 0 (left)", txt.AlignValue)
	}
}

func TestOverflow_Hidden(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowHidden, builder.AlignLeft, builder.SizingFixed, 200, 50)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.OverflowValue != 1 {
		t.Errorf("OverflowValue: got %d, want 1 (hidden)", txt.OverflowValue)
	}
}

func TestOverflow_Clipped(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowClipped, builder.AlignLeft, builder.SizingFixed, 200, 50)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.OverflowValue != 2 {
		t.Errorf("OverflowValue: got %d, want 2 (clipped)", txt.OverflowValue)
	}
}

func TestOverflow_Ellipsis(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowEllipsis, builder.AlignLeft, builder.SizingFixed, 120, 24)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.OverflowValue != 3 {
		t.Errorf("OverflowValue: got %d, want 3 (ellipsis)", txt.OverflowValue)
	}
	if txt.SizingValue != 2 {
		t.Errorf("SizingValue: got %d, want 2 (fixed)", txt.SizingValue)
	}
	if txt.Width == 0 {
		t.Error("Width should be non-zero for fixed sizing")
	}
	if txt.Height == 0 {
		t.Error("Height should be non-zero for fixed sizing")
	}
}

func TestOverflow_Fit(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowFit, builder.AlignLeft, builder.SizingFixed, 200, 50)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.OverflowValue != 4 {
		t.Errorf("OverflowValue: got %d, want 4 (fit)", txt.OverflowValue)
	}
}

func TestOverflow_AlignCenter_Ellipsis(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowEllipsis, builder.AlignCenter, builder.SizingFixed, 150, 30)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.AlignValue != 2 {
		t.Errorf("AlignValue: got %d, want 2 (center)", txt.AlignValue)
	}
	if txt.OverflowValue != 3 {
		t.Errorf("OverflowValue: got %d, want 3 (ellipsis)", txt.OverflowValue)
	}
}

func TestOverflow_AlignRight_AutoHeight(t *testing.T) {
	objs := buildOverflowScene(t, builder.OverflowClipped, builder.AlignRight, builder.SizingAutoHeight, 0, 0)
	txt := findText(objs)
	if txt == nil {
		t.Fatal("no Text object found")
	}
	if txt.AlignValue != 1 {
		t.Errorf("AlignValue: got %d, want 1 (right)", txt.AlignValue)
	}
	if txt.SizingValue != 1 {
		t.Errorf("SizingValue: got %d, want 1 (auto_height)", txt.SizingValue)
	}
	if txt.OverflowValue != 2 {
		t.Errorf("OverflowValue: got %d, want 2 (clipped)", txt.OverflowValue)
	}
}

func TestOverflow_EllipsisMatchesOfficialKeys(t *testing.T) {
	// Verify the exact property keys used by ellipsis.riv: sizingValue=2, overflowValue=3
	b := builder.New()
	ab := b.Artboard("New Artboard", 500, 500)
	font := ab.EmbedFont("Inter", testFont)

	txt := ab.Text("text1").
		Position(129.47, 175.14).
		Sizing(builder.SizingFixed).
		Size(120.53, 23.50).
		Overflow(builder.OverflowEllipsis)
	style := txt.Style(font, 20)
	style.Fill(0xFFFFFFFF)
	txt.Run("one two three", style)

	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	obj := findText(objs)
	if obj == nil {
		t.Fatal("no Text object found")
	}
	if obj.SizingValue != 2 {
		t.Errorf("SizingValue: got %d, want 2 (fixed)", obj.SizingValue)
	}
	if obj.OverflowValue != 3 {
		t.Errorf("OverflowValue: got %d, want 3 (ellipsis)", obj.OverflowValue)
	}
	if obj.AlignValue != 0 {
		t.Errorf("AlignValue: got %d, want 0 (left, default)", obj.AlignValue)
	}
}
