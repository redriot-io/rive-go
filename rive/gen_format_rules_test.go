package rive

import "testing"

// TestDefaults_MatchContract verifies that Properties() suppresses fields that
// are at their Rive runtime default (RiveDefault map) and emits fields that
// differ from the default.
func TestDefaults_MatchContract(t *testing.T) {
	// key 18: opacity — Rive default 1.0, Go zero 0.0
	// Properties() should suppress when Opacity=1.0, emit when Opacity=0.0.
	wc := &WorldTransformComponent{}
	wc.Opacity = 1.0
	for _, p := range wc.Properties() {
		if p.Key == 18 {
			t.Error("key 18 (opacity=1.0 = RiveDefault) must be suppressed, but was emitted")
		}
	}
	wc.Opacity = 0.0
	foundOpacity := false
	for _, p := range wc.Properties() {
		if p.Key == 18 {
			foundOpacity = true
		}
	}
	if !foundOpacity {
		t.Error("key 18 (opacity=0.0 ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// key 16: scaleX — Rive default 1.0
	tc := &TransformComponent{}
	tc.ScaleX = 1.0
	for _, p := range tc.Properties() {
		if p.Key == 16 {
			t.Error("key 16 (scaleX=1.0 = RiveDefault) must be suppressed, but was emitted")
		}
	}
	tc.ScaleX = 2.0
	foundScaleX := false
	for _, p := range tc.Properties() {
		if p.Key == 16 {
			foundScaleX = true
		}
	}
	if !foundScaleX {
		t.Error("key 16 (scaleX=2.0 ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// key 23: blendModeValue — Rive default 3 (SrcOver)
	d := &Drawable{}
	d.BlendModeValue = 3
	for _, p := range d.Properties() {
		if p.Key == 23 {
			t.Error("key 23 (blendModeValue=3 = RiveDefault) must be suppressed, but was emitted")
		}
	}
	d.BlendModeValue = 0
	foundBMV := false
	for _, p := range d.Properties() {
		if p.Key == 23 {
			foundBMV = true
		}
	}
	if !foundBMV {
		t.Error("key 23 (blendModeValue=0 ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// key 41: isVisible — Rive default true
	sp := &ShapePaint{}
	sp.IsVisible = true
	for _, p := range sp.Properties() {
		if p.Key == 41 {
			t.Error("key 41 (isVisible=true = RiveDefault) must be suppressed, but was emitted")
		}
	}
	sp.IsVisible = false
	foundVis := false
	for _, p := range sp.Properties() {
		if p.Key == 41 {
			foundVis = true
		}
	}
	if !foundVis {
		t.Error("key 41 (isVisible=false ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// key 47: thickness — Rive default 1.0
	st := &Stroke{}
	st.Thickness = 1.0
	for _, p := range st.Properties() {
		if p.Key == 47 {
			t.Error("key 47 (thickness=1.0 = RiveDefault) must be suppressed, but was emitted")
		}
	}
	st.Thickness = 3.0
	foundThick := false
	for _, p := range st.Properties() {
		if p.Key == 47 {
			foundThick = true
		}
	}
	if !foundThick {
		t.Error("key 47 (thickness=3.0 ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// key 164: linkCornerRadius — Rive default true
	r := &Rectangle{}
	r.LinkCornerRadius = true
	for _, p := range r.Properties() {
		if p.Key == 164 {
			t.Error("key 164 (linkCornerRadius=true = RiveDefault) must be suppressed, but was emitted")
		}
	}
	r.LinkCornerRadius = false
	foundLCR := false
	for _, p := range r.Properties() {
		if p.Key == 164 {
			foundLCR = true
		}
	}
	if !foundLCR {
		t.Error("key 164 (linkCornerRadius=false ≠ RiveDefault) must be emitted, but was suppressed")
	}

	// Verify RiveDefault map entries are consistent with tested behaviour.
	if RiveDefault[18] != float64(1.0) {
		t.Errorf("RiveDefault[18] = %v, want 1.0 (opacity)", RiveDefault[18])
	}
	if RiveDefault[16] != float64(1.0) {
		t.Errorf("RiveDefault[16] = %v, want 1.0 (scaleX)", RiveDefault[16])
	}
	if RiveDefault[23] != uint64(3) {
		t.Errorf("RiveDefault[23] = %v, want uint64(3) (blendModeValue)", RiveDefault[23])
	}
	if RiveDefault[41] != true {
		t.Errorf("RiveDefault[41] = %v, want true (isVisible)", RiveDefault[41])
	}
	if RiveDefault[47] != float64(1.0) {
		t.Errorf("RiveDefault[47] = %v, want 1.0 (thickness)", RiveDefault[47])
	}
	if RiveDefault[164] != true {
		t.Errorf("RiveDefault[164] = %v, want true (linkCornerRadius)", RiveDefault[164])
	}
}

func TestGenFormatRules_Sanity(t *testing.T) {
	// Key 212 is the bytes-proxy ToC key (field_index must be 1, not 4)
	if got := ToCIncludeKeys[212]; got != 1 {
		t.Errorf("ToCIncludeKeys[212] = %d, want 1", got)
	}

	// Property key 18 (opacity) has a non-zero Rive default (1.0, not 0.0)
	v, ok := RiveDefault[18]
	if !ok {
		t.Fatal("RiveDefault[18] missing")
	}
	if v != float64(1.0) {
		t.Errorf("RiveDefault[18] = %v, want 1.0", v)
	}

	// FontAsset (typeKey 141) is a global asset — no parent id
	rule, ok := FormatParentIdRules[141]
	if !ok {
		t.Fatal("FormatParentIdRules[141] missing")
	}
	if rule.HasParentId {
		t.Error("FormatParentIdRules[141].HasParentId = true, want false")
	}

	// Backboard (typeKey 23) is in the global phase
	phase, ok := TypePhase[23]
	if !ok {
		t.Fatal("TypePhase[23] missing")
	}
	if phase.Phase != PhaseGlobal {
		t.Errorf("TypePhase[23].Phase = %v, want PhaseGlobal", phase.Phase)
	}
}
