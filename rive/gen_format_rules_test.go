package rive

import "testing"

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
