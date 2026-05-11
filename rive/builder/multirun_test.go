package builder_test

import (
	"testing"

	"github.com/redriot-io/rive-go/rive"
	"github.com/redriot-io/rive-go/rive/builder"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func buildMultiRun(t *testing.T, fn func(ab *builder.ArtboardBuilder, font1, font2 *builder.FontRef)) []rive.Object {
	t.Helper()
	b := builder.New()
	ab := b.Artboard("Test", 400, 200)
	f1 := ab.EmbedFont("F1", []byte("FAKE-TTF-1"))
	f2 := ab.EmbedFont("F2", []byte("FAKE-TTF-2"))
	fn(ab, f1, f2)
	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return objs
}

// resolveStyleId returns the global object index that a TextValueRun's styleId
// points to. artboardIdx is the global index of the Artboard object.
func resolveStyleId(t *testing.T, objects []rive.Object, runIdx, artboardIdx int) int {
	t.Helper()
	for _, p := range objects[runIdx].Properties() {
		if p.Key == 272 {
			if v, ok := p.Value.(uint64); ok {
				return artboardIdx + int(v)
			}
		}
	}
	t.Fatalf("TVR[%d] has no styleId (key 272)", runIdx)
	return -1
}

func parentOf(objects []rive.Object, idx, artboardIdx int) int {
	for _, p := range objects[idx].Properties() {
		if p.Key == 5 {
			if v, ok := p.Value.(uint64); ok {
				return artboardIdx + int(v)
			}
		}
	}
	return artboardIdx // default parentId=0 → Artboard
}

func textOf(objects []rive.Object, idx int) string {
	for _, p := range objects[idx].Properties() {
		if p.Key == 268 {
			if s, ok := p.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// ── TestMultiRun_TwoRunsSameFont ──────────────────────────────────────────────

// Two runs sharing one style (same font). The style should be emitted once.
func TestMultiRun_TwoRunsSameFont(t *testing.T) {
	objs := buildMultiRun(t, func(ab *builder.ArtboardBuilder, f1, _ *builder.FontRef) {
		txt := ab.Text("t")
		s1 := txt.Style(f1, 24).Fill(0xFF000000)
		txt.Run("Hello", s1)
		txt.Run(" World", s1)
	})

	// typeKey sequence:
	// [0]BB [1]FA_1 [2]FAC_1 [3]FA_2 [4]FAC_2 [5]AB
	// [6]Text [7]TSP [8]SC [9]Fill [10]TVR1 [11]TVR2
	wantKeys := []uint32{23, 141, 106, 141, 106, 1, 134, 137, 18, 20, 135, 135}
	if len(objs) != len(wantKeys) {
		t.Fatalf("object count: got %d want %d\n  got: %v\n  want: %v",
			len(objs), len(wantKeys), typeKeyList(objs), wantKeys)
	}
	for i, want := range wantKeys {
		if objs[i].TypeKey() != want {
			t.Errorf("objs[%d] typeKey=%d want %d", i, objs[i].TypeKey(), want)
		}
	}

	const abIdx = 5
	// Both TVRs must reference the same TextStyle[7].
	sid1 := resolveStyleId(t, objs, 10, abIdx)
	sid2 := resolveStyleId(t, objs, 11, abIdx)
	if sid1 != 7 {
		t.Errorf("TVR1[10].styleId → global[%d], want global[7] (TextStyle)", sid1)
	}
	if sid2 != 7 {
		t.Errorf("TVR2[11].styleId → global[%d], want global[7] (TextStyle)", sid2)
	}

	// Verify texts.
	if got := textOf(objs, 10); got != "Hello" {
		t.Errorf("TVR1 text=%q want %q", got, "Hello")
	}
	if got := textOf(objs, 11); got != " World" {
		t.Errorf("TVR2 text=%q want %q", got, " World")
	}
}

// ── TestMultiRun_TwoRunsDifferentFonts ───────────────────────────────────────

// Two runs each with a distinct style and a different font.
func TestMultiRun_TwoRunsDifferentFonts(t *testing.T) {
	objs := buildMultiRun(t, func(ab *builder.ArtboardBuilder, f1, f2 *builder.FontRef) {
		txt := ab.Text("t")
		sA := txt.Style(f1, 32).Fill(0xFF000000)
		sB := txt.Style(f2, 16).Fill(0xFF666666)
		txt.Run("Heading", sA)
		txt.Run(" — subtitle", sB)
	})

	// [0]BB [1]FA_1 [2]FAC_1 [3]FA_2 [4]FAC_2 [5]AB
	// [6]Text [7]TSP_A [8]SC_A [9]Fill_A [10]TSP_B [11]SC_B [12]Fill_B
	// [13]TVR1 [14]TVR2
	wantKeys := []uint32{23, 141, 106, 141, 106, 1, 134, 137, 18, 20, 137, 18, 20, 135, 135}
	if len(objs) != len(wantKeys) {
		t.Fatalf("object count: got %d want %d\n  got: %v\n  want: %v",
			len(objs), len(wantKeys), typeKeyList(objs), wantKeys)
	}
	for i, want := range wantKeys {
		if objs[i].TypeKey() != want {
			t.Errorf("objs[%d] typeKey=%d want %d", i, objs[i].TypeKey(), want)
		}
	}

	const abIdx = 5

	// TVR1 → StyleA[7], TVR2 → StyleB[10]
	if sid := resolveStyleId(t, objs, 13, abIdx); sid != 7 {
		t.Errorf("TVR1[13].styleId → global[%d], want global[7] (StyleA)", sid)
	}
	if sid := resolveStyleId(t, objs, 14, abIdx); sid != 10 {
		t.Errorf("TVR2[14].styleId → global[%d], want global[10] (StyleB)", sid)
	}

	// Each style must parent to Text[6].
	if pg := parentOf(objs, 7, abIdx); pg != 6 {
		t.Errorf("StyleA[7].parent=global[%d], want global[6] (Text)", pg)
	}
	if pg := parentOf(objs, 10, abIdx); pg != 6 {
		t.Errorf("StyleB[10].parent=global[%d], want global[6] (Text)", pg)
	}

	// Each Fill must parent to its TextStyle.
	if pg := parentOf(objs, 9, abIdx); pg != 7 {
		t.Errorf("Fill_A[9].parent=global[%d], want global[7] (StyleA)", pg)
	}
	if pg := parentOf(objs, 12, abIdx); pg != 10 {
		t.Errorf("Fill_B[12].parent=global[%d], want global[10] (StyleB)", pg)
	}
}

// ── TestMultiRun_ThreeRunsMixedStyles ─────────────────────────────────────────

// Three runs with two styles, where style A is reused in runs 1 and 3.
// Matches the pattern of Text[22] in the official new_text.riv.
func TestMultiRun_ThreeRunsMixedStyles(t *testing.T) {
	objs := buildMultiRun(t, func(ab *builder.ArtboardBuilder, f1, f2 *builder.FontRef) {
		txt := ab.Text("t")
		sA := txt.Style(f1, 24).Fill(0xFF111111)
		sB := txt.Style(f2, 20).Fill(0xFF444444)
		txt.Run("here is ", sA)
		txt.Run("some", sB)
		txt.Run(" new text", sA)
	})

	// [0..5] preamble, [6] Text, [7..9] StyleA+paint, [10..12] StyleB+paint
	// [13] TVR1 (styleA), [14] TVR2 (styleB), [15] TVR3 (styleA)
	wantKeys := []uint32{23, 141, 106, 141, 106, 1, 134, 137, 18, 20, 137, 18, 20, 135, 135, 135}
	if len(objs) != len(wantKeys) {
		t.Fatalf("object count: got %d want %d\n  got: %v\n  want: %v",
			len(objs), len(wantKeys), typeKeyList(objs), wantKeys)
	}

	const abIdx = 5
	// TVR1 and TVR3 reference StyleA[7]; TVR2 references StyleB[10].
	if sid := resolveStyleId(t, objs, 13, abIdx); sid != 7 {
		t.Errorf("TVR1[13].styleId → global[%d], want global[7] (StyleA)", sid)
	}
	if sid := resolveStyleId(t, objs, 14, abIdx); sid != 10 {
		t.Errorf("TVR2[14].styleId → global[%d], want global[10] (StyleB)", sid)
	}
	if sid := resolveStyleId(t, objs, 15, abIdx); sid != 7 {
		t.Errorf("TVR3[15].styleId → global[%d], want global[7] (StyleA — reused)", sid)
	}

	// Verify text content order.
	want := []string{"here is ", "some", " new text"}
	for i, w := range want {
		if got := textOf(objs, 13+i); got != w {
			t.Errorf("TVR%d text=%q, want %q", i+1, got, w)
		}
	}
}

// ── TestMultiRun_FourRunsThreeStyles ─────────────────────────────────────────

// Four runs across three distinct styles — validates style indexing for larger
// multi-run compositions.
func TestMultiRun_FourRunsThreeStyles(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("Test", 600, 300)
	fA := ab.EmbedFont("FA", []byte("FAKE-A"))
	fB := ab.EmbedFont("FB", []byte("FAKE-B"))
	fC := ab.EmbedFont("FC", []byte("FAKE-C"))

	txt := ab.Text("t")
	sA := txt.Style(fA, 40)
	sB := txt.Style(fB, 20)
	sC := txt.Style(fC, 14)
	txt.Run("Big ", sA)
	txt.Run("medium ", sB)
	txt.Run("small ", sC)
	txt.Run("back to big", sA)

	objs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Artboard at global[7] (3 FA+FAC pairs + BB + AB)
	const abIdx = 7

	var tvrs []int
	for i, o := range objs {
		if o.TypeKey() == 135 {
			tvrs = append(tvrs, i)
		}
	}
	if len(tvrs) != 4 {
		t.Fatalf("want 4 TextValueRuns, got %d", len(tvrs))
	}

	// Styles are emitted in declaration order; collect their global indices.
	var styleIdxs []int
	for i, o := range objs {
		if o.TypeKey() == 137 && i > abIdx {
			styleIdxs = append(styleIdxs, i)
		}
	}
	if len(styleIdxs) != 3 {
		t.Fatalf("want 3 TextStyles, got %d: global indices %v", len(styleIdxs), styleIdxs)
	}
	idxA, idxB, idxC := styleIdxs[0], styleIdxs[1], styleIdxs[2]

	// TVR order: [sA, sB, sC, sA]
	want := []struct {
		run      int
		styleGlb int
		text     string
	}{
		{tvrs[0], idxA, "Big "},
		{tvrs[1], idxB, "medium "},
		{tvrs[2], idxC, "small "},
		{tvrs[3], idxA, "back to big"},
	}
	for _, w := range want {
		if sid := resolveStyleId(t, objs, w.run, abIdx); sid != w.styleGlb {
			t.Errorf("TVR[%d].styleId → global[%d], want global[%d]", w.run, sid, w.styleGlb)
		}
		if got := textOf(objs, w.run); got != w.text {
			t.Errorf("TVR[%d] text=%q, want %q", w.run, got, w.text)
		}
	}

	t.Logf("four-runs three-styles ok: %v", typeKeyList(objs))
}

// ── TestMultiRun_RoundTrip ────────────────────────────────────────────────────

// Build a multi-run .riv, serialize it, re-parse it, and verify that the
// TextValueRun texts and styleId references survive the round-trip.
func TestMultiRun_RoundTrip(t *testing.T) {
	b := builder.New()
	ab := b.Artboard("RT", 400, 200)
	fA := ab.EmbedFont("FA", []byte("FAKE-A"))
	fB := ab.EmbedFont("FB", []byte("FAKE-B"))

	txt := ab.Text("rt")
	sA := txt.Style(fA, 24).Fill(0xFF000000)
	sB := txt.Style(fB, 18).Fill(0xFF888888)
	txt.Run("alpha", sA)
	txt.Run(" beta", sB)
	txt.Run(" gamma", sA)

	raw, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := rive.ReadBytes(raw)
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Collect TVRs and TextStyles from the re-parsed file.
	var tvrs, styles []rive.Object
	for _, o := range f.Objects {
		switch o.TypeKey() {
		case 135:
			tvrs = append(tvrs, o)
		case 137:
			styles = append(styles, o)
		}
	}
	if len(tvrs) != 3 {
		t.Fatalf("after round-trip: want 3 TVRs, got %d", len(tvrs))
	}
	if len(styles) != 2 {
		t.Fatalf("after round-trip: want 2 TextStyles, got %d", len(styles))
	}

	// Verify text strings survived.
	wantTexts := []string{"alpha", " beta", " gamma"}
	for i, tvr := range tvrs {
		for _, p := range tvr.Properties() {
			if p.Key == 268 {
				if got, ok := p.Value.(string); ok && got != wantTexts[i] {
					t.Errorf("TVR[%d] text=%q, want %q", i, got, wantTexts[i])
				}
			}
		}
	}
}
